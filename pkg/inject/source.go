package inject

import (
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/pasataleo/go-config/pkg/config"
	"github.com/pasataleo/go-errorsx/pkg/errorsx"
	"github.com/pasataleo/go-reflectx/pkg/reflectx"
)

// Module is a self-configuring component. Register modules on a Source and
// they are installed by Init, which config.New calls before anything else
// touches the Config.
//
// Installing a module populates its own fields from every source in the Config
// (flags, env vars, defaults, bindings from modules installed before it), then
// calls Install so the module can register new bindings. A module must be a
// non-nil pointer to a struct.
//
// Install may itself call Source.Install to install further modules. Those
// complete before Install continues, so their bindings are available to the
// rest of the method body via Get and GetTag.
type Module interface {
	Install(s *Source) error
}

var (
	_ config.Source      = (*Source)(nil)
	_ config.Initializer = (*Source)(nil)

	errorType = reflect.TypeOf((*error)(nil)).Elem()
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

	// pending holds modules registered while cfg is nil, waiting for Init.
	pending []Module
	// cfg is non-nil only for the duration of Init. It doubles as the phase
	// flag: nil means Register queues, non-nil means Install runs immediately.
	cfg *config.Config
	// stack is the chain of modules currently being installed, innermost last.
	stack []reflect.Type
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
			panic("duplicate binding for type: " + r.outType.String() + s.while())
		}
		s.byType[r.outType] = r
		return
	}

	for _, tag := range tags {
		if _, exists := s.byTag[tag]; exists {
			panic("duplicate binding for tag: " + tag + s.while())
		}
		s.byTag[tag] = r
	}
}

// Register queues modules to be installed by the next call to Init, which
// config.New performs. Registration order is dependency order: a module can
// use bindings registered by modules registered before it.
//
// Register is the wiring-time entry point and panics if called while modules
// are installing - a module installing another module wants Install, which
// takes effect immediately.
func (s *Source) Register(modules ...Module) {
	if s.cfg != nil {
		panic("inject: Register called during Init; use Install to install a module from within another module")
	}
	s.pending = append(s.pending, modules...)
}

// Install configures and installs modules immediately, in order, and returns
// once they are done. Their bindings are then available to the caller, so a
// module can install a dependency and read what it bound via Get or GetTag.
//
// Install is only valid while modules are installing, which in practice means
// from inside another module's Install. Outside of that there is no Config to
// configure the module with, so use Register instead; calling Install there
// panics.
func (s *Source) Install(modules ...Module) error {
	if s.cfg == nil {
		panic("inject: Install called outside Init; use Register to register a module before config.New")
	}
	for _, mod := range modules {
		if err := s.install(mod); err != nil {
			return err
		}
	}
	return nil
}

// Init installs every registered module. It implements config.Initializer, so
// config.New calls it once the full source list is known.
//
// Modules registered after Init returns are queued for a subsequent Init.
func (s *Source) Init(cfg *config.Config) error {
	if len(s.stack) > 0 {
		return errorsx.New(ReentrantInit, nil, "config.New called while modules are installing")
	}

	s.cfg = cfg
	defer func() { s.cfg = nil }()

	// Drain, so that a second Init is a no-op rather than installing
	// everything twice. Nothing can be added to pending while cfg is non-nil.
	pending := s.pending
	s.pending = nil

	for _, mod := range pending {
		if err := s.install(mod); err != nil {
			return err
		}
	}
	return nil
}

