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
	_ config.Source            = (*Source)(nil)
	_ config.DynamicSource     = (*Source)(nil)
	_ config.Starter           = (*Source)(nil)
	_ config.AdditionalTargets = (*Source)(nil)

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

	// walk is the install state of the Assign this frame belongs to, shared with
	// every frame below it. Nil on the Source the caller built, which belongs to
	// no walk.
	walk *walk

	// modules is what Register has queued, in registration order. Only ever set
	// on the Source the caller built, since only that one can Register - a frame
	// reaches them with root().
	modules []Module
}

// walk is what one Assign has to remember across the frames it opens: which
// module instances it has installed, and which are installing right now. It is
// deliberately not on the Source the caller built - it is created by the frame
// at the root of a walk and dies with it, so a second Assign installs from
// clean and nothing an Assign does outlives it.
type walk struct {
	// stack is the types of the modules currently being installed, innermost
	// last. A module whose type is already on it is installing itself. It is
	// the type rather than the path that detects this: populating a module's
	// fields walks that module from a root of its own, so the path is the
	// field's, not a trail back to the struct the caller passed in.
	stack []reflect.Type

	// registered records that this walk has installed the registered modules.
	// A walk nested inside it - a module's own fields being populated - shares
	// this and so does not install them again.
	registered bool

	// path is where the innermost installing module was found, for the messages
	// of anything that goes wrong underneath it.
	path reflectx.Path

	// installed dedupes by instance identity, so an instance reached twice - by
	// two fields bound to it, say - is installed once. Dedupe is about side
	// effects rather than visibility, so it is flat for the whole walk rather
	// than per frame, and a module reached from two different scopes still
	// installs only once.
	//
	// Zero-sized modules are left out of it entirely; see install.
	installed map[Module]bool
}

func New() *Source {
	return &Source{}
}

// Enter opens a frame for one field. Everything the field's own Assign and its
// children bind lands here, and stops being reachable when the walk leaves.
func (s *Source) Enter(_ *config.Config, _ reflectx.Path) (config.Source, error) {
	w := s.walk
	if w == nil {
		// s belongs to no walk, so it is the Source the caller built and this is
		// the frame at the root of one. A walk nested inside another - a module's
		// own fields being populated - enters from a frame that already has the
		// state and shares it, which is what lets a cycle through two nested
		// walks still be seen as one chain.
		w = &walk{}
	}
	return &Source{parent: s, walk: w}, nil
}

// Register queues modules for every Assign to install, before it walks
// anything. These are the program's root modules: registration order is
// dependency order, and they are how a program picks its wiring at run time -
// a real implementation or a fake one - rather than from the shape of the
// struct being assigned.
//
// Register belongs to wiring time, and works only on the Source New built. Every
// other Source is a scope opened during an Assign, and registering into one
// panics: the module graph is fixed before a walk starts, so that Find can
// report every flag, env var and usage string a program has without installing
// anything. A module that needs another module declares it as a field.
func (s *Source) Register(modules ...Module) {
	if s.parent != nil {
		panic("inject: Register works only on the Source inject.New built, not on a scope opened during Assign; register before the walk, or declare the module as a struct field")
	}
	s.modules = append(s.modules, modules...)
}

// Start implements config.Starter. It installs the registered modules into the
// scope the Assign has just opened, so what they bind is in force for the whole
// of that walk and gone after it - the next Assign installs them again, against
// the same wiring rather than whatever the last one left behind.
func (s *Source) Start(cfg *config.Config) error {
	w := s.walk
	if w == nil || w.registered {
		// Either nothing opened a walk, or this is a walk nested inside one
		// that has already installed them - a module's own fields being
		// populated, which must not install the roots a second time.
		return nil
	}
	w.registered = true

	// The scope itself is what a registered module publishes into, not the one
	// above it: a root module's bindings are for the whole of this walk, the
	// way a module at the top level of the struct being assigned is.
	for _, mod := range s.root().modules {
		if err := s.install(cfg, nil, mod, s, true); err != nil {
			return err
		}
	}
	return nil
}

