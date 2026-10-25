package orchestrator_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bradhannah/berad-sdk/modules/berad-audio/authoring"
	"github.com/bradhannah/berad-sdk/modules/berad-audio/backend/ebitengine"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/model"
	"github.com/bradhannah/berad-engine/editor/orchestrator"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/plugins/workflow"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/engine/ids"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// toneWAV is a short mono 16-bit tone, so berad-audio can decode what is
// copied in.
func toneWAV(samples int) []byte {
	const rate = 22050
	var pcm bytes.Buffer
	for index := range samples {
		_ = binary.Write(&pcm, binary.LittleEndian, int16((index%50)*400-10000))
	}
	var wav bytes.Buffer
	wav.WriteString("RIFF")
	_ = binary.Write(&wav, binary.LittleEndian, uint32(36+pcm.Len()))
	wav.WriteString("WAVEfmt ")
	for _, value := range []any{uint32(16), uint16(1), uint16(1), uint32(rate), uint32(rate * 2), uint16(2), uint16(16)} {
		_ = binary.Write(&wav, binary.LittleEndian, value)
	}
	wav.WriteString("data")
	_ = binary.Write(&wav, binary.LittleEndian, uint32(pcm.Len()))
	wav.Write(pcm.Bytes())

	return wav.Bytes()
}

// soundFile writes a tone outside the project, where an author's sound
// would come from.
func soundFile(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, toneWAV(4000), 0o600); err != nil {
		t.Fatal(err)
	}

	return path
}

func audioEdit(t *testing.T, c *client.Client, edit string, args any) (protocol.Result, error) {
	t.Helper()
	data, err := json.Marshal(args)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return c.AudioEdit(ctx, protocol.AudioEdit{Edit: edit, Gesture: "", Args: data})
}

func mustAudioEdit(t *testing.T, c *client.Client, edit string, args any) protocol.Result {
	t.Helper()
	result, err := audioEdit(t, c, edit, args)
	if err != nil {
		t.Fatalf("%s: %v", edit, err)
	}

	return result
}

func cueOf(t *testing.T, c *client.Client, id string) (protocol.AudioCue, bool) {
	t.Helper()
	for _, cue := range askAudio(t, c).Cues {
		if cue.ID == id {
			return cue, true
		}
	}

	return protocol.AudioCue{}, false
}

// TestAudioEditsStartTheLibraryAndUndo starts a project's library with a new
// cue, copies a sound into it with its attribution, tells every window, and
// undoes both steps.
func TestAudioEditsStartTheLibraryAndUndo(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	createProject(t, dir)
	h := start(t, dir, nil)
	audio := h.dialView(t, "audio", project.IndexID(), "audio", project.IndexID())
	other := h.dial(t, "map")
	created := mustAudioEdit(t, audio, protocol.AudioCreateCue, protocol.NewCue{
		ID: "sfx/door-open", Name: "Door opens", Category: protocol.CategorySFX, HowToHear: "Open a door.",
	})
	if created.Item != "sfx/door-open" {
		t.Fatalf("the result names the cue: %+v", created)
	}
	nextUpdate(t, other, "the change", func(update client.Update) bool { return update.AudioChanged != nil })
	library := askAudio(t, audio)
	if library.Problem != "" || library.Schema != authoring.LibrarySchemaFour || len(library.Cues) != 1 {
		t.Fatalf("a schema-4 library with the cue: %+v", library)
	}
	cue := library.Cues[0]
	if !cue.CategorySet || cue.Category != protocol.CategorySFX || cue.Recordings[0].ID != "door_open" ||
		cue.Recordings[0].Status != protocol.RecordingNoFile {
		t.Fatalf("a cue with an empty slot, named in the descriptor: %+v", cue)
	}

	source := soundFile(t, "Door Open.wav")
	mustAudioEdit(t, audio, protocol.AudioAddSound, protocol.AddSound{
		Cue: "sfx/door-open", NewCue: nil, Path: source,
		Attribution: protocol.Attribution{Creator: " A. Recordist ", Source: "", License: "CC0-1.0", Commercial: true},
	})
	filled, _ := cueOf(t, audio, "sfx/door-open")
	recording := filled.Recordings[0]
	if len(filled.Recordings) != 1 || recording.Status != protocol.RecordingReady ||
		!strings.HasPrefix(recording.File, "imported/door_open_") || recording.OriginalFile != "Door Open.wav" {
		t.Fatalf("the sound fills the empty slot, copied in: %+v", filled)
	}
	if recording.Attribution != (protocol.Attribution{
		Creator: "A. Recordist", Source: protocol.Unknown, License: "CC0-1.0", Commercial: true,
	}) {
		t.Fatalf("its attribution: %+v", recording.Attribution)
	}
	copied := filepath.Join(dir, "audio", filepath.FromSlash(recording.File))
	if _, err := os.Stat(copied); err != nil {
		t.Fatal("the file was copied into the project:", err)
	}

	if _, err := undo(t, audio); err != nil {
		t.Fatal(err)
	}
	if emptied, _ := cueOf(t, audio, "sfx/door-open"); emptied.Recordings[0].Status != protocol.RecordingNoFile {
		t.Fatalf("undo empties the slot again: %+v", emptied)
	}
	if _, err := os.Stat(copied); err != nil {
		t.Fatal("undo keeps the copied file:", err)
	}
	if _, err := undo(t, audio); err != nil {
		t.Fatal(err)
	}
	if _, found := cueOf(t, audio, "sfx/door-open"); found {
		t.Fatal("undo takes the new cue away")
	}
	if _, err := redo(t, audio); err != nil {
		t.Fatal(err)
	}
	if again, found := cueOf(t, audio, "sfx/door-open"); !found || again.Category != protocol.CategorySFX {
		t.Fatalf("redo brings it back with its category: %+v", again)
	}
}

