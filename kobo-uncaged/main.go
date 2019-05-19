// Copyright 2019 Sherman Perry

// This file is part of Kobo UNCaGED.

// Kobo UNCaGED is free software: you can redistribute it and/or modify
// it under the terms of the Affero GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

// Kobo UNCaGED is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.

// You should have received a copy of the GNU Affero General Public License
// along with Kobo UNCaGED.  If not, see <https://www.gnu.org/licenses/>.

package main

import (
	"bytes"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"html"
	"io"
	"io/ioutil"
	"log"
	"log/syslog"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/disintegration/imaging"
	"github.com/gofrs/uuid"
	"github.com/kapmahc/epub"
	_ "github.com/mattn/go-sqlite3"
	"github.com/mitchellh/mapstructure"
	"github.com/pelletier/go-toml"
	"github.com/shermp/UNCaGED/uc"
)

type returnCode int

// Note, this is set by the go linker at build time
var kuVersion string

const (
	kuError           returnCode = 250
	kuSuccessNoAction returnCode = 0
	kuSuccessRerun    returnCode = 1
	kuPasswordError   returnCode = 100
)

const koboDBpath = ".kobo/KoboReader.sqlite"
const koboVersPath = ".kobo/version"
const calibreMDfile = "metadata.calibre"
const calibreDIfile = "driveinfo.calibre"
const kuUpdatedMDfile = "metadata_update.kobouc"

const onboardPrefix cidPrefix = "file:///mnt/onboard/"
const sdPrefix cidPrefix = "file:///mnt/sd/"

// imgIDFromContentID generates an imageID from a contentID, using the
// the replacement values as found in the Calibre Kobo driver
func imgIDFromContentID(contentID string) string {
	r := strings.NewReplacer("/", "_", " ", "_", ":", "_", ".", "_")
	return r.Replace(contentID)
}

type uncagedPassword struct {
	currPassIndex int
	passwordList  []string
}

func newUncagedPassword(passwordList []string) *uncagedPassword {
	pw := &uncagedPassword{}
	pw.passwordList = passwordList
	return pw
}

func (pw *uncagedPassword) nextPassword() string {
	password := ""
	if pw.currPassIndex < len(pw.passwordList) {
		password = pw.passwordList[pw.currPassIndex]
		pw.currPassIndex++
	}
	return password
}

// KoboUncaged contains the variables and methods required to use
// the UNCaGED library
type KoboUncaged struct {
	kup      kuPrinter
	device   koboDevice
	fw       [3]int
	KuConfig struct {
		PreferSDCard bool
		PreferKepub  bool
		PasswordList []string
		EnableDebug  bool
	}
	dbRootDir         string
	bkRootDir         string
	contentIDprefix   cidPrefix
	useSDCard         bool
	metadataMap       map[string]KoboMetadata
	updatedMetadata   []string
	passwords         *uncagedPassword
	driveInfo         uc.DeviceInfo
	nickelDB          *sql.DB
	wg                *sync.WaitGroup
	invalidCharsRegex *regexp.Regexp
}

// We use a constructor, because nested maps
func createKoboMetadata() KoboMetadata {
	md := KoboMetadata{}
	md.UserMetadata = make(map[string]interface{}, 0)
	md.UserCategories = make(map[string]interface{}, 0)
	md.AuthorSortMap = make(map[string]string, 0)
	md.AuthorLinkMap = make(map[string]string, 0)
	md.Identifiers = make(map[string]string, 0)
	return md
}

