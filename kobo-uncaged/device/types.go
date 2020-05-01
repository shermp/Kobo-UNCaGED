// Copyright 2019-2020 Sherman Perry

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
	"strings"
	"sync"

	"github.com/bamiaux/rez"
	"github.com/geek1011/koboutils/v2/kobo"
	"github.com/julienschmidt/httprouter"
	"github.com/shermp/UNCaGED/uc"
	"github.com/unrolled/render"
)

type cidPrefix string

type firmwareVersion struct {
	major int
	minor int
	build int
}

// KuOptions contains some options that are required
type KuOptions struct {
	PreferSDCard         bool
	PreferKepub          bool
	PasswordList         []string
	EnableDebug          bool
	AddMetadataByTrigger bool
	Thumbnail            thumbnailOption
}

type webStartRes struct {
	opts     KuOptions
	saveOpts bool
	err      error
}

// WebMsg is used to send messages to the web client
type WebMsg struct {
	Head     string
	Body     string
	Footer   string
	Progress int
}

// Kobo contains the variables and methods required to use
// the UNCaGED library
type Kobo struct {
	Device          kobo.Device
	fw              firmwareVersion
	KuConfig        *KuOptions
	DBRootDir       string
	BKRootDir       string
	ContentIDprefix cidPrefix
	useSDCard       bool
	MetadataMap     map[string]uc.CalibreBookMeta
	UpdatedMetadata map[string]struct{}
	BooksInDB       map[string]struct{}
	SeriesIDMap     map[string]string
	Passwords       *uncagedPassword
	DriveInfo       uc.DeviceInfo
	nickelDB        *sql.DB
	Wg              *sync.WaitGroup
	mux             *httprouter.Router
	rend            *render.Render
	readyChan       chan bool
	startChan       chan webStartRes
	MsgChan         chan WebMsg
}

// MetaIterator Kobo UNCaGED to lazy load book metadata
type MetaIterator struct {
	k        *Kobo
	cidList  []string
	cidIndex int
}

// NewMetaIter creates a new MetaIterator for use
func NewMetaIter(k *Kobo) *MetaIterator {
	iter := MetaIterator{k: k, cidIndex: -1}
	return &iter
}

// Add a client ID to the iterator
func (m *MetaIterator) Add(cid string) {
	m.cidList = append(m.cidList, cid)
}

// Next advances the iterator
func (m *MetaIterator) Next() bool {
	m.cidIndex++
	if m.cidIndex < len(m.cidList) {
		return true
	}
	return false
}

// Count gets the number items in the iterator
func (m *MetaIterator) Count() int {
	return len(m.cidList)
}

// Get the metadata of the current iteration
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
