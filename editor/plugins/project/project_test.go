package project_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/bradhannah/berad-engine/editor/model"
	"github.com/bradhannah/berad-engine/editor/plugin"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/engine/ids"
)

func id(seed byte) ids.ULID {
	var value ids.ULID
	value[15] = seed
	return value
}

func encode(t *testing.T, value any) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// TestLifecycleEditsMeetUndoContract checks creating, renaming and deleting
// entries, where create and delete undo each other.
func TestLifecycleEditsMeetUndoContract(t *testing.T) {
	t.Parallel()
	registry, err := plugin.NewRegistry(project.Plugin())
	if err != nil {
		t.Fatal(err)
	}
	index := project.NewIndex([]project.Entry{{ID: id(5), Name: "Town", Format: "zone"}})
	create := encode(t, project.DocumentArgs{
		ID: id(2), Format: "tilefloor", Name: "Ground", Data: json.RawMessage(`{"x":1}`),
	})
	if err := model.VerifyUndo(index, project.Create{}, create, registry.Edit); err != nil {
		t.Fatal(err)
	}
	applied, err := (project.Create{}).Apply(index, create)
	if err != nil {
		t.Fatal(err)
	}
	if applied.InverseEdit != project.DeleteEdit || len(applied.Covers) != 1 || applied.Covers[0] != id(2) {
		t.Fatalf("applied %+v", applied)
	}
	rename := encode(t, project.RenameArgs{ID: id(2), Name: "Cellar"})
	if err := model.VerifyUndo(index, project.Rename{}, rename, registry.Edit); err != nil {
		t.Fatal(err)
	}
	remove := encode(t, project.DocumentArgs{ID: id(5), Format: "zone", Name: "Town"})
	if err := model.VerifyUndo(index, project.Delete{}, remove, registry.Edit); err != nil {
		t.Fatal(err)
	}
	if entries := index.Entries(); len(entries) != 2 || entries[0].ID != id(2) {
		t.Fatalf("entries are kept in ID order: %+v", entries)
	}
	if _, err := (project.Create{}).Apply(index, create); !errors.Is(err, model.ErrArgs) {
		t.Fatalf("creating an existing ID: %v", err)
	}
	for _, name := range []string{"", " Padded", string(make([]byte, project.MaxNameLength+1))} {
		bad := encode(t, project.RenameArgs{ID: id(2), Name: name})
		if _, err := (project.Rename{}).Apply(index, bad); !errors.Is(err, model.ErrArgs) {
			t.Fatalf("name %q: %v", name, err)
		}
	}
	if got := index.OfFormat("zone"); len(got) != 1 || got[0].Name != "Town" {
		t.Fatalf("of format: %+v", got)
	}
}