// New creates a KoboUncaged object, ready for use
func New(dbRootDir, sdRootDir string, updatingMD bool) (*KoboUncaged, error) {
	var err error
	ku := &KoboUncaged{}
	ku.wg = &sync.WaitGroup{}
	ku.dbRootDir = dbRootDir
	ku.bkRootDir = dbRootDir
	ku.contentIDprefix = onboardPrefix
	ku.updatedMetadata = make([]string, 0)
	fntPath := filepath.Join(ku.dbRootDir, ".adds/kobo-uncaged/fonts/LiberationSans-Regular.ttf")
	ku.kup, err = newKuPrint(fntPath)
	if err != nil {
		return nil, err
	}
	configBytes, err := ioutil.ReadFile(filepath.Join(ku.dbRootDir, ".adds/kobo-uncaged/config/ku.toml"))
	if err != nil {
		ku.kup.kuPrintln(body, "Error loading config file")
		log.Print(err)
		return nil, err
	}
	err = toml.Unmarshal(configBytes, &ku.KuConfig)
	if err != nil {
		ku.kup.kuPrintln(body, "Error reading config file")
		log.Print(err)
		return nil, err
	}
	if sdRootDir != "" && ku.KuConfig.PreferSDCard {
		ku.useSDCard = true
		ku.bkRootDir = sdRootDir
		ku.contentIDprefix = sdPrefix
	}
	ku.passwords = newUncagedPassword(ku.KuConfig.PasswordList)
	headerStr := "Kobo-UNCaGED  " + kuVersion
	if ku.useSDCard {
		headerStr += "\nUsing SD Card"
	} else {
		headerStr += "\nUsing Internal Storage"
	}
	ku.kup.kuPrintln(header, headerStr)
	ku.kup.kuPrintln(body, "Gathering information about your Kobo")
	ku.invalidCharsRegex, err = regexp.Compile(`[\\?%\*:;\|\"\'><\$!]`)
	if err != nil {
		return nil, err
	}
	log.Println("Opening NickelDB")
	err = ku.openNickelDB()
	if err != nil {
		return nil, err
	}
	log.Println("Getting Kobo Info")
	err = ku.getKoboInfo()
	if err != nil {
		return nil, err
	}
	log.Println("Getting Device Info")
	err = ku.loadDeviceInfo()
	if err != nil {
		return nil, err
	}
	log.Println("Reading Metadata")
	err = ku.readMDfile()
	if err != nil {
		return nil, err
	}
	if updatingMD {
		err = ku.readUpdateMDfile()
		if err != nil {
			return nil, err
		}
	}
	return ku, nil
}

// genImagePath generates the directory structure used by
// kobo to store the cover image files.
// It has been ported from the implementation in the KoboTouch
// driver in Calibre
func (ku *KoboUncaged) genImageDirPath(imageID string) string {
	imgID := []byte(imageID)
	h := uint32(0x00000000)
	for _, x := range imgID {
		h = (h << 4) + uint32(x)
		h ^= (h & 0xf0000000) >> 23
		h &= 0x0fffffff
	}
	dir1 := h & (0xff * 1)
	dir2 := (h & (0xff00 * 1)) >> 8
	pathPrefix := ".kobo-images"
	if ku.useSDCard {
		pathPrefix = "koboExtStorage/images-cache"
	}
	return fmt.Sprintf("%s/%d/%d", pathPrefix, dir1, dir2)
}

func (ku *KoboUncaged) openNickelDB() error {
	err := error(nil)
	dbPath := filepath.Join(ku.dbRootDir, koboDBpath)
	sqlDSN := "file:" + dbPath + "?cache=shared&mode=rw"
	ku.nickelDB, err = sql.Open("sqlite3", sqlDSN)
	return err
}

func (ku *KoboUncaged) lpathToContentID(lpath string) string {
	newLpath := lpath
	if ku.lpathIsKepub(lpath) {
		newLpath += ".epub"
	}
	newLpath = strings.TrimPrefix(newLpath, "/")
	return string(ku.contentIDprefix) + newLpath
}

func (ku *KoboUncaged) contentIDtoLpath(contentID string) string {
	newCID := contentID
	if ku.contentIDisKepub(contentID) {
		newCID = strings.TrimSuffix(contentID, ".epub")
	}
	if strings.HasPrefix(newCID, string(ku.contentIDprefix)) {
		return strings.TrimPrefix(newCID, string(ku.contentIDprefix))
	}
	return newCID
}

func (ku *KoboUncaged) contentIDtoBkPath(contentID string) string {
	path := strings.TrimPrefix(contentID, string(ku.contentIDprefix))
	return filepath.Join(ku.bkRootDir, path)
}

func (ku *KoboUncaged) contentIDisBkDir(contentID string) bool {
	return strings.HasPrefix(contentID, string(ku.contentIDprefix))
}

func (ku *KoboUncaged) lpathIsKepub(lpath string) bool {
	return strings.HasSuffix(lpath, ".kepub")
}

func (ku *KoboUncaged) contentIDisKepub(contentID string) bool {
	return strings.HasSuffix(contentID, ".kepub.epub")
}

