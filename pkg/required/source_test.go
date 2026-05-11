package required

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
				Claimed string `required:"--claimed"`
			}{},
			want: map[string]string{
				"Claimed": "--claimed",
			},
		},
	}
	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			testingx.Capture(t, Find(tc.in)).Equal(tc.want)
		})
	}
}

func TestAssign(t *testing.T) {
	tcs := map[string]struct {
		in      interface{}
		wantErr bool
	}{
		"empty": {
			in: &struct{}{},
		},
		"private": {
			in: &struct {
				hidden string
			}{},
		},
		"unclaimed": {
			in: &struct {
				Unclaimed string
			}{},
		},
		"required_set": {
			in: &struct {
				Claimed string `required:"--claimed"`
			}{
				Claimed: "value",
			},
		},
		"required_unset": {
			in: &struct {
				Claimed string `required:"--claimed"`
			}{},
			wantErr: true,
		},
	}
	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			if tc.wantErr {
				testingx.Call(t, Assign, tc.in).Error()
			} else {
				testingx.Call(t, Assign, tc.in).NoError()
			}
		})
	}
}
