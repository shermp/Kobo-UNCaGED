// +build linux,arm

// Copyright 2019 Sherman Perry

// This file is part of Kobo UNCaGED.

// Kobo UNCaGED is free software: you can redistribute it and/or modify
// it under the terms of the Affero GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

// Kobo UNCaGED is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.

// You should have received a copy of the GNU Affero General Public License
// along with Kobo UNCaGED.  If not, see <https://www.gnu.org/licenses/>.

package main

import (
	"fmt"
	"image"
	"image/color"
	"log"

	"github.com/shermp/go-fbink-v2/gofbink"
)

type region struct {
	x, y, w, h uint32
}

type orientation struct {
	otCfg      gofbink.FBInkOTConfig
	refreshReg region
	xOff, yOff int16
}

type kuUserPrint struct {
	fbCfg   *gofbink.FBInkConfig
	rCfg    *gofbink.RestrictedConfig
	fbink   *gofbink.FBInk
	fbState *gofbink.FBInkState
	mbox    struct {
		portrait, landscape orientation
		hdrH, bdyH, ftrH    uint32
		mb                  *image.RGBA
	}
	headStr, bodyStr, footStr string
}

func newKuPrint(fontPath string) (*kuUserPrint, error) {
	kup := &kuUserPrint{}
	kup.fbCfg = &gofbink.FBInkConfig{Valign: gofbink.None, NoRefresh: true, IgnoreAlpha: true}
	kup.rCfg = &gofbink.RestrictedConfig{IsCentered: true}
	kup.mbox.landscape.otCfg = gofbink.FBInkOTConfig{SizePt: 10, IsCentred: true}
	kup.mbox.portrait.otCfg = gofbink.FBInkOTConfig{SizePt: 10, IsCentred: true}
	kup.fbink = gofbink.New(kup.fbCfg, kup.rCfg)
	kup.fbink.Init(kup.fbCfg)
	err := kup.fbink.AddOTfont(fontPath, gofbink.FntRegular)
	if err != nil {
		return nil, err
	}
	kup.fbState = &gofbink.FBInkState{}
	kup.fbink.GetState(kup.fbCfg, kup.fbState)

	//dpi := kup.fbState.ScreenDPI
	vw := kup.fbState.ViewWidth
	vh := kup.fbState.ViewHeight
	w := vw
	if w > vh {
		w = vh // in case we are landscape
	}
	w = uint32(float64(w) * 0.7)
	kup.mbox.hdrH = uint32(float64(w) * 0.2)
	kup.mbox.bdyH = uint32(float64(w) * 0.6)
	kup.mbox.ftrH = uint32(float64(w) * 0.2)
	kup.mbox.mb = createMessageBox(int(w), int(w))
	kup.setOffsets(int(vw), int(vh))
	kup.headStr, kup.bodyStr, kup.footStr = " ", " ", " "
	return kup, nil
}

func (kup *kuUserPrint) setOffsets(vw, vh int) {
	// portrait
	mbW := kup.mbox.mb.Rect.Max.X - kup.mbox.mb.Rect.Min.X
	mbH := kup.mbox.mb.Rect.Max.Y - kup.mbox.mb.Rect.Min.Y
	if vh > vw {
		kup.mbox.portrait.yOff = int16((vh - mbH) / 2)
		kup.mbox.portrait.xOff = int16((vw - mbW) / 2)
		kup.mbox.portrait.otCfg.Margins.Left = kup.mbox.portrait.xOff
		kup.mbox.portrait.otCfg.Margins.Right = kup.mbox.portrait.xOff
		kup.mbox.portrait.refreshReg.x = uint32(kup.mbox.portrait.xOff)
		kup.mbox.portrait.refreshReg.y = uint32(kup.mbox.portrait.yOff)
		kup.mbox.portrait.refreshReg.w = uint32(mbW)
		kup.mbox.portrait.refreshReg.h = uint32(mbH)
	} else {
		kup.mbox.landscape.yOff = int16((vw - mbH) / 2)
		kup.mbox.landscape.xOff = int16((vh - mbW) / 2)
		kup.mbox.landscape.otCfg.Margins.Left = kup.mbox.landscape.xOff
		kup.mbox.landscape.otCfg.Margins.Right = kup.mbox.landscape.xOff
		kup.mbox.landscape.refreshReg.x = uint32(kup.mbox.landscape.xOff)
		kup.mbox.landscape.refreshReg.y = uint32(kup.mbox.landscape.yOff)
		kup.mbox.landscape.refreshReg.w = uint32(mbW)
		kup.mbox.landscape.refreshReg.h = uint32(mbH)
	}

}

