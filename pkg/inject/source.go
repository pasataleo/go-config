package inject

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/pasataleo/go-config/pkg/config"
	"github.com/pasataleo/go-errorsx/pkg/errorsx"
	"github.com/pasataleo/go-reflectx/pkg/reflectx"
)

// Module is a self-configuring component. After the framework populates the
// module's own fields (env vars, flags, injected values from earlier modules),
// it calls Install, which registers new bindings on the inject source. Those
// bindings are immediately available for subsequent fields in the struct tree.
type Module interface {
	Install(s *Source) error
}

var (
	_ config.Source = (*Source)(nil)

	errorType  = reflect.TypeOf((*error)(nil)).Elem()
	moduleType = reflect.TypeOf((*Module)(nil)).Elem()
)

// Match describes how a field was matched during Find. Exactly one of Tag or
// Type will be set.
type Match struct {
	Tag  string
	Type reflect.Type
}

// resolver wraps both static values and factory functions behind a common
// interface. The resolve function receives the current field value, allowing
// factories to build on values already set by other sources.
type resolver struct {
	resolve func(current reflect.Value) (interface{}, error)
	outType reflect.Type
}

type Source struct {
	byTag  map[string]resolver
	byType map[reflect.Type]resolver
}

func New() *Source {
	return &Source{
		byTag:  make(map[string]resolver),
		byType: make(map[reflect.Type]resolver),
	}
}

// Bind registers a static value for injection. If no tags are specified, the
// value is bound by its type. If tags are specified, the value is bound to each
// tag name. Panics on duplicate bindings.
func (s *Source) Bind(value interface{}, tags ...string) {
	r := resolver{
		resolve: func(current reflect.Value) (interface{}, error) { return value, nil },
		outType: reflect.TypeOf(value),
	}
	s.bind(r, tags...)
}

// BindFunc registers a factory function for injection. The function must take
// one argument (the current field value) and return either one value or a value
// and an error. The argument type must match the return type. If no tags are
// specified, the function's return type is used for matching. If tags are
// specified, the function is bound to each tag name. Panics on invalid
// function signatures or duplicate bindings, as these are always programmer
// errors that should be caught immediately.
func (s *Source) BindFunc(fn interface{}, tags ...string) {
	ft := reflect.TypeOf(fn)
	if ft.Kind() != reflect.Func {
		panic("BindFunc argument must be a function")
	}
	if ft.NumIn() != 1 {
		panic("BindFunc function must take one argument")
	}
	if ft.NumOut() < 1 || ft.NumOut() > 2 {
		panic("BindFunc function must return one value or (value, error)")
	}
	if ft.NumOut() == 2 && !ft.Out(1).Implements(errorType) {
		panic("BindFunc function's second return value must be error")
	}
	if ft.In(0) != ft.Out(0) {
		panic("BindFunc function's argument type " + ft.In(0).String() + " must match return type " + ft.Out(0).String())
	}

	fv := reflect.ValueOf(fn)
	hasErr := ft.NumOut() == 2

	r := resolver{
		resolve: func(current reflect.Value) (interface{}, error) {
			results := fv.Call([]reflect.Value{current})
			if hasErr && !results[1].IsNil() {
				return nil, results[1].Interface().(error)
			}
			return results[0].Interface(), nil
		},
		outType: ft.Out(0),
	}
	s.bind(r, tags...)
}

func (s *Source) bind(r resolver, tags ...string) {
	if len(tags) == 0 {
		if _, exists := s.byType[r.outType]; exists {
			panic("duplicate binding for type: " + r.outType.String())
		}
		s.byType[r.outType] = r
		return
	}

	for _, tag := range tags {
		if _, exists := s.byTag[tag]; exists {
			panic("duplicate binding for tag: " + tag)
		}
		s.byTag[tag] = r
	}
}

func (s *Source) Name() string {
	return "inject"
}

func (s *Source) Find(_ *config.Config, path reflectx.Path, value reflect.Value, field reflect.StructField) (interface{}, error) {
	if tag, ok := field.Tag.Lookup("inject"); ok {
		r, exists := s.byTag[tag]
		if !exists {
			return nil, nil
		}
		if !canAssign(r.outType, field.Type) {
			return Match{Tag: tag}, errorsx.Annotate(errorsx.Newf(errorsx.Unknown, nil, "type %s is not assignable to %s", r.outType, field.Type), "path", path)
		}
		return Match{Tag: tag}, nil
	}

	if r, ok := s.lookupByType(field.Type); ok {
		return Match{Type: r.outType}, nil
	}
	return nil, nil
}

