/*
	This package implement an abstraction around a regular file system, a zip archive and a tgz archive.
	The limited set of possibilities of tgz set the limits to the other type of archive

	It provides:
	- Next()  error
	- Open() (io.ReadCloser,error)


*/

package archwalker

import (
	"io"
	"io/fs"
)

type ArchWalker interface {
	Next() (string, fs.FileInfo, error) // FilePath, FileInfo, Error
	Open() (io.ReadCloser, error)
}
