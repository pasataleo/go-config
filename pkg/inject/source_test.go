package inject

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/pasataleo/go-reflectx/pkg/reflectx"
	"github.com/pasataleo/go-testingx/pkg/diff"
	"github.com/pasataleo/go-testingx/pkg/render"
	"github.com/pasataleo/go-testingx/pkg/testingx"
)

func TestFind(t *testing.T) {
	tcs := map[string]struct {
		source *Source
		in     interface{}
		want   map[string]Match
	}{
		"empty": {
			source: New(),
			in:     &struct{}{},
			want:   map[string]Match{},
		},
		"private": {
			source: New(),
			in: &struct {
				hidden string
			}{},
			want: map[string]Match{},
		},
		"unclaimed": {
			source: New(),
			in: &struct {
				Unclaimed string
			}{},
			want: map[string]Match{},
		},
		"by_tag": {
			source: func() *Source {
				s := New()
				s.Bind("value", "my-tag")
				return s
			}(),
			in: &struct {
				Claimed string `inject:"my-tag"`
			}{},
			want: map[string]Match{
				"Claimed": {Tag: "my-tag"},
			},
		},
		"by_type": {
			source: func() *Source {
				s := New()
				s.Bind("value")
				return s
			}(),
			in: &struct {
				Claimed string
			}{},
			want: map[string]Match{
				"Claimed": {Type: reflect.TypeFor[string]()},
			},
		},
		"tag_not_bound": {
			source: New(),
			in: &struct {
				Claimed string `inject:"missing"`
			}{},
			want: map[string]Match{},
		},
	}
	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			testingx.Call(t, Find, tc.source, tc.in).
				Equal(tc.want, diff.WithDiffer(new(reflectx.TypeDiffer)))
		})
	}
}

func TestAssign(t *testing.T) {
	tcs := map[string]struct {
		source *Source
		in     interface{}
		want   interface{}
	}{
		"empty": {
			source: New(),
			in:     &struct{}{},
			want:   &struct{}{},
		},
		"private": {
			source: New(),
			in: &struct {
				hidden string
			}{},
			want: &struct {
				hidden string
			}{},
		},
		"unclaimed": {
			source: New(),
			in: &struct {
				Unclaimed string
			}{},
			want: &struct {
				Unclaimed string
			}{},
		},
		"by_tag": {
			source: func() *Source {
				s := New()
				s.Bind("injected", "my-tag")
				return s
			}(),
			in: &struct {
				Claimed string `inject:"my-tag"`
			}{},
			want: &struct {
				Claimed string `inject:"my-tag"`
			}{
				Claimed: "injected",
			},
		},
		"by_type": {
			source: func() *Source {
				s := New()
				s.Bind("injected")
				return s
			}(),
			in: &struct {
				Claimed string
			}{},
			want: &struct {
				Claimed string
			}{
				Claimed: "injected",
			},
		},
		"tag_not_bound": {
			source: New(),
			in: &struct {
				Claimed string `inject:"missing;optional"`
			}{},
			want: &struct {
				Claimed string `inject:"missing;optional"`
			}{},
		},
	}
	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			testingx.Call(t, Assign, tc.source, tc.in).NoError()
			testingx.Capture(t, tc.in).Equal(
				tc.want,
				diff.WithRenderOpts(render.NewOpts(t, render.WithSkipUnexported(true))))
		})
	}
}

// Test module types.

type staticModule struct{}

func (m *staticModule) Install(s *Source) error {
	s.Bind("installed-value", "provided")
	return nil
}

type errorModule struct{}

func (m *errorModule) Install(s *Source) error {
	return fmt.Errorf("install failed")
}

type chainingModule struct {
	Prefix string `inject:"prefix"`
}

func (m *chainingModule) Install(s *Source) error {
	s.Bind(m.Prefix+" world", "greeting")
	return nil
}

func TestAssignModule(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		type cfg struct {
			Mod    *staticModule `inject:"mod"`
			Result string        `inject:"provided"`
		}
		s := New()
		s.Bind(&staticModule{}, "mod")
		in := &cfg{}
		want := &cfg{
			Mod:    &staticModule{},
			Result: "installed-value",
		}
		testingx.Call(t, Assign, s, in).NoError()
		testingx.Capture(t, in).Equal(want)
	})

	t.Run("install_error", func(t *testing.T) {
		type cfg struct {
			Mod *errorModule `inject:"mod"`
		}
		s := New()
		s.Bind(&errorModule{}, "mod")
		in := &cfg{}
		testingx.Call(t, Assign, s, in).Error()
	})

	t.Run("chaining", func(t *testing.T) {
		type cfg struct {
			First  *staticModule   `inject:"first"`
			Second *chainingModule `inject:"second"`
			Result string          `inject:"greeting"`
		}
		s := New()
		s.Bind(&staticModule{}, "first")
		s.Bind(&chainingModule{}, "second")
		// staticModule.Install binds "installed-value" to "provided"
		// chainingModule has Prefix `inject:"prefix"` — bind that too
		s.Bind("hello", "prefix")
		in := &cfg{}
		want := &cfg{
			First:  &staticModule{},
			Second: &chainingModule{Prefix: "hello"},
			Result: "hello world",
		}
		testingx.Call(t, Assign, s, in).NoError()
		testingx.Capture(t, in).Equal(want)
	})

	t.Run("by_type", func(t *testing.T) {
		type cfg struct {
			Mod    *staticModule
			Result string `inject:"provided"`
		}
		s := New()
		s.Bind(&staticModule{})
		in := &cfg{}
		want := &cfg{
			Mod:    &staticModule{},
			Result: "installed-value",
		}
		testingx.Call(t, Assign, s, in).NoError()
		testingx.Capture(t, in).Equal(want)
	})
}

