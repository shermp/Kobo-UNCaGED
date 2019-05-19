package main

import (
	"path/filepath"
	"strings"
)

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
