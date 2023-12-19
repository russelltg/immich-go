package fshelper

import (
	"encoding/json"
	"io/fs"
)

// OpenJSON reads a JSON file from the provided file system (fs.FS)
// with the given name and unmarshals it into the provided type T.

func OpenJSON[T any](FSys fs.FS, name string) (*T, error) {
	return ReadAndCloseJSON[T](FSys.Open(name))
}

func ReadAndCloseJSON[T any](f fs.File, err error) (*T, error) {
	var object T
	if err != nil {
		return nil, err
	}
	defer f.Close()
	err = json.NewDecoder(f).Decode(&object)
	return &object, err
}
