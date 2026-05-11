package hidden

import (
	"testing"

	"github.com/pasataleo/go-testingx/pkg/testingx"
)

func TestFind(t *testing.T) {
	tcs := map[string]struct {
		in   interface{}
		want map[string]bool
	}{
		"empty": {
			in:   &struct{}{},
			want: map[string]bool{},
		},
		"private": {
			in: &struct {
				hidden string
			}{},
			want: map[string]bool{},
		},
		"unclaimed": {
			in: &struct {
				Unclaimed string
			}{},
			want: map[string]bool{},
		},
		"claimed": {
			in: &struct {
				Claimed string `hidden:"true"`
			}{},
			want: map[string]bool{
				"Claimed": true,
			},
		},
	}
	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			testingx.Call(t, Find, tc.in).Equal(tc.want)
		})
	}
}
