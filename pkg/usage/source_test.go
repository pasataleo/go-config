package usage

import (
	"testing"

	"github.com/pasataleo/go-testingx/pkg/testingx"
)

func TestFind(t *testing.T) {
	tcs := map[string]struct {
		in   interface{}
		want map[string]string
	}{
		"empty": {
			in:   &struct{}{},
			want: map[string]string{},
		},
		"private": {
			in: &struct {
				hidden string
			}{},
			want: map[string]string{},
		},
		"unclaimed": {
			in: &struct {
				Unclaimed string
			}{},
			want: map[string]string{},
		},
		"claimed": {
			in: &struct {
				Claimed string `usage:"some usage description"`
			}{},
			want: map[string]string{
				"Claimed": "some usage description",
			},
		},
	}
	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			testingx.Call(t, Find, tc.in).Equal(tc.want)
		})
	}
}
