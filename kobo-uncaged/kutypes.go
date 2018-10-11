package main

import "time"

type koboDeviceID string

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

// KoboMetadata contains the metadata for ebooks on kobo devices.
// It replicates the metadata available in the Kobo USBMS driver.
// Note, pointers are used where necessary to account for null JSON values
type KoboMetadata struct {
	Authors         []string               `json:"authors"`
	Languages       []string               `json:"languages"`
	UserMetadata    map[string]interface{} `json:"user_metadata"`
	UserCategories  map[string]interface{} `json:"user_categories"`
	Comments        *string                `json:"comments"`
	Tags            []string               `json:"tags"`
	Pubdate         *time.Time             `json:"pubdate"`
	SeriesIndex     *float64               `json:"series_index"`
	Thumbnail       interface{}            `json:"thumbnail"`
	PublicationType *string                `json:"publication_type"`
	Mime            *string                `json:"mime"`
	AuthorSort      string                 `json:"author_sort"`
	Series          *string                `json:"series"`
	Rights          *string                `json:"rights"`
	DbID            interface{}            `json:"db_id"`
	Cover           *string                `json:"cover"`
	ApplicationID   int                    `json:"application_id"`
	BookProducer    *string                `json:"book_producer"`
	Size            int                    `json:"size"`
	AuthorSortMap   map[string]string      `json:"author_sort_map"`
	Rating          *float64               `json:"rating"`
	Lpath           string                 `json:"lpath"`
	Publisher       *string                `json:"publisher"`
	Timestamp       time.Time              `json:"timestamp"`
	LastModified    time.Time              `json:"last_modified"`
	UUID            string                 `json:"uuid"`
	TitleSort       string                 `json:"title_sort"`
	AuthorLinkMap   map[string]string      `json:"author_link_map"`
	Title           string                 `json:"title"`
	Identifiers     map[string]string      `json:"identifiers"`
}
