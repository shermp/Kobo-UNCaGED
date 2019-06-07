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

package kunc

import (
	"fmt"
	"image"
	"io"
	"log"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/mitchellh/mapstructure"
	"github.com/shermp/Kobo-UNCaGED/kobo-uncaged/device"
	"github.com/shermp/Kobo-UNCaGED/kobo-uncaged/kuprint"
	"github.com/shermp/Kobo-UNCaGED/kobo-uncaged/util"
	"github.com/shermp/UNCaGED/uc"
)

type koboUncaged struct {
	k *device.Kobo
}

// New initialises the koboUncaged object that will be passed to UNCaGED
func New(kobo *device.Kobo) *koboUncaged {
	ku := &koboUncaged{}
	ku.k = kobo
	return ku
}

// GetClientOptions returns all the client specific options required for UNCaGED
func (ku *koboUncaged) GetClientOptions() uc.ClientOptions {
	opts := uc.ClientOptions{}
	opts.ClientName = "Kobo UNCaGED " // + kuVersion
	var ext []string
	var thumbSz image.Point
	ext, opts.DeviceModel, thumbSz = ku.k.GetDeviceOptions()
	opts.SupportedExt = append(opts.SupportedExt, ext...)
	opts.DeviceName = "Kobo"
	opts.CoverDims.Width, opts.CoverDims.Height = thumbSz.X, thumbSz.Y
	return opts
}

