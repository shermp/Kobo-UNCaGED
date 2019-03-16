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
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/disintegration/imaging"
	"github.com/gofrs/uuid"
	"github.com/kapmahc/epub"
	_ "github.com/mattn/go-sqlite3"
	"github.com/mitchellh/mapstructure"
	"github.com/shermp/UNCaGED/uc"
)

type returnCode int

const kuVersion = "v0.1.0alpha"

const (
	kuError           returnCode = -1
	kuSuccessNoAction returnCode = 0
	kuSuccessRerun    returnCode = 1
)

const koboDBpath = ".kobo/KoboReader.sqlite"
const koboVersPath = ".kobo/version"
const calibreMDfile = "metadata.calibre"
const calibreDIfile = "driveinfo.calibre"
const kuUpdatedMDfile = "metadata_update.kobouc"

const onboardPrefix cidPrefix = "file:///mnt/onboard/"
const sdPrefix cidPrefix = "file:///mnt/sd/"

// genImagePath generates the directory structure used by
// kobo to store the cover image files.
// It has been ported from the implementation in the KoboTouch
// driver in Calibre
func genImageDirPath(imageID string) string {
	imgID := []byte(imageID)
	h := uint32(0x00000000)
	for _, x := range imgID {
		h = (h << 4) + uint32(x)
		h ^= (h & 0xf0000000) >> 23
		h &= 0x0fffffff
	}
	dir1 := h & (0xff * 1)
	dir2 := (h & (0xff00 * 1)) >> 8
	return fmt.Sprintf(".kobo-images/%d/%d", dir1, dir2)
}

// imgIDFromContentID generates an imageID from a contentID, using the
// the replacement values as found in the Calibre Kobo driver
func imgIDFromContentID(contentID string) string {
	r := strings.NewReplacer("/", "_", " ", "_", ":", "_", ".", "_")
	return r.Replace(contentID)
}