func (ku *KoboUncaged) updateIfExists(cID string, len int) error {
	if _, exists := ku.metadataMap[cID]; exists {
		var currSize int
		// Make really sure this is in the Nickel DB
		// The query helpfully comes from Calibre
		testQuery := `SELECT ___FileSize 
                        FROM content 
                        WHERE ContentID = ? 
                        AND ContentType = 6`
		err := ku.nickelDB.QueryRow(testQuery, cID).Scan(&currSize)
		if err != nil {
			return err
		}
		if currSize != len {
			updateQuery := `UPDATE content 
						SET ___FileSize = ? 
						WHERE ContentId = ? 
						AND ContentType = 6`
			_, err = ku.nickelDB.Exec(updateQuery, len, cID)
			if err != nil {
				return err
			}
			log.Println("Updated existing book file length")
		}
	}
	return nil
}

func (ku *KoboUncaged) getKoboInfo() error {
	// Get the model ID and firmware version from the device
	versInfo, err := ioutil.ReadFile(filepath.Join(ku.dbRootDir, koboVersPath))
	if err != nil {
		return err
	}
	if len(versInfo) > 0 {
		vers := strings.TrimSpace(string(versInfo))
		versFields := strings.Split(vers, ",")
		fwStr := strings.Split(versFields[2], ".")
		for i, f := range fwStr {
			ku.fw[i], _ = strconv.Atoi(f)
		}
		ku.device = koboDevice(versFields[len(versFields)-1])
	}
	return nil
}

// readEpubMeta opens an epub (or kepub), and attempts to read the
// metadata it contains. This is used if the metadata has not yet
// been cached
func (ku *KoboUncaged) readEpubMeta(contentID string, md *KoboMetadata) error {
	lpath := ku.contentIDtoLpath(contentID)
	epubPath := ku.contentIDtoBkPath(contentID)
	bk, err := epub.Open(epubPath)
	if err != nil {
		return err
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
		pubDate := bk.Opf.Metadata.Date[0].Data
		md.Pubdate = &pubDate
	}
	for _, m := range bk.Opf.Metadata.Meta {
		switch m.Name {
		case "calibre:timestamp":
			timeStamp := m.Content
			md.Timestamp = &timeStamp
		case "calibre:series":
			series := m.Content
			md.Series = &series
		case "calibre:series_index":
			seriesIndex, _ := strconv.ParseFloat(m.Content, 64)
			md.SeriesIndex = &seriesIndex
		case "calibre:title_sort":
			md.TitleSort = m.Content
		case "calibre:author_link_map":
			almJSON := html.UnescapeString(m.Content)
			alm := make(map[string]string, 0)
			err := json.Unmarshal([]byte(almJSON), &alm)
			if err == nil {
				md.AuthorLinkMap = alm
			}
		}

	}
	return nil
}

