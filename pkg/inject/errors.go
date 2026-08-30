package inject

import "github.com/pasataleo/go-errorsx/pkg/errorsx"

const (
	// NotBound means a lookup found no binding for the requested type or tag.
	NotBound errorsx.Code = "inject.not_bound"
	// TypeMismatch means a binding was found but its value cannot be assigned
	// to the requested type.
	TypeMismatch errorsx.Code = "inject.type_mismatch"
	// ModuleCycle means a module transitively installed itself.
	ModuleCycle errorsx.Code = "inject.module_cycle"
	// InvalidModule means a registered module is not a non-nil pointer to a
	// struct, so its fields cannot be populated.
	InvalidModule errorsx.Code = "inject.invalid_module"
	// ReentrantInit means Init was called while modules were still installing,
	// which would leave the source bound to the wrong Config.
	ReentrantInit errorsx.Code = "inject.reentrant_init"
)
