package main

import "testing"

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