func (s *Source) Assign(cfg *config.Config, path reflectx.Path, value reflect.Value, field reflect.StructField) (bool, error) {
	if tag, ok := field.Tag.Lookup("inject"); ok {
		tag, optional := strings.CutSuffix(tag, ";optional")

		r, exists := s.byTag[tag]
		if !exists {
			if optional {
				return false, nil
			}
			panic(fmt.Errorf("required value not found for tag at %s: %s", path, tag))
		}
		v, err := r.resolve(value)
		if err != nil {
			return true, errorsx.Annotate(err, "path", path)
		}
		set(path, value, v)
		if err := s.installModule(cfg, path, value); err != nil {
			return true, err
		}
		return true, nil
	}

	r, ok := s.lookupByType(field.Type)
	if !ok {
		return false, nil
	}
	v, err := r.resolve(value)
	if err != nil {
		return true, errorsx.Annotate(err, "path", path)
	}
	set(path, value, v)
	if err := s.installModule(cfg, path, value); err != nil {
		return true, err
	}
	return true, nil
}

// installModule checks if the assigned value implements Module. If so, it
// populates the module's own fields using the full config, then calls Install
// to let the module register new bindings.
func (s *Source) installModule(cfg *config.Config, path reflectx.Path, value reflect.Value) error {
	mod, ok := asModule(value)
	if !ok {
		return nil
	}
	if err := cfg.Assign(mod); err != nil {
		return errorsx.Annotate(err, "path", path)
	}
	if err := mod.Install(s); err != nil {
		return errorsx.Annotate(err, "path", path)
	}
	return nil
}

// asModule extracts a Module from a reflect.Value, checking both the value
// itself and its address.
func asModule(value reflect.Value) (Module, bool) {
	if value.Type().Implements(moduleType) {
		mod, ok := value.Interface().(Module)
		return mod, ok
	}
	if value.CanAddr() && value.Addr().Type().Implements(moduleType) {
		mod, ok := value.Addr().Interface().(Module)
		return mod, ok
	}
	return nil, false
}

// lookupByType finds a resolver for the given type, trying exact match first,
// then checking pointer variants.
func (s *Source) lookupByType(t reflect.Type) (resolver, bool) {
	if r, ok := s.byType[t]; ok {
		return r, true
	}
	// Field is pointer, check if non-pointer is bound.
	if t.Kind() == reflect.Pointer {
		if r, ok := s.byType[t.Elem()]; ok {
			return r, true
		}
	}
	// Field is non-pointer, check if pointer is bound.
	if r, ok := s.byType[reflect.PointerTo(t)]; ok {
		return r, true
	}
	// Field is an interface, check if any bound type implements it.
	if t.Kind() == reflect.Interface {
		for _, r := range s.byType {
			if r.outType.Implements(t) {
				return r, true
			}
		}
	}
	return resolver{}, false
}

// canAssign reports whether src can be assigned to dst, including pointer
// wrapping and unwrapping.
func canAssign(src, dst reflect.Type) bool {
	if src.AssignableTo(dst) {
		return true
	}
	// src is non-pointer, dst is pointer: can wrap.
	if dst.Kind() == reflect.Pointer && src == dst.Elem() {
		return true
	}
	// src is pointer, dst is non-pointer: can deref.
	if src.Kind() == reflect.Pointer && src.Elem() == dst {
		return true
	}
	return false
}

func set(path reflectx.Path, target reflect.Value, value interface{}) {
	rv := reflect.ValueOf(value)

	// Direct assignment.
	if rv.Type().AssignableTo(target.Type()) {
		target.Set(rv)
		return
	}

	// Value is non-pointer, target is pointer: wrap in pointer.
	if target.Type().Kind() == reflect.Pointer && rv.Type() == target.Type().Elem() {
		ptr := reflect.New(rv.Type())
		ptr.Elem().Set(rv)
		target.Set(ptr)
		return
	}

	// Value is pointer, target is non-pointer: dereference.
	if rv.Type().Kind() == reflect.Pointer && rv.Type().Elem() == target.Type() {
		if rv.IsNil() {
			panic(fmt.Errorf("inject: cannot assign to nil pointer at %s", path))
		}
		target.Set(rv.Elem())
		return
	}

	panic(fmt.Errorf("type %s is not assignable to %s at %s", rv.Type(), target.Type(), path))
}

// Find returns a map from field path to the Match that describes how the field
// would be injected.
func Find(source *Source, v interface{}) map[string]Match {
	found := config.Find(source, v)
	f := make(map[string]Match, len(found))
	for k, v := range found {
		f[k] = v.(Match)
	}
	return f
}

// Assign populates the exported fields of v using the injector's bindings.
func Assign(source *Source, v interface{}) error {
	return config.Assign(source, v)
}