func TestBindFunc(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		type cfg struct {
			Value string
		}
		s := New()
		s.BindFunc(func(current string) string {
			return "transformed"
		})
		in := &cfg{}
		want := &cfg{Value: "transformed"}
		testingx.Call(t, Assign, s, in).NoError()
		testingx.Capture(t, in).Equal(want)
	})

	t.Run("with_error_return", func(t *testing.T) {
		type cfg struct {
			Value string
		}
		s := New()
		s.BindFunc(func(current string) (string, error) {
			return "ok", nil
		})
		in := &cfg{}
		want := &cfg{Value: "ok"}
		testingx.Call(t, Assign, s, in).NoError()
		testingx.Capture(t, in).Equal(want)
	})

	t.Run("error_returned", func(t *testing.T) {
		type cfg struct {
			Value string
		}
		s := New()
		s.BindFunc(func(current string) (string, error) {
			return "", fmt.Errorf("factory failed")
		})
		in := &cfg{}
		testingx.Call(t, Assign, s, in).Error()
	})

	t.Run("by_tag", func(t *testing.T) {
		type cfg struct {
			Value string `inject:"my-tag"`
		}
		s := New()
		s.BindFunc(func(current string) string {
			return "tagged"
		}, "my-tag")
		in := &cfg{}
		want := &cfg{Value: "tagged"}
		testingx.Call(t, Assign, s, in).NoError()
		testingx.Capture(t, in).Equal(want)
	})

	t.Run("receives_current_value", func(t *testing.T) {
		type cfg struct {
			Value string `inject:"val"`
		}
		s := New()
		s.BindFunc(func(current string) string {
			return current + "-appended"
		}, "val")
		in := &cfg{Value: "existing"}
		want := &cfg{Value: "existing-appended"}
		testingx.Call(t, Assign, s, in).NoError()
		testingx.Capture(t, in).Equal(want)
	})
}

func TestAssignPointerVariants(t *testing.T) {
	t.Run("bind_value_assign_pointer_field", func(t *testing.T) {
		// Bind a non-pointer, field is a pointer — set should wrap.
		type cfg struct {
			Value *string `inject:"val"`
		}
		s := New()
		s.Bind("hello", "val")
		in := &cfg{}
		hello := "hello"
		want := &cfg{Value: &hello}
		testingx.Call(t, Assign, s, in).NoError()
		testingx.Capture(t, in).Equal(want)
	})

	t.Run("bind_pointer_assign_value_field", func(t *testing.T) {
		// Bind a pointer, field is a non-pointer — set should deref.
		type cfg struct {
			Value string `inject:"val"`
		}
		s := New()
		v := "hello"
		s.Bind(&v, "val")
		in := &cfg{}
		want := &cfg{Value: "hello"}
		testingx.Call(t, Assign, s, in).NoError()
		testingx.Capture(t, in).Equal(want)
	})

	t.Run("bind_pointer_find_value_field_by_type", func(t *testing.T) {
		// Bind *int by type, field is int — lookupByType should find it.
		type cfg struct {
			Count int
		}
		s := New()
		v := 42
		s.Bind(&v)
		in := &cfg{}
		want := &cfg{Count: 42}
		testingx.Call(t, Assign, s, in).NoError()
		testingx.Capture(t, in).Equal(want)
	})

	t.Run("bind_value_find_pointer_field_by_type", func(t *testing.T) {
		// Bind int by type, field is *int — lookupByType should find it.
		type cfg struct {
			Count *int
		}
		s := New()
		s.Bind(42)
		in := &cfg{}
		v := 42
		want := &cfg{Count: &v}
		testingx.Call(t, Assign, s, in).NoError()
		testingx.Capture(t, in).Equal(want)
	})
}

func TestAssignInterfaceMatching(t *testing.T) {
	t.Run("bind_concrete_assign_interface_field", func(t *testing.T) {
		type cfg struct {
			Value fmt.Stringer
		}
		s := New()
		s.Bind(testStringer("hello"))
		in := &cfg{}
		testingx.Call(t, Assign, s, in).NoError()
		if in.Value == nil || in.Value.String() != "hello" {
			t.Fatalf("expected hello, got %v", in.Value)
		}
	})
}

type testStringer string

func (t testStringer) String() string { return string(t) }

func TestFindPointerVariants(t *testing.T) {
	t.Run("bind_pointer_find_value_field_by_type", func(t *testing.T) {
		type cfg struct {
			Count int
		}
		s := New()
		v := 42
		s.Bind(&v)
		testingx.Call(t, Find, s, &cfg{}).
			Equal(map[string]Match{
				"Count": {Type: reflect.TypeFor[*int]()},
			}, diff.WithDiffer(new(reflectx.TypeDiffer)))
	})

	t.Run("bind_value_find_pointer_field_by_type", func(t *testing.T) {
		type cfg struct {
			Count *int
		}
		s := New()
		s.Bind(42)
		testingx.Call(t, Find, s, &cfg{}).
			Equal(map[string]Match{
				"Count": {Type: reflect.TypeFor[int]()},
			}, diff.WithDiffer(new(reflectx.TypeDiffer)))
	})
}
