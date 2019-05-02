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

	"github.com/shermp/go-fbink-v2/gofbink"
)

type kuUserPrint struct {
	fbCfg    *gofbink.FBInkConfig
	rCfg     *gofbink.RestrictedConfig
	fbink    *gofbink.FBInk
	fbState  *gofbink.FBInkState
	mboxPort struct {
		otCfg     *gofbink.FBInkOTConfig
		xOff      int16
		yOff      int16
		width     int16
		height    int16
		scanlines []uint8
	}
	mboxLand struct {
		otCfg     *gofbink.FBInkOTConfig
		xOff      int16
		yOff      int16
		width     int16
		height    int16
		scanlines []uint8
	}
}

func newKuPrint(fontPath string) (*kuUserPrint, error) {
	kup := &kuUserPrint{}
	kup.fbCfg = &gofbink.FBInkConfig{}
	kup.rCfg = &gofbink.RestrictedConfig{}
	kup.mboxLand.otCfg = &gofbink.FBInkOTConfig{}
	kup.mboxPort.otCfg = &gofbink.FBInkOTConfig{}
	kup.mboxLand.otCfg.SizePt = 12
	kup.mboxLand.otCfg.IsCentred = true
	kup.mboxPort.otCfg.SizePt = 12
	kup.mboxPort.otCfg.IsCentred = true
	kup.fbink = gofbink.New(kup.fbCfg, kup.rCfg)
	kup.fbink.Init(kup.fbCfg)
	err := kup.fbink.AddOTfont(fontPath, gofbink.FntRegular)
	if err != nil {
		return nil, err
	}
	kup.fbState = &gofbink.FBInkState{}
	kup.fbink.GetState(kup.fbCfg, kup.fbState)

	dpi := int(kup.fbState.ScreenDPI)
	vw := int16(kup.fbState.ViewWidth)
	vh := int16(kup.fbState.ViewHeight)
	if vw > vh {
		kup.mboxLand.width = int16(float64(vw) * 0.667)
		kup.mboxLand.height = int16(float64(vh) * 0.333)
		kup.mboxLand.xOff = (vw - kup.mboxLand.width) / 2
		kup.mboxLand.yOff = int16(float64(vh) * 0.15)
		calcOTmargins(kup.mboxLand.width, kup.mboxLand.height, vw, vh, kup.mboxLand.xOff, kup.mboxLand.yOff, dpi, kup.mboxLand.otCfg)
		kup.mboxPort.width = int16(float64(vh) * 0.667)
		kup.mboxPort.height = int16(float64(vw) * 0.333)
		kup.mboxPort.xOff = (vh - kup.mboxPort.width) / 2
		kup.mboxPort.yOff = int16(float64(vw) * 0.15)
		calcOTmargins(kup.mboxPort.width, kup.mboxPort.height, vh, vw, kup.mboxPort.xOff, kup.mboxPort.yOff, dpi, kup.mboxPort.otCfg)
	} else {
		kup.mboxPort.width = int16(float64(vw) * 0.667)
		kup.mboxPort.height = int16(float64(vh) * 0.333)
		kup.mboxPort.xOff = (vw - kup.mboxPort.width) / 2
		kup.mboxPort.yOff = int16(float64(vh) * 0.15)
		calcOTmargins(kup.mboxPort.width, kup.mboxPort.height, vw, vh, kup.mboxPort.xOff, kup.mboxPort.yOff, dpi, kup.mboxPort.otCfg)
		kup.mboxLand.width = int16(float64(vh) * 0.667)
		kup.mboxLand.height = int16(float64(vw) * 0.333)
		kup.mboxLand.xOff = (vh - kup.mboxLand.width) / 2
		kup.mboxLand.yOff = int16(float64(vw) * 0.15)
		calcOTmargins(kup.mboxLand.width, kup.mboxLand.height, vh, vw, kup.mboxLand.xOff, kup.mboxLand.yOff, dpi, kup.mboxLand.otCfg)
	}
	kup.mboxLand.scanlines = createMessageBox(int(kup.mboxLand.width), int(kup.mboxLand.height))
	kup.mboxPort.scanlines = createMessageBox(int(kup.mboxPort.width), int(kup.mboxPort.height))
	kup.fbCfg.Valign = gofbink.Center
	kup.fbCfg.NoRefresh = true
	kup.fbCfg.IgnoreAlpha = true
	return kup, nil
}

func calcOTmargins(w, h, vw, vh, x, y int16, dpi int, otCfg *gofbink.FBInkOTConfig) {
	otCfg.Margins.Top = y + int16(mmToPx(2, dpi))
	otCfg.Margins.Left = x + int16(mmToPx(2, dpi))
	otCfg.Margins.Bottom = vh - (y + h) + int16(mmToPx(2, dpi))
	otCfg.Margins.Right = vw - (x + w) + int16(mmToPx(2, dpi))
}
func mmToPx(mm, dpi int) int {
	return int(float64(mm*dpi) / 25.4)
}

func createMessageBox(w, h int) []uint8 {
	mb := image.NewRGBA(image.Rect(0, 0, w, h))
	borderColor := color.RGBA{0, 0, 0, 255}
	bgColor := color.RGBA{255, 255, 255, 255}
	for x := 0; x < w; x++ {
		mb.SetRGBA(x, 0, borderColor)
		mb.SetRGBA(x, h-1, borderColor)
	}
	for y := 0; y < h; y++ {
		mb.SetRGBA(0, y, borderColor)
		mb.SetRGBA(w-1, y, borderColor)
	}
	for y := 1; y < h-1; y++ {
		for x := 1; x < w-1; x++ {
			mb.SetRGBA(x, y, bgColor)
		}
	}
	return mb.Pix
}

func (kup *kuUserPrint) kuClose() {
	kup.fbink.FreeOTfonts()
}

func (kup *kuUserPrint) kuPrintln(a ...interface{}) (n int, err error) {

	n = 0
	err = nil
	kup.fbink.ReInit(kup.fbCfg)
	kup.fbink.GetState(kup.fbCfg, kup.fbState)
	str := fmt.Sprint(a...)
	if kup.fbState.ViewWidth > kup.fbState.ViewHeight {
		kup.fbink.PrintRawData(kup.mboxLand.scanlines, int(kup.mboxLand.width), int(kup.mboxLand.height), uint16(kup.mboxLand.xOff), uint16(kup.mboxLand.yOff), kup.fbCfg)
		n, err = kup.fbink.PrintOT(str, kup.mboxLand.otCfg, kup.fbCfg)
		kup.fbink.Refresh(uint32(kup.mboxLand.yOff), uint32(kup.mboxLand.xOff), uint32(kup.mboxLand.width), uint32(kup.mboxLand.height), gofbink.DitherPassthrough, kup.fbCfg)
	} else {
		kup.fbink.PrintRawData(kup.mboxPort.scanlines, int(kup.mboxPort.width), int(kup.mboxPort.height), uint16(kup.mboxPort.xOff), uint16(kup.mboxPort.yOff), kup.fbCfg)
		n, err = kup.fbink.PrintOT(str, kup.mboxLand.otCfg, kup.fbCfg)
		kup.fbink.Refresh(uint32(kup.mboxPort.yOff), uint32(kup.mboxPort.xOff), uint32(kup.mboxPort.width), uint32(kup.mboxPort.height), gofbink.DitherPassthrough, kup.fbCfg)
	}
	return n, err
}
