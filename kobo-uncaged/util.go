package main

import (
	"encoding/json"
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
