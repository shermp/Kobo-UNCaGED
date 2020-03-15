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
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/bamiaux/rez"
	"github.com/geek1011/koboutils/v2/kobo"
	"github.com/google/uuid"
	"github.com/kapmahc/epub"
	"github.com/shermp/Kobo-UNCaGED/kobo-uncaged/kuprint"
	"github.com/shermp/Kobo-UNCaGED/kobo-uncaged/util"
	"github.com/shermp/UNCaGED/uc"
)

const koboDBpath = ".kobo/KoboReader.sqlite"
const koboVersPath = ".kobo/version"
const calibreMDfile = "metadata.calibre"
const calibreDIfile = "driveinfo.calibre"
const kuUpdatedMDfile = "metadata_update.kobouc"

const onboardPrefix cidPrefix = "file:///mnt/onboard/"
const sdPrefix cidPrefix = "file:///mnt/sd/"

func newUncagedPassword(passwordList []string) *uncagedPassword {
	return &uncagedPassword{passwordList: passwordList}
}

func (pw *uncagedPassword) NextPassword() string {
	var password string
	if pw.currPassIndex < len(pw.passwordList) {
		password = pw.passwordList[pw.currPassIndex]
		pw.currPassIndex++
	}
	return password
}

// New creates a Kobo object, ready for use
func New(dbRootDir, sdRootDir string, updatingMD bool, opts *KuOptions, vers string) (*Kobo, error) {
	var err error
	k := &Kobo{}
	k.Wg = &sync.WaitGroup{}
	k.DBRootDir = dbRootDir
	k.BKRootDir = dbRootDir
	k.ContentIDprefix = onboardPrefix

	k.KuConfig = opts
	if sdRootDir != "" && k.KuConfig.PreferSDCard {
		k.useSDCard = true
		k.BKRootDir = sdRootDir
		k.ContentIDprefix = sdPrefix
	}
	k.Passwords = newUncagedPassword(k.KuConfig.PasswordList)
	k.UpdatedMetadata = make(map[string]struct{}, 0)
	headerStr := "Kobo-UNCaGED " + vers
	if k.useSDCard {
		headerStr += "\nUsing SD Card"
	} else {
		headerStr += "\nUsing Internal Storage"
	}

	kuprint.Println(kuprint.Header, headerStr)
	kuprint.Println(kuprint.Body, "Gathering information about your Kobo")
	log.Println("Opening NickelDB")
	if err = k.openNickelDB(); err != nil {
		return nil, fmt.Errorf("New: failed to open Nickel DB: %w", err)
	}
	log.Println("Getting Kobo Info")
	if err = k.getKoboInfo(); err != nil {
		return nil, fmt.Errorf("New: failed to get kobo info: %w", err)
	}
	log.Println("Getting Device Info")
	if err = k.loadDeviceInfo(); err != nil {
		return nil, fmt.Errorf("New: failed to load device info: %w", err)
	}
	log.Println("Reading Metadata")
	if err = k.readMDfile(); err != nil {
		return nil, fmt.Errorf("New: failed to read metadata file: %w", err)
	}

	if k.KuConfig.AddMetadataByTrigger {
		if err = k.setupMetaTrigger(); err != nil {
			return nil, fmt.Errorf("New: failed to setup metadata trigger: %w", err)
		}
	} else {
		// clean up after ourselves by not leaving an unwanted table and trigger lingering
		// in the Nickel DB
		if err = k.removeMetaTrigger(); err != nil {
			return nil, fmt.Errorf("New: failed to remove metadata trigger: %w", err)
		}
	}
	if !updatingMD {
		return k, nil
	}
	if err = k.readUpdateMDfile(); err != nil {
		return nil, fmt.Errorf("New: failed to read updated metadata file: %w", err)
	}
	os.Remove(filepath.Join(k.BKRootDir, kuUpdatedMDfile))

	return k, err
}

func (k *Kobo) openNickelDB() error {
	var err error
	dsn := "file:" + filepath.Join(k.DBRootDir, koboDBpath) + "?cache=shared&mode=rw"
	if k.nickelDB, err = sql.Open("sqlite3", dsn); err != nil {
		err = fmt.Errorf("openNickelDB: sql open failed: %w", err)
	}
	return err
}

