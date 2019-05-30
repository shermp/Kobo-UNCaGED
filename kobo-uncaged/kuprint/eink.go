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

type einkPrint struct {
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

// NewPrinter returns an object which conforms to the KuPrinter interface
func NewPrinter(fontPath string) (Printer, error) {
	ep := &einkPrint{}
	ep.fbCfg = &gofbink.FBInkConfig{Valign: gofbink.None, NoRefresh: true, IgnoreAlpha: true}
	ep.rCfg = &gofbink.RestrictedConfig{IsCentered: true}
	ep.mbox.landscape.otCfg = gofbink.FBInkOTConfig{SizePt: 10, IsCentred: true}
	ep.mbox.portrait.otCfg = gofbink.FBInkOTConfig{SizePt: 10, IsCentred: true}
	ep.fbink = gofbink.New(ep.fbCfg, ep.rCfg)
	ep.fbink.Init(ep.fbCfg)
	err := ep.fbink.AddOTfont(fontPath, gofbink.FntRegular)
	if err != nil {
		return nil, err
	}
	ep.fbState = &gofbink.FBInkState{}
	ep.fbink.GetState(ep.fbCfg, ep.fbState)

	//dpi := ep.fbState.ScreenDPI
	vw := ep.fbState.ViewWidth
	vh := ep.fbState.ViewHeight
	w := vw
	if w > vh {
		w = vh // in case we are landscape
	}
	w = uint32(float64(w) * 0.7)
	ep.mbox.hdrH = uint32(float64(w) * 0.2)
	ep.mbox.bdyH = uint32(float64(w) * 0.6)
	ep.mbox.ftrH = uint32(float64(w) * 0.2)
	ep.mbox.mb = createMessageBox(int(w), int(w))
	ep.setOffsets(int(vw), int(vh))
	ep.headStr, ep.bodyStr, ep.footStr = " ", " ", " "
	return ep, nil
}

func (ep *einkPrint) setOffsets(vw, vh int) {
	// portrait
	mbW := ep.mbox.mb.Rect.Max.X - ep.mbox.mb.Rect.Min.X
	mbH := ep.mbox.mb.Rect.Max.Y - ep.mbox.mb.Rect.Min.Y
	if vh > vw {
		ep.mbox.portrait.yOff = int16((vh - mbH) / 2)
		ep.mbox.portrait.xOff = int16((vw - mbW) / 2)
		ep.mbox.portrait.otCfg.Margins.Left = ep.mbox.portrait.xOff
		ep.mbox.portrait.otCfg.Margins.Right = ep.mbox.portrait.xOff
		ep.mbox.portrait.refreshReg.x = uint32(ep.mbox.portrait.xOff)
		ep.mbox.portrait.refreshReg.y = uint32(ep.mbox.portrait.yOff)
		ep.mbox.portrait.refreshReg.w = uint32(mbW)
		ep.mbox.portrait.refreshReg.h = uint32(mbH)
	} else {
		ep.mbox.landscape.yOff = int16((vw - mbH) / 2)
		ep.mbox.landscape.xOff = int16((vh - mbW) / 2)
		ep.mbox.landscape.otCfg.Margins.Left = ep.mbox.landscape.xOff
		ep.mbox.landscape.otCfg.Margins.Right = ep.mbox.landscape.xOff
		ep.mbox.landscape.refreshReg.x = uint32(ep.mbox.landscape.xOff)
		ep.mbox.landscape.refreshReg.y = uint32(ep.mbox.landscape.yOff)
		ep.mbox.landscape.refreshReg.w = uint32(mbW)
		ep.mbox.landscape.refreshReg.h = uint32(mbH)
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
func (ep *einkPrint) Close() {
	ep.fbink.FreeOTfonts()
}

func (ep *einkPrint) printSection(orient *orientation, section MboxSection, vh uint32) error {
	var err error
	var str string
	if section == Header {
		orient.otCfg.Margins.Top = orient.yOff
		orient.otCfg.Margins.Bottom = int16(vh) - (orient.otCfg.Margins.Top + int16(ep.mbox.hdrH))
		orient.otCfg.SizePt = 12
		str = ep.headStr
	} else if section == Body {
		orient.otCfg.Margins.Top = orient.yOff + int16(ep.mbox.hdrH)
		orient.otCfg.Margins.Bottom = int16(vh) - (orient.otCfg.Margins.Top + int16(ep.mbox.bdyH))
		orient.otCfg.SizePt = 11
		str = ep.bodyStr
	} else {
		orient.otCfg.Margins.Top = orient.yOff + int16(ep.mbox.hdrH) + int16(ep.mbox.bdyH)
		orient.otCfg.Margins.Bottom = int16(vh) - (orient.otCfg.Margins.Top + int16(ep.mbox.ftrH))
		orient.otCfg.SizePt = 11
		str = ep.footStr
	}
	//log.Printf("Top Margin: %d, Bottom Margin: %d\n", orient.otCfg.Margins.Top, orient.otCfg.Margins.Bottom)
	ep.fbCfg.Valign = gofbink.Center
	_, err = ep.fbink.PrintOT(str, &orient.otCfg, ep.fbCfg)
	return err
}

// Println displays a message for the user
func (ep *einkPrint) Println(section MboxSection, a ...interface{}) (n int, err error) {
	n = 0
	err = nil
	// Reset Valign first, otherwise this triggers a nasty bug where button_scan fails
	ep.fbCfg.Valign = gofbink.None
	ep.fbink.ReInit(ep.fbCfg)
	ep.fbink.GetState(ep.fbCfg, ep.fbState)
	str := fmt.Sprint(a...)
	if section == Header {
		ep.headStr = str
	} else if section == Body {
		ep.bodyStr = str
	} else {
		ep.footStr = str
	}
	// Determine our orientation
	var orient *orientation
	if ep.fbState.ViewHeight > ep.fbState.ViewWidth {
		orient = &ep.mbox.portrait
	} else {
		orient = &ep.mbox.landscape
	}
	// Print the messagebox to FB
	err = ep.fbink.PrintRBGA(orient.xOff, orient.yOff, ep.mbox.mb, ep.fbCfg)
	if err != nil {
		return 0, err
	}
	// Print Header
	ep.printSection(orient, Header, ep.fbState.ViewHeight)
	// Then body
	ep.printSection(orient, Body, ep.fbState.ViewHeight)
	// Then footer
	ep.printSection(orient, Footer, ep.fbState.ViewHeight)
	// Finally, refresh
	err = ep.fbink.Refresh(orient.refreshReg.y, orient.refreshReg.x, orient.refreshReg.w, orient.refreshReg.h, gofbink.DitherPassthrough, ep.fbCfg)
	if err != nil {
		return 0, err
	}
	return n, err
}
