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
	name        string
}

func New(ctx context.Context, pathName string) (*ZipWalker, error) {
	zr, err := zip.OpenReader(pathName)
	if err != nil {
		return nil, err
	}

	w := ZipWalker{
		zipReader:   zr,
		currentFile: -1,
		name:        pathName,
	}
	w.files = w.zipReader.File
	return &w, nil
}

func (w ZipWalker) Name() string { return w.name }

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

func (w *ZipWalker) Close() error {
	return w.zipReader.Close()
}

func (w *ZipWalker) Rewind() error {
	w.currentFile = -1
	return nil
}