// GetDeviceBookList returns a slice of all the books currently on the device
// A nil slice is interpreted has having no books on the device
func (ku *koboUncaged) GetDeviceBookList() []uc.BookCountDetails {
	bc := []uc.BookCountDetails{}
	for _, md := range ku.k.MetadataMap {
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
func (ku *koboUncaged) GetMetadataList(books []uc.BookID) []map[string]interface{} {
	//spew.Dump(ku.k.MetadataMap)
	//spew.Dump(books)
	mdList := []map[string]interface{}{}
	if len(books) > 0 {
		for _, bk := range books {
			cid := util.LpathToContentID(bk.Lpath, string(ku.k.ContentIDprefix))
			fmt.Println(cid)
			md := map[string]interface{}{}
			//spew.Dump(ku.k.MetadataMap[cid])
			mapstructure.Decode(ku.k.MetadataMap[cid], &md)
			mdList = append(mdList, md)
		}
	} else {
		for _, kmd := range ku.k.MetadataMap {
			md := map[string]interface{}{}
			//spew.Dump(kmd)
			mapstructure.Decode(kmd, &md)
			mdList = append(mdList, md)
		}
	}
	return mdList
}

// GetDeviceInfo asks the client for information about the drive info to use
func (ku *koboUncaged) GetDeviceInfo() uc.DeviceInfo {
	return ku.k.DriveInfo
}

// SetDeviceInfo sets the new device info, as comes from calibre. Only the nested
// struct DevInfo is modified.
func (ku *koboUncaged) SetDeviceInfo(devInfo uc.DeviceInfo) {
	ku.k.DriveInfo = devInfo
	ku.k.SaveDeviceInfo()
}

// UpdateMetadata instructs the client to update their metadata according to the
// new slice of metadata maps
func (ku *koboUncaged) UpdateMetadata(mdList []map[string]interface{}) {
	for _, md := range mdList {
		koboMD := device.CreateKoboMetadata()
		mapstructure.Decode(md, &koboMD)
		koboMD.Thumbnail = nil
		cid := util.LpathToContentID(koboMD.Lpath, string(ku.k.ContentIDprefix))
		ku.k.MetadataMap[cid] = koboMD
		ku.k.UpdatedMetadata = append(ku.k.UpdatedMetadata, cid)
	}
	ku.k.WriteMDfile()
	ku.k.WriteUpdateMDfile()
}

// GetPassword gets a password from the user.
func (ku *koboUncaged) GetPassword(calibreInfo uc.CalibreInitInfo) string {
	return ku.k.Passwords.NextPassword()
}

// GetFreeSpace reports the amount of free storage space to Calibre
func (ku *koboUncaged) GetFreeSpace() uint64 {
	// Note, this method of getting available disk space is Linux specific...
	// Don't try to run this code on Windows. It will probably fall over
	var fs syscall.Statfs_t
	err := syscall.Statfs(ku.k.BKRootDir, &fs)
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
func (ku *koboUncaged) SaveBook(md map[string]interface{}, len int, lastBook bool) (book io.WriteCloser, newLpath string, err error) {
	koboMD := device.CreateKoboMetadata()
	mapstructure.Decode(md, &koboMD)
	// The calibre wireless driver does not sanitize the filepath for us. We sanitize it here,
	// and if lpath changes, inform Calibre of the new lpath.
	newLpath = ku.k.InvalidCharsRegex.ReplaceAllString(koboMD.Lpath, "_")
	// Also, for kepub files, Calibre defaults to using "book/path.kepub"
	// but we require "book/path.kepub.epub". We change that here if needed.
	newLpath = util.LpathKepubConvert(newLpath)
	if newLpath != koboMD.Lpath {
		koboMD.Lpath = newLpath
	} else {
		newLpath = ""
	}
	cID := util.LpathToContentID(koboMD.Lpath, string(ku.k.ContentIDprefix))
	bkPath := util.ContentIDtoBkPath(ku.k.BKRootDir, cID, string(ku.k.ContentIDprefix))
	bkDir, _ := filepath.Split(bkPath)
	err = os.MkdirAll(bkDir, 0777)
	if err != nil {
		return nil, "", err
	}
	book, err = os.OpenFile(bkPath, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return nil, "", err
	}
	ku.k.UpdatedMetadata = append(ku.k.UpdatedMetadata, cID)
	// Note, the JSON format for covers should be in the form 'thumbnail: [w, h, "base64string"]'
	if kt := koboMD.Thumbnail; kt != nil {
		ku.k.Wg.Add(1)
		go ku.k.SaveCoverImage(cID, image.Pt(int(kt[0].(float64)), int(kt[1].(float64))), kt[2].(string))
	}
	err = ku.k.UpdateIfExists(cID, len)
	if err != nil {
		log.Print(err)
	}
	ku.k.MetadataMap[cID] = koboMD
	if lastBook {
		ku.k.WriteMDfile()
		ku.k.WriteUpdateMDfile()
	}
	return book, newLpath, nil
}

// GetBook provides an io.ReadCloser, and the file len, from which UNCaGED can send the requested book to Calibre
// NOTE: filePos > 0 is not currently implemented in the Calibre source code, but that could
// change at any time, so best to handle it anyway.
func (ku *koboUncaged) GetBook(book uc.BookID, filePos int64) (io.ReadCloser, int64, error) {
	cid := util.LpathToContentID(book.Lpath, string(ku.k.ContentIDprefix))
	bkPath := util.ContentIDtoBkPath(ku.k.BKRootDir, cid, string(ku.k.ContentIDprefix))
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
func (ku *koboUncaged) DeleteBook(book uc.BookID) error {
	// Start with basic book deletion. A more fancy implementation can come later
	// (eg: removing cover image remnants etc)
	cid := util.LpathToContentID(book.Lpath, string(ku.k.ContentIDprefix))
	bkPath := util.ContentIDtoBkPath(ku.k.BKRootDir, cid, string(ku.k.ContentIDprefix))
	dir, _ := filepath.Split(bkPath)
	dirPath := filepath.Clean(dir)
	err := os.Remove(bkPath)
	if err != nil {
		log.Print(err)
		return err
	}
	for dirPath != filepath.Clean(ku.k.BKRootDir) {
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
	delete(ku.k.MetadataMap, cid)
	// As well as the updated metadata list, if it was added to the list this session
	l := len(ku.k.UpdatedMetadata)
	for n := 0; n < l; n++ {
		if ku.k.UpdatedMetadata[n] == cid {
			ku.k.UpdatedMetadata[n] = ku.k.UpdatedMetadata[len(ku.k.UpdatedMetadata)-1]
			ku.k.UpdatedMetadata = ku.k.UpdatedMetadata[:len(ku.k.UpdatedMetadata)-1]
			break
		}
	}
	// Finally, write the new metadata files
	ku.k.WriteMDfile()
	ku.k.WriteUpdateMDfile()
	return nil
}

// UpdateStatus gives status updates from the UNCaGED library
func (ku *koboUncaged) UpdateStatus(status uc.UCStatus, progress int) {
	footerStr := " "
	if progress >= 0 && progress <= 100 {
		footerStr = fmt.Sprintf("%d%%", progress)
	}
	switch status {
	case uc.Idle:
		fallthrough
	case uc.Connected:
		kuprint.Println(kuprint.Body, "Connected")
		kuprint.Println(kuprint.Footer, footerStr)
	case uc.Connecting:
		kuprint.Println(kuprint.Body, "Connecting to Calibre")
		kuprint.Println(kuprint.Footer, footerStr)
	case uc.SearchingCalibre:
		kuprint.Println(kuprint.Body, "Searching for Calibre")
		kuprint.Println(kuprint.Footer, footerStr)
	case uc.Disconnected:
		kuprint.Println(kuprint.Body, "Disconnected")
		kuprint.Println(kuprint.Footer, footerStr)
	case uc.SendingBook:
		kuprint.Println(kuprint.Body, "Sending book to Calibre")
		kuprint.Println(kuprint.Footer, footerStr)
	case uc.ReceivingBook:
		kuprint.Println(kuprint.Body, "Receiving book(s) from Calibre")
		kuprint.Println(kuprint.Footer, footerStr)
	case uc.EmptyPasswordReceived:
		kuprint.Println(kuprint.Body, "No valid password found!")
		kuprint.Println(kuprint.Footer, footerStr)
	}
}

// LogPrintf instructs the client to log informational and debug info, that aren't errors
func (ku *koboUncaged) LogPrintf(logLevel uc.UCLogLevel, format string, a ...interface{}) {
	log.Printf(format, a...)
}
