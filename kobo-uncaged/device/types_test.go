package device

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestKoboCoverString(t *testing.T) {
	for _, cover := range []koboCover{fullCover, libFull, libGrid} {
		if cover.String() == "" {
			t.Errorf("expected non-empty string for %#v", cover)
		}
		// it will also fail if it panics due to an unknown cover.
	}
}

func TestKoboCoverSize(t *testing.T) {
	for _, cover := range []koboCover{fullCover, libFull, libGrid} {
		if sz := cover.Size(touchAB); sz.X == 0 || sz.Y == 0 {
			t.Errorf("expected non-zero size for %#v, got %s", cover, sz)
		}
		// it will also fail if it panics due to an unknown cover.
	}
}

func TestKoboCoverRelPath(t *testing.T) {
	for _, tc := range []struct {
		cover    koboCover
		id, path string
	}{
		{fullCover, "file____mnt_onboard_perftesting_book0_kepub_epub", "50/74/file____mnt_onboard_perftesting_book0_kepub_epub - N3_FULL.parsed"},
		{libFull, "file____mnt_onboard_perftesting_book0_kepub_epub", "50/74/file____mnt_onboard_perftesting_book0_kepub_epub - N3_LIBRARY_FULL.parsed"},
		{libGrid, "file____mnt_onboard_perftesting_book0_kepub_epub", "50/74/file____mnt_onboard_perftesting_book0_kepub_epub - N3_LIBRARY_GRID.parsed"},
		// note: the actual sharding is tested in util_test.go
	} {
		rp := filepath.ToSlash(tc.cover.RelPath(tc.id))
		if !strings.HasSuffix(rp, ".parsed") {
			t.Errorf("cover must end in .parsed, got %#v", rp)
		}
		if len(strings.Split(rp, "/")) != 3 {
			t.Errorf("relpath should have 3 parts, got %#v", rp)
		}
		if rp != filepath.ToSlash(tc.path) {
			t.Errorf("expected cover for %#v to be %#v, got %#v", tc.id, tc.path, rp)
		}
	}
}
