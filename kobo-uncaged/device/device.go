package device

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"image"
	"image/jpeg"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bamiaux/rez"
	"github.com/doug-martin/goqu/v9"
	"github.com/godbus/dbus/v5"

	// Lets gpqu emit SQLite3 compatible code
	_ "github.com/doug-martin/goqu/v9/dialect/sqlite3"
	"github.com/google/uuid"
	"github.com/kapmahc/epub"
	"github.com/pgaskin/koboutils/v2/kobo"
	"github.com/shermp/Kobo-UNCaGED/kobo-uncaged/util"
	"github.com/shermp/UNCaGED/calibre"
	"github.com/shermp/UNCaGED/uc"
)

const koboDBpath = ".kobo/KoboReader.sqlite"
const koboVersPath = ".kobo/version"
const calibreMDfile = "metadata.calibre"
const calibreDIfile = "driveinfo.calibre"
const kuUpdatedMDfile = "metadata_update.kobouc"
const kuUpdatedSQL = ".adds/kobo-uncaged/updated-md.sql"
const kuBookReplaceSQL = ".adds/kobo-uncaged/replace-book.sql"
const kuPassCache = ".adds/kobo-uncaged/.ku_pwcache.json"
const kuConfigFile = ".adds/kobo-uncaged/config/kuconfig.json"
const ndbInterface = "com.github.shermp.nickeldbus"
const viewChangedName = ndbInterface + ".ndbViewChanged"

const onboardPrefix cidPrefix = "file:///mnt/onboard/"
const sdPrefix cidPrefix = "file:///mnt/sd/"

func isBrowserViewSignal(vs *dbus.Signal) (bool, error) {
	if vs.Name != viewChangedName || len(vs.Body) <= 0 {
		return false, fmt.Errorf("isBrowserViewSignal: not valid 'ndbViewChanged' signal")
	}
	return vs.Body[0].(string) == "N3BrowserView", nil
}

