/*
	This package implement an abstraction around a regular file system, a zip archive and a tgz archive.
	The limited set of possibilities of tgz set the limits to the other type of archive

	It provides:
	- Next()  error
	- Open() (fs.File,error)
	- Close() error
*/

package archwalker

import (
	"io/fs"
)

type Walker interface {
	Name() string                       // Walker's name for logs
	Next() (string, fs.DirEntry, error) // Seek the next file, and return file's information
	Open() (fs.File, error)             // Open the last sought file
	Close() error                       // Close the walker
	Rewind() error                      // Start over at the beginning of the walker
}
