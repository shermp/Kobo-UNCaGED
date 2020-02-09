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

package device

import (
	"database/sql"
	"fmt"
	"image"
	"path/filepath"
	"strings"
	"sync"

	"github.com/bamiaux/rez"
	"github.com/shermp/Kobo-UNCaGED/kobo-uncaged/util"
	"github.com/shermp/UNCaGED/uc"
)

type cidPrefix string

type firmwareVersion struct {
	major int
	minor int
	build int
}

// KuOptions contains some options that are required
type KuOptions struct {
	PreferSDCard bool
	PreferKepub  bool
	PasswordList []string
	Thumbnail    thumbnailOption
}

// Kobo contains the variables and methods required to use
// the UNCaGED library
type Kobo struct {
	Device          koboDevice
	fw              firmwareVersion
	KuConfig        *KuOptions
	DBRootDir       string
	BKRootDir       string
	ContentIDprefix cidPrefix
	useSDCard       bool
	MetadataMap     map[string]uc.CalibreBookMeta
	UpdatedMetadata []string
	Passwords       *uncagedPassword
	DriveInfo       uc.DeviceInfo
	nickelDB        *sql.DB
	Wg              *sync.WaitGroup
}

type MetaIterator struct {
	k        *Kobo
	cidList  []string
	cidIndex int
}

func NewMetaIter(k *Kobo) *MetaIterator {
	iter := MetaIterator{k: k, cidIndex: -1}
	iter.cidList = make([]string, 0)
	return &iter
}

func (m *MetaIterator) Add(cid string) {
	m.cidList = append(m.cidList, cid)
}
func (m *MetaIterator) Next() bool {
	m.cidIndex++
	if m.cidIndex < len(m.cidList) {
		return true
	}
	return false
}
func (m *MetaIterator) Count() int {
	return len(m.cidList)
}
func (m *MetaIterator) Get() (uc.CalibreBookMeta, error) {
	if m.Count() > 0 && m.cidIndex >= 0 {
		if md, exists := m.k.MetadataMap[m.cidList[m.cidIndex]]; exists {
			return md, nil
		}
	}
	return uc.CalibreBookMeta{}, fmt.Errorf("no metadata to get")
}

type uncagedPassword struct {
	currPassIndex int
	passwordList  []string
}

type koboDevice string

// Kobo model ID's from https://github.com/geek1011/KoboStuff/blob/gh-pages/kobofirmware.js#L11
const (
	touchAB      koboDevice = "00000000-0000-0000-0000-000000000310"
	touchC       koboDevice = "00000000-0000-0000-0000-000000000320"
	mini         koboDevice = "00000000-0000-0000-0000-000000000340"
	glo          koboDevice = "00000000-0000-0000-0000-000000000330"
	gloHD        koboDevice = "00000000-0000-0000-0000-000000000371"
	touch2       koboDevice = "00000000-0000-0000-0000-000000000372"
	aura         koboDevice = "00000000-0000-0000-0000-000000000360"
	auraHD       koboDevice = "00000000-0000-0000-0000-000000000350"
	auraH2O      koboDevice = "00000000-0000-0000-0000-000000000370"
	auraH2Oed2r1 koboDevice = "00000000-0000-0000-0000-000000000374"
	auraH2Oed2r2 koboDevice = "00000000-0000-0000-0000-000000000378"
	auraOne      koboDevice = "00000000-0000-0000-0000-000000000373"
	auraOneLE    koboDevice = "00000000-0000-0000-0000-000000000381"
	auraEd2r1    koboDevice = "00000000-0000-0000-0000-000000000375"
	auraEd2r2    koboDevice = "00000000-0000-0000-0000-000000000379"
	claraHD      koboDevice = "00000000-0000-0000-0000-000000000376"
	forma        koboDevice = "00000000-0000-0000-0000-000000000377"
	forma32gb    koboDevice = "00000000-0000-0000-0000-000000000380"
	libra        koboDevice = "00000000-0000-0000-0000-000000000384"
)

// Model returns the model name for the device.
func (d koboDevice) Model() string {
	switch d {
	case touch2, touchAB, touchC:
		return "Touch"
	case mini:
		return "Mini"
	case glo:
		return "Glo"
	case gloHD:
		return "Glo HD"
	case aura:
		return "Aura"
	case auraH2O:
		return "Aura H2O"
	case auraH2Oed2r1, auraH2Oed2r2:
		return "Aura H2O Ed. 2"
	case auraEd2r1, auraEd2r2:
		return "Aura Ed. 2"
	case auraHD:
		return "Aura HD"
	case auraOne, auraOneLE:
		return "Aura One"
	case claraHD:
		return "Clara HD"
	case forma, forma32gb:
		return "Forma"
	case libra:
		return "Libra H2O"
	default:
		return "Unknown Kobo"
	}
}