// AdditionalTargets implements config.AdditionalTargets. It returns every
// registered module, so a caller can Find the fields of modules its own struct
// does not reach - which is all of them, since a registered module is wired to
// the injector rather than declared anywhere the caller owns.
//
// The answer is the same whether or not an Assign has installed them, which is
// the point: help text does not wait on a walk.
func (s *Source) AdditionalTargets() []interface{} {
	modules := s.root().modules
	targets := make([]interface{}, 0, len(modules))
	for _, mod := range modules {
		if isNilModule(mod) {
			continue // Find would panic on it; install reports it as InvalidModule
		}
		targets = append(targets, mod)
	}
	return targets
}

// root returns the Source the caller built, the outermost frame of the chain.
func (s *Source) root() *Source {
	for s.parent != nil {
		s = s.parent
	}
	return s
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
			panic("duplicate binding for type: " + r.outType.String() + s.while())
		}
		if s.byType == nil {
			s.byType = make(map[reflect.Type]resolver)
		}
		s.byType[r.outType] = r
		return
	}

	for _, tag := range tags {
		if _, exists := s.byTag[tag]; exists {
			panic("duplicate binding for tag: " + tag + s.while())
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
	t := parseInjectTag(field)
	if t.name != "" {
		r, exists := s.lookupByTag(t.name)
		if !exists {
			return nil, nil
		}
		if !canAssign(r.outType, field.Type) {
			return Match{Tag: t.name}, errorsx.Annotate(errorsx.Newf(TypeMismatch, nil, "type %s is not assignable to %s", r.outType, field.Type), "path", path)
		}
		return Match{Tag: t.name}, nil
	}

	if r, ok := s.lookupByType(field.Type); ok {
		return Match{Type: r.outType}, nil
	}
	return nil, nil
}

// injectTag is a field's inject tag, read the same way by Find and Assign.
//
// No tag, or `inject:";optional"`, matches by type and leaves the field alone
// when nothing is bound. `inject:""` also matches by type, but the field says
// it must be injected, so nothing bound is a wiring mistake. `inject:"name"`
// matches the binding with that tag, required unless `;optional` follows it.
type injectTag struct {
	name     string
	required bool
}

func parseInjectTag(field reflect.StructField) injectTag {
	tag, ok := field.Tag.Lookup("inject")
	if !ok {
		return injectTag{}
	}
	name, optional := strings.CutSuffix(tag, ";optional")
	return injectTag{name: name, required: !optional}
}

func (s *Source) Assign(cfg *config.Config, path reflectx.Path, value reflect.Value, field reflect.StructField) (bool, error) {
	t := parseInjectTag(field)
	if t.name != "" {
		r, exists := s.lookupByTag(t.name)
		if !exists {
			if !t.required {
				return false, s.installModule(cfg, path, value, false)
			}
			panic(fmt.Errorf("required value not found for tag at %s: %s%s", path, t.name, s.while()))
		}
		v, err := r.resolve(value)
		if err != nil {
			return true, errorsx.Annotate(err, "path", path)
		}
		set(path, value, v)
		return true, s.installModule(cfg, path, value, true)
	}

	r, ok := s.lookupByType(field.Type)
	if !ok {
		if t.required {
			panic(fmt.Errorf("required value not found for type at %s: %s%s", path, field.Type, s.while()))
		}
		return false, s.installModule(cfg, path, value, false)
	}
	v, err := r.resolve(value)
	if err != nil {
		return true, errorsx.Annotate(err, "path", path)
	}
	set(path, value, v)
	return true, s.installModule(cfg, path, value, true)
}

// installModule checks whether the field holds a module, and installs it if so.
//
// populate says where the instance came from. A module a binding supplied
// arrived from outside the walk with nothing filled in, so its fields are
// populated here; one the walk reached as a struct field of its own was
// allocated and filled in by the walk already, since Visit runs after a field's
// children, and goes straight to Install.
//
// A walked module is installed but its field is not claimed - nothing was
// assigned to it, so the sources after this one should still see it.
func (s *Source) installModule(cfg *config.Config, path reflectx.Path, value reflect.Value, populate bool) error {
	mod, ok := asModule(value)
	if !ok || isNilModule(mod) {
		// Walk leaves a pointer nil where it declines to recurse, which is
		// precisely the self-referential case its seen map guards, and a nil
		// interface field that no binding claimed holds nothing to install.
		return nil
	}
	// A module publishes beside itself, into the scope its field belongs to,
	// rather than into the scope that closes with it. A nil parent means this
	// Source is not being walked by a Config at all, so its own is the only
	// place to publish to.
	publish := s.parent
	if publish == nil {
		publish = s
	}
	if err := s.install(cfg, path, mod, publish, populate); err != nil {
		return errorsx.Annotate(err, "path", path)
	}
	return nil
}

// install validates a module instance, populates its own fields from every
// source in the Config, and calls Install so it can register new bindings.
//
// An instance this walk has installed already is skipped, and a module reached
// from its own subtree is a cycle.
func (s *Source) install(cfg *config.Config, path reflectx.Path, mod Module, publish *Source, populate bool) error {
	// reflectx.Walk panics on anything that is not a non-nil pointer to a
	// struct, so reject those here, where the module can be named. This is a
	// panic rather than an error because nothing a user does can cause it: a
	// module with no address behind it is a binding wired to the wrong sort of
	// field, and it fails the same way every run.
	if v := reflect.ValueOf(mod); v.Kind() != reflect.Pointer || v.IsNil() || v.Elem().Kind() != reflect.Struct {
		panic(errorsx.Newf(InvalidModule, nil, "module %s must be a non-nil pointer to a struct", reflect.TypeOf(mod)))
	}

	w := s.walk
	if w == nil {
		// Nothing opened a walk, so this Source is being driven by hand rather
		// than by a Config: one install, with state of its own and nothing to
		// share it with.
		w = &walk{}
	}

	t := reflect.TypeOf(mod)

	// A zero-sized module has no identity to dedupe on: Go may put every
	// zero-size allocation at the same address, so two distinct instances of an
	// empty module are indistinguishable here. Install those every time and let
	// a genuine double registration collide loudly on its own bindings, rather
	// than silently dropping the second and calling it dedupe.
	dedupe := t.Elem().Size() > 0
	if dedupe && w.installed[mod] {
		return nil
	}
	if slices.Contains(w.stack, t) {
		// A module reached from its own subtree would recurse until the stack
		// ran out. It is a wiring mistake and never depends on the input, so it
		// panics rather than returning.
		panic(errorsx.Annotate(errorsx.Newf(ModuleCycle, nil, "module %s installs itself", t), "path", path))
	}

	// Push before the fields are populated, not just before Install: a BindFunc
	// factory can run while they are being assigned and re-enter through a
	// module field of its own.
	w.stack = append(w.stack, t)
	outer := w.path
	w.path = path
	defer func() {
		w.stack = w.stack[:len(w.stack)-1]
		w.path = outer
	}()

	if populate {
		if err := cfg.Assign(mod); err != nil {
			return errorsx.Wrapf(err, "failed to configure module %s", t)
		}
	}

	if err := mod.Install(publish); err != nil {
		return errorsx.Wrapf(err, "failed to install module %s", t)
	}

	if dedupe {
		if w.installed == nil {
			w.installed = make(map[Module]bool)
		}
		w.installed[mod] = true
	}
	return nil
}

// while describes where the module currently installing was found, for
// appending to panic messages raised underneath it - a duplicate binding in an
// Install, say, which otherwise says what collided but not who did it.
func (s *Source) while() string {
	if s.walk == nil || len(s.walk.stack) == 0 {
		return ""
	}
	return fmt.Sprintf(" (while installing %s at %s)", s.walk.stack[len(s.walk.stack)-1], s.walk.path)
}

// asModule extracts a Module from a reflect.Value, checking the value's address
// first, then the value itself, then what an interface holds.
//
// The address comes first because a struct field is addressable inside its
// parent and a pointer's method set includes the value receivers, so the
// address is always at least as good a module as the value - and unlike the
// value it is something the walk can populate. Taking it is what lets a module
// whose Install has a value receiver be built and filled in rather than
// rejected for not being a pointer.
func asModule(value reflect.Value) (Module, bool) {
	if value.Kind() == reflect.Struct && value.CanAddr() {
		if mod, ok := value.Addr().Interface().(Module); ok {
			return mod, true
		}
	}
	if value.Type().Implements(moduleType) {
		mod, ok := value.Interface().(Module)
		return mod, ok
	}
	if value.Kind() == reflect.Interface && !value.IsNil() {
		if mod, ok := value.Elem().Interface().(Module); ok {
			return mod, true
		}
	}
	if value.CanAddr() && value.Addr().Type().Implements(moduleType) {
		mod, ok := value.Addr().Interface().(Module)
		return mod, ok
	}
	return nil, false
}

// isNilModule reports whether mod is a typed nil pointer, which has a Module
// method set but no struct behind it.
func isNilModule(mod Module) bool {
	v := reflect.ValueOf(mod)
	return !v.IsValid() || (v.Kind() == reflect.Pointer && v.IsNil())
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

// Get returns the value bound to T, matching as an untagged field of that type
// would. It is how a module reads a binding back programmatically rather than
// by declaring a field for it, and it resolves against the scope of the Source
// it is given - so inside an Install it sees what is in scope where the module
// was found.
//
// It panics if nothing is bound to T, the same way a field tagged for a binding
// that does not exist does: the field and the binding are both fixed at compile
// time, so a missing one fails every run. TryGet is the form for a lookup that
// is genuinely optional.
//
// A BindFunc factory is called afresh on every Get, with the zero value of its
// type since there is no field to build on. A factory that fails panics as
// well, because Get has nowhere to put an error - a binding whose failure has
// to be handled belongs in a field, where Assign reports it.
func Get[T any](s *Source) T {
	out, ok := TryGet[T](s)
	if !ok {
		panic(errorsx.Newf(NotBound, nil, "no binding for type %s", reflect.TypeFor[T]()))
	}
	return out
}

// TryGet is Get for a binding that may legitimately not be there, reporting
// whether it was rather than panicking. Everything else still panics: a binding
// that exists but cannot be assigned to T is a wiring mistake either way.
func TryGet[T any](s *Source) (T, bool) {
	var out T
	target := reflect.ValueOf(&out).Elem()

	r, ok := s.lookupByType(target.Type())
	if !ok {
		return out, false
	}
	read(r, target)
	return out, true
}

// GetTag returns the value bound to a tag. It is the Get equivalent of a field
// tagged `inject:"<tag>"`.
func GetTag[T any](s *Source, tag string) T {
	out, ok := TryGetTag[T](s, tag)
	if !ok {
		panic(errorsx.Newf(NotBound, nil, "no binding for tag %s", tag))
	}
	return out
}

// TryGetTag is GetTag for a tag that may legitimately not be bound, the
// equivalent of a field tagged `inject:"<tag>;optional"`.
func TryGetTag[T any](s *Source, tag string) (T, bool) {
	var out T
	target := reflect.ValueOf(&out).Elem()

	r, ok := s.lookupByTag(tag)
	if !ok {
		return out, false
	}
	read(r, target)
	return out, true
}

// read resolves r into target. BindFunc guarantees a factory's argument type
// matches its return type, so the zero value of outType is always a valid
// stand-in for the field value a resolver would otherwise receive.
func read(r resolver, target reflect.Value) {
	if !canAssign(r.outType, target.Type()) {
		panic(errorsx.Newf(TypeMismatch, nil, "type %s is not assignable to %s", r.outType, target.Type()))
	}
	v, err := r.resolve(reflect.Zero(r.outType))
	if err != nil {
		panic(err)
	}
	if err := convert(target, v); err != nil {
		panic(err)
	}
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
