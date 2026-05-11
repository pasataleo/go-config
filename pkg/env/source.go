package env

import (
	"os"
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
	return "env"
}

func (s *Source) Find(_ *config.Config, path reflectx.Path, value reflect.Value, field reflect.StructField) (interface{}, error) {
	tag, ok := field.Tag.Lookup("env")
	if !ok {
		return nil, nil
	}
	if !reflectx.CanSetString(value.Type()) {
		return tag, errorsx.Annotate(errorsx.Newf(errorsx.Unknown, nil, "%q can't set from string", value.Type().String()), "path", path)
	}
	return tag, nil
}

func (s *Source) Assign(_ *config.Config, path reflectx.Path, value reflect.Value, field reflect.StructField) (bool, error) {
	tag, ok := field.Tag.Lookup("env")
	if !ok {
		return false, nil
	}

	v, ok := os.LookupEnv(tag)
	if !ok {
		return false, nil
	}

	if err := reflectx.SetString(reflectx.Unpack(value), v, reflectx.EmptyStringIsTrue()); err != nil {
		return true, errorsx.Annotate(errorsx.Newf(config.InvalidValue, nil, "unable to read %s: %s", tag, err), "path", path)
	}
	return true, nil
}

// Find returns a map from field path to the environment variable name that
// would be used to populate it, based on the "env" struct tag.
func Find(v interface{}) map[string]string {
	found := config.Find(new(Source), v)
	f := make(map[string]string, len(found))
	for k, v := range found {
		f[k] = v.(string)
	}
	return f
}

// Assign populates the exported fields of v from environment variables,
// using the "env" struct tag to determine the variable name for each field.
func Assign(v interface{}) error {
	return config.Assign(new(Source), v)
}
