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
	"path/filepath"
	"strings"

	"github.com/bamiaux/rez"
	"github.com/shermp/Kobo-UNCaGED/kobo-uncaged/util"
)

type cidPrefix string

// KoboMetadata contains the metadata for ebooks on kobo devices.
// It replicates the metadata available in the Kobo USBMS driver.
// Note, pointers are used where necessary to account for null JSON values
type KoboMetadata struct {
	Authors         []string               `json:"authors" mapstructure:"authors"`
	Languages       []string               `json:"languages" mapstructure:"languages"`
	UserMetadata    map[string]interface{} `json:"user_metadata" mapstructure:"user_metadata"`
	UserCategories  map[string]interface{} `json:"user_categories" mapstructure:"user_categories"`
	Comments        *string                `json:"comments" mapstructure:"comments"`
	Tags            []string               `json:"tags" mapstructure:"tags"`
	Pubdate         *string                `json:"pubdate" mapstructure:"pubdate"`
	SeriesIndex     *float64               `json:"series_index" mapstructure:"series_index"`
	Thumbnail       []interface{}          `json:"thumbnail" mapstructure:"thumbnail"`
	PublicationType *string                `json:"publication_type" mapstructure:"publication_type"`
	Mime            *string                `json:"mime" mapstructure:"mime"`
	AuthorSort      string                 `json:"author_sort" mapstructure:"author_sort"`
	Series          *string                `json:"series" mapstructure:"series"`
	Rights          *string                `json:"rights" mapstructure:"rights"`
	DbID            interface{}            `json:"db_id" mapstructure:"db_id"`
	Cover           *string                `json:"cover" mapstructure:"cover"`
	ApplicationID   int                    `json:"application_id" mapstructure:"application_id"`
	BookProducer    *string                `json:"book_producer" mapstructure:"book_producer"`
	Size            int                    `json:"size" mapstructure:"size"`
	AuthorSortMap   map[string]string      `json:"author_sort_map" mapstructure:"author_sort_map"`
	Rating          *float64               `json:"rating" mapstructure:"rating"`
	Lpath           string                 `json:"lpath" mapstructure:"lpath"`
	Publisher       *string                `json:"publisher" mapstructure:"publisher"`
	Timestamp       *string                `json:"timestamp" mapstructure:"timestamp"`
	LastModified    *string                `json:"last_modified" mapstructure:"last_modified"`
	UUID            string                 `json:"uuid" mapstructure:"uuid"`
	TitleSort       string                 `json:"title_sort" mapstructure:"title_sort"`
	AuthorLinkMap   map[string]string      `json:"author_link_map" mapstructure:"author_link_map"`
	Title           string                 `json:"title" mapstructure:"title"`
	Identifiers     map[string]string      `json:"identifiers" mapstructure:"identifiers"`
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

func (to *thumbnailOption) validate() {
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

func (to *thumbnailOption) setRezFilter() {
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
