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
//
// The Source handed to Install is the scope the module was found in, not the
// module's own: what it binds is visible to the module's siblings in the
// enclosing struct and to everything nested under them, and stops existing when
// the walk leaves that struct. See Source for the rest of the scoping rules.
type Module interface {
	Install(s *Source) error
}

var (
	_ config.Source        = (*Source)(nil)
	_ config.DynamicSource = (*Source)(nil)

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

// Source holds bindings for injection. Bindings are lexically scoped: a Source
// is one frame in a chain, and Assign opens a new frame for every field it
// walks.
//
// A module installing into a frame publishes to that frame's own fields and to
// everything nested under them, and to nothing outside it. A binding shadows one
// of the same name further out for as long as its frame is live, and the outer
// binding is back in force once the walk leaves. Two bindings of the same name
// in the same frame are a collision, and still panic.
//
// The Source a caller builds with New is the outermost frame of all: what it is
// given by Bind and BindFunc is visible to every walk and outlives all of them.
// A Source is not safe for concurrent use while it is being bound to, as today.
type Source struct {
	// parent is the frame this one was opened inside, or nil for the Source the
	// caller built. Lookups walk out along it; bindings never do.
	parent *Source

	// Both maps are allocated by bind, so a frame that binds nothing - which is
	// most of them, one per field walked - costs a single small struct.
	byTag  map[string]resolver
	byType map[reflect.Type]resolver
}

func New() *Source {
	return &Source{}
}

// Enter opens a frame for one field. Everything the field's own Assign and its
// children bind lands here, and stops being reachable when the walk leaves.
func (s *Source) Enter(_ *config.Config, _ reflectx.Path) (config.Source, error) {
	return &Source{parent: s}, nil
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

// bind writes into this frame alone. A duplicate here is a collision and
// panics; the same name further out is shadowed for as long as this frame
// lives, which is the point of the frames.
func (s *Source) bind(r resolver, tags ...string) {
	if len(tags) == 0 {
		if _, exists := s.byType[r.outType]; exists {
			panic("duplicate binding for type: " + r.outType.String())
		}
		if s.byType == nil {
			s.byType = make(map[reflect.Type]resolver)
		}
		s.byType[r.outType] = r
		return
	}

	for _, tag := range tags {
		if _, exists := s.byTag[tag]; exists {
			panic("duplicate binding for tag: " + tag)
		}
		if s.byTag == nil {
			s.byTag = make(map[string]resolver)
		}
		s.byTag[tag] = r
	}
}

func (s *Source) Name() string {
	return "inject"
}

func (s *Source) Find(_ *config.Config, path reflectx.Path, value reflect.Value, field reflect.StructField) (interface{}, error) {
	if tag, ok := field.Tag.Lookup("inject"); ok {
		r, exists := s.lookupByTag(tag)
		if !exists {
			return nil, nil
		}
		if !canAssign(r.outType, field.Type) {
			return Match{Tag: tag}, errorsx.Annotate(errorsx.Newf(TypeMismatch, nil, "type %s is not assignable to %s", r.outType, field.Type), "path", path)
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

		r, exists := s.lookupByTag(tag)
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
//
// The module's fields are populated with the config this Source was scoped by,
// so they resolve against everything in scope where the module was found. Its
// Install, though, is handed the enclosing frame: a module's bindings belong
// beside it, where its siblings read from, not in the frame that closes with it.
// Lookups inside Install start there too, which is where any module nested
// inside this one published - so a module sees what its children bound.
func (s *Source) installModule(cfg *config.Config, path reflectx.Path, value reflect.Value) error {
	mod, ok := asModule(value)
	if !ok {
		return nil
	}
	if err := cfg.Assign(mod); err != nil {
		return errorsx.Annotate(err, "path", path)
	}

	// A nil parent means this Source is not being walked by a Config at all, so
	// there is no enclosing frame and its own is the only place to publish to.
	enclosing := s.parent
	if enclosing == nil {
		enclosing = s
	}
	if err := mod.Install(enclosing); err != nil {
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

// lookupByTag finds a resolver for the given tag, innermost frame first, so a
// binding shadows one of the same name further out.
func (s *Source) lookupByTag(tag string) (resolver, bool) {
	for frame := s; frame != nil; frame = frame.parent {
		if r, ok := frame.byTag[tag]; ok {
			return r, true
		}
	}
	return resolver{}, false
}

// lookupByType finds a resolver for the given type, innermost frame first. Each
// frame is tried every way before moving out to the next, so a frame that
// matches at all shadows everything beyond it - matching frame by frame rather
// than one kind of match at a time is what keeps shadowing from inverting for
// the weaker matches.
func (s *Source) lookupByType(t reflect.Type) (resolver, bool) {
	for frame := s; frame != nil; frame = frame.parent {
		if r, ok := frame.typeInFrame(t); ok {
			return r, true
		}
	}
	return resolver{}, false
}

// typeInFrame finds a resolver for the given type in this frame alone, trying
// exact match first, then pointer variants, then interface satisfaction.
func (s *Source) typeInFrame(t reflect.Type) (resolver, bool) {
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

// set assigns value to target, panicking on failure. Assignment failures are
// programmer errors: the field and the binding are both fixed at compile time.
func set(path reflectx.Path, target reflect.Value, value interface{}) {
	if err := convert(target, value); err != nil {
		panic(errorsx.Annotate(err, "path", path))
	}
}

// convert assigns value to target, wrapping or dereferencing a pointer where
// that is the only difference between the two types.
func convert(target reflect.Value, value interface{}) error {
	rv := reflect.ValueOf(value)

	// Direct assignment.
	if rv.Type().AssignableTo(target.Type()) {
		target.Set(rv)
		return nil
	}

	// Value is non-pointer, target is pointer: wrap in pointer.
	if target.Type().Kind() == reflect.Pointer && rv.Type() == target.Type().Elem() {
		ptr := reflect.New(rv.Type())
		ptr.Elem().Set(rv)
		target.Set(ptr)
		return nil
	}

	// Value is pointer, target is non-pointer: dereference.
	if rv.Type().Kind() == reflect.Pointer && rv.Type().Elem() == target.Type() {
		if rv.IsNil() {
			return errorsx.New(TypeMismatch, nil, "cannot assign nil pointer to "+target.Type().String())
		}
		target.Set(rv.Elem())
		return nil
	}

	return errorsx.Newf(TypeMismatch, nil, "type %s is not assignable to %s", rv.Type(), target.Type())
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
