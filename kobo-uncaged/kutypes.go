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

import "image"

type cidPrefix string
type koboDevice string
type koboCoverEnding string

type mboxSection int
type kuPrinter interface {
	kuPrintln(section mboxSection, a ...interface{}) (n int, err error)
	kuClose()
}

const (
	header mboxSection = iota
	body
	footer
)

const (
	fullCover koboCoverEnding = " - N3_FULL.parsed"
	libFull   koboCoverEnding = " - N3_LIBRARY_FULL.parsed"
	libGrid   koboCoverEnding = " - N3_LIBRARY_GRID.parsed"
)

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

// CoverSize gets the appropriate cover dimensions for the device.
// These values come from https://github.com/kovidgoyal/calibre/blob/master/src/calibre/devices/kobo/driver.py
func (d koboDevice) CoverSize() (fullCover, libFull, libGrid image.Point) {
	var fc, lf, lg image.Point
	switch d {
	case glo, aura, auraEd2r1, auraEd2r2:
		fc = image.Pt(758, 1024)
		lf = image.Pt(355, 479)
		lg = image.Pt(149, 201)
	case gloHD, claraHD:
		fc = image.Pt(1072, 1448)
		lf = image.Pt(355, 479)
		lg = image.Pt(149, 201)
	case auraHD, auraH2Oed2r1, auraH2Oed2r2:
		fc = image.Pt(1080, 1440)
		lf = image.Pt(355, 471)
		lg = image.Pt(149, 198)
	case auraH2O:
		fc = image.Pt(1080, 1429)
		lf = image.Pt(355, 473)
		lg = image.Pt(149, 198)
	case auraOne, auraOneLE:
		fc = image.Pt(1404, 1872)
		lf = image.Pt(355, 473)
		lg = image.Pt(149, 198)
	case forma, forma32gb:
		fc = image.Pt(1440, 1920)
		lf = image.Pt(398, 530)
		lg = image.Pt(167, 223)
	default:
		fc = image.Pt(600, 800)
		lf = image.Pt(355, 473)
		lg = image.Pt(149, 198)
	}
	return fc, lf, lg
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
