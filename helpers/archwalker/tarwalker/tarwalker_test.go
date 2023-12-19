package tarwalker_test

import (
	"context"
	"io"
	"reflect"
	"sort"
	"testing"

	"github.com/kr/pretty"
	"github.com/simulot/immich-go/helpers/archwalker/tarwalker"
)

func Test_TarWalker(t *testing.T) {
	tc := []struct {
		name string
		path string
		rec  bool
		want []string
	}{
		{
			name: "tar",
			path: "TEST_DATA/TEST_DATA.tar",
			rec:  true,
			want: []string{
				"TEST_DATA/Google Photos/Photos from 2023/PXL_20231006_063000139.jpg.json",
				"TEST_DATA/Google Photos/Photos from 2023/PXL_20231006_063528961.jpg.json",
				"TEST_DATA/Google Photos/Sans titre(9)/PXL_20231006_063108407.jpg.json",
				"TEST_DATA/Google Photos/Sans titre(9)/métadonnées.json",
				"TEST_DATA/métadonnées.json",
			},
		},
		{
			name: "tgz",
			path: "TEST_DATA/TEST_DATA.tgz",
			rec:  true,
			want: []string{
				"TEST_DATA/Google Photos/Photos from 2023/PXL_20231006_063000139.jpg.json",
				"TEST_DATA/Google Photos/Photos from 2023/PXL_20231006_063528961.jpg.json",
				"TEST_DATA/Google Photos/Sans titre(9)/PXL_20231006_063108407.jpg.json",
				"TEST_DATA/Google Photos/Sans titre(9)/métadonnées.json",
				"TEST_DATA/métadonnées.json",
			},
		},
		{
			name: "tar.gz",
			path: "TEST_DATA/TEST_DATA.tar.gz",
			rec:  true,
			want: []string{
				"TEST_DATA/Google Photos/Photos from 2023/PXL_20231006_063000139.jpg.json",
				"TEST_DATA/Google Photos/Photos from 2023/PXL_20231006_063528961.jpg.json",
				"TEST_DATA/Google Photos/Sans titre(9)/PXL_20231006_063108407.jpg.json",
				"TEST_DATA/Google Photos/Sans titre(9)/métadonnées.json",
				"TEST_DATA/métadonnées.json",
			},
		},
	}

	for _, tt := range tc {
		t.Run(tt.name, func(t *testing.T) {
			w, err := tarwalker.Open(context.Background(), tt.path)
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
				w.Rewind()

				sort.Strings(tt.want)
				sort.Strings(got)
				if !reflect.DeepEqual(got, tt.want) {
					t.Errorf("result at iteration %d", i)
					pretty.Ldiff(t, tt.want, got)
				}
			}
			w.Close()
		})
	}

}