// TestAudioEditsChangeCuesAndRecordings changes a cue and its recordings, a
// recording's note and attribution, and deletes them.
func TestAudioEditsChangeCuesAndRecordings(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	createProject(t, dir)
	h := start(t, dir, nil)
	audio := h.dialView(t, "audio", project.IndexID(), "audio", project.IndexID())
	newCue := &protocol.NewCue{ID: "ui/chime", Name: "Chime", Category: "", HowToHear: ""}
	attribution := protocol.Attribution{Creator: "", Source: "", License: "", Commercial: false}
	mustAudioEdit(t, audio, protocol.AudioAddSound, protocol.AddSound{
		Cue: "", NewCue: newCue, Path: soundFile(t, "a.wav"), Attribution: attribution,
	})
	mustAudioEdit(t, audio, protocol.AudioAddSound, protocol.AddSound{
		Cue: "ui/chime", NewCue: nil, Path: soundFile(t, "b.wav"), Attribution: attribution,
	})
	chime, _ := cueOf(t, audio, "ui/chime")
	if len(chime.Recordings) != 2 || chime.Recordings[1].ID != "chime_2" || chime.HowToHear == "" ||
		chime.Recordings[0].Attribution.Commercial {
		t.Fatalf("a second sound is another recording, not cleared: %+v", chime)
	}
	name, gain := "Bright chime", 0.25
	mustAudioEdit(t, audio, protocol.AudioSetCue, protocol.CueChange{
		Cue: "ui/chime", Name: &name, Gain: &gain, HowToHear: nil, SearchTerms: &[]string{" bell ", ""},
	})
	mustAudioEdit(t, audio, protocol.AudioSetCategory, protocol.CategoryChange{Cue: "ui/chime", Category: "music"})
	mustAudioEdit(t, audio, protocol.AudioSetRecordings, protocol.CueRecordings{
		Cue: "ui/chime", Recordings: []string{"chime_2"},
	})
	note := "the brighter take"
	mustAudioEdit(t, audio, protocol.AudioSetRecording, protocol.RecordingChange{
		Recording: "chime_2", Note: &note, SearchTerms: nil,
	})
	mustAudioEdit(t, audio, protocol.AudioSetAttribution, protocol.AttributionChange{
		Recording: "chime_2", Attribution: protocol.Attribution{Creator: "c", Source: "s", License: "l", Commercial: true},
	})
	library := askAudio(t, audio)
	changed := library.Cues[0]
	if changed.Name != name || changed.Gain != gain || changed.SearchTerms[0] != "bell" ||
		changed.Category != protocol.CategoryMusic || len(changed.Recordings) != 1 ||
		changed.Recordings[0].Note != note || !changed.Recordings[0].Attribution.Commercial {
		t.Fatalf("the changes: %+v", changed)
	}
	if len(library.Unused) != 1 || library.Unused[0].ID != "chime" {
		t.Fatalf("the recording taken out is unused: %+v", library.Unused)
	}
	mustAudioEdit(t, audio, protocol.AudioDeleteRecording, protocol.AudioItem{ID: "chime"})
	if _, err := audioEdit(t, audio, protocol.AudioDeleteRecording, protocol.AudioItem{ID: "chime_2"}); err == nil {
		t.Fatal("a recording a cue plays is not deleted")
	}
	mustAudioEdit(t, audio, protocol.AudioDeleteCue, protocol.AudioItem{ID: "ui/chime"})
	if library := askAudio(t, audio); len(library.Cues) != 0 || len(library.Unused) != 1 {
		t.Fatalf("the cue goes and its recording stays: %+v", library)
	}
	descriptor, err := authoring.ReadProjectDescriptor(filepath.Join(dir, "audio", "project.json"))
	if err != nil || len(descriptor.Targets.CueCategories) != 0 {
		t.Fatalf("the deleted cue has no category left: %+v %v", descriptor.Targets, err)
	}
}

