package defaults

import (
	"fmt"
	"reflect"

	"github.com/pasataleo/go-config/pkg/config"
	"github.com/pasataleo/go-errorsx/pkg/errorsx"
	"github.com/pasataleo/go-reflectx/pkg/reflectx"
)

var (
	_ config.Source = (*Source)(nil)
)

type Source struct{}

func (s *Source) Name() string {
	return "defaults"
}

func (s *Source) Find(_ *config.Config, path reflectx.Path, value reflect.Value, field reflect.StructField) (interface{}, error) {
	tag, ok := field.Tag.Lookup("default")
	if !ok {
		return nil, nil
	}
	if !reflectx.CanSetString(value.Type()) {
		return tag, errorsx.Annotate(errorsx.Newf(errorsx.Unknown, nil, "%q can't set from string", value.Type().String()), "path", path)
	}
	return tag, nil
}

func (s *Source) Assign(_ *config.Config, path reflectx.Path, value reflect.Value, field reflect.StructField) (bool, error) {
	tag, ok := field.Tag.Lookup("default")
	if !ok {
		return false, nil
	}

	if !value.IsZero() {
		return true, nil
	}

	if err := reflectx.SetString(reflectx.Unpack(value), tag); err != nil {
		// failing to set a default value is a programmer error - it means they tried to assign an invalid value as
		// the default
		panic(fmt.Errorf("invalid default value %q for path %s: %w", tag, path, err))
	}

	return true, nil
}

// Find returns a map from field path to the default value string that would be
// used to populate it, based on the "default" struct tag.
func Find(v interface{}) map[string]string {
	found := config.Find(new(Source), v)
	f := make(map[string]string, len(found))
	for k, v := range found {
		f[k] = v.(string)
	}
	return f
}

// Assign populates the exported fields of v with default values, using the
// "default" struct tag. Fields that already have a non-zero value are skipped.
func Assign(v interface{}) error {
	return config.Assign(new(Source), v)
}
