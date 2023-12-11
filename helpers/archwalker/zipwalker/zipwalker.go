package zipwalker

import (
	"archive/zip"
	"context"
	"io"
	"io/fs"
)

type ZipWalker struct {
	zipReader   *zip.ReadCloser
	files       []*zip.File
	currentFile int
}

func New(ctx context.Context, pathName string) (*ZipWalker, error) {
	zr, err := zip.OpenReader(pathName)
	if err != nil {
		return nil, err
	}

	w := ZipWalker{
		zipReader:   zr,
		currentFile: -1,
	}
	w.files = w.zipReader.File
	return &w, nil
}

func (w *ZipWalker) Next() (string, fs.DirEntry, error) {
	var f *zip.File
	for {
		w.currentFile++
		if w.currentFile >= len(w.files) {
			return "", nil, io.EOF
		}
		f = w.files[w.currentFile]
		if !f.FileInfo().IsDir() {
			break
		}
	}
	return f.Name, &dirEntry{f.FileInfo()}, nil

}

type dirEntry struct {
	fs.FileInfo
}

func (d *dirEntry) Info() (fs.FileInfo, error) {
	return d.FileInfo, nil
}
func (d *dirEntry) Type() fs.FileMode {
	return d.Mode()
}

func (w *ZipWalker) Open() (fs.File, error) {
	return w.zipReader.Open(w.files[w.currentFile].Name)
}

func (w *ZipWalker) Close() {
	w.zipReader.Close()
}
