package config

import (
	"reflect"
	"testing"

	"github.com/pasataleo/go-errorsx/pkg/errorsx"
	"github.com/pasataleo/go-reflectx/pkg/reflectx"
	"github.com/pasataleo/go-testingx/pkg/testingx"
)

// recorder logs what a walk asks of it, and fails the fields and the finish it
// is told to.
type recorder struct {
	log        []string
	failField  string
	failFinish bool
}

func (r *recorder) Name() string {
	return "recorder"
}

func (r *recorder) Find(_ *Config, path reflectx.Path, _ reflect.Value, _ reflect.StructField) (interface{}, error) {
	r.log = append(r.log, "find "+path.String())
	return nil, nil
}

func (r *recorder) Assign(_ *Config, path reflectx.Path, _ reflect.Value, _ reflect.StructField) (bool, error) {
	r.log = append(r.log, "assign "+path.String())
	if path.String() == r.failField {
		return false, errorsx.New(InvalidValue, nil, "field failed")
	}
	return false, nil
}

func (r *recorder) Finish(_ *Config) error {
	r.log = append(r.log, "finish")
	if r.failFinish {
		return errorsx.New(MissingValue, nil, "finish failed")
	}
	return nil
}

type target struct {
	A string
	B string
}

func TestFinish_afterTheWalk(t *testing.T) {
	r := new(recorder)

	if err := New(r).Assign(new(target)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	testingx.Capture(t, r.log).Equal([]string{"assign A", "assign B", "finish"})
}

func TestFinish_errorsJoinTheWalks(t *testing.T) {
	r := &recorder{failField: "A", failFinish: true}

	err := New(r).Assign(new(target))

	testingx.Capture(t, len(errorsx.Errors(err))).Equal(2)
	testingx.Capture(t, errorsx.IsCode(err, InvalidValue)).True()
	testingx.Capture(t, errorsx.IsCode(err, MissingValue)).True()
}

func TestFinish_notCalledByFind(t *testing.T) {
	r := new(recorder)

	New(r).Find(new(target))
	Find(r, new(target))

	for _, entry := range r.log {
		if entry == "finish" {
			t.Fatalf("Find called Finish: %v", r.log)
		}
	}
}

// nester assigns a struct of its own through the Config it is handed, the way
// inject populates a module, and records what Data held when it did.
type nester struct {
	recorder
	inner  interface{}
	nested bool
	shared bool
}

func (n *nester) Assign(cfg *Config, path reflectx.Path, value reflect.Value, field reflect.StructField) (bool, error) {
	if path.String() != "A" || n.nested {
		return false, nil
	}
	n.nested = true
	if err := cfg.Assign(n.inner); err != nil {
		return false, err
	}
	_, n.shared = cfg.Data()["inner"]
	return false, nil
}

// innerSource writes to Data during the nested Assign.
type innerSource struct{ recorder }

func (i *innerSource) Assign(cfg *Config, path reflectx.Path, _ reflect.Value, _ reflect.StructField) (bool, error) {
	cfg.Data()["inner"] = path.String()
	return false, nil
}

func TestData_sharedWithNestedAssigns(t *testing.T) {
	n := &nester{inner: new(target)}
	i := new(innerSource)

	if err := New(n, i).Assign(new(target)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	testingx.Capture(t, n.shared).True()

	// Finish runs once, at the end of the outermost Assign, not after the
	// nested one too.
	finishes := 0
	for _, entry := range n.log {
		if entry == "finish" {
			finishes++
		}
	}
	testingx.Capture(t, finishes).Equal(1)
}

func TestData_freshPerAssign(t *testing.T) {
	var seen []map[string]any
	s := &dataSource{seen: &seen}
	cfg := New(s)

	_ = cfg.Assign(new(target))
	_ = cfg.Assign(new(target))

	testingx.Capture(t, len(seen)).Equal(2)
	if _, leaked := seen[1]["assigned"]; leaked {
		t.Errorf("second Assign saw the first's data: %v", seen[1])
	}
}

// dataSource snapshots Data at the start of each Assign, then writes to it.
type dataSource struct {
	recorder
	seen *[]map[string]any
}

func (d *dataSource) Assign(cfg *Config, path reflectx.Path, _ reflect.Value, _ reflect.StructField) (bool, error) {
	if path.String() == "A" {
		snapshot := map[string]any{}
		for k, v := range cfg.Data() {
			snapshot[k] = v
		}
		*d.seen = append(*d.seen, snapshot)
	}
	cfg.Data()["assigned"] = true
	return false, nil
}

func TestData_nilInFind(t *testing.T) {
	var data map[string]any
	s := &findData{data: &data}

	New(s).Find(new(target))
	if data != nil {
		t.Errorf("Find had data: %v", data)
	}
}

type findData struct {
	recorder
	data *map[string]any
}

func (f *findData) Find(cfg *Config, _ reflectx.Path, _ reflect.Value, _ reflect.StructField) (interface{}, error) {
	*f.data = cfg.Data()
	return nil, nil
}
