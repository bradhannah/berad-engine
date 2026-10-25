package audioview_test

import (
	"image"
	"reflect"
	"strings"
	"testing"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/audioview"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
)

type fixture struct {
	t        *testing.T
	state    *audioview.State
	docs     *frametest.Docs
	requests *frametest.Requests
	ctx      *frame.Context
}

func recording(id string, commercial bool) protocol.AudioRecording {
	return protocol.AudioRecording{
		ID: id, Extension: ".wav", Version: "v1", Size: 2048, CalibrationDB: 0, StartMs: 0, EndMs: 0,
		Status: protocol.RecordingReady, File: "imported/" + id + ".wav", OriginalFile: id + ".wav", Note: "",
		SearchTerms: nil,
		Attribution: protocol.Attribution{Creator: "A. Recordist", Source: "https://example.com", License: "CC0-1.0",
			Commercial: commercial},
	}
}

func cue(id, name, category string, recordings ...protocol.AudioRecording) protocol.AudioCue {
	return protocol.AudioCue{
		ID: id, Name: name, Category: category, CategorySet: true, Gain: 0.8, HowToHear: "Open a door.",
		SearchTerms: []string{"hinge"}, Recordings: recordings,
	}
}

// testLibrary has a cue of each category, one not cleared for a commercial
// game, one whose creator nobody recorded, and a recording no cue plays.
func testLibrary() protocol.AudioLibrary {
	unknown := recording("knock", true)
	unknown.Attribution.Creator = protocol.Unknown

	return protocol.AudioLibrary{Problem: "", Schema: 4, Cues: []protocol.AudioCue{
		cue("bed/river", "River", protocol.CategoryAmbient, recording("river", false)),
		cue("sfx/door", "Door opens", protocol.CategorySFX, recording("door_a", true), recording("door_b", true)),
		cue("sfx/knock", "Knock", protocol.CategorySFX, unknown),
		cue("theme/title", "Title theme", protocol.CategoryMusic, recording("title", true)),
	}, Unused: []protocol.AudioRecording{recording("spare", true)}}
}

func newFixture(t *testing.T) *fixture {
	t.Helper()

	return newFixtureWith(t, testLibrary())
}

// newFixtureWith opens the window on a library.
func newFixtureWith(t *testing.T, library protocol.AudioLibrary) *fixture {
	t.Helper()
	theme, err := ui.NewTheme()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(theme.Close)
	docs, requests := frametest.NewDocs(), &frametest.Requests{}
	f := &fixture{
		t: t, state: audioview.New(frametest.ID(1), theme), docs: docs, requests: requests,
		ctx: frametest.Context(docs, requests, image.Pt(1200, 800)),
	}
	f.state.Start(f.ctx)
	f.answer(library)

	return f
}

// answer answers the latest audio library question.
func (f *fixture) answer(library protocol.AudioLibrary) {
	f.t.Helper()
	if len(f.requests.AudioAsks) == 0 {
		f.t.Fatal("the window did not ask for the audio library")
	}
	asked := f.requests.AudioAsks[len(f.requests.AudioAsks)-1]
	f.requests.AudioAsks = nil
	asked(library, nil)
	f.frame(ui.NewInput(image.Pt(0, 0)))
}

func (f *fixture) frame(input ui.Input) { f.state.Update(&input, f.ctx) }

func (f *fixture) at(name string) image.Point {
	f.t.Helper()
	f.frame(ui.NewInput(image.Pt(0, 0)))
	point, ok := f.state.Locate(name, f.docs)
	if !ok {
		f.t.Fatalf("cannot locate %s", name)
	}

	return point
}

func (f *fixture) has(name string) bool {
	_, ok := f.state.Locate(name, f.docs)

	return ok
}

func (f *fixture) click(name string) {
	f.t.Helper()
	at := f.at(name)
	pressed := ui.NewInput(at)
	pressed.Pressed, pressed.Down = true, true
	f.frame(pressed)
	f.frame(ui.NewInput(at))
}

func (f *fixture) keys(names ...string) {
	input := ui.NewInput(image.Pt(0, 0))
	for _, name := range names {
		input.Presses = append(input.Presses, ui.ParsePress(name))
	}
	f.frame(input)
}