// New creates a Kobo object, ready for use
func New(dbRootDir, sdRootDir string, bindAddress string, disableNDB bool, vers string) (*Kobo, error) {
	var err error
	k := &Kobo{}
	k.Wg = &sync.WaitGroup{}
	k.DBRootDir = dbRootDir
	k.BKRootDir = dbRootDir
	k.ContentIDprefix = onboardPrefix
	if err = k.getUserOptions(); err != nil {
		return nil, fmt.Errorf("New: failed to read config file: %w", err)
	}
	if len(k.KuConfig.DirectConn) == 0 {
		k.KuConfig.DirectConnIndex = -1
		k.KuConfig.DirectConn = make([]calibre.ConnectionInfo, 0)
	}
	if sdRootDir != "" && k.KuConfig.PreferSDCard {
		k.UseSDCard = true
		k.BKRootDir = sdRootDir
		k.ContentIDprefix = sdPrefix
	}
	//k.Passwords = newUncagedPassword(k.KuConfig.PasswordList)
	k.UpdatedMetadata = make(map[string]struct{}, 0)
	k.SeriesIDMap = make(map[string]string, 0)
	k.PassCache = make(calPassCache)
	log.Println("Getting Kobo Info")
	if err = k.getKoboInfo(); err != nil {
		return nil, fmt.Errorf("New: failed to get kobo info: %w", err)
	}
	k.KuVers = vers
	k.webInfo = &webUIinfo{ScreenDPI: k.Device.DisplayPPI(), KUVersion: k.KuVers, StorageType: "Internal Storage"}
	if k.UseSDCard {
		k.webInfo.StorageType = "External SD Storage"
	}
	k.BrowserOpen = true
	k.useNDB = !disableNDB
	if k.useNDB {
		if k.ndbConn, err = dbus.SystemBus(); err != nil {
			return nil, fmt.Errorf("New: failed to connect to system d-bus: %w", err)
		}
		k.ndbObj = k.ndbConn.Object(ndbInterface, "/nickeldbus")
	}
	k.doneChan = make(chan bool)
	k.MsgChan = make(chan WebMsg)
	k.startChan = make(chan webConfig)
	k.AuthChan = make(chan *calPassword)
	k.calInstChan = make(chan uc.CalInstance)
	k.exitChan = make(chan bool)
	k.initWeb()
	go func() {
		if err = http.ListenAndServe(bindAddress, k.mux); err != nil {
			log.Println(err)
		}
	}()
	if k.useNDB {
		k.viewSignal = make(chan *dbus.Signal, 10)
		if err := k.ndbConn.AddMatchSignal(dbus.WithMatchObjectPath("/nickeldbus"),
			dbus.WithMatchInterface(ndbInterface),
			dbus.WithMatchMember("ndbViewChanged")); err != nil {
			return nil, fmt.Errorf("New: error adding ndbViewChanged match signal: %w", err)
		}
		k.ndbConn.Signal(k.viewSignal)
		var currView string
		// Note, the main reason for calling 'ndbCurrentView' here is to ensure the
		// 'ndbViewChanged' signal is connected
		if err = k.ndbObj.Call(ndbInterface+".ndbCurrentView", 0).Store(&currView); err != nil {
			return nil, fmt.Errorf("New: failed to get current view: %w", err)
		}
		if strings.HasSuffix(currView, "PowerView") {
			return nil, fmt.Errorf("New: currently in sleep mode. Aborting")
		}
		res := k.ndbObj.Call(ndbInterface+".bwmOpenBrowser", 0, true, "http://127.0.0.1:8181/")
		if res.Err != nil {
			return nil, fmt.Errorf("New: failed to open web browser")
		}
		select {
		case vs := <-k.viewSignal:
			valid, err := isBrowserViewSignal(vs)
			if err != nil {
				k.BrowserOpen = false
				return nil, fmt.Errorf("New: %w", err)
			} else if !valid {
				k.BrowserOpen = false
				return nil, fmt.Errorf("New: expected 'N3BrowserView', got '%s'", vs.Body[0].(string))
			}
		// Give the user some time to connect to Wifi if required
		case <-time.After(60 * time.Second):
			k.BrowserOpen = false
			k.ndbObj.Call(ndbInterface+".mwcToast", 0, 3000, "Kobo UNCaGED: Browser did not open after timeout")
			return nil, fmt.Errorf("New: timeout waiting for browser to open")
		}
		// Exit if we encounter a view changed signal from Nickel away from 'N3BrowserView'
		go func() {
			for v := range k.viewSignal {
				if isBV, err := isBrowserViewSignal(v); err == nil && !isBV {
					k.BrowserOpen = false
					k.ndbObj.Call(ndbInterface+".mwcToast", 0, 3000, "Browser closed. Kobo UNCaGED exiting")
					if k.UCExitChan != nil {
						k.UCExitChan <- true
					} else {
						k.exitChan <- true
					}
					return
				}
			}
		}()
	}
	select {
	case opt := <-k.startChan:
		if opt.err != nil {
			return nil, fmt.Errorf("New: failed to get start config: %w", err)
		}
		k.KuConfig = &opt.Opts
		k.KuConfig.Thumbnail.SetRezFilter()
		if err = k.SaveUserOptions(); err != nil {
			return nil, fmt.Errorf("New: failed to save updated config options to file: %w", err)
		}
	case <-k.exitChan:
		// Give the client time to request and render the final exit page before quitting
		time.Sleep(500 * time.Millisecond)
		return nil, nil
	}
	k.WebSend(WebMsg{ShowMessage: "Gathering information about your Kobo", Progress: -1})
	log.Println("Getting Device Info")
	if err = k.loadDeviceInfo(); err != nil {
		return nil, fmt.Errorf("New: failed to load device info: %w", err)
	}
	log.Println("Reading Metadata")
	if err = k.readMDfile(); err != nil {
		return nil, fmt.Errorf("New: failed to read metadata file: %w", err)
	}
	log.Println("Reading password cache")
	// Failing to retrieve the password cache isn't fatal. The user will be asked
	// for their password if required.
	if err = k.readPassCache(); err != nil {
		log.Print(err)
	}
	select {
	case <-k.exitChan:
		return nil, fmt.Errorf("New: browser exited prematurely")
	default:
		return k, nil
	}
}

func (k *Kobo) readPassCache() error {
	if _, err := util.ReadJSON(filepath.Join(k.DBRootDir, kuPassCache), &k.PassCache); err != nil {
		return fmt.Errorf("readPassCache: failed to read password cache: %w", err)
	}
	for calUUID := range k.PassCache {
		k.PassCache[calUUID].Attempts = 0
	}
	return nil
}

