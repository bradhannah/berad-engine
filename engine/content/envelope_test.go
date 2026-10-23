package content_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/bradhannah/berad-engine/engine/content"
	"github.com/bradhannah/berad-engine/engine/ids"
)

var testID = ids.MustParse("01J9XK4M7QF2ZB8T5N3R6W1VHC")

func renameField(from, to string) content.Migration {
	return func(data json.RawMessage) (json.RawMessage, error) {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(data, &fields); err != nil {
			return nil, err
		}
		fields[to] = fields[from]
		delete(fields, from)
		return json.Marshal(fields)
	}
}

func threeVersionFormat() content.Format {
	return content.Format{
		Name:    "sample",
		Current: 3,
		Migrations: map[int]content.Migration{
			1: renameField("a", "b"),
			2: renameField("b", "c"),
		},
	}
}

func TestUpgradeStepByStep(t *testing.T) {
	t.Parallel()
	registry := content.NewRegistry()
	if err := registry.Register(threeVersionFormat()); err != nil {
		t.Fatal(err)
	}
	old := content.Envelope{Format: "sample", Version: 1, ID: testID, Name: "x", Revision: 4, Data: json.RawMessage(`{"a":7}`)}
	upgraded, changed, err := registry.Upgrade(old)
	if err != nil {
		t.Fatal(err)
	}
	if !changed || upgraded.Version != 3 || string(upgraded.Data) != `{"c":7}` {
		t.Fatalf("upgraded = %+v changed=%v", upgraded, changed)
	}
	if upgraded.Revision != 4 || upgraded.ID != testID {
		t.Fatal("identity or revision changed")
	}

	current, changed, err := registry.Upgrade(upgraded)
	if err != nil || changed || current.Version != 3 {
		t.Fatalf("current upgrade: %v %v %v", current, changed, err)
	}
}

func TestRejectsNewerAndUnknown(t *testing.T) {
	t.Parallel()
	registry := content.NewRegistry()
	if err := registry.Register(threeVersionFormat()); err != nil {
		t.Fatal(err)
	}
	newer := content.Envelope{Format: "sample", Version: 4, ID: testID, Data: json.RawMessage(`{}`)}
	if _, _, err := registry.Upgrade(newer); !errors.Is(err, content.ErrNewerVersion) {
		t.Fatalf("newer: %v", err)
	}
	unknown := content.Envelope{Format: "other", Version: 1, ID: testID, Data: json.RawMessage(`{}`)}
	if _, _, err := registry.Upgrade(unknown); !errors.Is(err, content.ErrUnknownFormat) {
		t.Fatalf("unknown: %v", err)
	}
}

func TestMigrationFailureIsWrapped(t *testing.T) {
	t.Parallel()
	registry := content.NewRegistry()
	broken := errors.New("broken")
	err := registry.Register(content.Format{Name: "bad", Current: 2, Migrations: map[int]content.Migration{
		1: func(json.RawMessage) (json.RawMessage, error) { return nil, broken },
	}})
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = registry.Upgrade(content.Envelope{Format: "bad", Version: 1, ID: testID, Data: json.RawMessage(`{}`)})
	if !errors.Is(err, content.ErrMigration) || !errors.Is(err, broken) {
		t.Fatalf("err = %v", err)
	}
}

func TestRegisterValidation(t *testing.T) {
	t.Parallel()
	registry := content.NewRegistry()
	if err := registry.Register(content.Format{Name: "gap", Current: 3, Migrations: map[int]content.Migration{1: renameField("a", "b")}}); !errors.Is(err, content.ErrBadFormat) {
		t.Fatalf("missing step: %v", err)
	}
	if err := registry.Register(content.Format{Name: "", Current: 1}); !errors.Is(err, content.ErrBadFormat) {
		t.Fatalf("empty name: %v", err)
	}
	if err := registry.Register(content.Format{Name: "one", Current: 1}); err != nil {
		t.Fatal(err)
	}
	if err := registry.Register(content.Format{Name: "one", Current: 1}); !errors.Is(err, content.ErrDuplicateFormat) {
		t.Fatalf("duplicate: %v", err)
	}
	if names := registry.Names(); len(names) != 1 || names[0] != "one" {
		t.Fatalf("names = %v", names)
	}
}

func TestEncodeIsStableAndRoundTrips(t *testing.T) {
	t.Parallel()
	envelope := content.Envelope{Format: "sample", Version: 3, ID: testID, Name: "Floor 1", Revision: 9, Data: json.RawMessage(`{ "c" : [1, 2] }`)}
	first, err := content.Encode(envelope)
	if err != nil {
		t.Fatal(err)
	}
	second, err := content.Encode(envelope)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("encoding not stable")
	}
	if !bytes.HasSuffix(first, []byte("\n")) || !bytes.Contains(first, []byte(`"format": "sample"`)) {
		t.Fatalf("unexpected encoding:\n%s", first)
	}
	decoded, err := content.Decode(first)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.ID != testID || decoded.Revision != 9 || decoded.Name != "Floor 1" {
		t.Fatalf("decoded = %+v", decoded)
	}
	var data map[string][]int
	if err := json.Unmarshal(decoded.Data, &data); err != nil || len(data["c"]) != 2 {
		t.Fatalf("data = %s", decoded.Data)
	}
}

func TestDecodeRejectsMalformed(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{`not json`, `{"format":"x","version":1,"data":{}}`, `{"format":"","version":1,"id":"01J9XK4M7QF2ZB8T5N3R6W1VHC","data":{}}`} {
		if _, err := content.Decode([]byte(raw)); !errors.Is(err, content.ErrMalformed) {
			t.Fatalf("%s: err = %v", raw, err)
		}
	}
}