func createMessageBox(w, h int) *image.RGBA {
	log.Printf("MBox Width: %d   MBox Height: %d\n", w, h)
	mb := image.NewRGBA(image.Rect(0, 0, int(w), int(h)))
	bgColor := color.RGBA{255, 255, 255, 255}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			mb.Set(x, y, bgColor)
		}
	}
	return mb
}

func (kup *kuUserPrint) kuClose() {
	kup.fbink.FreeOTfonts()
}

func (kup *kuUserPrint) kuPrintSection(orient *orientation, section mboxSection, vh uint32) error {
	var err error
	var str string
	if section == header {
		orient.otCfg.Margins.Top = orient.yOff
		orient.otCfg.Margins.Bottom = int16(vh) - (orient.otCfg.Margins.Top + int16(kup.mbox.hdrH))
		orient.otCfg.SizePt = 13
		str = kup.headStr
	} else if section == body {
		orient.otCfg.Margins.Top = orient.yOff + int16(kup.mbox.hdrH)
		orient.otCfg.Margins.Bottom = int16(vh) - (orient.otCfg.Margins.Top + int16(kup.mbox.bdyH))
		orient.otCfg.SizePt = 11
		str = kup.bodyStr
	} else {
		orient.otCfg.Margins.Top = orient.yOff + int16(kup.mbox.hdrH) + int16(kup.mbox.bdyH)
		orient.otCfg.Margins.Bottom = int16(vh) - (orient.otCfg.Margins.Top + int16(kup.mbox.ftrH))
		orient.otCfg.SizePt = 11
		str = kup.footStr
	}
	log.Printf("Top Margin: %d, Bottom Margin: %d\n", orient.otCfg.Margins.Top, orient.otCfg.Margins.Bottom)
	kup.fbCfg.Valign = gofbink.Center
	_, err = kup.fbink.PrintOT(str, &orient.otCfg, kup.fbCfg)
	return err
}

func (kup *kuUserPrint) kuPrintln(section mboxSection, a ...interface{}) (n int, err error) {
	n = 0
	err = nil
	// Reset Valign first, otherwise this triggers a nasty bug where button_scan fails
	kup.fbCfg.Valign = gofbink.None
	kup.fbink.ReInit(kup.fbCfg)
	kup.fbink.GetState(kup.fbCfg, kup.fbState)
	str := fmt.Sprint(a...)
	if section == header {
		kup.headStr = str
	} else if section == body {
		kup.bodyStr = str
	} else {
		kup.footStr = str
	}
	// Determine our orientation
	var orient *orientation
	if kup.fbState.ViewHeight > kup.fbState.ViewWidth {
		orient = &kup.mbox.portrait
	} else {
		orient = &kup.mbox.landscape
	}
	// Print the messagebox to FB
	err = kup.fbink.PrintRBGA(orient.xOff, orient.yOff, kup.mbox.mb, kup.fbCfg)
	if err != nil {
		return 0, err
	}
	// Print Header
	kup.kuPrintSection(orient, header, kup.fbState.ViewHeight)
	// Then body
	kup.kuPrintSection(orient, body, kup.fbState.ViewHeight)
	// Then footer
	kup.kuPrintSection(orient, footer, kup.fbState.ViewHeight)
	// Finally, refresh
	err = kup.fbink.Refresh(orient.refreshReg.y, orient.refreshReg.x, orient.refreshReg.w, orient.refreshReg.h, gofbink.DitherPassthrough, kup.fbCfg)
	if err != nil {
		return 0, err
	}
	return n, err
}
