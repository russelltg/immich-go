//go:build e2e
// +build e2e

package gp

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/simulot/immich-go/browser"
	"github.com/simulot/immich-go/helpers/archwalker"
	"github.com/simulot/immich-go/helpers/archwalker/fswalker"
	"github.com/simulot/immich-go/helpers/archwalker/zipwalker"
	"github.com/simulot/immich-go/journal"
	"github.com/simulot/immich-go/logger"
)

type nopWriterCloser struct {
	io.Writer
}

func (n nopWriterCloser) Close() error { return nil }

func TestReadBigTakeout(t *testing.T) {
	logBuffer := bytes.NewBuffer(nil)
	log := logger.NewLogger(logger.Info, true, false)
	f, err := os.Create("bigread.log")
	if err != nil {
		t.Error(err)
		return
	}
	defer log.Close()
	log.SetWriter(f)
	jnl := journal.NewJournal(log)

	m, err := filepath.Glob("../../../test-data/full_takeout/*.zip")
	if err != nil {
		t.Error(err)
		return
	}
	cnt := 0
	ctx := context.Background()

	ws := []archwalker.Walker{}
	for _, f := range m {
		w, err := zipwalker.New(ctx, f)
		if err != nil {
			t.Errorf("can't open walker: %s", err)
			return
		}
		ws = append(ws, w)
	}

	to, err := NewTakeout(context.Background(), logger.NoLogger{}, &browser.Configuration{
		Journal: jnl,
	}, ws)
	if err != nil {
		t.Error(err)
		return
	}

	for range to.Browse(context.Background()) {
		cnt++
	}
	t.Logf("seen %d files", cnt)
	to.conf.Journal.Report()
	fmt.Println(logBuffer.String())
}

func Test_Walkers(t *testing.T) {
	logBuffer := bytes.NewBuffer(nil)
	log := logger.NewLogger(logger.Info, true, false)
	f, err := os.Create("testwalkers.log")
	if err != nil {
		t.Error(err)
		return
	}
	defer log.Close()
	log.SetWriter(f)
	jnl := journal.NewJournal(log)

	cnt := 0
	ctx := context.Background()

	ws := []archwalker.Walker{}
	for _, f := range []string{"../../../test-data/test walkers/zip1", "../../../test-data/test walkers/zip2"} {
		w, err := fswalker.New(ctx, os.DirFS(f), f, true)
		if err != nil {
			t.Errorf("can't open walker: %s", err)
			return
		}
		ws = append(ws, w)
	}

	to, err := NewTakeout(context.Background(), logger.NoLogger{}, &browser.Configuration{
		Journal: jnl,
	}, ws)
	if err != nil {
		t.Error(err)
		return
	}

	for range to.Browse(context.Background()) {
		cnt++
	}
	t.Logf("seen %d files", cnt)
	to.conf.Journal.Report()
	fmt.Println(logBuffer.String())
}