// WritePassCache writes the password cache to a file
func (k *Kobo) WritePassCache() error {
	// Delete any blank passwords in the cache before saving
	for calUUID := range k.PassCache {
		if k.PassCache[calUUID].Password == "" {
			delete(k.PassCache, calUUID)
		}
	}
	if err := util.WriteJSON(filepath.Join(k.DBRootDir, kuPassCache), k.PassCache); err != nil {
		return fmt.Errorf("readPassCache: failed to write password cache: %w", err)
	}
	return nil
}

// GetPassword provides a method of either using a cached password, or prompting
// the user for a new password
func (k *Kobo) GetPassword(calUUID, calLibName string) string {
	if _, exists := k.PassCache[calUUID]; !exists {
		k.PassCache[calUUID] = &calPassword{LibName: calLibName}
	}
	k.PassCache[calUUID].Attempts++
	if k.PassCache[calUUID].Attempts > 1 || k.PassCache[calUUID].Password == "" {
		k.WebSend(WebMsg{GetPassword: true})
		k.AuthChan <- k.PassCache[calUUID]
		k.PassCache[calUUID] = <-k.AuthChan
	}
	return k.PassCache[calUUID].Password
}

// GetCalibreInstance instructs the user to select from a list of available
// Calibre instances on their network
func (k *Kobo) GetCalibreInstance(calInstances []uc.CalInstance) uc.CalInstance {
	if len(calInstances) == 1 {
		return calInstances[0]
	}
	k.calInstances = calInstances
	k.WebSend(WebMsg{GetCalInstance: true})
	return <-k.calInstChan
}

func (k *Kobo) getUserOptions() error {
	// Note, we return opts, regardless of whether we successfully read the options file.
	// Our code can handle the default struct gracefully
	opts := &KuOptions{}
	notExists, err := util.ReadJSON(path.Join(k.DBRootDir, kuConfigFile), opts)
	if err != nil {
		return err
	} else if notExists {
		opts.PreferKepub = true
		// Note that opts.Thumbnail.Validate() sets thumbnail defaults, so no need
		// to set them here.
	}
	opts.Thumbnail.Validate()
	opts.Thumbnail.SetRezFilter()
	k.KuConfig = opts
	return nil
}

func (k *Kobo) SaveUserOptions() error {
	return util.WriteJSON(path.Join(k.DBRootDir, kuConfigFile), k.KuConfig)
}

// UpdateIfExists updates onboard metadata if it exists in the Nickel database
func (k *Kobo) UpdateIfExists(cID string, len int) error {
	var err error
	if _, exists := k.MetadataMap[cID]; exists {
		if k.MetadataMap[cID].Size == len {
			return nil
		}
		if k.replSQLWriter == nil {
			if k.replSQLWriter, err = newSQLWriter(filepath.Join(k.BKRootDir, kuBookReplaceSQL)); err != nil {
				return err
			}
		}
		dialect := goqu.Dialect("sqlite3")
		ds := dialect.Update("content").Set(goqu.Record{"___FileSize": len}).Where(goqu.Ex{"ContentID": cID, "ContentType": 6})
		sqlStr, _, _ := ds.ToSQL()
		k.replSQLWriter.writeQuery(sqlStr)
	}
	return nil
}

func (k *Kobo) getKoboInfo() error {
	_, vers, id, err := kobo.ParseKoboVersion(k.DBRootDir)
	if err != nil {
		return fmt.Errorf("New: %w", err)
	}
	if dev, ok := kobo.DeviceByID(id); ok {
		k.Device = dev
	} else {
		return fmt.Errorf("New: unknown device")
	}
	k.fw = firmwareVersion(vers)
	return nil
}

// GetDeviceOptions gets some device options that UNCaGED requires
func (k *Kobo) GetDeviceOptions() (ext []string, model string, thumbSz image.Point) {
	if k.KuConfig.PreferKepub {
		ext = []string{"kepub", "epub", "mobi", "pdf", "cbz", "cbr", "txt", "html", "rtf"}
	} else {
		ext = []string{"epub", "kepub", "mobi", "pdf", "cbz", "cbr", "txt", "html", "rtf"}
	}
	model = k.Device.Family()
	switch k.KuConfig.Thumbnail.GenerateLevel {
	case generateAll:
		thumbSz = k.Device.CoverSize(kobo.CoverTypeFull)
	case generatePartial:
		thumbSz = k.Device.CoverSize(kobo.CoverTypeLibFull)
	default:
		thumbSz = k.Device.CoverSize(kobo.CoverTypeLibGrid)
	}
	return ext, model, thumbSz
}

