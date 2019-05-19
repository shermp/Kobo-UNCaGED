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

func contentIDisBkDir(cid, cidPrefix string) bool {
	return strings.HasPrefix(cid, cidPrefix)
}

func lpathIsKepub(lpath string) bool {
	return strings.HasSuffix(lpath, ".kepub")
}

func contentIDisKepub(contentID string) bool {
	return strings.HasSuffix(contentID, ".kepub.epub")
}

func lpathToContentID(lpath, cidPrefix string) string {
	if lpathIsKepub(lpath) {
		lpath += ".epub"
	}
	return cidPrefix + strings.TrimPrefix(lpath, "/")
}

func contentIDtoLpath(cid, cidPrefix string) string {
	if contentIDisKepub(cid) {
		cid = strings.TrimSuffix(cid, ".epub")
	}
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
