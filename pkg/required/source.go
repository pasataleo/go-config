package required

import (
	"reflect"

	config "github.com/pasataleo/go-config/pkg/config"
	"github.com/pasataleo/go-errorsx/pkg/errorsx"
	"github.com/pasataleo/go-reflectx/pkg/reflectx"
)

var (
	_ config.Source = (*Source)(nil)
)

type Source struct{}

func (s *Source) Name() string {
	return "required"
}

func (s *Source) Find(_ *config.Config, path reflectx.Path, value reflect.Value, field reflect.StructField) (interface{}, error) {
	name, ok := field.Tag.Lookup("required")
	if !ok {
		return nil, nil
	}
	return name, nil
}

func (s *Source) Assign(_ *config.Config, path reflectx.Path, value reflect.Value, field reflect.StructField) (bool, error) {
	name, ok := field.Tag.Lookup("required")
	if !ok {
		return false, nil
	}

	if !value.IsZero() {
		return false, nil
	}

	return false, errorsx.Annotate(errorsx.Newf(config.MissingValue, nil, "required value not set: %s", name), "path", path)
}

// Find returns a map from field path to the required tag value (the display
// name for error messages), based on the "required" struct tag.
func Find(v interface{}) map[string]string {
	found := config.Find(new(Source), v)
	f := make(map[string]string, len(found))
	for k, v := range found {
		f[k] = v.(string)
	}
	return f
}

// Assign validates that all fields marked with the "required" struct tag have
// been set to a non-zero value. Returns an error for any required field that
// is still zero.
func Assign(v interface{}) error {
	return config.Assign(new(Source), v)
}
