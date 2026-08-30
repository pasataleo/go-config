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

func TestRegister(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		type cfg struct {
			Result string `inject:"provided"`
		}
		s := New()
		s.Register(&staticModule{})
		in := &cfg{}
		want := &cfg{Result: "installed-value"}
		testingx.Call(t, Assign, s, in).NoError()
		testingx.Capture(t, in).Equal(want)
	})

	t.Run("install_error", func(t *testing.T) {
		s := New()
		s.Register(&errorModule{})
		testingx.Call(t, Assign, s, &struct{}{}).Error()
	})

	t.Run("chaining", func(t *testing.T) {
		type cfg struct {
			Result string `inject:"greeting"`
		}
		s := New()
		// chainingModule has Prefix `inject:"prefix"` — bind that first.
		s.Bind("hello", "prefix")
		s.Register(&staticModule{}, &chainingModule{})
		in := &cfg{}
		want := &cfg{Result: "hello world"}
		testingx.Call(t, Assign, s, in).NoError()
		testingx.Capture(t, in).Equal(want)
	})

	t.Run("module_is_not_bound", func(t *testing.T) {
		// Register installs a module, it does not bind it.
		s := New()
		s.Register(&staticModule{})
		testingx.Call(t, Assign, s, &struct{}{}).NoError()
		testingx.Call(t, Get[*staticModule], s).ErrorCode(NotBound)
	})

	t.Run("caller_keeps_the_pointer", func(t *testing.T) {
		// Reading module state back means holding onto the module yourself.
		mod := &chainingModule{}
		s := New()
		s.Bind("hello", "prefix")
		s.Register(mod)
		testingx.Call(t, Assign, s, &struct{}{}).NoError()
		testingx.Capture(t, mod.Prefix).Equal("hello")
	})

	t.Run("invalid_module", func(t *testing.T) {
		s := New()
		s.Register((*staticModule)(nil))
		testingx.Call(t, Assign, s, &struct{}{}).ErrorCode(InvalidModule)
	})

	t.Run("register_during_init_panics", func(t *testing.T) {
		s := New()
		s.Register(&registeringModule{})
		testingx.Panics(t, nil, Assign, s, &struct{}{}).
			Contains("Register called during Init")
	})

	t.Run("install_outside_init_panics", func(t *testing.T) {
		s := New()
		testingx.Panics(t, nil, s.Install, &staticModule{}).
			Contains("Install called outside Init")
	})

	t.Run("duplicate_install_panics", func(t *testing.T) {
		// Two modules binding the same tag is a programmer error, and the
		// panic names the module that hit it.
		s := New()
		s.Register(&staticModule{}, &staticModule{})
		testingx.Panics(t, nil, Assign, s, &struct{}{}).
			Contains("while installing *inject.staticModule")
	})

	t.Run("second_init_is_a_no_op", func(t *testing.T) {
		// Installing twice would panic on the duplicate binding.
		s := New()
		s.Register(&staticModule{})
		testingx.Call(t, Assign, s, &struct{}{}).NoError()
		testingx.Call(t, Assign, s, &struct{}{}).NoError()
	})
}

// registeringModule calls Register at a point where only Install is legal.
type registeringModule struct{}

func (m *registeringModule) Install(s *Source) error {
	s.Register(&staticModule{})
	return nil
}

// Nested module types.

// bundleModule installs a child and then uses what the child bound.
type bundleModule struct{}

func (m *bundleModule) Install(s *Source) error {
	if err := s.Install(&childModule{}); err != nil {
		return err
	}
	child, err := GetTag[string](s, "child")
	if err != nil {
		return err
	}
	s.Bind(child+"+bundle", "bundle")
	return nil
}

type childModule struct{}

func (m *childModule) Install(s *Source) error {
	s.Bind("child", "child")
	return nil
}

// siblingModule is registered after bundleModule and depends on the binding
// the bundle's child registered.
type siblingModule struct {
	Child string `inject:"child"`
}

func (m *siblingModule) Install(s *Source) error {
	s.Bind(m.Child+"+sibling", "sibling")
	return nil
}

type outerModule struct{}

func (m *outerModule) Install(s *Source) error {
	order = append(order, "outer-start")
	if err := s.Install(&bundleModule{}); err != nil {
		return err
	}
	order = append(order, "outer-end")
	return nil
}

// order records completion ordering for the nesting test.
var order []string