// GetDirectConnection gets a direct connection if set
func (k *Kobo) GetDirectConnection() *uc.CalInstance {
	index := k.KuConfig.DirectConnIndex
	if index >= 0 && index < len(k.KuConfig.DirectConn) {
		return &k.KuConfig.DirectConn[index]
	}
	return nil
}

// readEpubMeta opens an epub (or kepub), and attempts to read the
// metadata it contains. This is used if the metadata has not yet
// been cached
func (k *Kobo) readEpubMeta(contentID string, md *uc.CalibreBookMeta) error {
	lpath := util.ContentIDtoLpath(contentID, string(k.ContentIDprefix))
	epubPath := util.ContentIDtoBkPath(k.BKRootDir, contentID, string(k.ContentIDprefix))
	bk, err := epub.Open(epubPath)
	if err != nil {
		return fmt.Errorf("readEpubMeta: error opening epub for metadata reading: %w", err)
	}
	defer bk.Close()
	md.Lpath = lpath
	// Try to get the book identifiers. Note, we prefer the Calibre
	// generated UUID, if available.
	for _, ident := range bk.Opf.Metadata.Identifier {
		switch strings.ToLower(ident.Scheme) {
		case "calibre":
			md.UUID = ident.Data
		case "uuid":
			if md.UUID == "" {
				md.UUID = ident.Data
			}
		default:
			md.Identifiers[ident.Scheme] = ident.Data
		}
	}
	if len(bk.Opf.Metadata.Title) > 0 {
		md.Title = bk.Opf.Metadata.Title[0]
	}
	if len(bk.Opf.Metadata.Description) > 0 {
		desc := html.UnescapeString(bk.Opf.Metadata.Description[0])
		md.Comments = &desc
	}
	if len(bk.Opf.Metadata.Language) > 0 {
		md.Languages = append(md.Languages, bk.Opf.Metadata.Language...)
	}
	for _, author := range bk.Opf.Metadata.Creator {
		if author.Role == "aut" {
			md.Authors = append(md.Authors, author.Data)
		}
	}
	if len(bk.Opf.Metadata.Publisher) > 0 {
		pub := bk.Opf.Metadata.Publisher[0]
		md.Publisher = &pub
	}
	if len(bk.Opf.Metadata.Date) > 0 {
		md.Pubdate = uc.ParseTime(bk.Opf.Metadata.Date[0].Data)
	}
	for _, m := range bk.Opf.Metadata.Meta {
		switch m.Name {
		case "calibre:timestamp":
			md.Timestamp = uc.ParseTime(m.Content)
		case "calibre:series":
			series := m.Content
			md.Series = &series
		case "calibre:series_index":
			seriesIndex, _ := strconv.ParseFloat(m.Content, 64)
			md.SeriesIndex = &seriesIndex
		case "calibre:title_sort":
			md.TitleSort = m.Content
		case "calibre:author_link_map":
			var alm map[string]string
			_ = json.Unmarshal([]byte(html.UnescapeString(m.Content)), &alm)
		}

	}
	return nil
}

