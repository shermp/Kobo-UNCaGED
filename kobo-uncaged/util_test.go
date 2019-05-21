package main

import (
	"image"
	"testing"
)

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
	} {
		if rz := resizeKeepAspectRatioByExpanding(tc.sz, tc.bounds); !rz.Eq(tc.res) {
			t.Errorf("resize %s to %s: expected %s, got %s", tc.sz, tc.bounds, tc.res, rz)
		}
	}
}
