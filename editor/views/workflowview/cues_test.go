package workflowview_test

import (
	"encoding/json"
	"image"
	"strings"
	"testing"

	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

func readyRecording(id string) protocol.AudioRecording {
	return protocol.AudioRecording{
		ID: id, Extension: ".wav", Version: "v1", Size: 9, CalibrationDB: 0, StartMs: 0, EndMs: 0,
		Status: protocol.RecordingReady,
	}
}

func testCue(id, name, category string, recordings ...protocol.AudioRecording) protocol.AudioCue {
	return protocol.AudioCue{
		ID: id, Name: name, Category: category, Gain: 1, SearchTerms: []string{"hinge"}, Recordings: recordings,
	}
}

// testLibrary has a cue of each category, and one whose recording is
// missing.
func testLibrary() protocol.AudioLibrary {
	return protocol.AudioLibrary{Problem: "", Cues: []protocol.AudioCue{
		testCue("bed/river", "River", protocol.CategoryAmbient, readyRecording("river")),
		testCue("chest/open", "Chest opens", protocol.CategorySFX, protocol.AudioRecording{
			ID: "chest", Extension: ".wav", Version: "", Size: 0, CalibrationDB: 0, StartMs: 0, EndMs: 0,
			Status: protocol.RecordingMissing,
		}),
		testCue("door/open", "Door opens", protocol.CategorySFX, readyRecording("door")),
		testCue("theme/title", "Title theme", protocol.CategoryMusic, readyRecording("title")),
	}}
}

// withPlaySound adds a Play sound after the start, selects it and answers
// the audio library question its arrival asks.
func withPlaySound(t *testing.T) *fixture {
	t.Helper()
	f := newFixture(t)
	f.click("node:start:1")
	f.click("palette:" + flow.KindPlaySound)
	f.frame(ui.NewInput(image.Pt(0, 0)))
	if len(f.requests.AudioAsks) != 1 {
		t.Fatalf("a workflow with a Play sound asks for the audio library once: %d", len(f.requests.AudioAsks))
	}
	f.requests.AudioAsks[0](testLibrary(), nil)

	return f
}

func (f *fixture) sounds() *frametest.Sounds { return f.ctx.Sounds.(*frametest.Sounds) }

// replaceNode changes a node as an edit made elsewhere would.
func (f *fixture) replaceNode(node flow.Node) {
	f.t.Helper()
	data, err := json.Marshal(workflowplugin.ReplaceArgs{Node: node})
	if err != nil {
		f.t.Fatal(err)
	}
	f.carryOut(frametest.Command{
		Command: protocol.Command{Document: workflowID, Edit: workflowplugin.ReplaceEdit, Gesture: "", Args: data}, Done: nil,
	})
}

// TestPlaySoundChoosesItsCue adds a Play sound in the Audio group with no
// cue, which is a problem; its chooser groups the cues by category, a search
// finds them by name, ID or search term, and clicking one chooses it.
func TestPlaySoundChoosesItsCue(t *testing.T) {
	t.Parallel()
	f := withPlaySound(t)
	if !f.has("selected:"+flow.KindPlaySound) || !f.has("problem:"+flow.KindPlaySound+":1") {
		t.Fatal("a new Play sound is selected, and has no cue yet")
	}
	f.click("ui:cue")
	if !f.has("chooser") || f.state.Focused() == nil || f.state.Focused().ID != "cue-search" {
		t.Fatal("clicking the cue opens the chooser with its search focused")
	}
	for _, cue := range []string{"cue:theme/title", "cue:door/open", "cue:bed/river"} {
		if _, ok := f.state.Locate(cue, f.docs); !ok {
			t.Fatalf("%s is listed", cue)
		}
	}
	music, _ := f.state.Locate("cue:theme/title", f.docs)
	sfx, _ := f.state.Locate("cue:door/open", f.docs)
	ambient, _ := f.state.Locate("cue:bed/river", f.docs)
	if music.Y >= sfx.Y || sfx.Y >= ambient.Y {
		t.Fatal("music comes first, then SFX, then ambient")
	}
	input := ui.NewInput(f.at("ui:cue-search"))
	input.Text = "hinge"
	f.frame(input)
	if _, ok := f.state.Locate("cue:door/open", f.docs); !ok {
		t.Fatal("a search term finds the door")
	}
	f.click("cue:door/open")
	if cue, err := f.nth(flow.KindPlaySound, 1).Cue("cue"); err != nil || cue != "door/open" {
		t.Fatalf("the cue is chosen: %q %v", cue, err)
	}
	if f.has("chooser") || f.has("problem:"+flow.KindPlaySound+":1") || !f.has("cue=door/open") {
		t.Fatal("choosing closes the chooser and the node has no problem")
	}
	if !strings.Contains(f.status(), "Door opens") {
		t.Fatalf("the status says which cue: %q", f.status())
	}
}

// TestPlaySoundIsHeard plays the chosen cue and stops it, plays a row's cue
// from the chooser without choosing it, and leaving the node stops what it
// played. A missing cue cannot be played.
func TestPlaySoundIsHeard(t *testing.T) {
	t.Parallel()
	f := withPlaySound(t)
	f.click("ui:cue")
	f.click("cue:theme/title")
	f.click("ui:listen")
	if played := f.sounds().Played; len(played) != 1 || played[0] != "theme/title" {
		t.Fatalf("Play plays the chosen cue: %v", played)
	}
	f.click("ui:listen")
	if f.sounds().Playing("theme/title") {
		t.Fatal("Stop stops it")
	}
	f.click("ui:cue")
	f.click("play:bed/river")
	if !f.sounds().Playing("bed/river") || !f.has("cue=theme/title") {
		t.Fatal("a row's play button plays its cue without choosing it")
	}
	if _, ok := f.state.Locate("play:chest/open", f.docs); ok {
		t.Fatal("a cue whose recording is missing has no play button")
	}
	f.keys("Escape")
	f.keys("Escape")
	if f.has("chooser") {
		t.Fatal("Escape closes the chooser")
	}
	stops := f.sounds().Stops
	f.click("node:start:1")
	if f.sounds().Stops != stops+1 {
		t.Fatal("leaving the node stops what it played")
	}
}

// TestChooserClosesFromItsCue closes the chooser when the cue is clicked
// again, forgetting the search.
func TestChooserClosesFromItsCue(t *testing.T) {
	t.Parallel()
	f := withPlaySound(t)
	f.click("ui:cue")
	input := ui.NewInput(f.at("ui:cue-search"))
	input.Text = "river"
	f.frame(input)
	if _, ok := f.state.Locate("cue:door/open", f.docs); ok {
		t.Fatal("the search hides the door")
	}
	f.click("ui:cue")
	if f.has("chooser") {
		t.Fatal("clicking the cue again closes the chooser")
	}
	f.click("ui:cue")
	if _, ok := f.state.Locate("cue:door/open", f.docs); !ok {
		t.Fatal("reopened, the chooser lists every cue again")
	}
}

// TestMissingCueIsAProblem marks a Play sound whose cue is not in the
// library, and one in a project with no library at all.
func TestMissingCueIsAProblem(t *testing.T) {
	t.Parallel()
	f := withPlaySound(t)
	node := f.nth(flow.KindPlaySound, 1)
	changed, err := node.WithParam("cue", "bell/ring")
	if err != nil {
		t.Fatal(err)
	}
	f.replaceNode(changed)
	if !f.has("problem:"+flow.KindPlaySound+":1") || f.has("cue=door/open") {
		t.Fatal("a cue the library does not have is a problem")
	}
	f.frame(ui.NewInput(image.Pt(0, 0)))
	empty := newFixture(t)
	empty.click("node:start:1")
	empty.click("palette:" + flow.KindPlaySound)
	empty.frame(ui.NewInput(image.Pt(0, 0)))
	empty.requests.AudioAsks[0](protocol.AudioLibrary{Problem: "This project has no audio library yet.", Cues: nil}, nil)
	empty.click("ui:cue")
	if !empty.has("chooser") || !empty.has("cues") {
		t.Fatal("the chooser opens and says why there is nothing to choose")
	}
}