// readMDfile loads cached metadata from the "metadata.calibre" JSON file
// and unmarshals (eventially) to a map of KoboMetadata structs, converting
// "lpath" to Kobo's "ContentID", and using that as the map keys
func (ku *KoboUncaged) readMDfile() error {
	log.Println(body, "Reading metadata.calibre")
	mdJSON, err := ioutil.ReadFile(filepath.Join(ku.bkRootDir, calibreMDfile))
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
	}
	koboMD := []KoboMetadata{}
	if len(mdJSON) > 0 {
		err = json.Unmarshal(mdJSON, &koboMD)
		if err != nil {
			return err
		}
	}
	// Make the metadatamap here instead of the constructer so we can pre-allocate
	// the memory with the right size.
	ku.metadataMap = make(map[string]KoboMetadata, len(koboMD))
	// make a temporary map for easy searching later
	tmpMap := make(map[string]int, len(koboMD))
	for n, md := range koboMD {
		contentID := ku.lpathToContentID(md.Lpath)
		tmpMap[contentID] = n
	}
	log.Println(body, "Gathering metadata")
	//spew.Dump(ku.metadataMap)
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
	query := `SELECT ContentID, Title, Attribution, Description, Publisher, Series, SeriesNumber, ContentType, MimeType
	FROM content
	WHERE ContentType=6 
	AND MimeType NOT LIKE 'image%'
	AND (IsDownloaded='true' OR IsDownloaded=1)
	AND ___FileSize>0
	AND Accessibility=-1 `
	query += fmt.Sprintf("AND ContentID LIKE 'file://%s%%';", ku.contentIDprefix)

	bkRows, err := ku.nickelDB.Query(query)
	if err != nil {
		return err
	}
	defer bkRows.Close()
	for bkRows.Next() {
		err = bkRows.Scan(&dbCID, &dbTitle, &dbAttr, &dbDesc, &dbPublisher, &dbSeries, &dbbSeriesNum, &dbContentType, &dbMimeType)
		if err != nil {
			return err
		}
		if _, exists := tmpMap[dbCID]; !exists {
			log.Printf("Book not in cache: %s\n", dbCID)
			bkMD := createKoboMetadata()
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
			if dbMimeType == "application/epub+zip" || dbMimeType == "application/x-kobo-epub+zip" {
				err = ku.readEpubMeta(dbCID, &bkMD)
				if err != nil {
					log.Print(err)
				}
			}
			fi, err := os.Stat(filepath.Join(ku.bkRootDir, bkMD.Lpath))
			if err == nil {
				bkSz := fi.Size()
				lastMod := fi.ModTime().Format(time.RFC3339)
				bkMD.LastModified = &lastMod
				bkMD.Size = int(bkSz)
			}
			//spew.Dump(bkMD)
			ku.metadataMap[dbCID] = bkMD
		} else {
			ku.metadataMap[dbCID] = koboMD[tmpMap[dbCID]]
		}
	}
	err = bkRows.Err()
	if err != nil {
		return err
	}
	// Hopefully, our metadata is now up to date. Update the cache on disk
	err = ku.writeMDfile()
	if err != nil {
		return err
	}
	return nil
}

func (ku *KoboUncaged) writeMDfile() error {
	// First, convert our metadata map to a slice
	metadata := make([]KoboMetadata, len(ku.metadataMap))
	n := 0
	for _, md := range ku.metadataMap {
		metadata[n] = md
		n++
	}
	// Convert it to JSON, prettifying it in the process
	mdJSON, _ := json.MarshalIndent(metadata, "", "    ")

	err := ioutil.WriteFile(filepath.Join(ku.bkRootDir, calibreMDfile), mdJSON, 0644)
	if err != nil {
		return err
	}
	return nil
}

func (ku *KoboUncaged) readUpdateMDfile() error {
	mdJSONarr, err := ioutil.ReadFile(filepath.Join(ku.bkRootDir, kuUpdatedMDfile))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		log.Print(err)
		return err
	}
	if len(mdJSONarr) > 0 {
		err = json.Unmarshal(mdJSONarr, &ku.updatedMetadata)
		if err != nil {
			log.Print(err)
			return err
		}
	}
	return nil
}

func (ku *KoboUncaged) writeUpdateMDfile() error {
	// We only write the file if there is new or updated metadata to write
	if len(ku.updatedMetadata) > 0 {
		mdJSON, _ := json.MarshalIndent(ku.updatedMetadata, "", "    ")
		err := ioutil.WriteFile(filepath.Join(ku.bkRootDir, kuUpdatedMDfile), mdJSON, 0644)
		if err != nil {
			return err
		}
	}
	return nil
}

func (ku *KoboUncaged) loadDeviceInfo() error {
	diJSON, err := ioutil.ReadFile(filepath.Join(ku.bkRootDir, calibreDIfile))
	if err != nil {
		if os.IsNotExist(err) {
			uuid4, _ := uuid.NewV4()
			ku.driveInfo.DevInfo.LocationCode = "main"
			ku.driveInfo.DevInfo.DeviceName = "Kobo " + ku.device.Model()
			ku.driveInfo.DevInfo.DeviceStoreUUID = uuid4.String()
			if ku.useSDCard {
				ku.driveInfo.DevInfo.LocationCode = "A"
			}
			return nil
		}
		return err
	}
	if len(diJSON) > 0 {
		err = json.Unmarshal(diJSON, &ku.driveInfo.DevInfo)
		if err != nil {
			return err
		}
	}
	return nil
}

func (ku *KoboUncaged) saveDeviceInfo() error {
	diJSON, err := json.MarshalIndent(ku.driveInfo.DevInfo, "", "    ")
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filepath.Join(ku.bkRootDir, calibreDIfile), diJSON, 0644)
}

