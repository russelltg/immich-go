package gp

import (
	"context"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/simulot/immich-go/browser"
	"github.com/simulot/immich-go/helpers/archwalker"
	"github.com/simulot/immich-go/helpers/fshelper"
	"github.com/simulot/immich-go/helpers/gen"
	"github.com/simulot/immich-go/journal"
	"github.com/simulot/immich-go/logger"
)

type Takeout struct {
	walkers    []archwalker.Walker
	catalogs   map[archwalker.Walker]walkerCatalog // file catalogs by walker
	jsonByYear map[jsonKey]*GoogleMetaData         // assets by year of capture and base name
	uploaded   map[fileKey]any                     // track files already uploaded
	albums     map[string]string                   // tack album names by folder
	log        logger.Logger
	conf       *browser.Configuration
}

// walkerCatalog collects all directory catalogs
type walkerCatalog map[string]directoryCatalog // by directory in the walker

// directoryCatalog captures all files in a given directory
type directoryCatalog struct {
	// isAlbum    bool                // true when the directory is recognized as an album
	// albumTitle string              // album title from album's metadata
	files map[string]fileInfo // map of fileInfo by base name
}

// fileInfo keep information collected during pass one
type fileInfo struct {
	length int             // file length in bytes
	md     *GoogleMetaData // will point to the associated metadata
}

// fileKey is the key of the uploaded files map
type fileKey struct {
	base   string
	length int
	year   int
}

// jsonKey allow to map jsons by base name and year of capture
type jsonKey struct {
	name string
	year int
}

type fileWalkerPath struct {
	w archwalker.Walker
	p string
}

func NewTakeout(ctx context.Context, log logger.Logger, conf *browser.Configuration, walkers []archwalker.Walker) (*Takeout, error) {
	to := Takeout{
		walkers:    walkers,
		jsonByYear: map[jsonKey]*GoogleMetaData{},
		albums:     map[string]string{},
		log:        log,
		conf:       conf,
	}
	err := to.passOne(ctx)
	if err != nil {
		return nil, err
	}

	to.solvePuzzle(ctx)
	return &to, err
}

// passOne scans all files in all walker to build the file catalog of the archive
// metadata files content is read and kept

func (to *Takeout) passOne(ctx context.Context) error {
	to.catalogs = map[archwalker.Walker]walkerCatalog{}
	for _, w := range to.walkers {
		to.catalogs[w] = walkerCatalog{}
		wName := w.Name()
		to.log.OK("Scanning the Google Photos takeout: %s", wName)
	nextWalker:
		for {
			select {
			case <-ctx.Done():
				// Check if the context has been cancelled
				return ctx.Err()
			default:
				name, dirInfo, err := w.Next()
				if err == io.EOF {
					break nextWalker
				}
				if err != nil {
					return err
				}

				dir, base := path.Split(name)
				dir = strings.TrimSuffix(dir, "/")
				ext := strings.ToLower(path.Ext(name))
				fullName := path.Join(wName, name)
				to.conf.Journal.AddEntry(fullName, journal.DISCOVERED_FILE, "")

				dirCatalog := to.catalogs[w][dir]
				if dirCatalog.files == nil {
					dirCatalog.files = map[string]fileInfo{}
				}
				finfo, err := dirInfo.Info()
				if err != nil {
					return err
				}

				switch ext {
				case ".json":
					md, err := fshelper.ReadAndCloseJSON[GoogleMetaData](w.Open())
					if err == nil {
						switch {
						case md.isAsset():
							to.addJson(w, dir, base, md)
							to.conf.Journal.AddEntry(fullName, journal.METADATA, "Asset Title: "+md.Title)
						case md.isAlbum():
							to.albums[dir] = md.Title
							to.conf.Journal.AddEntry(fullName, journal.METADATA, "Album title: "+md.Title)
						default:
							to.conf.Journal.AddEntry(fullName, journal.ERROR, "Unknown json file")
							continue
						}
					} else {
						to.conf.Journal.AddEntry(fullName, journal.ERROR, "Unknown json file")
						continue
					}
				default:
					m, err := fshelper.MimeFromExt(ext)
					if err != nil {
						to.conf.Journal.AddEntry(fullName, journal.UNSUPPORTED, "")
						continue
					}

					if strings.Contains(fullName, "Failed Videos") {
						to.conf.Journal.AddEntry(fullName, journal.FAILED_VIDEO, "")
						continue
					}
					dirCatalog.files[base] = fileInfo{
						length: int(finfo.Size()),
					}
					ss := strings.Split(m[0], "/")
					if ss[0] == "image" {
						to.conf.Journal.AddEntry(fullName, journal.SCANNED_IMAGE, "")
					} else {
						to.conf.Journal.AddEntry(fullName, journal.SCANNED_VIDEO, "")
					}
				}
				to.catalogs[w][dir] = dirCatalog
			}
		}
	}
	to.log.OK("Scanning the Google Photos takeout, pass one completed.")
	return nil
}