// install populates a module's fields from the whole Config and then installs
// it. The module is pushed onto s.stack for the duration, which both detects
// cycles and lets error and panic messages name the module in flight.
func (s *Source) install(mod Module) error {
	t := reflect.TypeOf(mod)
	if slices.Contains(s.stack, t) {
		return errorsx.Newf(ModuleCycle, nil, "module cycle: %s", chain(append(s.stack, t)))
	}

	// reflectx.Walk panics on anything that isn't a non-nil pointer to a
	// struct, so reject those up front. This costs nothing: if a type
	// implements Module then so does a pointer to it.
	if v := reflect.ValueOf(mod); v.Kind() != reflect.Pointer || v.IsNil() || v.Elem().Kind() != reflect.Struct {
		return errorsx.Newf(InvalidModule, nil, "module %s must be a non-nil pointer to a struct", t)
	}

	// Push before assigning fields, not just before Install: a BindFunc
	// factory can run during field assignment and re-enter through Install.
	s.stack = append(s.stack, t)
	defer func() { s.stack = s.stack[:len(s.stack)-1] }()

	if err := s.cfg.Assign(mod); err != nil {
		return errorsx.Wrapf(err, "failed to configure module %s", chain(s.stack))
	}
	if err := mod.Install(s); err != nil {
		return errorsx.Wrapf(err, "failed to install module %s", chain(s.stack))
	}
	return nil
}

// chain renders a module ancestry as "*pkg.Outer -> *pkg.Inner".
func chain(modules []reflect.Type) string {
	names := make([]string, 0, len(modules))
	for _, t := range modules {
		names = append(names, t.String())
	}
	return strings.Join(names, " -> ")
}

// while describes the module currently being installed, for appending to panic
// messages. Empty when no module is installing.
func (s *Source) while() string {
	if len(s.stack) == 0 {
		return ""
	}
	return " (while installing " + chain(s.stack) + ")"
}

func (s *Source) Name() string {
	return "inject"
}

func (s *Source) Find(path reflectx.Path, value reflect.Value, field reflect.StructField) (interface{}, error) {
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

func (s *Source) Assign(path reflectx.Path, value reflect.Value, field reflect.StructField) (bool, error) {
	if tag, ok := field.Tag.Lookup("inject"); ok {
		tag, optional := strings.CutSuffix(tag, ";optional")

		r, exists := s.byTag[tag]
		if !exists {
			if optional {
				return false, nil
			}
			panic(fmt.Errorf("required value not found for tag at %s: %s%s", path, tag, s.while()))
		}
		v, err := r.resolve(value)
		if err != nil {
			return true, errorsx.Annotate(err, "path", path)
		}
		set(path, value, v)
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
	return true, nil
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

// Get returns the value bound to T, using the same matching rules as an
// untagged field of that type. It is how a module reads back what a module it
// installed has bound.
//
// A binding registered with BindFunc is called with the zero value of its
// type, since there is no field to build on, and is called afresh on every
// Get. Returns a NotBound error if nothing is bound to T.
func Get[T any](s *Source) (T, error) {
	var out T
	target := reflect.ValueOf(&out).Elem()

	r, ok := s.lookupByType(target.Type())
	if !ok {
		return out, errorsx.Newf(NotBound, nil, "no binding for type %s", target.Type())
	}
	return out, s.get(r, target)
}

// GetTag returns the value bound to a tag. It is the Get equivalent of a field
// tagged `inject:"<tag>"`.
func GetTag[T any](s *Source, tag string) (T, error) {
	var out T
	target := reflect.ValueOf(&out).Elem()

	r, ok := s.byTag[tag]
	if !ok {
		return out, errorsx.Newf(NotBound, nil, "no binding for tag %s", tag)
	}
	return out, s.get(r, target)
}

// get resolves r into target. BindFunc guarantees a factory's argument type
// matches its return type, so the zero value of outType is always a valid
// stand-in for the field value a resolver would normally receive.
func (s *Source) get(r resolver, target reflect.Value) error {
	if !canAssign(r.outType, target.Type()) {
		return errorsx.Newf(TypeMismatch, nil, "type %s is not assignable to %s", r.outType, target.Type())
	}
	v, err := r.resolve(reflect.Zero(r.outType))
	if err != nil {
		return err
	}
	return convert(target, v)
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
