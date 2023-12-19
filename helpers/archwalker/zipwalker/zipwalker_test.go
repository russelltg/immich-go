package zipwalker_test

import (
	"context"
	"io"
	"reflect"
	"testing"

	"github.com/kr/pretty"
	"github.com/simulot/immich-go/helpers/archwalker/zipwalker"
)

func Test_ZipWalker(t *testing.T) {
	tc := []struct {
		name string
		path string
		rec  bool
		want []string
	}{
		{
			name: "zip",
			path: "TEST_DATA/archive.zip",
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
			w, err := zipwalker.New(context.Background(), tt.path)
			if err != nil {
				t.Errorf("can't create the walker: %s", err)
				return
			}

			for i := 0; i < 10; i++ {
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
				if !reflect.DeepEqual(got, tt.want) {
					t.Errorf("result")
					pretty.Ldiff(t, tt.want, got)
				}
				w.Rewind()
			}
			w.Close()
		})
	}

}