// addJson stores metadata and all paths where the combo base+year has been found
func (to *Takeout) addJson(w archwalker.Walker, dir, base string, md *GoogleMetaData) {
	k := jsonKey{
		name: base,
		year: md.PhotoTakenTime.Time().Year(),
	}

	if mdPresent, ok := to.jsonByYear[k]; ok {
		md = mdPresent
	}
	md.foundInPaths = append(md.foundInPaths, dir)
	to.jsonByYear[k] = md
}

type matcherFn func(jsonName string, fileName string) bool

// matchers is a list of matcherFn from the most likely to be used to the least one
var matchers = []matcherFn{
	normalMatch,
	matchWithOneCharOmitted,
	matchVeryLongNameWithNumber,
	matchDuplicateInYear,
	matchEditedName,
	matchForgottenDuplicates,
}

// solvePuzzle prepares metadata with information collected during pass one for each accepted files
//
// JSON files give important information about the relative photos / movies:
//   - The original name (useful when it as been truncated)
//   - The date of capture (useful when the files doesn't have this date)
//   - The GPS coordinates (will be useful in a future release)
//
// Each JSON is checked. JSON is duplicated in albums folder.
// Associated files with the JSON can be found in the JSON's folder, or in the Year photos.
// Once associated and sent to the main program, files are tagged for not been associated with an other one JSON.
// Association is done with the help of a set of matcher functions. Each one implement a rule
//
// 1 JSON can be associated with 1+ files that have a part of their name in common.
// -   the file is named after the JSON name
// -   the file name can be 1 UTF-16 char shorter (🤯) than the JSON name
// -   the file name is longer than 46 UTF-16 chars (🤯) is truncated. But the truncation can creates duplicates, then a number is added.
// -   if there are several files with same original name, the first instance kept as it is, the next has a sequence number.
//       File is renamed as IMG_1234(1).JPG and the JSON is renamed as IMG_1234.JPG(1).JSON
// -   of course those rules are likely to collide. They have to be applied from the most common to the least one.
// -   sometimes the file isn't in the same folder than the json... It can be found in Year's photos folder
//
// The duplicates files (same name, same length in bytes) found in the local source are discarded before been presented to the immich server.
//

func (to *Takeout) solvePuzzle(ctx context.Context) error {
	jsonKeys := gen.MapKeys(to.jsonByYear)
	sort.Slice(jsonKeys, func(i, j int) bool {
		yd := jsonKeys[i].year - jsonKeys[j].year
		switch {
		case yd < 0:
			return true
		case yd > 0:
			return false
		}
		return jsonKeys[i].name < jsonKeys[j].name
	})

	// For the most common matcher to the least,
	for _, matcher := range matchers {
		// Check files that match each json files
		for _, k := range jsonKeys {
			md := to.jsonByYear[k]

			// list of paths where to search the assets: paths where this json has been found + year path in all of the walkers
			paths := map[string]any{}
			paths[path.Join(path.Dir(md.foundInPaths[0]), fmt.Sprintf("Photos from %d", md.PhotoTakenTime.Time().Year()))] = nil
			for _, d := range md.foundInPaths {
				paths[d] = nil
			}
			for d := range paths {
				for _, w := range to.walkers {
					l := to.catalogs[w][d]
					for f := range l.files {
						if l.files[f].md == nil {
							if matcher(k.name, f) {
								to.conf.Journal.AddEntry(path.Join(w.Name(), d, f), journal.ASSOCIATED_META, fmt.Sprintf("%s (%d)", k.name, k.year))
								// if not already matched
								i := l.files[f]
								i.md = md
								l.files[f] = i
							}
						}
					}
					to.catalogs[w][d] = l
				}
			}
		}
	}
	return nil
}

