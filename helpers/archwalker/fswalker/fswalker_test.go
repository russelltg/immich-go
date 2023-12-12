package fswalker_test

import (
	"context"
	"io"
	"os"
	"reflect"
	"testing"

	"github.com/kr/pretty"
	"github.com/simulot/immich-go/helpers/archwalker/fswalker"
)

func Test_FsWalker(t *testing.T) {
	tc := []struct {
		name string
		path string
		rec  bool
		want []string
	}{
		{
			name: "non recursive",
			path: "TEST_DATA",
			rec:  false,
			want: []string{"métadonnées.json"},
		},
		{
			name: "recursive",
			path: "TEST_DATA",
			rec:  true,
			want: []string{
				"Google Photos/Photos from 2023/PXL_20231006_063000139.jpg.json",
				"Google Photos/Photos from 2023/PXL_20231006_063528961.jpg.json",
				"Google Photos/Sans titre(9)/PXL_20231006_063108407.jpg.json",
				"Google Photos/Sans titre(9)/métadonnées.json",
				"métadonnées.json",
			},
		},
	}

	for _, tt := range tc {
		t.Run(tt.name, func(t *testing.T) {
			w, err := fswalker.New(context.Background(), os.DirFS(tt.path), tt.rec)
			if err != nil {
				t.Errorf("can't create the walker: %s", err)
				return
			}
			got := []string{}
			for {
				p, _, err := w.Next()
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Errorf("unexpected error: %s", err)
					return
				}
				got = append(got, p)
				f, err := w.Open()
				if err != nil {
					t.Errorf("can't open the file %s error: %s", p, err)
					return
				}
				_, err = io.Copy(io.Discard, f)
				if err != nil {
					t.Errorf("can't read the file %s error: %s", p, err)
					return
				}
				f.Close()
			}
			w.Close()
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("result")
				pretty.Ldiff(t, tt.want, got)
			}
		})
	}

}
