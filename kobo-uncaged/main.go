package main

import (
	"fmt"
	"io"

	"github.com/shermp/UNCaGED/uc"
)

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

// UncagedKobo contains the variables and methods required to use
// the UNCaGED library
type UncagedKobo struct {
}

// GetClientOptions returns all the client specific options required for UNCaGED
func (uk *UncagedKobo) GetClientOptions() uc.ClientOptions {
	opts := uc.ClientOptions{}
	return opts
}

// GetDeviceBookList returns a slice of all the books currently on the device
// A nil slice is interpreted has having no books on the device
func (uk *UncagedKobo) GetDeviceBookList() []uc.BookCountDetails {
	bc := []uc.BookCountDetails{}
	return bc
}

// GetMetadataList sends complete metadata for the books listed in lpaths, or for
// all books on device if lpaths is empty
func (uk *UncagedKobo) GetMetadataList(books []uc.BookID) []map[string]interface{} {
	mdList := []map[string]interface{}{}
	return mdList
}

// GetDeviceInfo asks the client for information about the drive info to use
func (uk *UncagedKobo) GetDeviceInfo() uc.DeviceInfo {
	devInfo := uc.DeviceInfo{}
	return devInfo
}

// SetDeviceInfo sets the new device info, as comes from calibre. Only the nested
// struct DevInfo is modified.
func (uk *UncagedKobo) SetDeviceInfo(uc.DeviceInfo) {}

// UpdateMetadata instructs the client to update their metadata according to the
// new slice of metadata maps
func (uk *UncagedKobo) UpdateMetadata(mdList []map[string]interface{}) {}

// GetPassword gets a password from the user.
func (uk *UncagedKobo) GetPassword() string {
	return ""
}

// GetFreeSpace reports the amount of free storage space to Calibre
func (uk *UncagedKobo) GetFreeSpace() uint64 {
	return 1024 * 1024 * 1024
}

// SaveBook saves a book with the provided metadata to the disk.
// Implementations return an io.WriteCloser for UNCaGED to write the ebook to
// lastBook informs the client that this is the last book for this transfer
func (uk *UncagedKobo) SaveBook(md map[string]interface{}, lastBook bool) (io.WriteCloser, error) {
	return nil, nil
}

// GetBook provides an io.ReadCloser, and the file len, from which UNCaGED can send the requested book to Calibre
// NOTE: filePos > 0 is not currently implemented in the Calibre source code, but that could
// change at any time, so best to handle it anyway.
func (uk *UncagedKobo) GetBook(book uc.BookID, filePos int64) (io.ReadCloser, int64, error) {
	return nil, 0, nil
}

// DeleteBook instructs the client to delete the specified book on the device
// Error is returned if the book was unable to be deleted
func (uk *UncagedKobo) DeleteBook(book uc.BookID) error {
	return nil
}

// Println is used to print messages to the users display. Usage is identical to
// that of fmt.Println()
func (uk *UncagedKobo) Println(a ...interface{}) (n int, err error) {
	return 0, nil
}

// DisplayProgress Instructs the client to display the current progress to the user.
// percentage will be an integer between 0 and 100 inclusive
func (uk *UncagedKobo) DisplayProgress(percentage int) {}

func main() {

}
