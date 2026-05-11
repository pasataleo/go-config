package usage

import (
	"reflect"

	"github.com/pasataleo/go-config/pkg/config"
	"github.com/pasataleo/go-reflectx/pkg/reflectx"
)

var (
	_ config.Source = (*Source)(nil)
)

type Source struct{}

func (s *Source) Name() string {
	return "usage"
}

func (s *Source) Find(_ *config.Config, path reflectx.Path, value reflect.Value, field reflect.StructField) (interface{}, error) {
	tag, ok := field.Tag.Lookup("usage")
	if !ok {
		return nil, nil
	}
	return tag, nil
}

func (s *Source) Assign(_ *config.Config, path reflectx.Path, value reflect.Value, field reflect.StructField) (bool, error) {
	return false, nil
}

// Find returns a map from field path to the usage description string, based on
// the "usage" struct tag.
func Find(v interface{}) map[string]string {
	found := config.Find(new(Source), v)
	f := make(map[string]string, len(found))
	for k, v := range found {
		f[k] = v.(string)
	}
	return f
}