// FullCover gets the appropriate cover dimensions for the device. These values
// come from Image::sizeForType in the Kobo firmware.
// See https://github.com/shermp/Kobo-UNCaGED/issues/16#issuecomment-494229994
// for more details.
func (d koboDevice) FullCover() image.Point {
	switch d {
	case auraOne, auraOneLE: // daylight
		return image.Pt(1404, 1872)
	case gloHD, claraHD: // alyssum, nova
		return image.Pt(1072, 1448)
	case auraHD, auraH2O, auraH2Oed2r1, auraH2Oed2r2: // dragon
		if d == auraH2O {
			// Nickel's behaviour is incorrect as of 4.14.12777.
			// See https://github.com/shermp/Kobo-UNCaGED/pull/17#pullrequestreview-240281740
			return image.Pt(1080, 1429)
		}
		return image.Pt(1080, 1440)
	case glo, auraEd2r1, auraEd2r2: // kraken, star
		return image.Pt(758, 1024)
	case aura: // phoenix
		return image.Pt(758, 1014)
	case forma, forma32gb: // frost
		return image.Pt(1440, 1920)
	case libra: // storm
		return image.Pt(1264, 1680)
	default: // KoboWifi, KoboTouch, trilogy, KoboTouch2
		return image.Pt(600, 800)
	}
}

type koboCover int

const (
	fullCover koboCover = iota
	libFull
	libGrid
)

func (k koboCover) String() string {
	switch k {
	case fullCover:
		return "N3_FULL"
	case libFull:
		return "N3_LIBRARY_FULL"
	case libGrid:
		return "N3_LIBRARY_GRID"
	default:
		panic("unknown cover type")
	}
}

// Resize returnes the dimensions to resize sz to for the cover type.
func (k koboCover) Resize(d koboDevice, sz image.Point) image.Point {
	switch k {
	case fullCover:
		return util.ResizeKeepAspectRatio(sz, k.Size(d), false)
	case libFull, libGrid:
		return util.ResizeKeepAspectRatio(sz, k.Size(d), true)
	default:
		panic("unknown cover type")
	}
}

// Size gets the target image size for the cover type.
func (k koboCover) Size(d koboDevice) image.Point {
	switch k {
	case fullCover:
		return d.FullCover()
	case libFull:
		return image.Pt(355, 530)
	case libGrid:
		return image.Pt(149, 223)
	default:
		panic("unknown cover type")
	}
}

// RelPath gets the path to the cover file relative to the images dir.
func (k koboCover) RelPath(imageID string) string {
	dir1, dir2, basename := util.HashedImageParts(imageID)
	return filepath.Join(dir1, dir2, fmt.Sprintf("%s - %s.parsed", basename, k.String()))
}

type thumbnailOption struct {
	GenerateLevel   string
	ResizeAlgorithm string
	JpegQuality     int
	rezFilter       rez.Filter
}

const (
	generateAll     string = "all"
	generatePartial string = "partial"
	generateNone    string = "none"
)

const (
	//resizeNN  string = "nearest"
	resizeBL  string = "bilinear"
	resizeBC  string = "bicubic"
	resizeLC2 string = "lanczos2"
	resizeLC3 string = "lanczos3"
)

func (to *thumbnailOption) Validate() {
	switch strings.ToLower(to.GenerateLevel) {
	case generateAll, generatePartial, generateNone:
		to.GenerateLevel = strings.ToLower(to.GenerateLevel)
	default:
		to.GenerateLevel = generateAll
	}

	switch strings.ToLower(to.ResizeAlgorithm) {
	case resizeBL, resizeBC, resizeLC2, resizeLC3:
		to.ResizeAlgorithm = strings.ToLower(to.ResizeAlgorithm)
	default:
		to.ResizeAlgorithm = resizeBC
	}

	if to.JpegQuality < 1 || to.JpegQuality > 100 {
		to.JpegQuality = 90
	}
}

func (to *thumbnailOption) SetRezFilter() {
	switch to.ResizeAlgorithm {
	case resizeBL:
		to.rezFilter = rez.NewBilinearFilter()
	case resizeBC:
		to.rezFilter = rez.NewBicubicFilter()
	case resizeLC2:
		to.rezFilter = rez.NewLanczosFilter(2)
	case resizeLC3:
		to.rezFilter = rez.NewLanczosFilter(3)
	default:
		to.rezFilter = rez.NewBicubicFilter()
	}
}