func (ku *KoboUncaged) saveCoverImage(contentID string, thumb []interface{}) {
	defer ku.wg.Done()
	thumbW := int(thumb[0].(float64))
	thumbH := int(thumb[1].(float64))
	imgB64 := thumb[2].(string)
	imgID := imgIDFromContentID(contentID)
	imgDir := path.Join(ku.bkRootDir, ku.genImageDirPath(imgID))
	_, libFullSize, libGridSize := ku.device.CoverSize()
	err := os.MkdirAll(imgDir, 0744)
	if err == nil {
		imgBin, err := base64.StdEncoding.DecodeString(imgB64)
		if err == nil {
			libFullSaved := false
			// No need to perform any image processing for the library thumb if it meets all our requirements
			// Note, we asked Calibre to give us a thumbnail with a given height...
			if thumbW <= libFullSize.X {
				libFullSaved = true
				err = ioutil.WriteFile(path.Join(imgDir, (imgID+string(libFull))), imgBin, 0644)
				if err != nil {
					log.Println(err)
				}
			}
			// Now we do our resizing
			origCover, err := imaging.Decode(bytes.NewReader(imgBin))
			if err == nil {
				var libFW, libFH, gridFW, gridFH int
				if (float64(thumbW) / float64(thumbH)) < 1.0 {
					libFH, gridFH = libFullSize.Y, libGridSize.Y
				} else {
					libFW, gridFW = libFullSize.X, libGridSize.X
				}

				// If we haven't allready saved the libFull thumbnail, resize it now.
				if !libFullSaved {
					libImg := imaging.Resize(origCover, libFW, libFH, imaging.Linear)
					lc, err := os.OpenFile(path.Join(imgDir, (imgID+string(libFull))), os.O_WRONLY|os.O_CREATE, 0644)
					if err == nil {
						defer lc.Close()
						imaging.Encode(lc, libImg, imaging.JPEG)
					} else {
						log.Println(err)
					}
				}
				// And finally, the "library grid" image
				gridImg := imaging.Resize(origCover, gridFW, gridFH, imaging.Linear)
				gc, err := os.OpenFile(path.Join(imgDir, (imgID+string(libGrid))), os.O_WRONLY|os.O_CREATE, 0644)
				if err == nil {
					defer gc.Close()
					imaging.Encode(gc, gridImg, imaging.JPEG)
				} else {
					log.Println(err)
				}
			}
		} else {
			log.Println(err)
		}
	} else {
		log.Println(err)
	}
}

// updateNickelDB updates the Nickel database with updated metadata obtained from a previous run
func (ku *KoboUncaged) updateNickelDB() error {
	// No matter what happens, we remove the 'metadata_update.kobouc' file when we're done
	defer os.Remove(filepath.Join(ku.bkRootDir, kuUpdatedMDfile))
	query := `UPDATE content SET 
	Description=?,
	Series=?,
	SeriesNumber=?,
	SeriesNumberFloat=? 
	WHERE ContentID=?`
	stmt, err := ku.nickelDB.Prepare(query)
	if err != nil {
		return err
	}
	var desc, series, seriesNum *string
	var seriesNumFloat *float64
	for _, cID := range ku.updatedMetadata {
		desc, series, seriesNum, seriesNumFloat = nil, nil, nil, nil
		if ku.metadataMap[cID].Comments != nil && *ku.metadataMap[cID].Comments != "" {
			desc = ku.metadataMap[cID].Comments
		}
		if ku.metadataMap[cID].Series != nil && *ku.metadataMap[cID].Series != "" {
			series = ku.metadataMap[cID].Series
		}
		if ku.metadataMap[cID].SeriesIndex != nil && *ku.metadataMap[cID].SeriesIndex != 0.0 {
			sn := strconv.FormatFloat(*ku.metadataMap[cID].SeriesIndex, 'f', -1, 64)
			seriesNum = &sn
			seriesNumFloat = ku.metadataMap[cID].SeriesIndex
		}
		_, err = stmt.Exec(desc, series, seriesNum, seriesNumFloat, cID)
		if err != nil {
			log.Print(err)
		}
	}
	return nil
}

