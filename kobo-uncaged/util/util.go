package util

import (
	"encoding/json"
	"fmt"
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

// StringSliceContains test if val is contained in strSlice
func StringSliceContains(strSlice []string, val string) bool {
	for _, s := range strSlice {
		if s == val {
			return true
		}
	}
	return false
}

// SafeSQLString constructs a safe string to feed to SQLite3 CLI
// Queries made in Go should use prepared statements/parameters
// instead. This is also not safe for LIKE queries
func SafeSQLString(s *string) string {
	if s != nil {
		return fmt.Sprintf("'%s'", strings.ReplaceAll(*s, "'", "''"))
	}
	return "NULL"
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
	f, err := GetFileRead(fn)
	if err != nil {
		return false, fmt.Errorf("ReadJSON Open: %w", err)
	} else if f == nil {
		return true, nil
	}
	defer f.Close()
	if err = json.NewDecoder(f).Decode(out); err != nil {
		err = fmt.Errorf("ReadJSON Decode: %w", err)
	}
	return false, err
}

// GetFileRead opens fn in read only mode. If the returned file
// and err are both nil, the file is empty or does not exist.
// If err is not nil, a different error occurred.
func GetFileRead(fn string) (f *os.File, err error) {
	f, err = os.Open(fn)
	if os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("GetFile Open: %w", err)
	}
	fi, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("GetFile Stat: %w", err)
	} else if fi.Size() == 0 {
		f.Close()
		return nil, nil
	}
	return f, nil
}