func (k *Kobo) setupMetaTrigger() error {
	var err error
	tx, err := k.nickelDB.Begin()
	if err != nil {
		return fmt.Errorf("setupMetaTrigger: Error beginning transaction: %w", err)
	}
	// Table to hold temporary metadata for the trigger to use
	metaTableQuery := `
	CREATE TABLE IF NOT EXISTS _ku_meta (
		ContentID    TEXT NOT NULL UNIQUE,
		Description  TEXT,
		Series       TEXT,
		SeriesNumber TEXT,
		PRIMARY KEY(ContentID)
	);`
	if _, err = tx.Exec(metaTableQuery); err != nil {
		tx.Rollback()
		return fmt.Errorf("setupMetaTrigger: Create _ku_meta table error: %w", err)
	}
	// Trigger fired when Nickel inserts a book into the content table
	// It replaces and/or adds metadata after the record has been inserted
	triggerQuery := `
	DROP TRIGGER IF EXISTS _ku_meta_content_insert;
	CREATE TRIGGER _ku_meta_content_insert
		AFTER INSERT ON content WHEN
			(new.ImageId LIKE "file____mnt_%") AND
			(SELECT count() FROM _ku_meta WHERE ContentID = new.ContentID)
		BEGIN
			UPDATE content
			SET
				Description  = (SELECT Description  FROM _ku_meta WHERE ContentID = new.ContentID),
				Series       = (SELECT Series       FROM _ku_meta WHERE ContentID = new.ContentID),
				SeriesNumber = (SELECT SeriesNumber FROM _ku_meta WHERE ContentID = new.ContentID)
			WHERE ContentID = new.ContentID;
			DELETE FROM _ku_meta WHERE ContentID = new.ContentID;
		END;`
	if _, err = tx.Exec(triggerQuery); err != nil {
		tx.Rollback()
		return fmt.Errorf("setupMetaTrigger: Create trigger error: %w", err)
	}
	// Make sure the _ku_meta has no existing records before beginning. Makes sure we aren't
	// adding a duplicate row
	if _, err = tx.Exec(`DELETE FROM _ku_meta;`); err != nil {
		tx.Rollback()
		return fmt.Errorf("setupMetaTrigger: Delete rows error: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("setupMetaTrigger: Error committing transaction: %w", err)
	}
	return nil
}

func (k *Kobo) removeMetaTrigger() error {
	var err error
	tx, err := k.nickelDB.Begin()
	if err != nil {
		return fmt.Errorf("removeMetaTrigger: Error beginning transaction: %w", err)
	}
	if _, err = tx.Exec(`DROP TABLE IF EXISTS _ku_meta;`); err != nil {
		tx.Rollback()
		return fmt.Errorf("removeMetaTrigger: drop _ku_meta error: %w", err)
	}
	if _, err = tx.Exec(`DROP TRIGGER IF EXISTS _ku_meta_content_insert;`); err != nil {
		tx.Rollback()
		return fmt.Errorf("removeMetaTrigger: drop _ku_meta_content_insert error: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("removeMetaTrigger: Error committing transaction: %w", err)
	}
	return nil
}

// UpdateIfExists updates onboard metadata if it exists in the Nickel database
func (k *Kobo) UpdateIfExists(cID string, len int) error {
	if _, exists := k.MetadataMap[cID]; exists {
		var currSize int
		// Make really sure this is in the Nickel DB
		// The query helpfully comes from Calibre
		testQuery := `
			SELECT ___FileSize 
			FROM content 
			WHERE ContentID = ? 
			AND ContentType = 6`
		if err := k.nickelDB.QueryRow(testQuery, cID).Scan(&currSize); err != nil {
			return fmt.Errorf("UpdateIfExists: error querying row: %w", err)
		}
		if currSize != len {
			updateQuery := `
				UPDATE content 
				SET ___FileSize = ? 
				WHERE ContentId = ? 
				AND ContentType = 6`
			if _, err := k.nickelDB.Exec(updateQuery, len, cID); err != nil {
				return fmt.Errorf("UpdateIfExists: error updating filesize field: %w", err)
			}
			log.Println("Updated existing book file length")
		}
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
	fwStr := strings.Split(vers, ".")
	k.fw.major, _ = strconv.Atoi(fwStr[0])
	k.fw.minor, _ = strconv.Atoi(fwStr[1])
	k.fw.build, _ = strconv.Atoi(fwStr[2])
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
	//spew.Dump(k.MetadataMap)
	// Now that we have our map, we need to check for any books in the DB not in our
	// metadata cache, or books that are in our cache but not in the DB
	var (
		dbCID         string
		dbTitle       *string
		dbAttr        *string
		dbDesc        *string
		dbPublisher   *string
		dbSeries      *string
		dbbSeriesNum  *string
		dbContentType int
		dbMimeType    string
	)
	query := fmt.Sprintf(`
		SELECT ContentID, Title, Attribution, Description, Publisher, Series, SeriesNumber, ContentType, MimeType
		FROM content
		WHERE ContentType=6
		AND MimeType NOT LIKE 'image%%'
		AND (IsDownloaded='true' OR IsDownloaded=1)
		AND ___FileSize>0
		AND Accessibility=-1
		AND ContentID LIKE '%s%%';`, k.ContentIDprefix)

	bkRows, err := k.nickelDB.Query(query)
	if err != nil {
		return fmt.Errorf("readMDfile: error getting book rows: %w", err)
	}
	defer bkRows.Close()
	for bkRows.Next() {
		err = bkRows.Scan(&dbCID, &dbTitle, &dbAttr, &dbDesc, &dbPublisher, &dbSeries, &dbbSeriesNum, &dbContentType, &dbMimeType)
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
			fi, err := os.Stat(filepath.Join(k.BKRootDir, bkMD.Lpath))
			if err == nil {
				bkSz := fi.Size()
				lastMod := uc.ConvertTime(fi.ModTime())
				bkMD.LastModified = &lastMod
				bkMD.Size = int(bkSz)
			}
			//spew.Dump(bkMD)
			k.MetadataMap[dbCID] = bkMD
		} else {
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

func (k *Kobo) readUpdateMDfile() error {
	emptyOrNotExist, err := util.ReadJSON(filepath.Join(k.BKRootDir, kuUpdatedMDfile), &k.UpdatedMetadata)
	if emptyOrNotExist {
		// ignore
	} else if err != nil {
		return fmt.Errorf("readUpdateMDfile: error reading update metadata JSON: %w", err)
	}
	return nil
}

// WriteUpdateMDfile writes updated metadata to file
func (k *Kobo) WriteUpdateMDfile() error {
	var err error
	// We only write the file if there is new or updated metadata to write
	if len(k.UpdatedMetadata) == 0 {
		return nil
	}
	// Don't write the file if we are updating metadata via DB trigger
	if k.KuConfig.AddMetadataByTrigger {
		return nil
	}
	if err = util.WriteJSON(filepath.Join(k.BKRootDir, kuUpdatedMDfile), k.UpdatedMetadata); err != nil {
		err = fmt.Errorf("WriteUpdateMDfile: error writing update metadata JSON: %w", err)
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
		if k.useSDCard {
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
		nfn := filepath.Join(k.BKRootDir, cover.GeneratePath(k.useSDCard, imgID))
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

// UpdateNickelDB updates the Nickel database with updated metadata obtained from a previous run,
// or this run if updating via triggers
func (k *Kobo) UpdateNickelDB() (bool, error) {
	rerun := false
	var err error
	tx, err := k.nickelDB.Begin()
	if err != nil {
		return rerun, fmt.Errorf("UpdateNickelDB: Error beginning transaction: %w", err)
	}
	// Insert prepared statement if using triggers
	var insertStmt *sql.Stmt
	if k.KuConfig.AddMetadataByTrigger {
		insertQuery := `
		INSERT INTO _ku_meta (ContentID, Description, Series, SeriesNumber)
		VALUES (?, ?, ?, ?);`
		insertStmt, err = tx.Prepare(insertQuery)
		if err != nil {
			tx.Rollback()
			return rerun, fmt.Errorf("UpdateNickelDB: prepared insert statement failed: %w", err)
		}
	}
	// Update statment for books already in the content table
	updateQuery := `
		UPDATE content SET 
		Description=?,
		Series=?,
		SeriesNumber=?,
		SeriesNumberFloat=? 
		WHERE ContentID=?;`
	updateStmt, err := tx.Prepare(updateQuery)
	if err != nil {
		tx.Rollback()
		return rerun, fmt.Errorf("UpdateNickelDB: prepared statement failed: %w", err)
	}
	var updateErr error
	var desc, series, seriesNum *string
	var seriesNumFloat *float64
	for cid := range k.UpdatedMetadata {
		desc, series, seriesNum, seriesNumFloat = nil, nil, nil, nil
		if k.MetadataMap[cid].Comments != nil && *k.MetadataMap[cid].Comments != "" {
			desc = k.MetadataMap[cid].Comments
		}
		if k.MetadataMap[cid].Series != nil && *k.MetadataMap[cid].Series != "" {
			series = k.MetadataMap[cid].Series
		}
		if k.MetadataMap[cid].SeriesIndex != nil && *k.MetadataMap[cid].SeriesIndex != 0.0 {
			sn := strconv.FormatFloat(*k.MetadataMap[cid].SeriesIndex, 'f', -1, 64)
			seriesNum = &sn
			seriesNumFloat = k.MetadataMap[cid].SeriesIndex
		}
		// Note, not rolling back transaction on error. Is this allowed?
		// Don't want one bad update to derail the whole thing, hence avoiding rollback
		if _, ok := k.BooksInDB[cid]; ok {
			_, err = updateStmt.Exec(desc, series, seriesNum, seriesNumFloat, cid)
			if err != nil {
				updateErr = fmt.Errorf("UpdateNickelDB: %w", err)
			}
			delete(k.UpdatedMetadata, cid)
		} else {
			rerun = true
			if k.KuConfig.AddMetadataByTrigger {
				_, err = insertStmt.Exec(cid, desc, series, seriesNum)
				if err != nil {
					updateErr = fmt.Errorf("UpdateNickelDB: %w", err)
				}
				delete(k.UpdatedMetadata, cid)
			}
		}
	}
	if err = tx.Commit(); err != nil {
		return rerun, fmt.Errorf("UpdateNickelDB: Error committing transaction: %w", err)
	}
	// Note, this should only write to the file if new books are added, and AddMetadataByTrigger is false
	if err = k.WriteUpdateMDfile(); err != nil {
		return false, fmt.Errorf("UpdateNickelDB: %w", err)
	}
	return rerun, updateErr
}

// Close the kobo object when we're finished with it
func (k *Kobo) Close() {
	k.Wg.Wait()
	k.nickelDB.Close()
}
