// Copyright 2019-2020 Sherman Perry

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

	"github.com/shermp/Kobo-UNCaGED/kobo-uncaged/device"
	"github.com/shermp/Kobo-UNCaGED/kobo-uncaged/util"
	"github.com/shermp/UNCaGED/uc"
)

type koboUncaged struct {
	k *device.Kobo
}

// New initialises the koboUncaged object that will be passed to UNCaGED
func New(kobo *device.Kobo) *koboUncaged {
	return &koboUncaged{kobo}
}

func (ku *koboUncaged) SelectCalibreInstance(calInstances []uc.CalInstance) uc.CalInstance {
	return calInstances[0]
}

// GetClientOptions returns all the client specific options required for UNCaGED
func (ku *koboUncaged) GetClientOptions() (uc.ClientOptions, error) {
	var opts uc.ClientOptions
	opts.ClientName = "Kobo UNCaGED " // + kuVersion
	ext, devModel, thumbSz := ku.k.GetDeviceOptions()
	opts.DeviceModel = devModel
	opts.SupportedExt = append(opts.SupportedExt, ext...)
	opts.DeviceName = "Kobo"
	opts.CoverDims.Width, opts.CoverDims.Height = thumbSz.X, thumbSz.Y
	return opts, nil
}

// GetDeviceBookList returns a slice of all the books currently on the device
// A nil slice is interpreted has having no books on the device
func (ku *koboUncaged) GetDeviceBookList() ([]uc.BookCountDetails, error) {
	bc := []uc.BookCountDetails{}
	for k, md := range ku.k.MetadataMap {
		fmt.Println(k)
		lastMod := time.Now()
		if md.LastModified.GetTime() != nil {
			lastMod = *md.LastModified.GetTime()
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
	return bc, nil
}

func (ku *koboUncaged) GetMetadataIter(books []uc.BookID) uc.MetadataIter {
	iter := device.NewMetaIter(ku.k)
	if len(books) > 0 {
		for _, bk := range books {
			cid := util.LpathToContentID(bk.Lpath, string(ku.k.ContentIDprefix))
			iter.Add(cid)
		}
	} else {
		for cid := range ku.k.MetadataMap {
			iter.Add(cid)
		}
	}
	return iter
}

// GetDeviceInfo asks the client for information about the drive info to use
func (ku *koboUncaged) GetDeviceInfo() (uc.DeviceInfo, error) {
	return ku.k.DriveInfo, nil
}

// SetDeviceInfo sets the new device info, as comes from calibre. Only the nested
// struct DevInfo is modified.
func (ku *koboUncaged) SetDeviceInfo(devInfo uc.DeviceInfo) error {
	ku.k.DriveInfo = devInfo
	ku.k.SaveDeviceInfo()
	return nil
}

// UpdateMetadata instructs the client to update their metadata according to the
// new slice of metadata maps
func (ku *koboUncaged) UpdateMetadata(mdList []uc.CalibreBookMeta) error {
	for _, md := range mdList {
		md.Thumbnail = nil
		cid := util.LpathToContentID(md.Lpath, string(ku.k.ContentIDprefix))
		ku.k.MetadataMap[cid] = md
		ku.k.UpdatedMetadata[cid] = struct{}{}
	}
	ku.k.WriteMDfile()
	return nil
}

// GetPassword gets a password from the user.
func (ku *koboUncaged) GetPassword(calibreInfo uc.CalibreInitInfo) (string, error) {
	return ku.k.Passwords.NextPassword(), nil
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

// CheckLpath asks the client to verify a provided Lpath, and change it if required
// Return the original string if the Lpath does not need changing
func (ku *koboUncaged) CheckLpath(lpath string) (newLpath string) {
	// The calibre wireless driver does not sanitize the filepath for us. We sanitize it here,
	// and if lpath changes, inform Calibre of the new lpath.
	newLpath = util.SanitizeFilepath(lpath)
	// Also, for kepub files, Calibre defaults to using "book/path.kepub"
	// but we require "book/path.kepub.epub". We change that here if needed.
	newLpath = util.LpathKepubConvert(newLpath)
	return newLpath
}

// SaveBook saves a book with the provided metadata to the disk.
// Implementations return an io.WriteCloser (book) for UNCaGED to write the ebook to
// lastBook informs the client that this is the last book for this transfer
// newLpath informs UNCaGED of an Lpath change. Use this if the lpath field in md is
// not valid (eg filesystem limitations.). Return an empty string if original lpath is valid
func (ku *koboUncaged) SaveBook(md uc.CalibreBookMeta, book io.Reader, len int, lastBook bool) (err error) {
	cID := util.LpathToContentID(md.Lpath, string(ku.k.ContentIDprefix))
	bkPath := util.ContentIDtoBkPath(ku.k.BKRootDir, cID, string(ku.k.ContentIDprefix))
	bkDir, _ := filepath.Split(bkPath)
	err = os.MkdirAll(bkDir, 0777)
	if err != nil {
		return fmt.Errorf("SaveBook: error making book directories: %w", err)
	}
	destBook, err := os.OpenFile(bkPath, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("SaveBook: error opening ebook file: %w", err)
	}
	defer destBook.Close()
	ku.k.UpdatedMetadata[cID] = struct{}{}
	// Note, the JSON format for covers should be in the form 'thumbnail: [w, h, "base64string"]'
	if md.Thumbnail.Exists() {
		w, h := md.Thumbnail.Dimensions()
		ku.k.Wg.Add(1)
		go ku.k.SaveCoverImage(cID, image.Pt(w, h), md.Thumbnail.ImgBase64())
	}
	if _, err = io.CopyN(destBook, book, int64(len)); err != nil {
		return fmt.Errorf("SaveBook: error writing ebook to file: %w", err)
	}
	ku.k.UpdateIfExists(cID, len)
	ku.k.MetadataMap[cID] = md
	if lastBook {
		ku.k.WriteMDfile()
	}
	return err
}

// GetBook provides an io.ReadCloser, and the file len, from which UNCaGED can send the requested book to Calibre
// NOTE: filePos > 0 is not currently implemented in the Calibre source code, but that could
// change at any time, so best to handle it anyway.
func (ku *koboUncaged) GetBook(book uc.BookID, filePos int64) (io.ReadCloser, int64, error) {
	cid := util.LpathToContentID(book.Lpath, string(ku.k.ContentIDprefix))
	bkPath := util.ContentIDtoBkPath(ku.k.BKRootDir, cid, string(ku.k.ContentIDprefix))
	fi, err := os.Stat(bkPath)
	if err != nil {
		return nil, 0, fmt.Errorf("GetBook: error getting book stats: %w", err)
	}
	bookLen := fi.Size()
	ebook, err := os.OpenFile(bkPath, os.O_RDONLY, 0644)
	if err != nil {
		err = fmt.Errorf("GetBook: error opening book file: %w", err)
	}
	return ebook, bookLen, err
}

// DeleteBook instructs the client to delete the specified book on the device
// Error is returned if the book was unable to be deleted
func (ku *koboUncaged) DeleteBook(book uc.BookID) error {
	var err error
	// Start with basic book deletion. A more fancy implementation can come later
	// (eg: removing cover image remnants etc)
	cid := util.LpathToContentID(book.Lpath, string(ku.k.ContentIDprefix))
	bkPath := util.ContentIDtoBkPath(ku.k.BKRootDir, cid, string(ku.k.ContentIDprefix))
	dir, _ := filepath.Split(bkPath)
	dirPath := filepath.Clean(dir)
	if ku.k.KuConfig.EnableDebug {
		log.Printf("[DEBUG] CID: %s, bkPath: %s, dir: %s, dirPath: %s\n", cid, bkPath, dir, dirPath)
	}
	if err = os.Remove(bkPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("DeleteBook: error deleting file: %w", err)
	}
	for dirPath != filepath.Clean(ku.k.BKRootDir) {
		// Note, os.Remove only removes empty directories, so it should be safe to call
		if err = os.Remove(dirPath); err != nil {
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
	delete(ku.k.UpdatedMetadata, cid)
	// Finally, write the new metadata files
	if err = ku.k.WriteMDfile(); err != nil {
		return fmt.Errorf("DeleteBook: error writing metadata file: %w", err)
	}
	return nil
}

// UpdateStatus gives status updates from the UNCaGED library
func (ku *koboUncaged) UpdateStatus(status uc.Status, progress int) {
	p := -1
	if progress >= 0 && progress <= 100 {
		p = progress
	}
	switch status {
	case uc.Idle:
		ku.k.MsgChan <- device.WebMsg{Progress: p}
	case uc.Connected:
		ku.k.MsgChan <- device.WebMsg{Body: "Connected", Progress: p}
		// kuprint.Println(kuprint.Body, "Connected")
		// kuprint.Println(kuprint.Footer, footerStr)
	case uc.Connecting:
		ku.k.MsgChan <- device.WebMsg{Body: "Connecting to Calibre", Progress: p}
		// kuprint.Println(kuprint.Body, "Connecting to Calibre")
		// kuprint.Println(kuprint.Footer, footerStr)
	case uc.SearchingCalibre:
		ku.k.MsgChan <- device.WebMsg{Body: "Searching for Calibre", Progress: p}
		// kuprint.Println(kuprint.Body, "Searching for Calibre")
		// kuprint.Println(kuprint.Footer, footerStr)
	case uc.Disconnected:
		ku.k.MsgChan <- device.WebMsg{Body: "Disconnected", Progress: p}
		// kuprint.Println(kuprint.Body, "Disconnected")
		// kuprint.Println(kuprint.Footer, footerStr)
	case uc.SendingBook:
		ku.k.MsgChan <- device.WebMsg{Body: "Sending book to Calibre", Progress: p}
		// kuprint.Println(kuprint.Body, "Sending book to Calibre")
		// kuprint.Println(kuprint.Footer, footerStr)
	case uc.ReceivingBook:
		ku.k.MsgChan <- device.WebMsg{Body: "Receiving book(s) from Calibre", Progress: p}
		// kuprint.Println(kuprint.Body, "Receiving book(s) from Calibre")
		// kuprint.Println(kuprint.Footer, footerStr)
	case uc.EmptyPasswordReceived:
		ku.k.MsgChan <- device.WebMsg{Body: "No valid password found!", Progress: p}
		// kuprint.Println(kuprint.Body, "No valid password found!")
		// kuprint.Println(kuprint.Footer, footerStr)
	}
}

// LogPrintf instructs the client to log informational and debug info, that aren't errors
func (ku *koboUncaged) LogPrintf(logLevel uc.LogLevel, format string, a ...interface{}) {
	log.Printf(format, a...)
}