func (f *fixture) typeInto(field, text string) {
	f.t.Helper()
	f.click("ui:" + field)
	f.keys("Cmd+A")
	input := ui.NewInput(f.at("ui:" + field))
	input.Text = text
	f.frame(input)
}

// edit returns the only audio edit of a kind.
func (f *fixture) edit(name string) frametest.AudioEdit {
	f.t.Helper()
	edits := f.requests.AudioOf(name)
	if len(edits) != 1 {
		f.t.Fatalf("one %s, not %d", name, len(edits))
	}

	return edits[0]
}

// TestListsCuesAndRecordings lists cues under their categories, and
// recordings, those no cue plays apart, with their problems.
func TestListsCuesAndRecordings(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	for _, name := range []string{"cue:bed/river", "cue:sfx/door", "cue:theme/title", "section:category:music"} {
		if !f.has(name) {
			t.Fatalf("lists %s", name)
		}
	}
	if !f.has("state:cues=4") || f.has("rec:spare") {
		t.Fatal("four cues, and no recordings until that tab")
	}
	f.click("ui:tab-recordings")
	if !f.has("rec:spare") || !f.has("rec:door_b") || f.has("cue:sfx/door") {
		t.Fatal("the recordings tab lists every recording, used or not")
	}
}

// TestFiltersAndSearch shows the cues with problems, those not cleared for
// a commercial game, and those a search finds.
func TestFiltersAndSearch(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.click("ui:filter-problems")
	if !f.has("cue:sfx/knock") || f.has("cue:bed/river") || f.has("cue:sfx/door") {
		t.Fatal("problems: an unknown creator, not the uncleared river")
	}
	f.click("ui:filter-uncleared")
	if !f.has("cue:bed/river") || f.has("cue:sfx/knock") {
		t.Fatal("not cleared: the river")
	}
	f.click("ui:filter-all")
	f.typeInto("search", "title")
	if !f.has("cue:theme/title") || f.has("cue:sfx/door") {
		t.Fatal("a search finds a cue by its ID or name")
	}
}

// TestAddSoundAsksWhatItIsAndWhoMadeIt asks for a file, suggests a name and
// an ID from it, asks who made it and whether it may be used commercially,
// and sends one edit that copies it in; the new cue is then selected.
func TestAddSoundAsksWhatItIsAndWhoMadeIt(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.ctx.Files.(*frametest.Files).Chosen = "/sounds/Door Knock.wav"
	f.click("ui:add-sound")
	if !f.has("state:dialog") {
		t.Fatal("choosing a file opens the dialog")
	}
	f.click("ui:dialog-" + protocol.CategoryMusic)
	f.typeInto("creator", "A. Recordist")
	f.typeInto("source-type", "recorded")
	f.typeInto("license", "CC BY 4.0")
	f.click("ui:add-step")
	f.typeInto("step-what-0", "trimmed")
	f.click("ui:add-step")
	if !f.has("ui:step-what-1") {
		t.Fatal("Add step adds another")
	}
	f.click("ui:step-remove-1")
	if f.has("ui:step-what-1") {
		t.Fatal("its × takes it out")
	}
	f.click("ui:commercial")
	f.click("ui:dialog-ok")
	added := f.edit(protocol.AudioAddSound)
	var args protocol.AddSound
	if err := added.Decode(&args); err != nil {
		t.Fatal(err)
	}
	if args.NewCue == nil || args.NewCue.ID != "music/door_knock" || args.NewCue.Name != "Door Knock" ||
		args.NewCue.Category != protocol.CategoryMusic || args.Path != "/sounds/Door Knock.wav" ||
		!reflect.DeepEqual(args.Attribution, protocol.Attribution{
			Creator: "A. Recordist", Source: "", SourceType: "recorded", License: "CC BY 4.0",
			Processing: []protocol.ProcessingStep{{Description: "trimmed", Tool: "", PerformedBy: ""}}, Commercial: true,
		}) {
		t.Fatalf("the sound and its attribution: %+v %+v", args, args.NewCue)
	}
	if !f.has("state:dialog") {
		t.Fatal("the dialog waits for the copy")
	}
	added.Succeed("music/door_knock")
	f.state.Changed(client.Update{AudioChanged: &protocol.AudioChanged{Revision: 1}}, f.ctx)
	library := testLibrary()
	library.Cues = append(library.Cues, cue("music/door_knock", "Door Knock", protocol.CategoryMusic,
		recording("door_knock", true)))
	f.answer(library)
	if f.has("state:dialog") || !f.has("state:selected=cue:music/door_knock") {
		t.Fatal("the new cue is selected once listed")
	}
}

