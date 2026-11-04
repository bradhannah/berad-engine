package workflow_test

import (
	"encoding/json"
	"errors"
	"slices"
	"testing"

	"github.com/bradhannah/berad-engine/engine/content"
	"github.com/bradhannah/berad-engine/engine/workflow"
)

// TestKindsAndZonesRoundTrip saves a zone loop running in a zone and reads
// it back, and reads a workflow saved before kinds as an NPC behaviour
// running in any zone.
func TestKindsAndZonesRoundTrip(t *testing.T) {
	t.Parallel()
	loop, err := workflow.NewFor(id(1), workflow.PurposeZoneLoop)
	if err != nil {
		t.Fatal(err)
	}
	if loop.Purpose() != workflow.PurposeZoneLoop || !loop.RunsIn().IsZero() {
		t.Fatal("a new zone loop runs in any zone")
	}
	if previous := loop.SetRunsIn(id(40)); !previous.IsZero() {
		t.Fatalf("setting the zone returns the one it ran in: %v", previous)
	}
	data, err := loop.Encode()
	if err != nil {
		t.Fatal(err)
	}
	read, err := workflow.Decode(data)
	if err != nil || !read.Equal(loop) || read.Purpose() != workflow.PurposeZoneLoop || read.RunsIn() != id(40) {
		t.Fatalf("a zone loop reads back with its zone: %v", err)
	}
	if _, err := workflow.NewFor(id(1), "cut-scene"); !errors.Is(err, workflow.ErrNode) {
		t.Fatal("there is no such kind of workflow")
	}
	registry := content.NewRegistry()
	if err := registry.Register(workflow.Format()); err != nil {
		t.Fatal(err)
	}
	seven := `{"start":"` + id(1).String() + `","nodes":[{"id":"` + id(1).String() + `","kind":"start"}]}`
	upgraded, changed, err := registry.Upgrade(content.Envelope{
		Format: workflow.FormatName, Version: 7, ID: id(9), Name: "Old", Revision: 1, Data: json.RawMessage(seven),
	})
	if err != nil || !changed || upgraded.Version != workflow.FormatVersion {
		t.Fatalf("upgrade %v %v", changed, err)
	}
	old, err := workflow.Decode(upgraded.Data)
	if err != nil || old.Purpose() != workflow.PurposeBehaviour || !old.RunsIn().IsZero() {
		t.Fatalf("a version 7 workflow is an NPC behaviour running in any zone: %v", err)
	}
}

// TestAZoneLoopRunsNoBody refuses a Go to, a Move, a Find NPC nearby and a Say in
// a zone loop, which an NPC behaviour takes, and offers neither in its
// palette.
func TestAZoneLoopRunsNoBody(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{workflow.KindGoTo, workflow.KindMove, workflow.KindFindNPC, workflow.KindSay} {
		loop, err := workflow.NewFor(id(1), workflow.PurposeZoneLoop)
		if err != nil {
			t.Fatal(err)
		}
		if err := loop.Insert([]workflow.Node{node(t, 2, kind, id(0))}, nil, nil); !errors.Is(err, workflow.ErrNode) {
			t.Fatalf("a zone loop refuses a %s", kind)
		}
		behaviour, err := workflow.New(id(1))
		if err != nil {
			t.Fatal(err)
		}
		if err := behaviour.Insert([]workflow.Node{node(t, 2, kind, id(0))}, nil, nil); err != nil {
			t.Fatalf("an NPC behaviour takes a %s: %v", kind, err)
		}
		if slices.ContainsFunc(workflow.KindsFor(workflow.PurposeZoneLoop), func(offered workflow.Kind) bool {
			return offered.Name == kind
		}) {
			t.Fatalf("a zone loop's palette offers no %s", kind)
		}
	}
	loop, err := workflow.NewFor(id(1), workflow.PurposeZoneLoop)
	if err != nil {
		t.Fatal(err)
	}
	if err := loop.Insert([]workflow.Node{node(t, 2, workflow.KindWait, id(0))}, nil, nil); err != nil {
		t.Fatalf("a zone loop waits: %v", err)
	}
	data, err := loop.Encode()
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]any
	if err := json.Unmarshal(data, &wire); err != nil {
		t.Fatal(err)
	}
	wire["nodes"] = append(wire["nodes"].([]any), map[string]any{"id": id(3).String(), "kind": workflow.KindGoTo})
	broken, err := json.Marshal(wire)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := workflow.Decode(broken); !errors.Is(err, workflow.ErrData) {
		t.Fatal("a saved zone loop holding a Go to is refused")
	}
}