// readMDfile loads cached metadata from the "metadata.calibre" JSON file
// and unmarshals (eventially) to a map of KoboMetadata structs, converting
// "lpath" to Kobo's "ContentID", and using that as the map keys
func (k *Kobo) readMDfile() error {
	log.Println("Reading metadata.calibre")

	var koboMD []uc.CalibreBookMeta
	emptyOrNotExist, err := util.ReadJSON(filepath.Join(k.BKRootDir, calibreMDfile), &koboMD)
	if emptyOrNotExist {
		// ignore
	} else if err != nil {
		return fmt.Errorf("readMDfile: error reading metadata.calibre JSON: %w", err)
	}

	// Make the metadatamap here instead of the constructer so we can pre-allocate
	// the memory with the right size.
	k.MetadataMap = make(map[string]uc.CalibreBookMeta, len(koboMD))
	// make a temporary map for easy searching later
	tmpMap := make(map[string]int, len(koboMD))
	for n, md := range koboMD {
		contentID := util.LpathToContentID(util.LpathKepubConvert(md.Lpath), string(k.ContentIDprefix))
		tmpMap[contentID] = n
	}
	log.Println("Gathering metadata")
	var nickelDB *sql.DB
	dsn := "file:" + filepath.Join(k.DBRootDir, koboDBpath) + "?_timeout=2000&_journal=WAL&mode=ro&_mutex=full&_sync=NORMAL"
	if nickelDB, err = sql.Open("sqlite3", dsn); err != nil {
		return fmt.Errorf("openNickelDB: sql open failed: %w", err)
	}
	defer nickelDB.Close()
	//spew.Dump(k.MetadataMap)
	// Now that we have our map, we need to check for any books in the DB not in our
	// metadata cache, or books that are in our cache but not in the DB
	var (
		dbCID        string
		dbTitle      *string
		dbAttr       *string
		dbDesc       *string
		dbPublisher  *string
		dbSeries     *string
		dbbSeriesNum *string
		dbMimeType   string
		dbFileSize   int
	)
	query := `
		SELECT ContentID, Title, Attribution, Description, Publisher, Series, SeriesNumber, MimeType, ___FileSize 
		FROM content
		WHERE ContentType=6
		AND MimeType NOT LIKE 'image%%'
		AND (IsDownloaded='true' OR IsDownloaded=1)
		AND ___FileSize>0
		AND Accessibility=-1
		AND ContentID LIKE ?;`

	bkRows, err := nickelDB.Query(query, fmt.Sprintf("%s%%", k.ContentIDprefix))
	if err != nil {
		return fmt.Errorf("readMDfile: error getting book rows: %w", err)
	}
	defer bkRows.Close()
	for bkRows.Next() {
		err = bkRows.Scan(&dbCID, &dbTitle, &dbAttr, &dbDesc, &dbPublisher, &dbSeries, &dbbSeriesNum, &dbMimeType, &dbFileSize)
		if err != nil {
			return fmt.Errorf("readMDfile: row decoding error: %w", err)
		}
		if _, exists := tmpMap[dbCID]; !exists {
			log.Printf("Book not in cache: %s\n", dbCID)
			bkMD := uc.CalibreBookMeta{}
			bkMD.Lpath = util.ContentIDtoLpath(dbCID, string(onboardPrefix))
			uuidV4, _ := uuid.NewRandom()
			bkMD.UUID = uuidV4.String()
			bkMD.Comments, bkMD.Publisher, bkMD.Series = dbDesc, dbPublisher, dbSeries
			if dbTitle != nil {
				bkMD.Title = *dbTitle
			}
			if dbbSeriesNum != nil {
				index, err := strconv.ParseFloat(*dbbSeriesNum, 64)
				if err == nil {
					bkMD.SeriesIndex = &index
				}
			}
			if dbAttr != nil {
				bkMD.Authors = strings.Split(*dbAttr, ",")
				for i := range bkMD.Authors {
					bkMD.Authors[i] = strings.TrimSpace(bkMD.Authors[i])
				}
			}
			// if dbMimeType == "application/epub+zip" || dbMimeType == "application/x-kobo-epub+zip" {
			// 	err = k.readEpubMeta(dbCID, &bkMD)
			// 	if err != nil {
			// 		log.Print(err)
			// 	}
			// }
			bkMD.Size = dbFileSize
			fi, err := os.Stat(filepath.Join(k.BKRootDir, bkMD.Lpath))
			if err == nil {
				lastMod := uc.ConvertTime(fi.ModTime())
				bkMD.LastModified = &lastMod
			}
			//spew.Dump(bkMD)
			k.MetadataMap[dbCID] = bkMD
		} else {
			// Make sure we are using the filesize as exists in the DB
			koboMD[tmpMap[dbCID]].Size = dbFileSize
			k.MetadataMap[dbCID] = koboMD[tmpMap[dbCID]]
		}
	}
	if err = bkRows.Err(); err != nil {
		return fmt.Errorf("readMDfile: bkRows error: %w", err)
	}
	// Finally, store a snapshot of books in database before we make any additions/deletions
	k.BooksInDB = make(map[string]struct{}, len(k.MetadataMap))
	for cid := range k.MetadataMap {
		k.BooksInDB[cid] = struct{}{}
	}
	// Hopefully, our metadata is now up to date. Update the cache on disk
	if err = k.WriteMDfile(); err != nil {
		return fmt.Errorf("readMDfile: error writing metadata to disk: %w", err)
	}
	return nil
}

