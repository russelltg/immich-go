package fswalker

import (
	"context"
	"io"
	"io/fs"
	"sync"
)

type FsWalker struct {
	ctx       context.Context
	fsys      fs.FS
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

func New(ctx context.Context, fsys fs.FS, recursive bool) (*FsWalker, error) {
	w := FsWalker{
		fsys:      fsys,
		ctx:       ctx,
		recursive: recursive,
	}
	w.start()
	return &w, nil
}

func (w *FsWalker) start() {
	w.fileChan = make(chan fileinfo)
	w.stopChan = make(chan any)
	w.running.Add(1)
	go w.run(w.ctx)
}
func (w *FsWalker) run(ctx context.Context) {
	defer w.running.Done()

	err := fs.WalkDir(w.fsys, ".", func(name string, d fs.DirEntry, err error) error {
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
			return io.EOF
		case w.fileChan <- fileinfo{fullName: name, dirEntry: d}:
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
	return w.fsys.Open(w.currFile.fullName)
}

func (w *FsWalker) Close() error {
	close(w.stopChan)
	<-w.running.Wait()
	return nil
}

func (w *FsWalker) Rewind() error {
	err := w.Close()
	if err != nil {
		return nil
	}
	w.start()
	return nil
}
