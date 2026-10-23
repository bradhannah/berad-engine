package textdoc_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/bradhannah/berad-engine/editor/model"
	"github.com/bradhannah/berad-engine/editor/plugin"
	"github.com/bradhannah/berad-engine/editor/plugins/textdoc"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/tilefloor"
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

func TestEditsMeetUndoContract(t *testing.T) {
	t.Parallel()
	registry, err := plugin.NewRegistry(textdoc.Plugin())
	if err != nil {
		t.Fatal(err)
	}
	document := textdoc.NewDocument()
	body := encode(t, textdoc.SetBodyArgs{Body: "Line one\nLine two: café"})
	if err := model.VerifyUndo(document, textdoc.SetBody{}, body, registry.Edit); err != nil {
		t.Fatal(err)
	}
	location := encode(t, textdoc.SetLocationArgs{Location: tilefloor.AnchorRef{Floor: id(1), Anchor: id(2)}})
	if err := model.VerifyUndo(document, textdoc.SetLocation{}, location, registry.Edit); err != nil {
		t.Fatal(err)
	}
	if _, err := (textdoc.SetLocation{}).Apply(document, location); err != nil {
		t.Fatal(err)
	}
	references := document.References()
	if len(references) != 1 || references[0].Document != id(1) || references[0].Item != id(2).String() {
		t.Fatalf("references %+v", references)
	}
	half := encode(t, textdoc.SetLocationArgs{Location: tilefloor.AnchorRef{Floor: id(1), Anchor: ids.ULID{}}})
	if _, err := (textdoc.SetLocation{}).Apply(document, half); !errors.Is(err, model.ErrArgs) {
		t.Fatalf("half a location: %v", err)
	}
	huge := encode(t, textdoc.SetBodyArgs{Body: strings.Repeat("x", textdoc.MaxBody+1)})
	if _, err := (textdoc.SetBody{}).Apply(document, huge); !errors.Is(err, model.ErrArgs) {
		t.Fatalf("oversized body: %v", err)
	}
	data, err := document.Encode()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := textdoc.Decode(data)
	if err != nil || decoded.(*textdoc.Document).Location != document.Location {
		t.Fatalf("round trip %v", err)
	}
}
