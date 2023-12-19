package fswalker

import (
	"context"
	"io"
	"io/fs"
)

type FsWalker struct {
	ctx       context.Context
	fsys      fs.FS
	recursive bool
	curIdx    int
	entries   []fileInfo
	name      string
}

type fileInfo struct {
	fullName string
	dirEntry fs.DirEntry
}

func New(ctx context.Context, fsys fs.FS, name string, recursive bool) (*FsWalker, error) {
	w := FsWalker{
		fsys:      fsys,
		ctx:       ctx,
		recursive: recursive,
		curIdx:    -1,
		name:      name,
	}
	err := w.scan(ctx)
	return &w, err
}

func (w FsWalker) Name() string { return w.name }

func (w *FsWalker) scan(ctx context.Context) error {

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
		default:
			w.entries = append(w.entries, fileInfo{name, d})
		}
		return nil
	})
	return err
}

func (w *FsWalker) Next() (string, fs.DirEntry, error) {
	w.curIdx++
	if w.curIdx >= len(w.entries) {
		return "", nil, io.EOF
	}
	e := w.entries[w.curIdx]
	return e.fullName, e.dirEntry, nil
}

func (w *FsWalker) Open() (fs.File, error) {
	return w.fsys.Open(w.entries[w.curIdx].fullName)
}

func (w *FsWalker) Close() error {
	return nil
}

func (w *FsWalker) Rewind() error {
	w.curIdx = -1
	return nil
}