// TestNewCueNeedsAValidID refuses an ID the orchestrator would.
func TestNewCueNeedsAValidID(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.click("ui:new-cue")
	f.typeInto("cue-name", "Bell")
	f.typeInto("cue-id", "Bell Tower")
	f.click("ui:dialog-ok")
	if len(f.requests.AudioEdits) != 0 || !f.has("state:dialog") {
		t.Fatal("an ID with capitals and spaces is refused in the dialog")
	}
	f.typeInto("cue-id", "sfx/bell")
	f.click("ui:dialog-ok")
	var args protocol.NewCue
	if err := f.edit(protocol.AudioCreateCue).Decode(&args); err != nil || args.ID != "sfx/bell" || args.Name != "Bell" {
		t.Fatalf("the cue: %+v %v", args, err)
	}
}

// TestChangesACue renames a cue with Enter, lowers its gain, changes its
// category and its order of recordings, and plays one of them.
func TestChangesACue(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.click("cue:sfx/door")
	f.typeInto("name", "Door creaks")
	f.keys("Enter")
	f.click("ui:quieter")
	f.click("ui:category-" + protocol.CategoryAmbient)
	f.click("plays:door_b")
	f.click("ui:recording-up")
	f.click("ui:play-recording")
	var renamed, quieter protocol.CueChange
	changes := f.requests.AudioOf(protocol.AudioSetCue)
	if len(changes) != 2 || changes[0].Decode(&renamed) != nil || changes[1].Decode(&quieter) != nil {
		t.Fatalf("two cue changes: %d", len(changes))
	}
	if renamed.Name == nil || *renamed.Name != "Door creaks" || quieter.Gain == nil || *quieter.Gain != 0.7 {
		t.Fatalf("renamed, and a step quieter: %+v %+v", renamed, quieter)
	}
	var category protocol.CategoryChange
	if err := f.edit(protocol.AudioSetCategory).Decode(&category); err != nil ||
		category.Category != protocol.CategoryAmbient {
		t.Fatalf("the category: %+v", category)
	}
	var order protocol.CueRecordings
	if err := f.edit(protocol.AudioSetRecordings).Decode(&order); err != nil ||
		strings.Join(order.Recordings, ",") != "door_b,door_a" {
		t.Fatalf("door_b moves up: %+v", order)
	}
	if sounds := f.ctx.Sounds.(*frametest.Sounds); !sounds.PlayingRecording("door_b") {
		t.Fatalf("the chosen recording plays on its own: %v", sounds.Played)
	}
}

// TestDeleteCueSaysWhereItIsUsed asks where a cue is used before deleting it.
func TestDeleteCueSaysWhereItIsUsed(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.click("cue:sfx/door")
	f.requests.Uses = nil
	f.click("ui:delete-cue")
	if len(f.requests.Uses) != 1 {
		t.Fatal("deleting asks where the cue is used")
	}
	f.requests.Uses[0]([]protocol.Use{{Name: "Doorman", Label: "a Play sound node"}}, nil)
	f.click("ui:delete")
	var item protocol.AudioItem
	if err := f.edit(protocol.AudioDeleteCue).Decode(&item); err != nil || item.ID != "sfx/door" {
		t.Fatalf("the cue is deleted: %+v %v", item, err)
	}
}

// TestOpenedForAnItem selects a cue the window was opened for, or starts
// adding a sound.
func TestOpenedForAnItem(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.state.Changed(client.Update{Attention: true, Item: "theme/title"}, f.ctx)
	f.answer(testLibrary())
	if !f.has("state:selected=cue:theme/title") {
		t.Fatal("the cue is selected")
	}
	files := f.ctx.Files.(*frametest.Files)
	f.state.Changed(client.Update{Attention: true, Item: audioview.ItemAddSound}, f.ctx)
	if files.Asked != 1 || f.has("state:dialog") {
		t.Fatal("adding a sound asks for a file; cancelling it opens nothing")
	}
}
