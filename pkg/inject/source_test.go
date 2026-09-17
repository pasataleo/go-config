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

// shadowModule binds a tag the Source itself already binds, to check that the
// module's value wins inside its scope and the Source's is back afterwards.
type shadowModule struct{}

func (m *shadowModule) Install(s *Source) error {
	s.Bind("red", "colour")
	return nil
}

// outerModule holds a module of its own, and reads what that one bound before
// binding something itself.
type outerModule struct {
	Inner *staticModule `inject:"inner-mod"`
	Got   string        `inject:"provided"`
}

func (m *outerModule) Install(s *Source) error {
	s.Bind("outer:"+m.Got, "outer-result")
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

func TestScopes(t *testing.T) {
	t.Run("sibling_sees_an_earlier_siblings_binding", func(t *testing.T) {
		type cfg struct {
			Mod  *staticModule `inject:"mod"`
			Peer string        `inject:"provided"`
		}
		s := New()
		s.Bind(&staticModule{}, "mod")
		in := &cfg{}
		want := &cfg{Mod: &staticModule{}, Peer: "installed-value"}
		testingx.Call(t, Assign, s, in).NoError()
		testingx.Capture(t, in).Equal(want)
	})

	t.Run("a_nested_scope_is_not_visible_above_it", func(t *testing.T) {
		type inner struct {
			Mod  *staticModule `inject:"mod"`
			Peer string        `inject:"provided"`
		}
		type cfg struct {
			Inner inner
			Above string `inject:"provided;optional"`
		}
		s := New()
		s.Bind(&staticModule{}, "mod")
		in := &cfg{}
		want := &cfg{
			Inner: inner{Mod: &staticModule{}, Peer: "installed-value"},
			Above: "", // died with inner's scope
		}
		testingx.Call(t, Assign, s, in).NoError()
		testingx.Capture(t, in).Equal(want)
	})

	t.Run("a_nested_scope_shadows_an_outer_binding", func(t *testing.T) {
		type inner struct {
			Mod   *shadowModule `inject:"shadow-mod"`
			Value string        `inject:"colour"`
		}
		type cfg struct {
			Before string `inject:"colour"`
			Inner  inner
			After  string `inject:"colour"`
		}
		s := New()
		s.Bind("blue", "colour")
		s.Bind(&shadowModule{}, "shadow-mod")
		in := &cfg{}
		want := &cfg{
			Before: "blue",
			Inner:  inner{Mod: &shadowModule{}, Value: "red"},
			After:  "blue", // the outer binding is back in force
		}
		testingx.Call(t, Assign, s, in).NoError()
		testingx.Capture(t, in).Equal(want)
	})

	t.Run("a_module_sees_what_its_own_module_bound", func(t *testing.T) {
		type cfg struct {
			Mod    *outerModule `inject:"outer-mod"`
			Result string       `inject:"outer-result"`
		}
		s := New()
		s.Bind(&outerModule{}, "outer-mod")
		s.Bind(&staticModule{}, "inner-mod")
		in := &cfg{}
		want := &cfg{
			Mod:    &outerModule{Inner: &staticModule{}, Got: "installed-value"},
			Result: "outer:installed-value",
		}
		testingx.Call(t, Assign, s, in).NoError()
		testingx.Capture(t, in).Equal(want)
	})

	t.Run("two_modules_in_the_same_scope_collide", func(t *testing.T) {
		type cfg struct {
			First  *staticModule `inject:"first"`
			Second *staticModule `inject:"second"`
		}
		s := New()
		s.Bind(&staticModule{}, "first")
		s.Bind(&staticModule{}, "second")

		defer func() {
			if recover() == nil {
				t.Fatal("expected a panic from the duplicate binding")
			}
		}()
		_ = Assign(s, &cfg{})
	})

	t.Run("bindings_do_not_survive_the_assign_that_made_them", func(t *testing.T) {
		type first struct {
			Mod    *staticModule `inject:"mod"`
			Result string        `inject:"provided"`
		}
		type second struct {
			Result string `inject:"provided;optional"`
		}
		s := New()
		s.Bind(&staticModule{}, "mod")

		one := &first{}
		testingx.Call(t, Assign, s, one).NoError()
		testingx.Capture(t, one).Equal(&first{Mod: &staticModule{}, Result: "installed-value"})

		two := &second{}
		testingx.Call(t, Assign, s, two).NoError()
		testingx.Capture(t, two).Equal(&second{Result: ""})
	})

	t.Run("a_failed_assign_leaves_no_debris", func(t *testing.T) {
		type broken struct {
			Bad *errorModule  `inject:"bad"`
			Mod *staticModule `inject:"mod"`
		}
		type after struct {
			Result string `inject:"provided;optional"`
		}
		s := New()
		s.Bind(&errorModule{}, "bad")
		s.Bind(&staticModule{}, "mod")

		testingx.Call(t, Assign, s, &broken{}).Error()

		next := &after{}
		testingx.Call(t, Assign, s, next).NoError()
		testingx.Capture(t, next).Equal(&after{Result: ""})
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