// GetClientOptions returns all the client specific options required for UNCaGED
func (ku *KoboUncaged) GetClientOptions() uc.ClientOptions {
	opts := uc.ClientOptions{}
	opts.ClientName = "Kobo UNCaGED " + kuVersion
	var ext []string
	if ku.KuConfig.PreferKepub {
		ext = []string{"kepub", "epub", "mobi", "pdf", "cbz", "cbr", "txt", "html", "rtf"}
	} else {
		ext = []string{"epub", "kepub", "mobi", "pdf", "cbz", "cbr", "txt", "html", "rtf"}
	}
	opts.SupportedExt = append(opts.SupportedExt, ext...)
	opts.DeviceName = "Kobo"
	opts.DeviceModel = ku.device.Model()
	_, lf, _ := ku.device.CoverSize()
	opts.CoverDims.Width, opts.CoverDims.Height = lf.X, lf.Y
	return opts
}

// GetDeviceBookList returns a slice of all the books currently on the device
// A nil slice is interpreted has having no books on the device
func (ku *KoboUncaged) GetDeviceBookList() []uc.BookCountDetails {
	bc := []uc.BookCountDetails{}
	for _, md := range ku.metadataMap {
		lastMod := time.Now()
		if md.LastModified != nil {
			lastMod, _ = time.Parse(time.RFC3339, *md.LastModified)
		}
		bcd := uc.BookCountDetails{
			UUID:         md.UUID,
			Lpath:        md.Lpath,
			LastModified: lastMod,
		}
		bcd.Extension = filepath.Ext(md.Lpath)
		bc = append(bc, bcd)
	}
	//spew.Dump(bc)
	return bc
}

// GetMetadataList sends complete metadata for the books listed in lpaths, or for
// all books on device if lpaths is empty
func (ku *KoboUncaged) GetMetadataList(books []uc.BookID) []map[string]interface{} {
	//spew.Dump(ku.metadataMap)
	//spew.Dump(books)
	mdList := []map[string]interface{}{}
	if len(books) > 0 {
		for _, bk := range books {
			cID := ku.lpathToContentID(bk.Lpath)
			fmt.Println(cID)
			md := map[string]interface{}{}
			//spew.Dump(ku.metadataMap[cID])
			mapstructure.Decode(ku.metadataMap[cID], &md)
			mdList = append(mdList, md)
		}
	} else {
		for _, kmd := range ku.metadataMap {
			md := map[string]interface{}{}
			//spew.Dump(kmd)
			mapstructure.Decode(kmd, &md)
			mdList = append(mdList, md)
		}
	}
	return mdList
}

// GetDeviceInfo asks the client for information about the drive info to use
func (ku *KoboUncaged) GetDeviceInfo() uc.DeviceInfo {
	return ku.driveInfo
}

// SetDeviceInfo sets the new device info, as comes from calibre. Only the nested
// struct DevInfo is modified.
func (ku *KoboUncaged) SetDeviceInfo(devInfo uc.DeviceInfo) {
	ku.driveInfo = devInfo
	ku.saveDeviceInfo()
}

// UpdateMetadata instructs the client to update their metadata according to the
// new slice of metadata maps
func (ku *KoboUncaged) UpdateMetadata(mdList []map[string]interface{}) {
	for _, md := range mdList {
		koboMD := createKoboMetadata()
		mapstructure.Decode(md, &koboMD)
		koboMD.Thumbnail = nil
		cID := ku.lpathToContentID(koboMD.Lpath)
		ku.metadataMap[cID] = koboMD
		ku.updatedMetadata = append(ku.updatedMetadata, cID)
	}
	ku.writeMDfile()
	ku.writeUpdateMDfile()
}

// GetPassword gets a password from the user.
func (ku *KoboUncaged) GetPassword(calibreInfo uc.CalibreInitInfo) string {
	return ku.passwords.nextPassword()
}

// GetFreeSpace reports the amount of free storage space to Calibre
func (ku *KoboUncaged) GetFreeSpace() uint64 {
	// Note, this method of getting available disk space is Linux specific...
	// Don't try to run this code on Windows. It will probably fall over
	fs := syscall.Statfs_t{}
	err := syscall.Statfs(ku.bkRootDir, &fs)
	if err != nil {
		log.Println(err)
		// Despite the error, we return an arbitrary amount. Thoughts on this?
		return 1024 * 1024 * 1024
	}
	return fs.Bavail * uint64(fs.Bsize)
}