// normalMatch
//
//	PXL_20230922_144936660.jpg.json
//	PXL_20230922_144936660.jpg
func normalMatch(jsonName string, fileName string) bool {
	base := strings.TrimSuffix(jsonName, path.Ext(jsonName))
	return base == fileName
}

// matchWithOneCharOmitted
//
//	PXL_20230809_203449253.LONG_EXPOSURE-02.ORIGIN.json
//	PXL_20230809_203449253.LONG_EXPOSURE-02.ORIGINA.jpg
//
//	05yqt21kruxwwlhhgrwrdyb6chhwszi9bqmzu16w0 2.jp.json
//	05yqt21kruxwwlhhgrwrdyb6chhwszi9bqmzu16w0 2.jpg
//
//  😀😃😄😁😆😅😂🤣🥲☺️😊😇🙂🙃😉😌😍🥰😘😗😙😚😋.json
//  😀😃😄😁😆😅😂🤣🥲☺️😊😇🙂🙃😉😌😍🥰😘😗😙😚😋😛.jpg

func matchWithOneCharOmitted(jsonName string, fileName string) bool {
	base := strings.TrimSuffix(jsonName, path.Ext(jsonName))
	if strings.HasPrefix(fileName, base) {
		if fshelper.IsExtensionPrefix(path.Ext(base)) {
			// Trim only if the EXT is known extension, and not .COVER or .ORIGINAL
			base = strings.TrimSuffix(base, path.Ext(base))
		}
		fileName = strings.TrimSuffix(fileName, path.Ext(fileName))
		a, b := utf8.RuneCountInString(fileName), utf8.RuneCountInString(base)
		if a-b <= 1 {
			return true
		}
	}
	return false
}

// matchVeryLongNameWithNumber
//
//	Backyard_ceremony_wedding_photography_xxxxxxx_(494).json
//	Backyard_ceremony_wedding_photography_xxxxxxx_m(494).jpg
func matchVeryLongNameWithNumber(jsonName string, fileName string) bool {
	jsonName = strings.TrimSuffix(jsonName, path.Ext(jsonName))

	p1JSON := strings.Index(jsonName, "(")
	if p1JSON < 0 {
		return false
	}
	p2JSON := strings.Index(jsonName, ")")
	if p2JSON < 0 || p2JSON != len(jsonName)-1 {
		return false
	}
	p1File := strings.Index(fileName, "(")
	if p1File < 0 || p1File != p1JSON+1 {
		return false
	}
	if jsonName[:p1JSON] != fileName[:p1JSON] {
		return false
	}
	p2File := strings.Index(fileName, ")")
	return jsonName[p1JSON+1:p2JSON] == fileName[p1File+1:p2File]
}

// matchDuplicateInYear
//
//	IMG_3479.JPG(2).json
//	IMG_3479(2).JPG
func matchDuplicateInYear(jsonName string, fileName string) bool {
	jsonName = strings.TrimSuffix(jsonName, path.Ext(jsonName))
	p1JSON := strings.Index(jsonName, "(")
	if p1JSON < 1 {
		return false
	}
	p2JSON := strings.Index(jsonName, ")")
	if p2JSON < 0 || p2JSON != len(jsonName)-1 {
		return false
	}

	num := jsonName[p1JSON:]
	jsonName = strings.TrimSuffix(jsonName, num)
	ext := path.Ext(jsonName)
	jsonName = strings.TrimSuffix(jsonName, ext) + num + ext
	return jsonName == fileName
}

// matchEditedName
//   PXL_20220405_090123740.PORTRAIT.jpg.json
//   PXL_20220405_090123740.PORTRAIT.jpg
//   PXL_20220405_090123740.PORTRAIT-modifié.jpg

func matchEditedName(jsonName string, fileName string) bool {
	base := strings.TrimSuffix(jsonName, path.Ext(jsonName))
	ext := path.Ext(base)
	if ext != "" {
		if _, err := fshelper.MimeFromExt(ext); err == nil {
			base := strings.TrimSuffix(base, ext)
			fname := strings.TrimSuffix(fileName, path.Ext(fileName))
			return strings.HasPrefix(fname, base)
		}
	}
	return false
}

