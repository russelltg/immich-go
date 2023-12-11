package fswalker

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"sync"
)

type FsWalker struct {
	recursive bool
	fileChan  chan fileinfo
	stopChan  chan any
	running   sync.WaitGroup
	currFile  fileinfo
}

type fileinfo struct {
	err      error
	fullName string
	dirEntry fs.DirEntry
}

func New(ctx context.Context, pathName string, recursive bool) (*FsWalker, error) {
	info, err := os.Stat(pathName)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("'%s' isn't a directory", pathName)
	}

	w := FsWalker{
		recursive: recursive,
		fileChan:  make(chan fileinfo),
		stopChan:  make(chan any),
	}
	w.running.Add(1)
	go w.run(ctx, pathName)
	return &w, nil
}

func (w *FsWalker) run(ctx context.Context, p string) {
	defer w.running.Done()
	fsys := os.DirFS(p)

	err := fs.WalkDir(fsys, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if name == "." || w.recursive {
				return nil
			}
			return fs.SkipDir
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-w.stopChan:
			return fs.SkipAll
		case w.fileChan <- fileinfo{fullName: path.Join(p, name), dirEntry: d}:
		}
		return nil
	})
	if err != nil {
		w.fileChan <- fileinfo{err, "", nil}
		return
	}
	w.fileChan <- fileinfo{io.EOF, "", nil}
}

func (w *FsWalker) Next() (string, fs.DirEntry, error) {
	info := <-w.fileChan
	if info.err != nil {
		return "", nil, info.err
	}
	w.currFile = info
	return info.fullName, info.dirEntry, nil
}

func (w *FsWalker) Open() (fs.File, error) {
	return os.Open(w.currFile.fullName)
}

func (w *FsWalker) Close() {
	close(w.stopChan)
	w.running.Wait()
}
