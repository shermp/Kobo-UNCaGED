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

package kuprint

import (
	"fmt"
	"image"
	"image/color"

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

var einkPrint struct {
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

// InitPrinter returns an object which conforms to the KuPrinter interface
func InitPrinter(fontPath string) error {
	einkPrint.fbCfg = &gofbink.FBInkConfig{Valign: gofbink.None, NoRefresh: true, IgnoreAlpha: true}
	einkPrint.rCfg = &gofbink.RestrictedConfig{IsCentered: true}
	einkPrint.mbox.landscape.otCfg = gofbink.FBInkOTConfig{SizePt: 10, IsCentred: true}
	einkPrint.mbox.portrait.otCfg = gofbink.FBInkOTConfig{SizePt: 10, IsCentred: true}
	einkPrint.fbink = gofbink.New(einkPrint.fbCfg, einkPrint.rCfg)
	einkPrint.fbink.Init(einkPrint.fbCfg)
	err := einkPrint.fbink.AddOTfont(fontPath, gofbink.FntRegular)
	if err != nil {
		return err
	}
	einkPrint.fbState = &gofbink.FBInkState{}
	einkPrint.fbink.GetState(einkPrint.fbCfg, einkPrint.fbState)

	//dpi := einkPrint.fbState.ScreenDPI
	vw := einkPrint.fbState.ViewWidth
	vh := einkPrint.fbState.ViewHeight
	w := vw
	if w > vh {
		w = vh // in case we are landscape
	}
	w = uint32(float64(w) * 0.7)
	einkPrint.mbox.hdrH = uint32(float64(w) * 0.2)
	einkPrint.mbox.bdyH = uint32(float64(w) * 0.6)
	einkPrint.mbox.ftrH = uint32(float64(w) * 0.2)
	einkPrint.mbox.mb = createMessageBox(int(w), int(w))
	setOffsets(int(vw), int(vh))
	einkPrint.headStr, einkPrint.bodyStr, einkPrint.footStr = " ", " ", " "
	return nil
}

func setOffsets(vw, vh int) {
	// portrait
	mbW := einkPrint.mbox.mb.Rect.Max.X - einkPrint.mbox.mb.Rect.Min.X
	mbH := einkPrint.mbox.mb.Rect.Max.Y - einkPrint.mbox.mb.Rect.Min.Y
	if vh > vw {
		einkPrint.mbox.portrait.yOff = int16((vh - mbH) / 2)
		einkPrint.mbox.portrait.xOff = int16((vw - mbW) / 2)
		einkPrint.mbox.portrait.otCfg.Margins.Left = einkPrint.mbox.portrait.xOff
		einkPrint.mbox.portrait.otCfg.Margins.Right = einkPrint.mbox.portrait.xOff
		einkPrint.mbox.portrait.refreshReg.x = uint32(einkPrint.mbox.portrait.xOff)
		einkPrint.mbox.portrait.refreshReg.y = uint32(einkPrint.mbox.portrait.yOff)
		einkPrint.mbox.portrait.refreshReg.w = uint32(mbW)
		einkPrint.mbox.portrait.refreshReg.h = uint32(mbH)
	} else {
		einkPrint.mbox.landscape.yOff = int16((vw - mbH) / 2)
		einkPrint.mbox.landscape.xOff = int16((vh - mbW) / 2)
		einkPrint.mbox.landscape.otCfg.Margins.Left = einkPrint.mbox.landscape.xOff
		einkPrint.mbox.landscape.otCfg.Margins.Right = einkPrint.mbox.landscape.xOff
		einkPrint.mbox.landscape.refreshReg.x = uint32(einkPrint.mbox.landscape.xOff)
		einkPrint.mbox.landscape.refreshReg.y = uint32(einkPrint.mbox.landscape.yOff)
		einkPrint.mbox.landscape.refreshReg.w = uint32(mbW)
		einkPrint.mbox.landscape.refreshReg.h = uint32(mbH)
	}

}

func createMessageBox(w, h int) *image.RGBA {
	//log.Printf("MBox Width: %d   MBox Height: %d\n", w, h)
	mb := image.NewRGBA(image.Rect(0, 0, int(w), int(h)))
	bgColor := color.RGBA{255, 255, 255, 255}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			mb.Set(x, y, bgColor)
		}
	}
	return mb
}

// Close safely closes
func Close() {
	einkPrint.fbink.FreeOTfonts()
}

func printSection(orient *orientation, section MboxSection, vh uint32) error {
	var err error
	var str string
	if section == Header {
		orient.otCfg.Margins.Top = orient.yOff
		orient.otCfg.Margins.Bottom = int16(vh) - (orient.otCfg.Margins.Top + int16(einkPrint.mbox.hdrH))
		orient.otCfg.SizePt = 12
		str = einkPrint.headStr
	} else if section == Body {
		orient.otCfg.Margins.Top = orient.yOff + int16(einkPrint.mbox.hdrH)
		orient.otCfg.Margins.Bottom = int16(vh) - (orient.otCfg.Margins.Top + int16(einkPrint.mbox.bdyH))
		orient.otCfg.SizePt = 11
		str = einkPrint.bodyStr
	} else {
		orient.otCfg.Margins.Top = orient.yOff + int16(einkPrint.mbox.hdrH) + int16(einkPrint.mbox.bdyH)
		orient.otCfg.Margins.Bottom = int16(vh) - (orient.otCfg.Margins.Top + int16(einkPrint.mbox.ftrH))
		orient.otCfg.SizePt = 11
		str = einkPrint.footStr
	}
	//log.Printf("Top Margin: %d, Bottom Margin: %d\n", orient.otCfg.Margins.Top, orient.otCfg.Margins.Bottom)
	einkPrint.fbCfg.Valign = gofbink.Center
	_, err = einkPrint.fbink.PrintOT(str, &orient.otCfg, einkPrint.fbCfg)
	return err
}

// Println displays a message for the user
func Println(section MboxSection, a ...interface{}) (n int, err error) {
	n = 0
	err = nil
	// Reset Valign first, otherwise this triggers a nasty bug where button_scan fails
	einkPrint.fbCfg.Valign = gofbink.None
	einkPrint.fbink.ReInit(einkPrint.fbCfg)
	einkPrint.fbink.GetState(einkPrint.fbCfg, einkPrint.fbState)
	str := fmt.Sprint(a...)
	if section == Header {
		einkPrint.headStr = str
	} else if section == Body {
		einkPrint.bodyStr = str
	} else {
		einkPrint.footStr = str
	}
	// Determine our orientation
	var orient *orientation
	if einkPrint.fbState.ViewHeight > einkPrint.fbState.ViewWidth {
		orient = &einkPrint.mbox.portrait
	} else {
		orient = &einkPrint.mbox.landscape
	}
	// Print the messagebox to FB
	err = einkPrint.fbink.PrintRBGA(orient.xOff, orient.yOff, einkPrint.mbox.mb, einkPrint.fbCfg)
	if err != nil {
		return 0, err
	}
	// Print Header
	printSection(orient, Header, einkPrint.fbState.ViewHeight)
	// Then body
	printSection(orient, Body, einkPrint.fbState.ViewHeight)
	// Then footer
	printSection(orient, Footer, einkPrint.fbState.ViewHeight)
	// Finally, refresh
	err = einkPrint.fbink.Refresh(orient.refreshReg.y, orient.refreshReg.x, orient.refreshReg.w, orient.refreshReg.h, gofbink.DitherPassthrough, einkPrint.fbCfg)
	if err != nil {
		return 0, err
	}
	return n, err
}