//TODO: This one interferes with matchVeryLongNameWithNumber

// matchForgottenDuplicates
// original_1d4caa6f-16c6-4c3d-901b-9387de10e528_.json
// original_1d4caa6f-16c6-4c3d-901b-9387de10e528_P.jpg
// original_1d4caa6f-16c6-4c3d-901b-9387de10e528_P(1).jpg

func matchForgottenDuplicates(jsonName string, fileName string) bool {
	jsonName = strings.TrimSuffix(jsonName, path.Ext(jsonName))
	fileName = strings.TrimSuffix(fileName, path.Ext(fileName))
	if strings.HasPrefix(fileName, jsonName) {
		a, b := utf8.RuneCountInString(jsonName), utf8.RuneCountInString(fileName)
		if b-a < 10 {
			return true
		}
	}
	return false
}

// Browse return a channel of assets
//
// Walkers are rewind, and scanned again
// each file net yet sent to immich is sent with associated metadata

func (to *Takeout) Browse(ctx context.Context) chan *browser.LocalAssetFile {
	to.uploaded = map[fileKey]any{}
	assetChan := make(chan *browser.LocalAssetFile)
	go func() {
		defer close(assetChan)

		for _, w := range to.walkers {
			err := w.Rewind()
			if err != nil {
				assetChan <- &browser.LocalAssetFile{
					Err: err,
				}
			}
			for {
				name, dirInfo, err := w.Next()
				if err == io.EOF {
					break // next walker
				}
				if err != nil {
					to.log.Error("can't browse: %s", err)
					return
				}

				dir, base := path.Split(name)
				dir = strings.TrimSuffix(dir, "/")
				ext := path.Ext(base)
				if _, err := fshelper.MimeFromExt(ext); err != nil {
					continue
				}
				fullPath := path.Join(w.Name(), name)
				f, exist := to.catalogs[w][dir].files[base]
				if !exist {
					// this file isn't an asset
					continue
				}

				if f.md == nil {
					to.conf.Journal.AddEntry(fullPath, journal.ERROR, "JSON File not found for this file")
					continue
				}
				finfo, err := dirInfo.Info()
				if err != nil {
					to.log.Error("can't browse: %s", err)
					return
				}

				key := fileKey{
					base:   base,
					length: int(finfo.Size()),
					year:   f.md.PhotoTakenTime.Time().Year(),
				}
				if _, exists := to.uploaded[key]; exists {
					to.conf.Journal.AddEntry(fullPath, journal.LOCAL_DUPLICATE, "")
					continue
				}
				a := to.googleMDToAsset(f.md, key)
				select {
				case <-ctx.Done():
					return
				case assetChan <- a:
					to.uploaded[key] = nil
				}
			}
		}
	}()

	return assetChan
}

// googleMDToAsset makes a localAssetFile based on the google metadata
func (to *Takeout) googleMDToAsset(md *GoogleMetaData, key fileKey) *browser.LocalAssetFile {
	// Change file's title with the asset's title and the actual file's extension
	title := md.Title
	titleExt := path.Ext(title)
	fileExt := path.Ext(key.base)
	if titleExt != fileExt {
		title = strings.TrimSuffix(title, titleExt)
		titleExt = path.Ext(title)
		if titleExt != fileExt {
			title = strings.TrimSuffix(title, titleExt) + fileExt
		}
	}

	a := browser.LocalAssetFile{
		FileName:    key.base,
		FileSize:    key.length,
		Title:       title,
		Description: md.Description,
		Altitude:    md.GeoDataExif.Altitude,
		Latitude:    md.GeoDataExif.Latitude,
		Longitude:   md.GeoDataExif.Longitude,
		Archived:    md.Archived,
		FromPartner: md.isPartner(),
		Trashed:     md.Trashed,
		DateTaken:   md.PhotoTakenTime.Time(),
		Favorite:    md.Favorited,
	}

	for _, p := range md.foundInPaths {
		if album, exists := to.albums[p]; exists {
			a.Albums = append(a.Albums, browser.LocalAlbum{Path: p, Name: album})
		}
	}
	return &a
}