func TestInstallNested(t *testing.T) {
	t.Run("child_completes_before_parent_continues", func(t *testing.T) {
		type cfg struct {
			Bundle string `inject:"bundle"`
		}
		s := New()
		s.Register(&bundleModule{})
		in := &cfg{}
		want := &cfg{Bundle: "child+bundle"}
		testingx.Call(t, Assign, s, in).NoError()
		testingx.Capture(t, in).Equal(want)
	})

	t.Run("sibling_sees_the_childs_binding", func(t *testing.T) {
		type cfg struct {
			Sibling string `inject:"sibling"`
		}
		s := New()
		s.Register(&bundleModule{}, &siblingModule{})
		in := &cfg{}
		want := &cfg{Sibling: "child+sibling"}
		testingx.Call(t, Assign, s, in).NoError()
		testingx.Capture(t, in).Equal(want)
	})

	t.Run("two_levels", func(t *testing.T) {
		order = nil
		s := New()
		s.Register(&outerModule{})
		testingx.Call(t, Assign, s, &struct{}{}).NoError()
		testingx.Capture(t, order).Equal([]string{"outer-start", "outer-end"})
		// The bundle two levels down completed inside outer.
		testingx.Call(t, GetTag[string], s, "bundle").NoError()
	})
}

// Cyclic module types.

type cycleA struct{}

func (m *cycleA) Install(s *Source) error { return s.Install(&cycleB{}) }

type cycleB struct{}

func (m *cycleB) Install(s *Source) error { return s.Install(&cycleA{}) }

type selfCycle struct{}

func (m *selfCycle) Install(s *Source) error { return s.Install(&selfCycle{}) }

func TestInstallCycle(t *testing.T) {
	t.Run("mutual", func(t *testing.T) {
		s := New()
		s.Register(&cycleA{})
		err := Assign(s, &struct{}{})
		testingx.Capture(t, err).ErrorCode(ModuleCycle)
		testingx.Capture(t, err).ErrorContains("*inject.cycleA -> *inject.cycleB -> *inject.cycleA")
	})

	t.Run("self", func(t *testing.T) {
		s := New()
		s.Register(&selfCycle{})
		testingx.Call(t, Assign, s, &struct{}{}).ErrorCode(ModuleCycle)
	})
}

func TestGet(t *testing.T) {
	t.Run("by_type", func(t *testing.T) {
		s := New()
		s.Bind(42)
		testingx.Call(t, Get[int], s).NoError().Equal(42)
	})

	t.Run("by_tag", func(t *testing.T) {
		s := New()
		s.Bind("value", "my-tag")
		testingx.Call(t, GetTag[string], s, "my-tag").NoError().Equal("value")
	})

	t.Run("bound_value_get_pointer", func(t *testing.T) {
		s := New()
		s.Bind(42)
		want := 42
		testingx.Call(t, Get[*int], s).NoError().Equal(&want)
	})

	t.Run("bound_pointer_get_value", func(t *testing.T) {
		s := New()
		v := 42
		s.Bind(&v)
		testingx.Call(t, Get[int], s).NoError().Equal(42)
	})

	t.Run("interface_target", func(t *testing.T) {
		s := New()
		s.Bind(testStringer("hello"))
		testingx.Call(t, Get[fmt.Stringer], s).NoError().Equal(testStringer("hello"))
	})

	t.Run("factory_receives_zero_value", func(t *testing.T) {
		s := New()
		s.BindFunc(func(current string) string { return current + "-appended" }, "val")
		testingx.Call(t, GetTag[string], s, "val").NoError().Equal("-appended")
	})

	t.Run("factory_error", func(t *testing.T) {
		s := New()
		s.BindFunc(func(current string) (string, error) {
			return "", fmt.Errorf("factory failed")
		}, "val")
		testingx.Call(t, GetTag[string], s, "val").Error()
	})

	t.Run("not_bound_by_type", func(t *testing.T) {
		testingx.Call(t, Get[int], New()).ErrorCode(NotBound)
	})

	t.Run("not_bound_by_tag", func(t *testing.T) {
		testingx.Call(t, GetTag[string], New(), "missing").ErrorCode(NotBound)
	})

	t.Run("type_mismatch", func(t *testing.T) {
		s := New()
		s.Bind(42, "val")
		testingx.Call(t, GetTag[string], s, "val").ErrorCode(TypeMismatch)
	})
}

func TestFindDoesNotInstall(t *testing.T) {
	type cfg struct {
		Result string `inject:"provided"`
	}
	s := New()
	s.Register(&staticModule{})

	// Find must not run modules, so the binding is not reported...
	testingx.Call(t, Find, s, &cfg{}).Equal(map[string]Match{})

	// ...and the module is still queued for a later Assign.
	in := &cfg{}
	testingx.Call(t, Assign, s, in).NoError()
	testingx.Capture(t, in).Equal(&cfg{Result: "installed-value"})
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
