package mapview_test

import (
	"testing"

	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
	"github.com/bradhannah/berad-engine/engine/play"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// withSoundingClerk places Ada Clerk at (9, 8), following "play the door
// opening, then wait an hour".
func withSoundingClerk(t *testing.T, f *fixture) {
	t.Helper()
	withClerk(f)
	routine, err := flow.New(frametest.ID(80))
	if err != nil {
		t.Fatal(err)
	}
	sound, err := flow.NewNode(flow.KindPlaySound)
	if err != nil {
		t.Fatal(err)
	}
	sound.ID = frametest.ID(83)
	if sound, err = sound.WithParam("cue", "door/open"); err != nil {
		t.Fatal(err)
	}
	wait, err := flow.NewNode(flow.KindWait)
	if err != nil {
		t.Fatal(err)
	}
	wait.ID = frametest.ID(82)
	if wait, err = wait.WithParam("duration", flow.Duration{Amount: 1, Unit: flow.UnitHours, Clock: flow.ClockGame}); err != nil {
		t.Fatal(err)
	}
	if err := routine.Insert([]flow.Node{sound, wait}, nil, []flow.Link{
		{From: frametest.ID(80), Output: flow.OutputNext, To: sound.ID},
		{From: sound.ID, Output: flow.OutputNext, To: wait.ID},
	}); err != nil {
		t.Fatal(err)
	}
	f.docs.Put(routineID, "Clerk day", &workflowplugin.Document{Workflow: routine})
	ground := groundFloor(t)
	if err := ground.Floor.InsertPlacement(0, floor.Placement{ID: clerkPlacement, NPC: clerkID, Col: 9, Row: 8}); err != nil {
		t.Fatal(err)
	}
	f.docs.Put(groundID, "Ground", ground)
}

// TestNPCsPlayTheirSounds fetches the cues of the workflow the clerk follows
// ahead, plays the door once when the clerk reaches its Play sound, and
// stopping the preview stops the sound.
func TestNPCsPlayTheirSounds(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	withSoundingClerk(t, f)
	f.startPreview("cell:3,3")
	f.idle(play.StepTicks)
	sounds, ok := f.ctx.Sounds.(*frametest.Sounds)
	if !ok {
		t.Fatal("the test window plays through the fake")
	}
	if len(sounds.Prepared) == 0 || len(sounds.Prepared[0]) != 1 || sounds.Prepared[0][0] != "door/open" {
		t.Fatalf("the workflow's cues are fetched ahead: %v", sounds.Prepared)
	}
	if len(sounds.Played) != 1 || sounds.Played[0] != "door/open" || !f.has("heard:door/open") {
		t.Fatalf("the door plays once, when the clerk reaches it: %v", sounds.Played)
	}
	f.keys("P")
	if sounds.Stops != 1 {
		t.Fatal("stopping the preview stops its sounds")
	}
}
