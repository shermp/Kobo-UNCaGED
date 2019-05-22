package main

import (
	"encoding/json"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strings"
)

// imgIDFromContentID generates an imageID from a contentID, using the
// the replacement values as found in the Calibre Kobo driver
func imgIDFromContentID(contentID string) string {
	r := strings.NewReplacer("/", "_", " ", "_", ":", "_", ".", "_")
	return r.Replace(contentID)
}

func contentIDtoBkPath(rootDir, cid, cidPrefix string) string {
	return filepath.Join(rootDir, strings.TrimPrefix(cid, cidPrefix))
}

func lpathIsKepub(lpath string) bool {
	return strings.HasSuffix(lpath, ".kepub")
}

func lpathKepubConvert(lpath string) string {
	if lpathIsKepub(lpath) {
		lpath += ".epub"
	}
	return lpath
}

func lpathToContentID(lpath, cidPrefix string) string {
	return cidPrefix + strings.TrimPrefix(lpath, "/")
}

func contentIDtoLpath(cid, cidPrefix string) string {
	return strings.TrimPrefix(cid, cidPrefix)
}

func writeJSON(fn string, v interface{}) error {
	f, err := os.OpenFile(fn, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "    ")
	return enc.Encode(v)
}

func readJSON(fn string, out interface{}) (emptyOrNotExist bool, err error) {
	f, err := os.Open(fn)
	if os.IsNotExist(err) {
		return true, nil
	} else if err != nil {
		return false, err
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return false, err
	} else if fi.Size() == 0 {
		return true, nil
	}

	return false, json.NewDecoder(f).Decode(out)
}

// resizeKeepAspectRatio resizes a sz to fill bounds while keeping the aspect
// ratio. It is based on the code for QSize::scaled with the modes
// Qt::KeepAspectRatio and Qt::KeepAspectRatioByExpanding.
func resizeKeepAspectRatio(sz image.Point, bounds image.Point, expand bool) image.Point {
	if sz.X == 0 || sz.Y == 0 {
		return sz
	}

	var useHeight bool
	rw := bounds.Y * sz.X / sz.Y

	if !expand {
		useHeight = rw <= bounds.X
	} else {
		useHeight = rw >= bounds.X
	}

	if useHeight {
		return image.Pt(rw, bounds.Y)
	}
	return image.Pt(bounds.X, bounds.X*sz.Y/sz.X)
}

// hashedImageParts returns the parts needed for constructing the path to the
// cached image. The result can be applied like:
// .kobo-images/{dir1}/{dir2}/{basename} - N3_SOMETHING.jpg
func hashedImageParts(imageID string) (dir1, dir2, basename string) {
	imgID := []byte(imageID)
	h := uint32(0x00000000)
	for _, x := range imgID {
		h = (h << 4) + uint32(x)
		h ^= (h & 0xf0000000) >> 23
		h &= 0x0fffffff
	}
	return fmt.Sprintf("%d", h&(0xff*1)), fmt.Sprintf("%d", (h&(0xff00*1))>>8), imageID
}