// WriteMDfile writes metadata to file
func (k *Kobo) WriteMDfile() error {
	var n int
	var err error
	metadata := make([]uc.CalibreBookMeta, len(k.MetadataMap))
	for _, md := range k.MetadataMap {
		metadata[n] = md
		n++
	}
	if err = util.WriteJSON(filepath.Join(k.BKRootDir, calibreMDfile), metadata); err != nil {
		err = fmt.Errorf("WriteMDfile: %w", err)
	}
	return err
}

func (k *Kobo) loadDeviceInfo() error {
	emptyOrNotExist, err := util.ReadJSON(filepath.Join(k.BKRootDir, calibreDIfile), &k.DriveInfo.DevInfo)
	if emptyOrNotExist {
		uuid4, _ := uuid.NewRandom()
		k.DriveInfo.DevInfo.LocationCode = "main"
		k.DriveInfo.DevInfo.DeviceName = k.Device.Family()
		k.DriveInfo.DevInfo.DeviceStoreUUID = uuid4.String()
		if k.UseSDCard {
			k.DriveInfo.DevInfo.LocationCode = "A"
		}
	} else if err != nil {
		return fmt.Errorf("loadDeviceInfo: error reading device info JSON: %w", err)
	}
	return nil
}

// SaveDeviceInfo save device info to file
func (k *Kobo) SaveDeviceInfo() error {
	if err := util.WriteJSON(filepath.Join(k.BKRootDir, calibreDIfile), k.DriveInfo.DevInfo); err != nil {
		return fmt.Errorf("SaveDeviceInfo: error saving device info JSON: %w", err)
	}
	return nil
}

// SaveCoverImage generates cover image and thumbnails, and save to appropriate locations
func (k *Kobo) SaveCoverImage(contentID string, size image.Point, imgB64 string) {
	defer k.Wg.Done()

	img, _, err := image.Decode(base64.NewDecoder(base64.StdEncoding, strings.NewReader(imgB64)))
	if err != nil {
		log.Println(err)
		return
	}
	sz := img.Bounds().Size()

	imgID := kobo.ContentIDToImageID(contentID)
	//fmt.Printf("Image ID is: %s\n", imgID)
	jpegOpts := jpeg.Options{Quality: k.KuConfig.Thumbnail.JpegQuality}

	var coverEndings []kobo.CoverType
	switch k.KuConfig.Thumbnail.GenerateLevel {
	case generateAll:
		coverEndings = []kobo.CoverType{kobo.CoverTypeFull, kobo.CoverTypeLibFull, kobo.CoverTypeLibGrid}
	case generatePartial:
		coverEndings = []kobo.CoverType{kobo.CoverTypeLibFull, kobo.CoverTypeLibGrid}
	}
	for _, cover := range coverEndings {
		nsz := k.Device.CoverSized(cover, sz)
		nfn := filepath.Join(k.BKRootDir, cover.GeneratePath(k.UseSDCard, imgID))
		//fmt.Printf("Cover file path is: %s\n", nfn)
		log.Printf("Resizing %s cover to %s (target %s) for %s\n", sz, nsz, k.Device.CoverSize(cover), cover)

		var nimg image.Image
		if !sz.Eq(nsz) {
			nimg = image.NewYCbCr(image.Rect(0, 0, nsz.X, nsz.Y), img.(*image.YCbCr).SubsampleRatio)
			rez.Convert(nimg, img, k.KuConfig.Thumbnail.rezFilter)
			log.Printf(" -- Resized to %s\n", nimg.Bounds().Size())
		} else {
			nimg = img
			log.Println(" -- Skipped resize: already correct size")
		}
		// Optimization. No need to resize libGrid from the full cover size...
		if cover == kobo.CoverTypeLibFull {
			img = nimg
		}

		if err := os.MkdirAll(filepath.Dir(nfn), 0755); err != nil {
			log.Println(err)
			continue
		}

		lf, err := os.OpenFile(nfn, os.O_WRONLY|os.O_CREATE, 0644)
		if err != nil {
			log.Println(err)
			continue
		}

		if err := jpeg.Encode(lf, nimg, &jpegOpts); err != nil {
			log.Println(err)
			lf.Close()
		}
		lf.Close()
	}
}

