package gp

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/simulot/immich-go/helpers/tzone"
)

type GoogleMetaData struct {
	Title              string         `json:"title"`
	Description        string         `json:"description"`
	Category           string         `json:"category"`
	DatePresent        googIsPresent  `json:"date"` // true when the file is a folder metadata
	PhotoTakenTime     googTimeObject `json:"photoTakenTime"`
	GeoDataExif        googGeoData    `json:"geoDataExif"`
	Trashed            bool           `json:"trashed,omitempty"`
	Archived           bool           `json:"archived,omitempty"`
	URLPresent         googIsPresent  `json:"url"`                 // true when the file is an asset metadata
	Favorited          bool           `json:"favorited,omitempty"` // true when starred in GP
	GooglePhotosOrigin struct {
		FromPartnerSharing googIsPresent `json:"fromPartnerSharing"` // true when this is a partner's asset
	} `json:"googlePhotosOrigin"`
	foundInPaths []string // Not in the JSON, keep track of paths where the json has been found
	// inAlbums     []fileWalkerPath // list albums that contain this file
}

func (gmd GoogleMetaData) isAlbum() bool {
	return bool(gmd.DatePresent)
}

func (gmd GoogleMetaData) isPartner() bool {
	return bool(gmd.GooglePhotosOrigin.FromPartnerSharing)
}

func (gmd GoogleMetaData) isAsset() bool {
	return bool(gmd.URLPresent)
}

// Key return an expected unique key for the asset
// based on the title and the timestamp
func (md GoogleMetaData) Key() string {
	return fmt.Sprintf("%s,%s", md.Title, md.PhotoTakenTime.Timestamp)
}

// googIsPresent is set when the field is present. The content of the field is not relevant
type googIsPresent bool

func (p *googIsPresent) UnmarshalJSON(b []byte) error {
	var bl bool
	err := json.Unmarshal(b, &bl)
	if err == nil {
		return nil
	}

	*p = len(b) > 0
	return nil
}

// googGeoData contains GPS coordinates
type googGeoData struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Altitude  float64 `json:"altitude"`
}

// googTimeObject to handle the epoch timestamp
type googTimeObject struct {
	Timestamp string `json:"timestamp"`
	// Formatted string    `json:"formatted"`
}

// Time return the time.Time of the epoch
func (gt googTimeObject) Time() time.Time {
	ts, _ := strconv.ParseInt(gt.Timestamp, 10, 64)
	t := time.Unix(ts, 0)
	local, _ := tzone.Local()
	//	t = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), time.UTC)
	return t.In(local)
}
