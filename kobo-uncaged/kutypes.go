package main

import "time"

type cidPrefix string
type koboDeviceID string
type koboCoverEnding string

type kuPrinter interface {
	kuPrintln(a ...interface{}) (n int, err error)
	kuClose()
}

// Kobo model ID's from https://github.com/geek1011/KoboStuff/blob/gh-pages/kobofirmware.js#L11
const (
	touchAB      koboDeviceID = "00000000-0000-0000-0000-000000000310"
	touchC       koboDeviceID = "00000000-0000-0000-0000-000000000320"
	mini         koboDeviceID = "00000000-0000-0000-0000-000000000340"
	glo          koboDeviceID = "00000000-0000-0000-0000-000000000330"
	gloHD        koboDeviceID = "00000000-0000-0000-0000-000000000371"
	touch2       koboDeviceID = "00000000-0000-0000-0000-000000000372"
	aura         koboDeviceID = "00000000-0000-0000-0000-000000000360"
	auraHD       koboDeviceID = "00000000-0000-0000-0000-000000000350"
	auraH2O      koboDeviceID = "00000000-0000-0000-0000-000000000370"
	auraH2Oed2r1 koboDeviceID = "00000000-0000-0000-0000-000000000374"
	auraH2Oed2r2 koboDeviceID = "00000000-0000-0000-0000-000000000378"
	auraOne      koboDeviceID = "00000000-0000-0000-0000-000000000373"
	auraOneLE    koboDeviceID = "00000000-0000-0000-0000-000000000381"
	auraEd2r1    koboDeviceID = "00000000-0000-0000-0000-000000000375"
	auraEd2r2    koboDeviceID = "00000000-0000-0000-0000-000000000379"
	claraHD      koboDeviceID = "00000000-0000-0000-0000-000000000376"
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
	Pubdate         *time.Time             `json:"pubdate" mapstructure:"pubdate"`
	SeriesIndex     *float64               `json:"series_index" mapstructure:"series_index"`
	Thumbnail       interface{}            `json:"thumbnail" mapstructure:"thumbnail"`
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
	Timestamp       time.Time              `json:"timestamp" mapstructure:"timestamp"`
	LastModified    time.Time              `json:"last_modified" mapstructure:"last_modified"`
	UUID            string                 `json:"uuid" mapstructure:"uuid"`
	TitleSort       string                 `json:"title_sort" mapstructure:"title_sort"`
	AuthorLinkMap   map[string]string      `json:"author_link_map" mapstructure:"author_link_map"`
	Title           string                 `json:"title" mapstructure:"title"`
	Identifiers     map[string]string      `json:"identifiers" mapstructure:"identifiers"`
}

type coverDims struct {
	width  int
	height int
}