// WriteUpdatedMetadataSQL writes SQL to write updated metadata to
// the Kobo database. The SQLite CLI client will be used to perform the import.
func (k *Kobo) WriteUpdatedMetadataSQL() error {
	var err error
	if len(k.UpdatedMetadata) == 0 {
		return nil
	}
	updateSQL, err := newSQLWriter(filepath.Join(k.BKRootDir, kuUpdatedSQL))
	if err != nil {
		return fmt.Errorf("WriteUpdatedMetadataSQL: failed to create SQL writer: %w", err)
	}
	defer updateSQL.close()
	dialect := goqu.Dialect("sqlite3")
	var desc, series, seriesNum, subtitle *string
	var seriesNumFloat *float64
	for cid := range k.UpdatedMetadata {
		desc, series, seriesNum, seriesNumFloat, subtitle = nil, nil, nil, nil, nil
		if k.MetadataMap[cid].Comments != nil && *k.MetadataMap[cid].Comments != "" {
			desc = k.MetadataMap[cid].Comments
		}
		if k.MetadataMap[cid].Series != nil && *k.MetadataMap[cid].Series != "" {
			// TODO: Fuzzy series matching to deal with 'The' prefixes and 'Series' postfixes?
			series = k.MetadataMap[cid].Series
		}
		if k.MetadataMap[cid].SeriesIndex != nil && *k.MetadataMap[cid].SeriesIndex != 0.0 {
			sn := strconv.FormatFloat(*k.MetadataMap[cid].SeriesIndex, 'f', -1, 64)
			seriesNum = &sn
			seriesNumFloat = k.MetadataMap[cid].SeriesIndex
		}
		if field, exists := k.KuConfig.LibOptions[k.LibInfo.LibraryUUID]; exists && field.SubtitleColumn != "" {
			col := field.SubtitleColumn
			md := k.MetadataMap[cid]
			st := ""
			if col == "languages" {
				st = md.LangString()
			} else if col == "tags" {
				st = md.TagString()
			} else if col == "publisher" {
				st = md.PubString()
			} else if col == "rating" {
				st = md.RatingString()
			} else if strings.HasPrefix(col, "#") {
				if cc, exists := md.UserMetadata[col]; exists {
					st = cc.ContextualString()
				}
			}
			if st != "" {
				subtitle = &st
			}
		}
		ds := dialect.Update("content").Set(goqu.Record{
			"Description": desc, "Series": series, "SeriesNumber": seriesNum, "SeriesNumberFloat": seriesNumFloat, "Subtitle": subtitle,
		}).Where(goqu.Ex{"ContentID": cid})
		sqlStr, _, err := ds.ToSQL()
		if err != nil {
			return fmt.Errorf("WriteUpdatedMetadataSQL: failed ")
		}
		updateSQL.writeQuery(sqlStr)
	}
	// Note, the SeriesID stuff was implemented in FW 4.20.14601
	if kobo.VersionCompare(string(k.fw), "4.20.14601") >= 0 {
		// Set the SeriesID column correctly
		// Note, UPDATE FROM is brand spanking new in SQLite 3.33.0 (2020-08-14). We're going to need the latest
		// client for this one
		updateSQL.writeQuery(
			`UPDATE content SET SeriesID = c.SeriesID 
FROM (
	SELECT DISTINCT Series, SeriesID FROM content 
	WHERE ContentType = 6 AND ContentID NOT LIKE 'file://%' AND (Series IS NOT NULL AND Series <> '') AND (SeriesID IS NOT NULL AND SeriesID <> '')
) AS c 
WHERE content.Series = c.Series;`)
		updateSQL.writeQuery(`UPDATE content SET SeriesID=Series WHERE ContentType = 6 AND (Series IS NOT NULL OR Series <> '') AND (SeriesID IS NULL OR SeriesID <> '');`)
	}
	return nil
}

// Close the kobo object when we're finished with it
func (k *Kobo) Close() {
	k.Wg.Wait()
	if k.replSQLWriter != nil {
		k.replSQLWriter.close()
	}
	if k.useNDB && !k.BrowserOpen {
		k.ndbObj.Call(ndbInterface+".mwcToast", 0, 3000, k.FinishedMsg)
	} else {
		k.WebSend(WebMsg{Finished: k.FinishedMsg})
	}
	if k.ndbConn != nil {
		k.ndbConn.Close()
	}
}
