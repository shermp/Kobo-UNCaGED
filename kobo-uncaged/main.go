package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/kapmahc/epub"
	_ "github.com/mattn/go-sqlite3"
	"github.com/shermp/UNCaGED/uc"
	"github.com/shermp/go-fbink-v2/gofbink"
	"github.com/shermp/kobo-sim-usb/simusb"
)

const koboDBpath = ".kobo/KoboReader.sqlite"
const koboVersPath = ".kobo/version"
const calibreMDfile = ".metadata.calibre"
const calibreDIfile = ".driveinfo.calibre"

// genImagePath generates the directory structure used by
// kobo to store the cover image files.
// It has been ported from the implementation in the KoboTouch
// driver in Calibre
func genImagePath(imageID string) string {
	imgID := []byte(imageID)
	h := uint32(0x00000000)
	for _, x := range imgID {
		h = (h << 4) + uint32(x)
		h ^= (h & 0xf0000000) >> 23
		h &= 0x0fffffff
	}
	dir1 := h & (0xff * 1)
	dir2 := (h & (0xff00 * 1)) >> 8
	return fmt.Sprintf("./kobo-images/%d/%d", dir1, dir2)
}

// KoboUncaged contains the variables and methods required to use
// the UNCaGED library
type KoboUncaged struct {
	fbI      *gofbink.FBInk
	fbCfg    *gofbink.FBInkConfig
	koboInfo struct {
		model koboDeviceID
		fw    [3]int
	}
	dbRootDir       string
	bkRootDir       string
	contentIDprefix string
	metadataMap     map[string]KoboMetadata
	nickelDB        *sql.DB
}

func (ku *KoboUncaged) openNickelDB() error {
	err := error(nil)
	dbPath := filepath.Join(ku.dbRootDir, koboDBpath)
	sqlDSN := "file:" + dbPath + "?cache=shared&mode=rw"
	ku.nickelDB, err = sql.Open("sqlite3", sqlDSN)
	return err
}

func (ku *KoboUncaged) lpathToContentID(lpath string) string {
	return filepath.Join(ku.contentIDprefix, lpath)
}

func (ku *KoboUncaged) contentIDtoLpath(contentID string) string {
	if strings.HasPrefix(contentID, ku.contentIDprefix) {
		return strings.TrimPrefix(contentID, ku.contentIDprefix)
	}
	return contentID
}

func (ku *KoboUncaged) contentIDisBkDir(contentID string) bool {
	return strings.HasPrefix(contentID, ku.contentIDprefix)
}

func (ku *KoboUncaged) getModelInfo() error {
	versInfo, err := ioutil.ReadFile(filepath.Join(ku.dbRootDir, koboVersPath))
	if err != nil {
		return err
	}
	if len(versInfo) > 0 {
		vers := strings.TrimSpace(string(versInfo))
		versFields := strings.Split(vers, ",")
		fwStr := strings.Split(versFields[2], ".")
		for i, f := range fwStr {
			ku.koboInfo.fw[i], _ = strconv.Atoi(f)
		}
		ku.koboInfo.model = koboDeviceID(versFields[len(versFields)-1])
	}
	return nil
}

