package worldstate_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

func id(n byte) ids.ULID {
	var made ids.ULID
	made[0], made[15] = 1, n

	return made
}

func TestNames(t *testing.T) {
	t.Parallel()
	for _, good := range []string{"mood", "gold_owed", "a1", "x"} {
		if err := worldstate.CheckName(good); err != nil {
			t.Errorf("%q: %v", good, err)
		}
	}
	for _, bad := range []string{"", "george's", "Mood", "1st", "_x", "has space", "a.b", "abcdefghijklmnopqrstuvwxyzabcdefghijklmno"} {
		if err := worldstate.CheckName(bad); !errors.Is(err, worldstate.ErrValue) {
			t.Errorf("%q should be refused: %v", bad, err)
		}
	}
}

func TestValuesOfEachType(t *testing.T) {
	t.Parallel()
	mood := worldstate.New(id(1), "mood", worldstate.ScopeNPC, worldstate.TypeChoice)
	mood.Choices = []worldstate.Choice{{ID: "c1", Name: "calm"}, {ID: "c2", Name: "cross"}}
	if err := mood.Check(worldstate.ScopeNPC); err != nil {
		t.Fatal(err)
	}
	if got := mood.Label(mood.Zero()); got != "calm" {
		t.Fatalf("a list starts at its default: %q", got)
	}
	cross, err := mood.Parse(json.RawMessage(`"c2"`))
	if err != nil || mood.Label(cross) != "cross" {
		t.Fatalf("a choice reads by its ID: %v %v", cross, err)
	}
	if _, err := mood.Parse(json.RawMessage(`"cross"`)); !errors.Is(err, worldstate.ErrValue) {
		t.Fatalf("a choice's name is not its ID: %v", err)
	}
	mood.Choices[1].Name = "furious"
	if mood.Label(cross) != "furious" {
		t.Fatal("renaming a choice keeps what was set")
	}
	for _, check := range []struct {
		kind  worldstate.Type
		saved string
		label string
	}{
		{worldstate.TypeNumber, "-12", "-12"},
		{worldstate.TypeYesNo, "true", "yes"},
		{worldstate.TypeNPC, `""`, "none"},
	} {
		declared := worldstate.New(id(2), "value", worldstate.ScopeLocal, check.kind)
		value, err := declared.Parse(json.RawMessage(check.saved))
		if err != nil || declared.Label(value) != check.label || !bytes.Equal(value.Encode(), []byte(check.saved)) {
			t.Errorf("%s: %+v %v", check.kind, value, err)
		}
	}
	count := worldstate.New(id(3), "count", worldstate.ScopeWorld, worldstate.TypeNumber)
	if _, err := count.Parse(json.RawMessage(`2000000000`)); !errors.Is(err, worldstate.ErrValue) {
		t.Fatalf("numbers are bounded: %v", err)
	}
	if _, err := count.Parse(json.RawMessage(`true`)); !errors.Is(err, worldstate.ErrValue) {
		t.Fatalf("a number is not yes or no: %v", err)
	}
}

func TestScopesAndTypes(t *testing.T) {
	t.Parallel()
	someone := worldstate.New(id(1), "customer", worldstate.ScopeNPC, worldstate.TypeNPC)
	if err := someone.Check(worldstate.Scopes()...); !errors.Is(err, worldstate.ErrValue) {
		t.Fatalf("an NPC type is for local values only: %v", err)
	}
	someone.Scope = worldstate.ScopeLocal
	if err := someone.Check(worldstate.ScopeLocal); err != nil {
		t.Fatal(err)
	}
	if err := someone.Check(worldstate.Scopes()...); !errors.Is(err, worldstate.ErrValue) {
		t.Fatalf("the world state declares no local values: %v", err)
	}
	empty := worldstate.New(id(2), "mood", worldstate.ScopeNPC, worldstate.TypeChoice)
	empty.Choices = nil
	if err := empty.Check(worldstate.ScopeNPC); !errors.Is(err, worldstate.ErrValue) {
		t.Fatalf("a list needs a choice: %v", err)
	}
}

func TestDeclarations(t *testing.T) {
	t.Parallel()
	document := worldstate.NewDocument()
	mood := worldstate.New(id(2), "mood", worldstate.ScopeNPC, worldstate.TypeChoice)
	robbed := worldstate.New(id(1), "shop_robbed", worldstate.ScopeWorld, worldstate.TypeYesNo)
	for _, declared := range []worldstate.Declaration{mood, robbed} {
		if err := document.Values.Insert(declared); err != nil {
			t.Fatal(err)
		}
	}
	if values := document.Values.Values(); len(values) != 2 || values[0].ID != robbed.ID {
		t.Fatalf("kept in the order made: %+v", values)
	}
	again := worldstate.New(id(3), "mood", worldstate.ScopeNPC, worldstate.TypeNumber)
	if err := document.Values.Insert(again); !errors.Is(err, worldstate.ErrValue) {
		t.Fatalf("one name per scope: %v", err)
	}
	again.Scope = worldstate.ScopeZone
	if err := document.Values.Insert(again); err != nil {
		t.Fatalf("the same name in another scope is another value: %v", err)
	}
	renamed := mood.Clone()
	renamed.Name = "temper"
	previous, err := document.Values.Replace(renamed)
	if err != nil || !previous.Equal(mood) {
		t.Fatalf("replace returns the value as it was: %+v %v", previous, err)
	}
	renamed.Name = "shop_robbed"
	renamed.Scope = worldstate.ScopeWorld
	if _, err := document.Values.Replace(renamed); !errors.Is(err, worldstate.ErrValue) {
		t.Fatalf("a rename cannot clash: %v", err)
	}
	data, err := document.Encode()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := worldstate.Decode(data)
	if err != nil || !decoded.Values.Equal(document.Values) {
		t.Fatalf("saved and read unchanged: %v", err)
	}
	removed, err := decoded.Values.Remove(robbed.ID)
	if err != nil || removed.ID != robbed.ID || decoded.Values.Len() != 2 {
		t.Fatalf("remove: %+v %v", removed, err)
	}
	if _, err := worldstate.Decode(json.RawMessage(`{"values":[{"id":"","name":"x"}]}`)); !errors.Is(err, worldstate.ErrData) {
		t.Fatalf("a broken value is refused: %v", err)
	}
	if empty, err := worldstate.Decode(json.RawMessage(`{"values":[]}`)); err != nil || empty.Values.Len() != 0 {
		t.Fatalf("an empty world state: %v", err)
	}
}
