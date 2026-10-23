package config

import "github.com/pasataleo/go-errorsx/pkg/errorsx"

const (
	// InvalidValue means a string value (from a flag or environment variable) couldn't be assigned to a
	// specific value (because, for example, it required an int or a bool).
	InvalidValue errorsx.Code = "config.invalid_value"
	MissingValue errorsx.Code = "config.missing_value"

	// ConflictingValues means more than one value was set where only one may be, such as two fields of the same
	// required oneof group.
	ConflictingValues errorsx.Code = "config.conflicting_values"
)