// readEpubMeta opens an epub (or kepub), and attempts to read the
// metadata it contains. This is used if the metadata has not yet
// been cached
func (ku *KoboUncaged) readEpubMeta(contentID string) (KoboMetadata, error) {
	md := KoboMetadata{}
	lpath := ku.contentIDtoLpath(contentID)
	epubPath := filepath.Join(ku.bkRootDir, lpath)
	bk, err := epub.Open(epubPath)
	if err != nil {
		return md, err
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
		pubDate, _ := time.Parse(time.RFC3339, bk.Opf.Metadata.Date[0].Data)
		md.Pubdate = &pubDate
	}
	for _, m := range bk.Opf.Metadata.Meta {
		switch m.Name {
		case "calibre:timestamp":
			md.Timestamp, _ = time.Parse(time.RFC3339, m.Content)
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
	return md, nil
}

// readMDfile loads cached metadata from the ".metadata.calibre" JSON file
// and unmarshals (eventially) to a map of KoboMetadata structs, converting
// "lpath" to Kobo's "ContentID", and using that as the map keys
func (ku *KoboUncaged) readMDfile() error {
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
	// Check the Nickel DB to see if the book still exists. We perform the check before
	// adding the book to the metadata map below
	rowStmt, err := ku.nickelDB.Prepare("SELECT ContentID FROM content WHERE ContentID=? AND ContentType=6")
	if err != nil {
		return err
	}
	defer rowStmt.Close()
	// convert the list to a map, to make it easier to search later
	for _, md := range koboMD {
		contentID := ku.lpathToContentID(md.Lpath)
		var (
			dbCID string
		)
		err = rowStmt.QueryRow(contentID).Scan(&dbCID)
		if err != nil {
			if err == sql.ErrNoRows {
				// Book not in DB, so we don't proceed further in this loop iteration
				continue
			} else {
				return err
			}
		}
		ku.metadataMap[contentID] = md
	}

	// Now that we have our map, we need to check for any books in the DB not in our
	// metadata cache
	var (
		dbCID         string
		dbTitle       string
		dbAttr        string
		dbContentType int
		dbMimeType    string
	)
	query := `SELECT ContentID, Title, Attribution, ContentType, MimeType
	FROM content
	WHERE ContentType=6 
	AND (MimeType='application/epub+zip' OR MimeType='application/x-kobo-epub+zip')
	AND ContentID LIKE ?`
	bkStmt, err := ku.nickelDB.Prepare(query)
	if err != nil {
		return err
	}
	defer bkStmt.Close()
	bkRows, err := bkStmt.Query(ku.contentIDprefix + "%")
	if err != nil {
		return err
	}
	defer bkRows.Close()
	for bkRows.Next() {
		err = bkRows.Scan(&dbCID, &dbTitle, &dbAttr, &dbContentType, &dbMimeType)
		if err != nil {
			return err
		}
		if _, exists := ku.metadataMap[dbCID]; !exists {
			bkMD, err := ku.readEpubMeta(dbCID)
			if err != nil {
				return err
			}
			fi, err := os.Stat(filepath.Join(ku.bkRootDir, bkMD.Lpath))
			if err == nil {
				bkSz := fi.Size()
				bkMD.Size = int(bkSz)
			}
			ku.metadataMap[dbCID] = bkMD
		}
	}
	err = bkRows.Err()
	if err != nil {
		return err
	}
	return nil
}

func (ku *KoboUncaged) writeMDfile() error {
	// First, convert our metadata map to a slice
	metadata := make([]KoboMetadata, len(ku.metadataMap))
	for _, md := range ku.metadataMap {
		metadata = append(metadata, md)
	}
	// Convert it to JSON, prettifying it in the process
	mdJSON, _ := json.MarshalIndent(metadata, "", "    ")

	err := ioutil.WriteFile(filepath.Join(ku.bkRootDir, calibreMDfile), mdJSON, 0644)
	if err != nil {
		return err
	}
	return nil
}

// GetClientOptions returns all the client specific options required for UNCaGED
func (ku *KoboUncaged) GetClientOptions() uc.ClientOptions {
	opts := uc.ClientOptions{}
	return opts
}

// GetDeviceBookList returns a slice of all the books currently on the device
// A nil slice is interpreted has having no books on the device
func (ku *KoboUncaged) GetDeviceBookList() []uc.BookCountDetails {
	bc := []uc.BookCountDetails{}
	return bc
}

// GetMetadataList sends complete metadata for the books listed in lpaths, or for
// all books on device if lpaths is empty
func (ku *KoboUncaged) GetMetadataList(books []uc.BookID) []map[string]interface{} {
	mdList := []map[string]interface{}{}
	return mdList
}

// GetDeviceInfo asks the client for information about the drive info to use
func (ku *KoboUncaged) GetDeviceInfo() uc.DeviceInfo {
	devInfo := uc.DeviceInfo{}
	return devInfo
}

// SetDeviceInfo sets the new device info, as comes from calibre. Only the nested
// struct DevInfo is modified.
func (ku *KoboUncaged) SetDeviceInfo(uc.DeviceInfo) {}

// UpdateMetadata instructs the client to update their metadata according to the
// new slice of metadata maps
func (ku *KoboUncaged) UpdateMetadata(mdList []map[string]interface{}) {}

// GetPassword gets a password from the user.
func (ku *KoboUncaged) GetPassword() string {
	return ""
}

// GetFreeSpace reports the amount of free storage space to Calibre
func (ku *KoboUncaged) GetFreeSpace() uint64 {
	return 1024 * 1024 * 1024
}

// SaveBook saves a book with the provided metadata to the disk.
// Implementations return an io.WriteCloser for UNCaGED to write the ebook to
// lastBook informs the client that this is the last book for this transfer
func (ku *KoboUncaged) SaveBook(md map[string]interface{}, lastBook bool) (io.WriteCloser, error) {
	return nil, nil
}

// GetBook provides an io.ReadCloser, and the file len, from which UNCaGED can send the requested book to Calibre
// NOTE: filePos > 0 is not currently implemented in the Calibre source code, but that could
// change at any time, so best to handle it anyway.
func (ku *KoboUncaged) GetBook(book uc.BookID, filePos int64) (io.ReadCloser, int64, error) {
	return nil, 0, nil
}

// DeleteBook instructs the client to delete the specified book on the device
// Error is returned if the book was unable to be deleted
func (ku *KoboUncaged) DeleteBook(book uc.BookID) error {
	return nil
}

// Println is used to print messages to the users display. Usage is identical to
// that of fmt.Println()
func (ku *KoboUncaged) Println(a ...interface{}) (n int, err error) {
	return ku.fbI.Println(a...)
}

// DisplayProgress Instructs the client to display the current progress to the user.
// percentage will be an integer between 0 and 100 inclusive
func (ku *KoboUncaged) DisplayProgress(percentage int) {
	ku.fbI.PrintProgressBar(uint8(percentage), ku.fbCfg)
}

func main() {
	fbiOpts := gofbink.FBInkConfig{
		Row: 2,
	}
	fbiRopts := gofbink.RestrictedConfig{
		Fontmult:   3,
		Fontname:   gofbink.IBM,
		IsCentered: false,
		NoViewport: true,
	}
	fbi := gofbink.New(&fbiOpts, &fbiRopts)
	fbi.Open()
	fbi.Init(&fbiOpts)
	defer fbi.Close()
	usbms, err := simusb.New(fbi)
	if err != nil {
		return
	}
	err = usbms.Start(true, true)
	defer usbms.End(true)

}
