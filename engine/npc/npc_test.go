package npc_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/npc"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

func TestNPCRoundTrips(t *testing.T) {
	t.Parallel()
	var routine ids.ULID
	routine[15] = 9
	var look ids.ULID
	look[15] = 10
	clerk := npc.New()
	var market ids.ULID
	market[15] = 11
	clerk.Zone, clerk.Behaviour, clerk.Sprite, clerk.Path = market, routine, look, "clerk"
	clerk.Notes = "Works the front desk.\nNever shown to players."
	var secret, guards ids.ULID
	secret[0], secret[15], guards[0], guards[15] = 1, 1, 1, 2
	if err := clerk.Owned.Values.Insert(worldstate.New(secret, "secret", worldstate.ScopeNPC, worldstate.TypeBool)); err != nil {
		t.Fatal(err)
	}
	if err := clerk.Owned.Apply(guards); err != nil {
		t.Fatal(err)
	}
	data, err := clerk.Encode()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := npc.Decode(data)
	if err != nil || !decoded.Equal(clerk) {
		t.Fatalf("round trip %+v %v", decoded, err)
	}
	copied := clerk.Clone()
	if _, err := copied.Owned.Values.Remove(secret); err != nil || copied.Equal(clerk) {
		t.Fatalf("a copy's values are its own: %v", err)
	}
	copied = clerk.Clone()
	if copied.Sprite = routine; copied.Equal(clerk) {
		t.Fatal("NPCs with different sprites differ")
	}
	copied = clerk.Clone()
	if copied.Zone = routine; copied.Equal(clerk) {
		t.Fatal("NPCs that live in different zones differ")
	}
	empty, err := npc.New().Encode()
	if err != nil || string(empty) != "{}" {
		t.Fatalf("an NPC with nothing set saves as {}: %s %v", empty, err)
	}
}

func TestDecodeRefusesBadData(t *testing.T) {
	t.Parallel()
	long, err := json.Marshal(map[string]string{"notes": strings.Repeat("a", npc.MaxNotes+1)})
	if err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string]json.RawMessage{"not json": json.RawMessage(`{`), "long notes": long} {
		if _, err := npc.Decode(data); !errors.Is(err, npc.ErrData) {
			t.Fatalf("%s: %v", name, err)
		}
	}
}

func TestPathNames(t *testing.T) {
	t.Parallel()
	for display, want := range map[string]string{
		"George": "george", "Old Tom": "old_tom", "George's brother": "george_s_brother", "NPC 1": "npc_1",
		"7 Dwarves": "npc_7_dwarves", "!!!": "npc", "  Mary-Kate  ": "mary_kate",
	} {
		if got := npc.SuggestPath(display); got != want || npc.CheckPath(got) != nil {
			t.Errorf("%q: %q, want %q", display, got, want)
		}
	}
	if long := npc.SuggestPath(strings.Repeat("a b ", 30)); npc.CheckPath(long) != nil {
		t.Errorf("a long name is cut to fit: %q", long)
	}
	taken := map[string]bool{"george": true, "george_2": true}
	if got := npc.UniquePath("george", taken); got != "george_3" {
		t.Fatalf("unique: %q", got)
	}
	if err := npc.CheckPath("george's"); !errors.Is(err, npc.ErrPath) {
		t.Fatalf("punctuation is refused: %v", err)
	}
	if _, err := npc.Decode(json.RawMessage(`{"path":"Bad Name"}`)); !errors.Is(err, npc.ErrData) {
		t.Fatalf("a saved bad path is refused: %v", err)
	}
	if err := npc.CheckPath(""); err != nil {
		t.Fatalf("no path name yet is allowed: %v", err)
	}
}

// TestOlderVersionsReadUnchanged upgrades an NPC saved before NPCs had
// sprites, path names, values of their own, a zone or values of their
// templates' constants.
func TestOlderVersionsReadUnchanged(t *testing.T) {
	t.Parallel()
	format := npc.Format()
	if format.Current != npc.FormatVersion || npc.FormatVersion != 6 {
		t.Fatalf("version %d", format.Current)
	}
	upgraded := json.RawMessage(`{"notes":"Saved before sprites."}`)
	for version := 1; version < format.Current; version++ {
		var err error
		if upgraded, err = format.Migrations[version](upgraded); err != nil {
			t.Fatal(err)
		}
	}
	decoded, err := npc.Decode(upgraded)
	if err != nil || decoded.Notes != "Saved before sprites." || !decoded.Sprite.IsZero() || decoded.Path != "" ||
		decoded.Owned.Len() != 0 || !decoded.Zone.IsZero() {
		t.Fatalf("upgraded %+v %v", decoded, err)
	}
}
