package tarwalker

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"strings"
	"sync"
)

// TarWalker implement a Waler for tar kind of archives
// Next can't be called until the previous file isn't closed
type TarWalker struct {
	fileReader *os.File
	gzipReader *gzip.Reader
	tarReader  *tar.Reader
	hdr        *tar.Header
	opener     func(tr *TarWalker) error
	closer     func(tr *TarWalker) error
	name       string
	lockNext   sync.RWMutex // Prevent calling Next before the previous file is close
}

// Open a tar, tgz or tar.gz file
func Open(ctx context.Context, pathName string) (*TarWalker, error) {
	ext := path.Ext(pathName)
	extExt := path.Ext(strings.TrimSuffix(pathName, ext))

	ext = strings.ToLower(ext)
	extExt = strings.ToLower(extExt)
	switch {
	case ext == ".tar":
		return OpenTar(ctx, pathName)
	case ext == ".tgz":
		return OpenTgz(ctx, pathName)
	case extExt == ".tar" && ext == ".gz":
		return OpenTgz(ctx, pathName)
	}
	return nil, fmt.Errorf("unknown file format: %s", pathName)
}

// OpenTar opens a .tar file
func OpenTar(ctx context.Context, pathName string) (*TarWalker, error) {
	tr := newReader(ctx, pathName, func(w *TarWalker) error {
		f, err := os.Open(pathName)
		if err != nil {
			return err
		}
		w.fileReader = f
		w.tarReader = tar.NewReader(f)
		return nil
	}, func(w *TarWalker) error {
		w.tarReader = nil
		return w.fileReader.Close()

	})
	return tr, nil
}

// OpenTgz open a .tar.gz file or a .tgz file
func OpenTgz(ctx context.Context, pathName string) (*TarWalker, error) {
	tr := newReader(ctx, pathName, func(w *TarWalker) error {
		f, err := os.Open(pathName)
		if err != nil {
			return err
		}
		w.fileReader = f
		g, err := gzip.NewReader(f)
		if err != nil {
			return err
		}
		w.gzipReader = g
		w.tarReader = tar.NewReader(g)
		return nil
	}, func(w *TarWalker) error {
		var err error
		w.tarReader = nil
		errors.Join(err, w.gzipReader.Close())
		errors.Join(err, w.fileReader.Close())
		return err
	})
	return tr, nil
}

func newReader(ctx context.Context, name string, opener func(w *TarWalker) error, closer func(w *TarWalker) error) *TarWalker {
	w := TarWalker{
		name:   name,
		opener: opener,
		closer: closer,
	}

	w.opener(&w)
	return &w
}

// Name returns the walker's name for log purpose
func (w *TarWalker) Name() string { return w.name }

// Next return the next entry of the tar file
func (w *TarWalker) Next() (string, fs.DirEntry, error) {
	w.lockNext.RLock() // Block if the previous file is still opened
	defer w.lockNext.RUnlock()
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

// dirEntry implements the fs.DirEntry interface from a fs.FileInfo
type dirEntry struct {
	fs.FileInfo
}

func (d *dirEntry) Info() (fs.FileInfo, error) {
	return d.FileInfo, nil
}
func (d *dirEntry) Type() fs.FileMode {
	return d.Mode()
}

// Close the tar walker. It close all underlying files, gzip and tar reader
func (w *TarWalker) Close() error {
	return w.closer(w)
}

// Rewind close and reopen the walker, ready for another walk
func (w *TarWalker) Rewind() error {
	err := w.Close()
	if err != nil {
		return err
	}
	return w.opener(w)
}

// Open the current file
func (w *TarWalker) Open() (fs.File, error) {
	// Lock the Next function
	w.lockNext.Lock()
	return &file{w: w}, nil
}

// file implement the fs.File from a *tar.Reader
type file struct {
	w *TarWalker
}

func (f *file) Read(b []byte) (int, error) {
	return f.w.tarReader.Read(b)
}

func (f *file) Close() error {
	// Unlock the Next function
	f.w.lockNext.Unlock()
	return nil
}
func (f *file) Stat() (fs.FileInfo, error) {
	return f.w.hdr.FileInfo(), nil
}
