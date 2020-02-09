package util

import (
	"encoding/json"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var invalidCharsRegex = regexp.MustCompile(`[\\?%\*:;\|\"\'><\$!]`)

// SanitizeFilepath replaces all illegal characters for a fat32 filesystem
// with underscores
func SanitizeFilepath(filePath string) string {
	return invalidCharsRegex.ReplaceAllString(filePath, "_")
}

// ImgIDFromContentID generates an imageID from a contentID, using the
// the replacement values as found in the Calibre Kobo driver
func ImgIDFromContentID(contentID string) string {
	r := strings.NewReplacer("/", "_", " ", "_", ":", "_", ".", "_")
	return r.Replace(contentID)
}

// ContentIDtoBkPath converts the kobo content ID to a file path
func ContentIDtoBkPath(rootDir, cid, cidPrefix string) string {
	return filepath.Join(rootDir, strings.TrimPrefix(cid, cidPrefix))
}

// LpathIsKepub tests if the provided Lpath is a kepub file
func LpathIsKepub(lpath string) bool {
	return strings.HasSuffix(lpath, ".kepub")
}

// LpathKepubConvert converts a kepub lpath from calibre, to one
// we can use on a kobo
func LpathKepubConvert(lpath string) string {
	if LpathIsKepub(lpath) {
		lpath += ".epub"
	}
	return lpath
}

// LpathToContentID converts an lpath from Calibre to a Kobo content ID
func LpathToContentID(lpath, cidPrefix string) string {
	return cidPrefix + strings.TrimPrefix(lpath, "/")
}

// ContentIDtoLpath converts a Kobo content ID to calibre lpath
func ContentIDtoLpath(cid, cidPrefix string) string {
	return strings.TrimPrefix(cid, cidPrefix)
}

// WriteJSON is a helper function to write JSON to a file
func WriteJSON(fn string, v interface{}) error {
	var err error
	f, err := os.OpenFile(fn, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("WriteJSON OpenFile: %w", err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "    ")
	if err = enc.Encode(v); err != nil {
		err = fmt.Errorf("WriteJSON Encode: %w", err)
	}
	return err
}

// ReadJSON is a helper function to read JSON from a file
func ReadJSON(fn string, out interface{}) (emptyOrNotExist bool, err error) {
	f, err := os.Open(fn)
	if os.IsNotExist(err) {
		return true, nil
	} else if err != nil {
		return false, fmt.Errorf("ReadJSON Open: %w", err)
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return false, fmt.Errorf("ReadJSON Stat: %w", err)
	} else if fi.Size() == 0 {
		return true, nil
	}
	if err = json.NewDecoder(f).Decode(out); err != nil {
		err = fmt.Errorf("ReadJSON Decode: %w", err)
	}
	return false, err
}

// ResizeKeepAspectRatio resizes a sz to fill bounds while keeping the aspect
// ratio. It is based on the code for QSize::scaled with the modes
// Qt::KeepAspectRatio and Qt::KeepAspectRatioByExpanding.
func ResizeKeepAspectRatio(sz image.Point, bounds image.Point, expand bool) image.Point {
	if sz.X == 0 || sz.Y == 0 {
		return sz
	}

	var useHeight bool
	ar := float64(sz.X) / float64(sz.Y)
	rw := int(float64(bounds.Y) * ar)

	if !expand {
		useHeight = rw <= bounds.X
	} else {
		useHeight = rw >= bounds.X
	}

	if useHeight {
		return image.Pt(rw, bounds.Y)
	}
	return image.Pt(bounds.X, int(float64(bounds.X)/ar))
}

// HashedImageParts returns the parts needed for constructing the path to the
// cached image. The result can be applied like:
// .kobo-images/{dir1}/{dir2}/{basename} - N3_SOMETHING.jpg
func HashedImageParts(imageID string) (dir1, dir2, basename string) {
	imgID := []byte(imageID)
	h := uint32(0x00000000)
	for _, x := range imgID {
		h = (h << 4) + uint32(x)
		h ^= (h & 0xf0000000) >> 23
		h &= 0x0fffffff
	}
	return fmt.Sprintf("%d", h&(0xff*1)), fmt.Sprintf("%d", (h&(0xff00*1))>>8), imageID
}
