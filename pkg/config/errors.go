package config

import "github.com/pasataleo/go-errorsx/pkg/errorsx"

const (
	// InvalidValue means a string value (from a flag or environment variable) couldn't be assigned to a
	// specific value (because, for example, it required an int or a bool).
	InvalidValue errorsx.Code = "config.invalid_value"
	MissingValue errorsx.Code = "config.missing_value"
)
