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
	"bufio"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/bamiaux/rez"
	"github.com/godbus/dbus/v5"
	"github.com/julienschmidt/httprouter"
	"github.com/pgaskin/koboutils/v2/kobo"
	"github.com/shermp/UNCaGED/uc"
	"github.com/unrolled/render"
)

type cidPrefix string

type firmwareVersion string

// KuOptions contains some options that are required
type KuOptions struct {
	PreferSDCard bool            `json:"preferSDCard"`
	PreferKepub  bool            `json:"preferKepub"`
	EnableDebug  bool            `json:"enableDebug"`
	Thumbnail    thumbnailOption `json:"thumbnail"`
}

type webUIinfo struct {
	KUVersion      string `json:"kuVersion"`
	StorageType    string `json:"storageType"`
	ScreenDPI      int    `json:"screenDPI"`
	ExitPath       string `json:"exitPath"`
	DisconnectPath string `json:"disconnectPath"`
	AuthPath       string `json:"authPath"`
	SSEPath        string `json:"ssePath"`
	ConfigPath     string `json:"configPath"`
	InstancePath   string `json:"instancePath"`
}

type webConfig struct {
	Opts     KuOptions `json:"opts"`
	SaveOpts bool      `json:"saveOpts"`
	err      error
}

// WebMsg is used to send messages to the web client
type WebMsg struct {
	ShowMessage    string
	Progress       int
	GetPassword    bool
	GetCalInstance bool
	Finished       string
}

type calPassCache map[string]*calPassword

type calPassword struct {
	Attempts int    `json:"attempts"`
	LibName  string `json:"libName"`
	Password string `json:"password"`
}

// Kobo contains the variables and methods required to use
// the UNCaGED library
type Kobo struct {
	KuVers          string
	Device          kobo.Device
	fw              firmwareVersion
	KuConfig        *KuOptions
	DBRootDir       string
	BKRootDir       string
	ContentIDprefix cidPrefix
	UseSDCard       bool
	MetadataMap     map[string]uc.CalibreBookMeta
	UpdatedMetadata map[string]struct{}
	BooksInDB       map[string]struct{}
	SeriesIDMap     map[string]string
	PassCache       calPassCache
	DriveInfo       uc.DeviceInfo
	Wg              *sync.WaitGroup
	mux             *httprouter.Router
	rend            *render.Render
	webInfo         *webUIinfo
	replSQLWriter   *sqlWriter
	ndbConn         *dbus.Conn
	ndbObj          dbus.BusObject
	calInstances    []uc.CalInstance
	useNDB          bool
	FinishedMsg     string
	BrowserOpen     bool
	doneChan        chan bool
	startChan       chan webConfig
	MsgChan         chan WebMsg
	AuthChan        chan *calPassword
	exitChan        chan bool
	UCExitChan      chan<- bool
	calInstChan     chan uc.CalInstance
	viewSignal      chan *dbus.Signal
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
	GenerateLevel   string `json:"generateLevel"`
	ResizeAlgorithm string `json:"resizeAlgorithm"`
	JpegQuality     int    `json:"jpegQuality"`
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

type sqlWriter struct {
	sqlFile       *os.File
	sqlBuffWriter *bufio.Writer
}

func newSQLWriter(path string) (*sqlWriter, error) {
	var err error
	s := &sqlWriter{}
	if s.sqlFile, err = os.Create(path); err != nil {
		return nil, err
	}
	s.sqlBuffWriter = bufio.NewWriter(s.sqlFile)
	return s, nil
}

func (s *sqlWriter) writeBegin() {
	s.sqlBuffWriter.WriteString("BEGIN;\n")
}

func (s *sqlWriter) writeCommit() {
	s.sqlBuffWriter.WriteString("COMMIT;\n")
}

func (s *sqlWriter) writeQuery(query string) {
	s.sqlBuffWriter.WriteString(query)
	// Make sure our query always ends with a terminating ';'
	if !strings.HasSuffix(query, ";") {
		s.sqlBuffWriter.WriteRune(';')
	}
	s.sqlBuffWriter.WriteRune('\n')
}

func (s *sqlWriter) close() {
	defer s.sqlFile.Close()
	s.sqlBuffWriter.Flush()
}
