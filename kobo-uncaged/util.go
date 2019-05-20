package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
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

func wrapPos(err error) error {
	if err != nil {
		pc, file, line, ok := runtime.Caller(1)
		if !ok {
			return wrap(err, "[unknown pos]")
		}
		return wrap(err, "[0x%X %s:%d]", pc, file, line)
	}
	return nil
}

func wrap(err error, format string, a ...interface{}) error {
	if err != nil {
		return fmt.Errorf("%s: %v", fmt.Sprintf(format, a...), err)
	}
	return nil
}
