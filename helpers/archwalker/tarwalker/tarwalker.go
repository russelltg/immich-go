package tarwalker

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path"
	"strings"
)

type TarWalker struct {
	fileReader *os.File
	gzipReader *gzip.Reader
	tarReader  *tar.Reader
	hdr        *tar.Header
}

func New(ctx context.Context, pathName string) (*TarWalker, error) {
	ext := path.Ext(pathName)
	extExt := path.Ext(strings.TrimSuffix(pathName, ext))

	ext = strings.ToLower(ext)
	extExt = strings.ToLower(extExt)
	switch {
	case ext == ".tar":
		return NewTar(ctx, pathName)
	case ext == ".tgz":
		return NewTgz(ctx, pathName)
	case extExt == ".tar" && ext == ".gz":
		return NewTgz(ctx, pathName)
	}
	return nil, fmt.Errorf("unknown file format: %s", pathName)
}

func NewTar(ctx context.Context, pathName string) (*TarWalker, error) {
	f, err := os.Open(pathName)
	if err != nil {
		return nil, err
	}
	r := tar.NewReader(f)
	tr := newReader(ctx, r)
	tr.fileReader = f
	return tr, nil
}

func NewTgz(ctx context.Context, pathName string) (*TarWalker, error) {
	f, err := os.Open(pathName)
	if err != nil {
		return nil, err
	}
	g, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	r := tar.NewReader(g)
	tr := newReader(ctx, r)
	tr.fileReader = f
	tr.gzipReader = g
	return tr, nil
}

func newReader(ctx context.Context, tr *tar.Reader) *TarWalker {

	w := TarWalker{
		tarReader: tr,
	}
	return &w
}

func (w *TarWalker) Next() (string, fs.DirEntry, error) {
	var err error
	for {
		w.hdr, err = w.tarReader.Next()
		if err != nil {
			return "", nil, err
		}
		if !w.hdr.FileInfo().IsDir() {
			break
		}
	}
	return w.hdr.Name, &dirEntry{FileInfo: w.hdr.FileInfo()}, nil
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

func (w *TarWalker) Close() {
	if w.gzipReader != nil {
		w.gzipReader.Close()
	}
	if w.fileReader != nil {
		w.fileReader.Close()
	}
}

func (w *TarWalker) Open() (fs.File, error) {
	return &file{w: w}, nil
}

type file struct {
	w *TarWalker
}

func (f *file) Read(b []byte) (int, error) {
	return f.w.tarReader.Read(b)
}

func (f *file) Close() error {
	return nil
}
func (f *file) Stat() (fs.FileInfo, error) {
	return f.w.hdr.FileInfo(), nil
}