// KoboUncaged contains the variables and methods required to use
// the UNCaGED library
type KoboUncaged struct {
	kup      kuPrinter
	koboInfo struct {
		model        koboDeviceID
		modelName    string
		fw           [3]int
		coverDetails map[koboCoverEnding]coverDims
	}
	dbRootDir       string
	bkRootDir       string
	contentIDprefix cidPrefix
	metadataMap     map[string]KoboMetadata
	updatedMetadata map[string]KoboMetadata
	driveInfo       uc.DeviceInfo
	nickelDB        *sql.DB
	wg              *sync.WaitGroup
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
func New(dbRootDir, bkRootDir string, contentIDprefix cidPrefix) (*KoboUncaged, error) {
	var err error
	ku := &KoboUncaged{}
	ku.kup, err = newKuPrint()
	if err != nil {
		return nil, err
	}
	ku.wg = &sync.WaitGroup{}
	ku.dbRootDir = dbRootDir
	ku.bkRootDir = bkRootDir
	ku.contentIDprefix = contentIDprefix
	ku.koboInfo.coverDetails = make(map[koboCoverEnding]coverDims)
	ku.metadataMap = make(map[string]KoboMetadata)
	ku.updatedMetadata = make(map[string]KoboMetadata)
	fmt.Println("Opening NickelDB")
	err = ku.openNickelDB()
	if err != nil {
		return nil, err
	}
	fmt.Println("Getting Kobo Info")
	err = ku.getKoboInfo()
	if err != nil {
		return nil, err
	}
	fmt.Println("Getting Device Info")
	err = ku.loadDeviceInfo()
	if err != nil {
		return nil, err
	}
	fmt.Println("Reading Metadata")
	err = ku.readMDfile()
	if err != nil {
		return nil, err
	}
	return ku, nil
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
			ku.koboInfo.fw[i], _ = strconv.Atoi(f)
		}
		ku.koboInfo.model = koboDeviceID(versFields[len(versFields)-1])
	}
	// Once we have the model number, we set the appropriate cover image dims
	// These values come from https://github.com/kovidgoyal/calibre/blob/master/src/calibre/devices/kobo/driver.py
	switch ku.koboInfo.model {
	case glo, aura, auraEd2r1, auraEd2r2:
		ku.koboInfo.coverDetails[fullCover] = coverDims{width: 758, height: 1024}
		ku.koboInfo.coverDetails[libFull] = coverDims{width: 355, height: 479}
		ku.koboInfo.coverDetails[libGrid] = coverDims{width: 149, height: 201}
	case gloHD, claraHD:
		ku.koboInfo.coverDetails[fullCover] = coverDims{width: 1072, height: 1448}
		ku.koboInfo.coverDetails[libFull] = coverDims{width: 355, height: 479}
		ku.koboInfo.coverDetails[libGrid] = coverDims{width: 149, height: 201}
	case auraHD, auraH2O, auraH2Oed2r1, auraH2Oed2r2:
		ku.koboInfo.coverDetails[fullCover] = coverDims{width: 1080, height: 1440}
		ku.koboInfo.coverDetails[libFull] = coverDims{width: 355, height: 471}
		ku.koboInfo.coverDetails[libGrid] = coverDims{width: 149, height: 198}
	case auraOne, auraOneLE:
		ku.koboInfo.coverDetails[fullCover] = coverDims{width: 1404, height: 1872}
		ku.koboInfo.coverDetails[libFull] = coverDims{width: 355, height: 473}
		ku.koboInfo.coverDetails[libGrid] = coverDims{width: 149, height: 198}
	case forma:
		ku.koboInfo.coverDetails[fullCover] = coverDims{width: 1440, height: 1920}
		ku.koboInfo.coverDetails[libFull] = coverDims{width: 355, height: 473}
		ku.koboInfo.coverDetails[libGrid] = coverDims{width: 149, height: 198}
	default:
		ku.koboInfo.coverDetails[fullCover] = coverDims{width: 600, height: 800}
		ku.koboInfo.coverDetails[libFull] = coverDims{width: 355, height: 473}
		ku.koboInfo.coverDetails[libGrid] = coverDims{width: 149, height: 198}
	}

	// Populate model name
	switch ku.koboInfo.model {
	case touch2, touchAB, touchC:
		ku.koboInfo.modelName = "Touch"
	case mini:
		ku.koboInfo.modelName = "Mini"
	case glo:
		ku.koboInfo.modelName = "Glo"
	case gloHD:
		ku.koboInfo.modelName = "Glo HD"
	case aura:
		ku.koboInfo.modelName = "Aura"
	case auraH2O:
		ku.koboInfo.modelName = "Aura H2O"
	case auraH2Oed2r1, auraH2Oed2r2:
		ku.koboInfo.modelName = "Aura H2O Ed. 2"
	case auraEd2r1, auraEd2r2:
		ku.koboInfo.modelName = "Aura Ed. 2"
	case auraHD:
		ku.koboInfo.modelName = "Aura HD"
	case auraOne, auraOneLE:
		ku.koboInfo.modelName = "Aura One"
	case claraHD:
		ku.koboInfo.modelName = "Clara HD"
	case forma:
		ku.koboInfo.modelName = "Forma"
	default:
		ku.koboInfo.modelName = "Unknown Kobo"
	}
	return nil
}

// readEpubMeta opens an epub (or kepub), and attempts to read the
// metadata it contains. This is used if the metadata has not yet
// been cached
func (ku *KoboUncaged) readEpubMeta(contentID string) (KoboMetadata, error) {
	md := createKoboMetadata()
	lpath := ku.contentIDtoLpath(contentID)
	epubPath := ku.contentIDtoBkPath(contentID)
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
	return md, nil
}