// SaveBook saves a book with the provided metadata to the disk.
// Implementations return an io.WriteCloser (book) for UNCaGED to write the ebook to
// lastBook informs the client that this is the last book for this transfer
// newLpath informs UNCaGED of an Lpath change. Use this if the lpath field in md is
// not valid (eg filesystem limitations.). Return an empty string if original lpath is valid
func (ku *KoboUncaged) SaveBook(md map[string]interface{}, len int, lastBook bool) (book io.WriteCloser, newLpath string, err error) {
	koboMD := createKoboMetadata()
	mapstructure.Decode(md, &koboMD)
	// The calibre wireless driver does not sanitize the filepath for us. We sanitize it here,
	// and if lpath changes, inform Calibre of the new lpath.
	newLpath = ku.invalidCharsRegex.ReplaceAllString(koboMD.Lpath, "_")
	if newLpath != koboMD.Lpath {
		koboMD.Lpath = newLpath
	} else {
		newLpath = ""
	}
	cID := ku.lpathToContentID(koboMD.Lpath)
	bkPath := ku.contentIDtoBkPath(cID)
	bkDir, _ := filepath.Split(bkPath)
	err = os.MkdirAll(bkDir, 0777)
	if err != nil {
		return nil, "", err
	}
	book, err = os.OpenFile(bkPath, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return nil, "", err
	}

	ku.metadataMap[cID] = koboMD
	ku.updatedMetadata = append(ku.updatedMetadata, cID)
	// Note, the JSON format for covers should be in the form 'thumbnail: [w, h, "base64string"]'
	if koboMD.Thumbnail != nil {
		ku.wg.Add(1)
		go ku.saveCoverImage(cID, koboMD.Thumbnail)
	}
	err = ku.updateIfExists(cID, len)
	if err != nil {
		log.Print(err)
	}
	if lastBook {
		ku.writeMDfile()
		ku.writeUpdateMDfile()
	}
	return book, newLpath, nil
}

// GetBook provides an io.ReadCloser, and the file len, from which UNCaGED can send the requested book to Calibre
// NOTE: filePos > 0 is not currently implemented in the Calibre source code, but that could
// change at any time, so best to handle it anyway.
func (ku *KoboUncaged) GetBook(book uc.BookID, filePos int64) (io.ReadCloser, int64, error) {
	cID := ku.lpathToContentID(book.Lpath)
	bkPath := ku.contentIDtoBkPath(cID)
	fi, err := os.Stat(bkPath)
	if err != nil {
		return nil, 0, err
	}
	bookLen := fi.Size()
	ebook, err := os.OpenFile(bkPath, os.O_RDONLY, 0644)
	return ebook, bookLen, err
}

// DeleteBook instructs the client to delete the specified book on the device
// Error is returned if the book was unable to be deleted
func (ku *KoboUncaged) DeleteBook(book uc.BookID) error {
	// Start with basic book deletion. A more fancy implementation can come later
	// (eg: removing cover image remnants etc)
	cID := ku.lpathToContentID(book.Lpath)
	bkPath := ku.contentIDtoBkPath(cID)
	dir, _ := filepath.Split(bkPath)
	dirPath := filepath.Clean(dir)
	err := os.Remove(bkPath)
	if err != nil {
		log.Print(err)
		return err
	}
	for dirPath != filepath.Clean(ku.bkRootDir) {
		// Note, os.Remove only removes empty directories, so it should be safe to call
		err := os.Remove(dirPath)
		if err != nil {
			log.Print(err)
			// We don't consider failure to remove parent directories an error, so
			// long as the book file itself was deleted.
			break
		}
		// Walk 'up' the path
		dirPath = filepath.Clean(filepath.Join(dirPath, "../"))
	}
	// Now we remove the book from the metadata map
	delete(ku.metadataMap, cID)
	// As well as the updated metadata list, if it was added to the list this session
	l := len(ku.updatedMetadata)
	for n := 0; n < l; n++ {
		if ku.updatedMetadata[n] == cID {
			ku.updatedMetadata[n] = ku.updatedMetadata[len(ku.updatedMetadata)-1]
			ku.updatedMetadata = ku.updatedMetadata[:len(ku.updatedMetadata)-1]
			break
		}
	}
	// Finally, write the new metadata files
	ku.writeMDfile()
	ku.writeUpdateMDfile()
	return nil
}

