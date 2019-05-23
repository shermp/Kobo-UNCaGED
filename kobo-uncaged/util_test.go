package main

import (
	"image"
	"testing"
)

func TestImgIDFromContentID(t *testing.T) {
	for _, c := range []struct{ cid, iid string }{
		{"file:///mnt/onboard/.kobo/guide/userguide.pdf", "file____mnt_onboard__kobo_guide_userguide_pdf"},
		{"file:///mnt/onboard/a_ book: with some.charact-ers.pdf", "file____mnt_onboard_a__book__with_some_charact-ers_pdf"},
	} {
		if iid := imgIDFromContentID(c.cid); iid != c.iid {
			t.Errorf("expected iid for %#v to be %#v, got %#v", c.cid, c.iid, iid)
		}
	}
}

func TestResizeKeepAspectRatioByExpanding(t *testing.T) {
	for _, tc := range []struct{ sz, bounds, res image.Point }{
		// don't resize if width or height is zero
		{image.Pt(0, 0), image.Pt(0, 0), image.Pt(0, 0)},
		{image.Pt(1, 0), image.Pt(0, 0), image.Pt(1, 0)},
		{image.Pt(0, 1), image.Pt(0, 0), image.Pt(0, 1)},
		// same aspect ratio
		{image.Pt(1, 1), image.Pt(1, 1), image.Pt(1, 1)},
		{image.Pt(1, 1), image.Pt(5, 5), image.Pt(5, 5)},
		{image.Pt(5, 5), image.Pt(1, 1), image.Pt(1, 1)},
		// limited by width
		{image.Pt(2, 3), image.Pt(6, 6), image.Pt(6, 9)},
		{image.Pt(2, 4), image.Pt(6, 6), image.Pt(6, 12)},
		{image.Pt(6, 9), image.Pt(2, 3), image.Pt(2, 3)},
		{image.Pt(6, 12), image.Pt(2, 4), image.Pt(2, 4)},
		// limited by height
		{image.Pt(3, 2), image.Pt(6, 6), image.Pt(9, 6)},
		{image.Pt(4, 2), image.Pt(6, 6), image.Pt(12, 6)},
		{image.Pt(9, 6), image.Pt(3, 2), image.Pt(3, 2)},
		{image.Pt(12, 6), image.Pt(4, 2), image.Pt(4, 2)},
		// fractional stuff
		{image.Pt(1391, 2200), image.Pt(355, 530), image.Pt(355, 561)},
	} {
		if rz := resizeKeepAspectRatio(tc.sz, tc.bounds, true); !rz.Eq(tc.res) {
			t.Errorf("resize %s to %s: expected %s, got %s", tc.sz, tc.bounds, tc.res, rz)
		}
	}
}

func TestHashedImageParts(t *testing.T) {
	for _, tc := range []struct{ id, dir1, dir2 string }{
		{"file____mnt_onboard_perftesting_book0_kepub_epub", "50", "74"},
		{"file____mnt_onboard_perftesting_book40_kepub_epub", "114", "169"},
		{"file____mnt_onboard_perftesting_book80_kepub_epub", "82", "169"},
		{"file____mnt_onboard_perftesting_book120_kepub_epub", "146", "156"},
		{"file____mnt_onboard_perftesting_book160_kepub_epub", "178", "156"},
		{"file____mnt_onboard_perftesting_book200_kepub_epub", "210", "156"},
		{"file____mnt_onboard_perftesting_book240_kepub_epub", "210", "156"},
		{"file____mnt_onboard_perftesting_book280_kepub_epub", "242", "156"},
		{"file____mnt_onboard_perftesting_book320_kepub_epub", "18", "156"},
		{"file____mnt_onboard_perftesting_book360_kepub_epub", "50", "156"},
		{"file____mnt_onboard_perftesting_book400_kepub_epub", "82", "156"},
		{"file____mnt_onboard_perftesting_book440_kepub_epub", "82", "156"},
		{"file____mnt_onboard_perftesting_book480_kepub_epub", "114", "156"},
		{"file____mnt_onboard_perftesting_book520_kepub_epub", "146", "159"},
		{"file____mnt_onboard_perftesting_book560_kepub_epub", "178", "159"},
		{"file____mnt_onboard_perftesting_book600_kepub_epub", "210", "159"},
		{"file____mnt_onboard_perftesting_book640_kepub_epub", "210", "159"},
		{"file____mnt_onboard_perftesting_book680_kepub_epub", "242", "159"},
		{"file____mnt_onboard_perftesting_book720_kepub_epub", "18", "159"},
		{"file____mnt_onboard_perftesting_book760_kepub_epub", "50", "159"},
		{"file____mnt_onboard_perftesting_book800_kepub_epub", "82", "159"},
		{"file____mnt_onboard_perftesting_book840_kepub_epub", "82", "159"},
		{"file____mnt_onboard_perftesting_book880_kepub_epub", "114", "159"},
		{"file____mnt_onboard_perftesting_book920_kepub_epub", "146", "158"},
		{"file____mnt_onboard_perftesting_book960_kepub_epub", "178", "158"},
	} {
		d1, d2, bn := hashedImageParts(tc.id)
		if d1 != tc.dir1 {
			t.Errorf("unexpected dir1 for %#v, expected %#v, got %#v", tc.id, tc.dir1, d1)
		}
		if d2 != tc.dir2 {
			t.Errorf("unexpected dir2 for %#v, expected %#v, got %#v", tc.id, tc.dir2, d2)
		}
		if bn != tc.id {
			t.Errorf("unexpected basename for %#v, should be same same as image id, got %#v", tc.id, bn)
		}
	}
}