// readMDfile loads cached metadata from the "metadata.calibre" JSON file
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
	//spew.Dump(ku.metadataMap)
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
	AND MimeType NOT LIKE 'image%' 
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
		fmt.Println(dbCID)
		if _, exists := ku.metadataMap[dbCID]; !exists {
			bkMD, err := ku.readEpubMeta(dbCID)
			if err != nil {
				return err
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

func (ku *KoboUncaged) writeUpdateMDfile() error {
	// We only write the file if there is new or updated metadata to write
	if len(ku.updatedMetadata) > 0 {
		updatedMeta := make([]KoboMetadata, len(ku.updatedMetadata))
		for _, md := range ku.updatedMetadata {
			updatedMeta = append(updatedMeta, md)
		}
		// Convert it to JSON, prettifying it in the process
		mdJSON, _ := json.MarshalIndent(updatedMeta, "", "    ")
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
			ku.driveInfo.DevInfo.DeviceName = "Kobo " + ku.koboInfo.modelName
			ku.driveInfo.DevInfo.DeviceStoreUUID = uuid4.String()
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
	thumbW := int(thumb[0].(float64))
	thumbH := int(thumb[1].(float64))
	koMaxW := ku.koboInfo.coverDetails[fullCover].width
	koMaxH := ku.koboInfo.coverDetails[fullCover].height
	imgB64 := thumb[2].(string)
	imgID := imgIDFromContentID(contentID)
	imgDir := path.Join(ku.bkRootDir, genImageDirPath(imgID))
	err := os.MkdirAll(imgDir, 0744)
	if err == nil {
		imgBin, err := base64.StdEncoding.DecodeString(imgB64)
		if err == nil {
			// No need to perform any image processing for the full cover if it meets all our requirements
			fullCoverSaved := false
			if (thumbW <= koMaxW && thumbH <= koMaxH) && (thumbW == koMaxW || thumbH == koMaxH) {
				fullCoverSaved = true
				err = ioutil.WriteFile(path.Join(imgDir, (imgID+string(fullCover))), imgBin, 0644)
				if err != nil {
					log.Println(err)
				}
			}
			// Now we do our resizing
			origCover, err := imaging.Decode(bytes.NewReader(imgBin))
			if err == nil {
				koAspectRatio := float64(koMaxW) / float64(koMaxH)
				coverAspectRatio := float64(thumbW) / float64(thumbH)
				var covFW, covFH, libFW, libFH, gridFW, gridFH int
				if coverAspectRatio < koAspectRatio {
					covFH = ku.koboInfo.coverDetails[fullCover].height
					libFH = ku.koboInfo.coverDetails[libFull].height
					gridFH = ku.koboInfo.coverDetails[libGrid].height
				} else {
					covFW = ku.koboInfo.coverDetails[fullCover].width
					libFW = ku.koboInfo.coverDetails[libFull].width
					gridFW = ku.koboInfo.coverDetails[libGrid].width
				}
				// We resize the full cover image if we haven't already saved it.
				if !fullCoverSaved {
					fullCovImg := imaging.Resize(origCover, covFW, covFH, imaging.Lanczos)
					fc, err := os.OpenFile(path.Join(imgDir, (imgID+string(fullCover))), os.O_WRONLY|os.O_CREATE, 0644)
					if err == nil {
						defer fc.Close()
						imaging.Encode(fc, fullCovImg, imaging.JPEG)
					} else {
						log.Println(err)
					}
				}
				// Followed by the "library fill" image
				libImg := imaging.Resize(origCover, libFW, libFH, imaging.Lanczos)
				lc, err := os.OpenFile(path.Join(imgDir, (imgID+string(libFull))), os.O_WRONLY|os.O_CREATE, 0644)
				if err == nil {
					defer lc.Close()
					imaging.Encode(lc, libImg, imaging.JPEG)
				} else {
					log.Println(err)
				}
				// And finally, the "library grid" image
				gridImg := imaging.Resize(origCover, gridFW, gridFH, imaging.Lanczos)
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
	ku.wg.Done()
}

// GetClientOptions returns all the client specific options required for UNCaGED
func (ku *KoboUncaged) GetClientOptions() uc.ClientOptions {
	opts := uc.ClientOptions{}
	opts.ClientName = "Kobo UNCaGED " + kuVersion
	ext := []string{"kepub", "epub"}
	opts.SupportedExt = append(opts.SupportedExt, ext...)
	opts.DeviceName = "Kobo"
	opts.DeviceModel = ku.koboInfo.modelName
	opts.CoverDims.Height = ku.koboInfo.coverDetails[fullCover].height
	opts.CoverDims.Width = ku.koboInfo.coverDetails[fullCover].width
	return opts
}

// GetDeviceBookList returns a slice of all the books currently on the device
// A nil slice is interpreted has having no books on the device
func (ku *KoboUncaged) GetDeviceBookList() []uc.BookCountDetails {
	bc := []uc.BookCountDetails{}
	for _, md := range ku.metadataMap {
		lastMod, _ := time.Parse(time.RFC3339, *md.LastModified)
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
		ku.updatedMetadata[cID] = koboMD
	}
	ku.writeMDfile()
	ku.writeUpdateMDfile()
}

// GetPassword gets a password from the user.
func (ku *KoboUncaged) GetPassword() string {
	// TODO obtain password for user. Using on-screen-keyboard? Config file?
	return ""
}

// GetFreeSpace reports the amount of free storage space to Calibre
func (ku *KoboUncaged) GetFreeSpace() uint64 {
	// TODO obtain the actual free space on device
	return 1024 * 1024 * 1024
}

// SaveBook saves a book with the provided metadata to the disk.
// Implementations return an io.WriteCloser for UNCaGED to write the ebook to
// lastBook informs the client that this is the last book for this transfer
func (ku *KoboUncaged) SaveBook(md map[string]interface{}, lastBook bool) (io.WriteCloser, error) {
	koboMD := createKoboMetadata()
	mapstructure.Decode(md, &koboMD)
	cID := ku.lpathToContentID(koboMD.Lpath)
	bkPath := ku.contentIDtoBkPath(cID)
	bkDir, _ := filepath.Split(bkPath)
	err := os.MkdirAll(bkDir, 0777)
	if err != nil {
		return nil, err
	}
	book, err := os.OpenFile(bkPath, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return nil, err
	}
	ku.metadataMap[cID] = koboMD
	ku.updatedMetadata[cID] = koboMD
	// Note, the JSON format for covers should be in the form 'thumbnail: [w, h, "base64string"]'
	if koboMD.Thumbnail != nil {
		ku.wg.Add(1)
		go ku.saveCoverImage(cID, koboMD.Thumbnail)
	}
	if lastBook {
		ku.writeMDfile()
		ku.writeUpdateMDfile()
	}
	return book, nil
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
	// TODO implement this
	return nil
}

// Println is used to print messages to the users display. Usage is identical to
// that of fmt.Println()
func (ku *KoboUncaged) Println(a ...interface{}) (n int, err error) {
	return ku.kup.kuPrintln(a...)
}

// DisplayProgress Instructs the client to display the current progress to the user.
// percentage will be an integer between 0 and 100 inclusive
func (ku *KoboUncaged) DisplayProgress(percentage int) {
	// TODO implement display progress
}

func mainWithErrCode() returnCode {
	onboardMntPtr := flag.String("onboardmount", "/mnt/onboard", "If changed, specify the new new mountpoint of '/mnt/onboard'")
	sdMntPtr := flag.String("sdmount", "/mnt/sd", "If changed, specify the new new mountpoint of '/mnt/sd'")
	contentLocPtr := flag.String("location", "onboard", "Choose location to save to. Choices are 'onboard' (default) and 'sd'.")
	mdPtr := flag.Bool("metadata", false, "Updates the Kobo DB with new metadata")

	flag.Parse()

	// Now we decide if we are merely printing, or connecting to Calibre
	if *mdPtr {
	} else {
		useOnboard := true
		bkRootDir := *onboardMntPtr
		dbRootDir := *onboardMntPtr
		cidPrefix := onboardPrefix
		if *contentLocPtr == "onboard" {
			useOnboard = true
		} else if *contentLocPtr == "sd" {
			useOnboard = false
		} else {
			log.Println("Unrecognized location string. Defaulting to 'onboard'")
			useOnboard = true
		}
		if !useOnboard {
			bkRootDir = *sdMntPtr
			cidPrefix = sdPrefix
		}
		ku, err := New(dbRootDir, bkRootDir, cidPrefix)
		if err != nil {
			log.Print(err)
			return kuError
		}
		defer ku.kup.kuClose()
		cc, err := uc.New(ku)
		if err != nil {
			log.Print(err)
			return kuError
		}
		err = cc.Start()
		ku.wg.Done()
		if err != nil {
			log.Print(err)
			return kuError
		}
		if len(ku.updatedMetadata) > 0 {
			return kuSuccessRerun
		}
	}

	return kuSuccessNoAction
}
func main() {
	os.Exit(int(mainWithErrCode()))
}
