package inject

import "github.com/pasataleo/go-errorsx/pkg/errorsx"

// Who made the mistake decides how it is reported. A value a user supplied -
// an unparseable flag, a missing environment variable - comes back as an error,
// because the program is working and the input is not. Wiring the injector up
// wrongly panics: a duplicate binding, a tag nothing is bound to, a module that
// is not a pointer. None of those depend on the input, so they fail on the
// first run and every run after it, and an error return would only carry them
// further from the line that caused them.
//
// These codes are used both ways. A panic carries the coded error as its value,
// so a recover can still ask what went wrong; Get and GetTag return theirs,
// because asking whether something is bound is a question rather than a fault.
const (
	// NotBound means a lookup found no binding for the requested type or tag.
	NotBound errorsx.Code = "inject.not_bound"
	// TypeMismatch means a binding was found but its value cannot be assigned
	// to the requested type.
	TypeMismatch errorsx.Code = "inject.type_mismatch"
	// ModuleCycle means a module transitively installed itself.
	ModuleCycle errorsx.Code = "inject.module_cycle"
	// InvalidModule means a module is not a non-nil pointer to a struct, so its
	// fields cannot be populated.
	InvalidModule errorsx.Code = "inject.invalid_module"
)
