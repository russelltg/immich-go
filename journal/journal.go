package journal

import (
	"strings"

	"github.com/simulot/immich-go/logger"
)

type Journal struct {
	// sync.RWMutex
	// files  map[string]Entries
	counts map[Action]int
	log    logger.Logger
}

// type Entries struct {
// 	terminated bool
// 	entries    []Entry
// }

// type Entry struct {
// 	ts      time.Time
// 	action  Action
// 	comment string
// }

type Action string

const (
	DISCOVERED_FILE  Action = "File"
	SCANNED_IMAGE    Action = "Scanned image"
	SCANNED_VIDEO    Action = "Scanned video"
	DISCARDED        Action = "Discarded"
	UPLOADED         Action = "Uploaded"
	UPGRADED         Action = "Server's asset upgraded"
	ERROR            Action = "Error"
	LOCAL_DUPLICATE  Action = "Local duplicate"
	SERVER_DUPLICATE Action = "Server has photo"
	STACKED          Action = "Stacked"
	SERVER_BETTER    Action = "Server's asset is better"
	ALBUM            Action = "Added to an album"
	LIVE_PHOTO       Action = "Live photo"
	FAILED_VIDEO     Action = "Failed video"
	UNSUPPORTED      Action = "File type not supported"
	METADATA         Action = "Metadata files"
	ASSOCIATED_META  Action = "Associated with metadata"
	INFO             Action = "Info"
)

func NewJournal(log logger.Logger) *Journal {
	return &Journal{
		// files:  map[string]Entries{},
		log:    log,
		counts: map[Action]int{},
	}
}

func (j *Journal) AddEntry(file string, action Action, comment ...string) {
	if j == nil {
		return
	}
	c := strings.Join(comment, ", ")
	if j.log != nil {
		switch action {
		case ERROR:
			j.log.Error("%-25s: %s: %s", action, file, c)
		case UPLOADED, SCANNED_IMAGE, SCANNED_VIDEO:
			j.log.OK("%-25s: %s: %s", action, file, c)
		default:
			j.log.Info("%-25s: %s: %s", action, file, c)
		}
	}
	j.counts[action] = j.counts[action] + 1
}

/*
	func (j *Journal) Counters() map[Action]int {
		counts := map[Action]int{}
		terminated := 0

		for _, es := range j.files {
			for _, e := range es.entries {
				counts[e.action]++
			}
			if es.terminated {
				terminated++
			}
		}
		return counts
	}
*/
func (j *Journal) Report() {
	// counts := j.Counters()

	j.log.OK("Upload report:")
	j.log.OK("%6d files", j.counts[DISCOVERED_FILE])
	j.log.OK("%6d photos", j.counts[SCANNED_IMAGE])
	j.log.OK("%6d videos", j.counts[SCANNED_VIDEO])
	j.log.OK("%6d metadata files", j.counts[METADATA])
	j.log.OK("%6d files having a type not supported", j.counts[UNSUPPORTED])
	j.log.OK("%6d discarded files because in folder failed videos", j.counts[FAILED_VIDEO])
	j.log.OK("%6d errors", j.counts[ERROR])
	j.log.OK("%6d files with metadata", j.counts[ASSOCIATED_META])
	j.log.OK("%6d discarded files because duplicated in the input", j.counts[LOCAL_DUPLICATE])
	j.log.OK("%6d files already on the server", j.counts[SERVER_DUPLICATE])
	j.log.OK("%6d uploaded files on the server", j.counts[UPLOADED])
	j.log.OK("%6d upgraded files on the server", j.counts[UPGRADED])
	j.log.OK("%6d discarded files because of options", j.counts[DISCARDED])
	j.log.OK("%6d discarded files because server has a better image", j.counts[SERVER_BETTER])

}

/*
func (j *Journal) WriteJournal(events ...Action) {
	keys := gen.MapKeys(j.files)
	writeUnhandled := slices.Contains(events, UNHANDLED)
	sort.Strings(keys)
	for _, k := range keys {
		es := j.files[k]
		printFile := true
		mustTerminate := false
		for _, e := range es.entries {
			if slices.Contains(events, e.action) || (writeUnhandled && !es.terminated) {
				mustTerminate = true
				if printFile {
					j.log.OK("File: %s", k)
					printFile = false
				}
			}
			if slices.Contains(events, e.action) {
				j.log.MessageContinue(logger.OK, "\t%s", e.action)
				if len(e.comment) > 0 {
					j.log.MessageContinue(logger.OK, ", %s", e.comment)
				}
			}
		}
		if writeUnhandled && !es.terminated {
			j.log.MessageContinue(logger.OK, "\t%s, missing JSON", UNHANDLED)
		}
		if mustTerminate {
			j.log.MessageTerminate(logger.OK, "")
		}
	}
}
*/