// TestAudioEditsRefuseWhatCannotBe refuses bad input and leaves the library
// as it was.
func TestAudioEditsRefuseWhatCannotBe(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	createProject(t, dir)
	h := start(t, dir, nil)
	audio := h.dialView(t, "audio", project.IndexID(), "audio", project.IndexID())
	if _, err := audioEdit(t, audio, protocol.AudioSetCue, protocol.CueChange{Cue: "x"}); err == nil {
		t.Fatal("no library, nothing to change")
	}
	if _, err := os.Stat(filepath.Join(dir, "audio", "library.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("a refused edit starts no library")
	}
	for _, id := range []string{"Door", "door open", "/door", "door//open", ""} {
		_, err := audioEdit(t, audio, protocol.AudioCreateCue, protocol.NewCue{ID: id, Name: "n", Category: "", HowToHear: ""})
		if err == nil {
			t.Fatalf("%q is not a cue ID", id)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "audio", "library.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("a refused new cue leaves no library behind")
	}
	mustAudioEdit(t, audio, protocol.AudioCreateCue, protocol.NewCue{ID: "a/b", Name: "B", Category: "", HowToHear: ""})
	gain := 2.0
	notAudio := filepath.Join(t.TempDir(), "notes.wav")
	if err := os.WriteFile(notAudio, []byte("not a sound"), 0o600); err != nil {
		t.Fatal(err)
	}
	for edit, args := range map[string]any{
		protocol.AudioCreateCue:   protocol.NewCue{ID: "a/b", Name: "Again", Category: "", HowToHear: ""},
		protocol.AudioSetCue:      protocol.CueChange{Cue: "a/b", Gain: &gain},
		protocol.AudioSetCategory: protocol.CategoryChange{Cue: "a/b", Category: "loud"},
		protocol.AudioAddSound: protocol.AddSound{Cue: "a/b", NewCue: nil, Path: notAudio,
			Attribution: protocol.Attribution{Creator: "", Source: "", License: "", Commercial: false}},
		"audio.unknown": protocol.AudioItem{ID: "a/b"},
	} {
		if _, err := audioEdit(t, audio, edit, args); err == nil {
			t.Fatalf("%s %+v is refused", edit, args)
		}
	}
	if cue, _ := cueOf(t, audio, "a/b"); cue.Name != "B" || cue.Gain != 1 || cue.Recordings[0].Status != protocol.RecordingNoFile {
		t.Fatalf("the cue is as it was: %+v", cue)
	}
}

// TestAudioUndoIsRefusedAfterAnotherChange refuses to undo a sound change
// another window changed afterwards, or another program changed outside the
// editor.
func TestAudioUndoIsRefusedAfterAnotherChange(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	createProject(t, dir)
	h := start(t, dir, nil)
	audio := h.dialView(t, "audio", project.IndexID(), "audio", project.IndexID())
	browser := h.dialView(t, "browser", project.IndexID(), "", project.IndexID())
	mustAudioEdit(t, audio, protocol.AudioCreateCue, protocol.NewCue{ID: "a/b", Name: "B", Category: "", HowToHear: ""})
	name := "Renamed"
	mustAudioEdit(t, audio, protocol.AudioSetCue, protocol.CueChange{Cue: "a/b", Name: &name})
	mustAudioEdit(t, browser, protocol.AudioDeleteCue, protocol.AudioItem{ID: "a/b"})
	if _, err := undo(t, audio); err == nil {
		t.Fatal("the browser deleted the cue since, so the rename is not undone")
	}
	if _, err := undo(t, browser); err != nil {
		t.Fatal("the browser undoes its delete:", err)
	}

	mustAudioEdit(t, audio, protocol.AudioCreateCue, protocol.NewCue{ID: "c/d", Name: "D", Category: "", HowToHear: ""})
	mustAudioEdit(t, audio, protocol.AudioSetCue, protocol.CueChange{Cue: "c/d", Name: &name})
	path := filepath.Join(dir, "audio", "library.json")
	outside, err := authoring.OpenLibraryProject(path, authoring.DefaultEditorOptions(ebitengine.Decode))
	if err != nil {
		t.Fatal(err)
	}
	cue := outside.Cue("c/d")
	cue.Name = "Changed outside"
	if err := errors.Join(outside.SetCue("c/d", cue), outside.SaveChanged()); err != nil {
		t.Fatal(err)
	}
	if _, err := undo(t, audio); !errors.Is(err, client.ErrRejected) || !strings.Contains(err.Error(), "outside") {
		t.Fatalf("a change outside the editor stops the undo: %v", err)
	}
	if cue, _ := cueOf(t, audio, "c/d"); cue.Name != "Changed outside" {
		t.Fatalf("the outside change stays: %+v", cue)
	}
}

// TestAudioChangesOutsideReachWindows tells windows when another program
// changes the audio files.
func TestAudioChangesOutsideReachWindows(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	createProject(t, dir)
	h := start(t, dir, func(cfg *orchestrator.Config) { cfg.FlushInterval = 20 * time.Millisecond })
	c := h.dial(t, "map")
	writeAudio(t, dir, "library.json", testLibrary)
	nextUpdate(t, c, "the outside change", func(update client.Update) bool { return update.AudioChanged != nil })
}

// TestCuesInWorkflowsAreWhereUsed lists the workflows whose Play sound nodes
// play a cue.
func TestCuesInWorkflowsAreWhereUsed(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	createProject(t, dir)
	h := start(t, dir, nil)
	browser := h.dialView(t, "browser", project.IndexID(), "", project.IndexID())
	routine := createDocument(t, browser, flow.FormatName, "Doorman", ids.ULID{}, workflow.NewData(), "")
	editor := h.dialView(t, "workflow", routine, "workflow:"+routine.String(), routine)
	sound, err := flow.NewNode(flow.KindPlaySound)
	if err == nil {
		sound, err = sound.WithParam("cue", "sfx/door-open")
	}
	if err != nil {
		t.Fatal(err)
	}
	mustCommand(t, editor, routine, workflow.AddEdit, "", workflow.AddArgs{
		Nodes: []flow.Node{sound}, Boxes: map[ids.ULID]flow.Box{{}: {X: 160, Y: 40, W: 0, H: 0}}, Links: nil,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	uses, err := browser.WhereUsed(ctx, model.AudioLibraryID(), "sfx/door-open")
	if err != nil || len(uses) != 1 || uses[0].Name != "Doorman" || uses[0].Label != "a Play sound node" {
		t.Fatalf("the workflow plays the cue: %+v %v", uses, err)
	}
	if uses, _ := browser.WhereUsed(ctx, model.AudioLibraryID(), "sfx/other"); len(uses) != 0 {
		t.Fatalf("another cue is unused: %+v", uses)
	}
}

// TestOpenItemShowsItInTheOpenWindow names the item to show when a window
// is opened again.
func TestOpenItemShowsItInTheOpenWindow(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	createProject(t, dir)
	h := start(t, dir, nil)
	audio := h.dialView(t, "audio", project.IndexID(), "audio", project.IndexID())
	browser := h.dialView(t, "browser", project.IndexID(), "", project.IndexID())
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := browser.OpenItem(ctx, "audio", project.IndexID(), "sfx/door-open"); err != nil {
		t.Fatal(err)
	}
	nextUpdate(t, audio, "the item", func(update client.Update) bool {
		return update.Attention && update.Item == "sfx/door-open"
	})
}

// TestFirstEditUpgradesTheLibrary writes a schema-3 library as schema 4 when
// the editor first changes it, with no recording cleared for commercial use,
// and leaves it alone while it only reads it.
func TestFirstEditUpgradesTheLibrary(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	createProject(t, dir)
	writeAudio(t, dir, "library.json", testLibrary)
	h := start(t, dir, nil)
	audio := h.dialView(t, "audio", project.IndexID(), "audio", project.IndexID())
	if library := askAudio(t, audio); library.Schema != authoring.LibrarySchemaThree {
		t.Fatalf("reading keeps schema 3: %d", library.Schema)
	}
	data, err := os.ReadFile(filepath.Join(dir, "audio", "library.json"))
	if err != nil || string(data) != testLibrary {
		t.Fatalf("reading changes nothing: %v", err)
	}
	name := "Hum, low"
	mustAudioEdit(t, audio, protocol.AudioSetCue, protocol.CueChange{Cue: "bed/hum", Name: &name})
	data, err = os.ReadFile(filepath.Join(dir, "audio", "library.json"))
	if err != nil {
		t.Fatal(err)
	}
	document, err := authoring.ParseDocument(data, nil)
	if err != nil || document.SchemaVersion != authoring.LibrarySchemaFour {
		t.Fatalf("the first edit writes schema 4: %v", err)
	}
	for id, recording := range document.Recordings {
		if recording.Provenance.CommercialUseAllowed() {
			t.Fatalf("%s is not cleared until its author says so", id)
		}
	}
	if _, err := undo(t, audio); err != nil {
		t.Fatal(err)
	}
	if cue, _ := cueOf(t, audio, "bed/hum"); cue.Name != "Hum" || askAudio(t, audio).Schema != authoring.LibrarySchemaFour {
		t.Fatalf("undo takes back the edit, not the upgrade: %+v", cue)
	}
}