// UpdateStatus gives status updates from the UNCaGED library
func (ku *KoboUncaged) UpdateStatus(status uc.UCStatus, progress int) {
	footerStr := " "
	if progress >= 0 && progress <= 100 {
		footerStr = fmt.Sprintf("%d%%", progress)
	}
	switch status {
	case uc.Idle:
		fallthrough
	case uc.Connected:
		ku.kup.kuPrintln(body, "Connected")
		ku.kup.kuPrintln(footer, footerStr)
	case uc.Connecting:
		ku.kup.kuPrintln(body, "Connecting to Calibre")
		ku.kup.kuPrintln(footer, footerStr)
	case uc.SearchingCalibre:
		ku.kup.kuPrintln(body, "Searching for Calibre")
		ku.kup.kuPrintln(footer, footerStr)
	case uc.Disconnected:
		ku.kup.kuPrintln(body, "Disconnected")
		ku.kup.kuPrintln(footer, footerStr)
	case uc.SendingBook:
		ku.kup.kuPrintln(body, "Sending book to Calibre")
		ku.kup.kuPrintln(footer, footerStr)
	case uc.ReceivingBook:
		ku.kup.kuPrintln(body, "Receiving book(s) from Calibre")
		ku.kup.kuPrintln(footer, footerStr)
	case uc.EmptyPasswordReceived:
		ku.kup.kuPrintln(body, "No valid password found!")
		ku.kup.kuPrintln(footer, footerStr)
	}
}

// LogPrintf instructs the client to log informational and debug info, that aren't errors
func (ku *KoboUncaged) LogPrintf(logLevel uc.UCLogLevel, format string, a ...interface{}) {
	log.Printf(format, a...)
}

func mainWithErrCode() returnCode {
	w, e := syslog.New(syslog.LOG_DEBUG, "KoboUNCaGED")
	if e == nil {
		log.SetOutput(w)
	}
	onboardMntPtr := flag.String("onboardmount", "/mnt/onboard", "If changed, specify the new new mountpoint of '/mnt/onboard'")
	sdMntPtr := flag.String("sdmount", "", "If changed, specify the new new mountpoint of '/mnt/sd'")
	mdPtr := flag.Bool("metadata", false, "Updates the Kobo DB with new metadata")

	flag.Parse()
	log.Println("Started Kobo-UNCaGED")

	log.Println("Creating KU object")
	ku, err := New(*onboardMntPtr, *sdMntPtr, *mdPtr)
	if err != nil {
		log.Print(err)
		return kuError
	}
	defer ku.kup.kuClose()
	if *mdPtr {
		log.Println("Updating Metadata")
		ku.kup.kuPrintln(body, "Updading Metadata!")
		err = ku.updateNickelDB()
		if err != nil {
			log.Print(err)
			return kuError
		}
		ku.kup.kuPrintln(body, "Metadata Updated!\n\nReturning to Home screen")
	} else {
		log.Println("Preparing Kobo UNCaGED!")
		cc, err := uc.New(ku, ku.KuConfig.EnableDebug)
		if err != nil {
			log.Print(err)
			// TODO: Probably need to come up with a set of error codes for
			//       UNCaGED instead of this string comparison
			if err.Error() == "calibre server not found" {
				ku.kup.kuPrintln(body, "Calibre not found!\nHave you enabled the Calibre Wireless service?")
			}
			return kuError
		}
		log.Println("Starting Calibre Connection")
		ku.kup.kuPrintln(body, "Finishing up")
		ku.wg.Wait()
		err = cc.Start()
		if err != nil {
			if err.Error() == "no password entered" {
				ku.kup.kuPrintln(body, "No valid password found!")
				return kuPasswordError
			}
			log.Print(err)
			return kuError
		}

		if len(ku.updatedMetadata) > 0 {
			ku.kup.kuPrintln(body, "Kobo-UNCaGED will restart automatically to update metadata")
			return kuSuccessRerun
		}
		ku.kup.kuPrintln(body, "Nothing more to do!\n\nReturning to Home screen")
	}

	return kuSuccessNoAction
}
func main() {
	os.Exit(int(mainWithErrCode()))
}
