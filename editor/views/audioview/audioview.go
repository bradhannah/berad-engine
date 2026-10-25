// Package audioview is the Audio window: the project's audio library, its
// cues and their recordings, edited as they are listed. Authors add sounds,
// copying their files into the project with their attribution and whether
// they may be used in a commercial game; make, rename and delete cues; give a
// cue its category, gain, description and search terms; and set the
// recordings it plays in turn, listening to each. Every change is saved as it
// is made, and undone with Cmd+Z (D3, D9); typed text waits for Enter or Save.
//
// The window never reads audio files: the orchestrator lists the library,
// sends recordings to play and copies chosen files in (D1, D8, D11).
package audioview

import (
	"encoding/json"
	"image"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/model"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/ui/textedit"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// ViewName is the Audio window's view name; its target is the project index,
// and its undo group is UndoGroup.
const (
	ViewName  = "audio"
	UndoGroup = "audio"
)

// ItemAddSound is the item that starts adding a sound when the window is
// opened for it: the Open dialog first. Its leading slash is never in a cue
// ID.
const ItemAddSound = "/add-sound"

// Modes of the list: cues, or recordings.
const (
	modeCues       = "cues"
	modeRecordings = "recordings"
)

// Filters of the list.
const (
	filterAll       = "all"
	filterProblems  = "problems"
	filterUncleared = "uncleared"
)

// Row ID prefixes: a cue, a recording, a section.
const (
	cuePrefix       = "cue:"
	recordingPrefix = "rec:"
	sectionPrefix   = "section:"
)

// Field sizes, in bytes.
const (
	nameBytes  = 120
	textBytes  = 400
	termsBytes = 200
)

// State is the Audio window.
type State struct {
	window ids.ULID
	theme  *ui.Theme
	size   image.Point
	subbed bool
	// library is the audio library as last listed; asking is set while a
	// question is out, and again when another is wanted after it.
	library *protocol.AudioLibrary
	asking  bool
	again   bool
	// selectNext is the item to select once the library lists it, such as a
	// cue just made; playedNext is the same for the cue's recordings.
	selectNext, playedNext string

	mode    string
	filter  string
	tabs    []*ui.Button
	filters []*ui.Button
	search  *ui.TextField
	list    *ui.List
	closed  map[string]bool
	played  *ui.List

	addSound, newCue, checkFiles, stopAll *ui.Button
	panel                                 *panel
	inspector                             *inspector
	trim                                  *trimmer
	soundscape                            *soundscape
	// broken holds the version of each recording's file the last check
	// could not decode.
	broken map[string]string

	focus   ui.Focus
	guard   frame.Guard
	dialog  *soundDialog
	confirm *ui.Dialog
	// prompt asks for one value, such as a level or a family's name.
	prompt *fieldDialog
	// deleting is the cue or recording the confirm dialog would delete.
	deleting string
	sounds   frame.Sounds
	mixer    frame.Soundscape
}

// New returns the Audio window.
func New(window ids.ULID, theme *ui.Theme) *State {
	button := ui.NewButton
	view := &State{
		window: window, theme: theme, size: image.Pt(0, 0), subbed: false,
		library: nil, asking: false, again: false, selectNext: "", playedNext: "",
		mode: modeCues, filter: filterAll,
		tabs: []*ui.Button{button("tab-"+modeCues, "Cues"), button("tab-"+modeRecordings, "Recordings")},
		filters: []*ui.Button{
			button("filter-"+filterAll, "All"), button("filter-"+filterProblems, "Problems"),
			button("filter-"+filterUncleared, "Not cleared"),
		},
		search:   ui.NewTextField("search", textedit.NewField("", false, termsBytes), false, theme),
		list:     &ui.List{ID: "sounds", Rect: ui.Zero(), Rows: nil, Selected: "", CanDrop: nil},
		closed:   map[string]bool{},
		played:   &ui.List{ID: "played", Rect: ui.Zero(), Rows: nil, Selected: "", CanDrop: nil},
		addSound: button("add-sound", "Add sound…"), newCue: button("new-cue", "New cue…"),
		checkFiles: button("check-files", "Check files"), stopAll: button("stop-all", "Stop all"),
		panel: newPanel(), inspector: newInspector(theme), trim: newTrimmer(theme),
		soundscape: newSoundscape(), broken: map[string]string{},
		focus: ui.Focus{}, guard: frame.Guard{}, dialog: nil, confirm: nil, prompt: nil, deleting: "", sounds: nil,
		mixer: nil,
	}
	view.addSound.Primary = true
	view.search.Placeholder, view.search.Unmarked = "Search names, IDs, files and licences", true
	view.focus.Register(view.search, view.trim.start, view.trim.end)
	view.focus.Register(view.inspector.fields()...)

	return view
}

// Title is the window title.
func (s *State) Title(frame.Docs) string { return "Audio" }

// Focused returns the focused text field, in a dialog or the window.
func (s *State) Focused() *ui.TextField {
	if s.dialog != nil {
		return s.dialog.focus.Current()
	}

	return s.focus.Current()
}

// Start subscribes to the project index, as the target every window needs,
// and asks for the library.
func (s *State) Start(ctx *frame.Context) {
	if !s.subbed {
		s.subbed = true
		ctx.Requests.Subscribe(project.IndexID(), nil)
	}
	s.ask(ctx)
}

// Changed lists the library again when it changes, and shows an item the
// window was opened for: a cue, or adding a sound.
func (s *State) Changed(update client.Update, ctx *frame.Context) {
	if update.AudioChanged != nil {
		s.ask(ctx)
	}
	if update.Attention && update.Item != "" {
		s.show(ctx, update.Item)
	}
}

// show selects a cue, or starts adding a sound.
func (s *State) show(ctx *frame.Context, item string) {
	if item == ItemAddSound {
		s.chooseSound(ctx, soundTarget{cue: "", recording: ""})

		return
	}
	s.mode = modeCues
	s.selectNext = cuePrefix + item
	s.ask(ctx)
}

// ask asks for the library, or again once the question out now is answered.
func (s *State) ask(ctx *frame.Context) {
	if s.asking {
		s.again = true

		return
	}
	s.asking = true
	ctx.Requests.AudioLibrary(func(library protocol.AudioLibrary, err error) {
		s.asking = false
		if err != nil {
			ctx.Frame.Status("The audio library could not be listed: " + err.Error())
		} else {
			s.library = &library
			s.inspector.listed(s)
		}
		if s.again {
			s.again = false
			s.ask(ctx)
		}
	})
}

// edit sends an audio edit; done runs with the result once it succeeds, and
// a refusal is reported in the status bar.
func (s *State) edit(ctx *frame.Context, name string, args any, done func(protocol.Result)) {
	s.editIn(ctx, name, args, done, nil)
}

// editIn sends an audio edit; failed runs when it is refused.
func (s *State) editIn(ctx *frame.Context, name string, args any, done func(protocol.Result), failed func()) {
	data, err := json.Marshal(args)
	if err != nil {
		ctx.Frame.Status(err.Error())

		return
	}
	ctx.Requests.AudioEdit(protocol.AudioEdit{Edit: name, Gesture: "", Args: data},
		func(result protocol.Result, err error) {
			if err != nil {
				ctx.Frame.Report("")(result, err)
				if failed != nil {
					failed()
				}

				return
			}
			if done != nil {
				done(result)
			}
		})
}

// cue returns a listed cue.
func (s *State) cue(cueID string) (protocol.AudioCue, bool) {
	if s.library != nil {
		for _, cue := range s.library.Cues {
			if cue.ID == cueID {
				return cue, true
			}
		}
	}
	var none protocol.AudioCue

	return none, false
}

// recording returns a listed recording, from a cue or the unused ones.
func (s *State) recording(recordingID string) (protocol.AudioRecording, bool) {
	var none protocol.AudioRecording
	if s.library == nil {
		return none, false
	}
	for _, cue := range s.library.Cues {
		for _, recording := range cue.Recordings {
			if recording.ID == recordingID {
				return recording, true
			}
		}
	}
	for _, recording := range s.library.Unused {
		if recording.ID == recordingID {
			return recording, true
		}
	}

	return none, false
}

// usedBy lists the cues that play a recording.
func (s *State) usedBy(recording string) []protocol.AudioCue {
	var cues []protocol.AudioCue
	if s.library != nil {
		for _, cue := range s.library.Cues {
			for _, played := range cue.Recordings {
				if played.ID == recording {
					cues = append(cues, cue)

					break
				}
			}
		}
	}

	return cues
}

// selectedCue returns the cue selected in the list.
func (s *State) selectedCue() (protocol.AudioCue, bool) {
	cueID, ok := cutPrefix(s.list.Selected, cuePrefix)
	if !ok {
		var none protocol.AudioCue

		return none, false
	}

	return s.cue(cueID)
}

// selectedRecording returns the recording selected in the list.
func (s *State) selectedRecording() (protocol.AudioRecording, bool) {
	recordingID, ok := cutPrefix(s.list.Selected, recordingPrefix)
	if !ok {
		var none protocol.AudioRecording

		return none, false
	}

	return s.recording(recordingID)
}

func cutPrefix(value, prefix string) (string, bool) {
	if len(value) <= len(prefix) || value[:len(prefix)] != prefix {
		return "", false
	}

	return value[len(prefix):], true
}

// whereUsed asks which workflows play a cue.
func (s *State) whereUsed(ctx *frame.Context, cue string, done func([]protocol.Use)) {
	ctx.Requests.WhereUsed(model.AudioLibraryID(), cue, func(uses []protocol.Use, err error) {
		if err != nil {
			ctx.Frame.Status("Where it is used could not be found: " + err.Error())
			uses = nil
		}
		done(uses)
	})
}

// Unsaved counts typed changes not saved yet, and a drafted region.
func (s *State) Unsaved(frame.Docs) int {
	count := frame.UnsavedFields(s.inspector.fields()...)
	if s.trim.dirty() {
		count++
	}

	return count
}

// UnsavedName names what the typed changes belong to.
func (s *State) UnsavedName(frame.Docs) string {
	if s.inspector.subject == "" {
		return s.trim.recording
	}

	return s.inspector.subject
}

// Save saves every typed change and a drafted region.
func (s *State) Save(ctx *frame.Context) bool {
	if s.trim.dirty() {
		s.applyRegion(ctx)
	}

	return s.inspector.saveAll(s, ctx)
}

// Leave asks about typed changes before the window closes.
func (s *State) Leave(ctx *frame.Context, then func()) {
	s.guard.Ask(s.Unsaved(ctx.Docs), s.UnsavedName(ctx.Docs), then)
}

// discard drops typed changes and a drafted region.
func (s *State) discard() {
	for _, field := range s.inspector.fields() {
		field.Field.Revert()
	}
	s.trim.draft = copyWindow(s.trim.saved)
}

// check decodes every recording, and marks those that cannot play.
func (s *State) check(ctx *frame.Context) {
	ctx.Frame.Status("Checking every file…")
	ctx.Requests.AudioCheck(func(result protocol.AudioCheck, err error) {
		if err != nil {
			ctx.Frame.Status("The files could not be checked: " + err.Error())

			return
		}
		s.broken = map[string]string{}
		for _, problem := range result.Broken {
			if recording, found := s.recording(problem.Recording); found {
				s.broken[problem.Recording] = recording.Version
			}
		}
		if len(result.Broken) == 0 {
			ctx.Frame.Status("Every file plays.")

			return
		}
		ctx.Frame.Status(itoa(len(result.Broken)) + " files cannot be decoded; the Problems filter shows them.")
	})
}
