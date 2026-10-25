package browser

import (
	"encoding/json"
	"image"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/model"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/audioview"
	"github.com/bradhannah/berad-engine/editor/views/frame"
)

// The Audio section lists the audio library's cues under their categories.
// Its rows are not documents: their IDs carry these prefixes.
const (
	sectionAudio        = sectionPrefix + "audio"
	audioCategoryPrefix = "audio-category:"
	audioCuePrefix      = "audio-cue:"
	// newSound, from New ▾ or the section's [+], adds a sound in the Audio
	// window, starting with the Open dialog.
	newSound = "new-sound"
	// contextOpenAudio opens a cue in the Audio window.
	contextOpenAudio = "context-open-audio"
)

// audioCategories are the cue categories in the order the browser lists
// them, with their names.
func audioCategories() [][2]string {
	return [][2]string{
		{protocol.CategoryMusic, "Music"}, {protocol.CategorySFX, "SFX"}, {protocol.CategoryAmbient, "Ambient"},
	}
}

// audioState is what the browser knows of the audio library: its cues as
// last listed, and where the selected cue is used.
type audioState struct {
	library *protocol.AudioLibrary
	asking  bool
	again   bool
	uses    []protocol.Use
	usesFor string
	open    *ui.Button
	// deleting is the cue the delete dialog would delete.
	deleting string
}

func newAudioState() *audioState {
	return &audioState{
		library: nil, asking: false, again: false, uses: nil, usesFor: "",
		open: ui.NewButton("open-audio", "Open in Audio window"), deleting: "",
	}
}

// askAudio lists the audio library, or again once the question out now is
// answered.
func (s *State) askAudio(ctx *frame.Context) {
	audio := s.audio
	if audio.asking {
		audio.again = true

		return
	}
	audio.asking = true
	ctx.Requests.AudioLibrary(func(library protocol.AudioLibrary, err error) {
		audio.asking = false
		if err == nil {
			audio.library = &library
		}
		if audio.again {
			audio.again = false
			s.askAudio(ctx)
		}
	})
}

// cue reports whether a cue matches the search: by its name, ID or search
// terms. Cues have no tags, so a search for tags finds none.
func (m matcher) cue(cue protocol.AudioCue) bool {
	if !m.active() {
		return true
	}
	if m.tags {
		return false
	}
	for _, text := range append([]string{cue.Name, cue.ID}, cue.SearchTerms...) {
		if strings.Contains(strings.ToLower(text), m.text) {
			return true
		}
	}

	return false
}

// audioRows lists the Audio section: its categories, each with its cues, a
// cue showing its ID beside its name. The section's [+] adds a sound.
func (s *State) audioRows(match matcher) []ui.Row {
	library := s.audio.library
	if library == nil {
		return nil
	}
	var shown []protocol.AudioCue
	for _, cue := range library.Cues {
		if match.cue(cue) {
			shown = append(shown, cue)
		}
	}
	rows := []ui.Row{s.section(sectionAudio, "Audio", len(shown), true, match.active())}
	if s.closed[sectionAudio] && !match.active() {
		return rows
	}
	for _, category := range audioCategories() {
		var held []protocol.AudioCue
		for _, cue := range shown {
			if cue.Category == category[0] {
				held = append(held, cue)
			}
		}
		if len(held) > 0 || !match.active() {
			rows = append(rows, s.categoryRows(category, held, match.active())...)
		}
	}

	return rows
}

// categoryRows lists a category, like a folder with its count, and, when it
// is open, its cues, each with its ID beside its name.
func (s *State) categoryRows(category [2]string, held []protocol.AudioCue, searching bool) []ui.Row {
	rowID := audioCategoryPrefix + category[0]
	row := ui.NewRow(rowID, category[1])
	row.Icon, row.Detail = ui.IconFolder, strconv.Itoa(len(held))
	if len(held) > 0 {
		row.Toggle = ui.DisclosureOpen
		if s.closed[rowID] && !searching {
			row.Toggle = ui.DisclosureClosed
		}
	}
	rows := []ui.Row{row}
	if row.Toggle != ui.DisclosureOpen {
		return rows
	}
	for _, cue := range held {
		cueRow := ui.NewRow(audioCuePrefix+cue.ID, cue.Name)
		cueRow.Icon, cueRow.Indent, cueRow.Aside, cueRow.Detail = ui.IconSound, 1, cue.ID, audioDetail(cue)
		rows = append(rows, cueRow)
	}

	return rows
}

// audioDetail says, for a cue's row, that it has no sound yet, that it is
// not cleared for a commercial game, or how many recordings it plays in turn.
func audioDetail(cue protocol.AudioCue) string {
	cleared, empty := true, true
	for _, recording := range cue.Recordings {
		if recording.Status != protocol.RecordingNoFile {
			empty = false
			cleared = cleared && recording.Attribution.Commercial
		}
	}
	switch {
	case empty:
		return "no sound yet"
	case !cleared:
		return "not cleared"
	case len(cue.Recordings) > 1:
		return strconv.Itoa(len(cue.Recordings)) + " in turn"
	}

	return ""
}

// selectedCue returns the cue selected in the list.
func (s *State) selectedCue() (protocol.AudioCue, bool) {
	var none protocol.AudioCue
	cueID, ok := strings.CutPrefix(s.list.Selected, audioCuePrefix)
	if !ok || s.audio.library == nil {
		return none, false
	}
	for _, cue := range s.audio.library.Cues {
		if cue.ID == cueID {
			return cue, true
		}
	}

	return none, false
}

// audioItem reports whether a row is in the Audio section.
func audioItem(row string) bool {
	return row == sectionAudio || strings.HasPrefix(row, audioCategoryPrefix) || strings.HasPrefix(row, audioCuePrefix)
}

// openCue opens the selected cue in the Audio window.
func (s *State) openCue(ctx *frame.Context) bool {
	cue, ok := s.selectedCue()
	if ok {
		ctx.Requests.OpenItem(audioview.ViewName, project.IndexID(), cue.ID)
	}

	return ok
}

// addSound opens the Audio window to add a sound, with the Open dialog.
func (s *State) addSound(ctx *frame.Context) {
	ctx.Requests.OpenItem(audioview.ViewName, project.IndexID(), audioview.ItemAddSound)
}

// audioContextItems lists the Audio section's right-click menu: adding a
// sound anywhere in it, and opening or deleting a cue.
func (s *State) audioContextItems() []ui.MenuItem {
	add := contextItem(newSound, "Add sound…", ui.IconSound)
	if _, ok := s.selectedCue(); !ok {
		return []ui.MenuItem{add}
	}

	return []ui.MenuItem{
		contextItem(contextOpenAudio, "Open in Audio window", ui.IconNone), ruled(add),
		ruled(dangerous(contextItem(contextDelete, "Delete", ui.IconNone))),
	}
}

// syncCueUses asks where a newly selected cue is used, for the inspector.
func (s *State) syncCueUses(ctx *frame.Context) {
	cue, ok := s.selectedCue()
	if !ok || s.audio.usesFor == cue.ID {
		return
	}
	s.audio.uses, s.audio.usesFor = nil, cue.ID
	ctx.Requests.WhereUsed(model.AudioLibraryID(), cue.ID, func(uses []protocol.Use, err error) {
		if err == nil && s.audio.usesFor == cue.ID {
			s.audio.uses = uses
		}
	})
}

// deleteCue asks where the selected cue is used, then whether to delete it.
func (s *State) deleteCue(ctx *frame.Context) {
	cue, ok := s.selectedCue()
	if !ok {
		return
	}
	ctx.Requests.WhereUsed(model.AudioLibraryID(), cue.ID, func(uses []protocol.Use, _ error) {
		lines := []string{"Its recordings stay in the audio library, unused."}
		if len(uses) > 0 {
			lines = append(lines, "These play it, and will play nothing:")
			for _, use := range uses {
				lines = append(lines, "  "+use.Name+" ("+use.Label+")")
			}
		}
		lines = append(lines, "You can undo this.")
		s.dialog = ui.NewDialog("Delete “"+cue.Name+"”?", lines, ui.DialogButton("delete", "Delete"),
			ui.DialogButton(ui.DialogCancel, "Cancel"))
		s.dialogKind, s.audio.deleting = dialogDeleteCue, cue.ID
	})
}

// removeCue deletes a cue from the audio library.
func (s *State) removeCue(ctx *frame.Context, cue string) {
	args, err := json.Marshal(protocol.AudioItem{ID: cue})
	if err != nil {
		ctx.Frame.Status(err.Error())

		return
	}
	ctx.Requests.AudioEdit(protocol.AudioEdit{Edit: protocol.AudioDeleteCue, Gesture: "", Args: args},
		func(result protocol.Result, err error) {
			if err != nil {
				ctx.Frame.Report("")(result, err)

				return
			}
			s.list.Selected = ""
			ctx.Frame.Status("Deleted the cue. Undo brings it back.")
		})
}

// layoutCue places the selected cue's buttons in the inspector.
func (s *State) layoutCue(left, width int) {
	panel := frame.Inspector(s.size)
	top := headerRule + ui.Inset + ui.LineHeight*cueFacts
	s.audio.open.Rect = image.Rect(left, top, left+width, top+ui.ButtonH)
	bottom := panel.Max.Y - ui.Inset
	s.remove.Rect = image.Rect(left, bottom-ui.ButtonH, left+width, bottom)
	s.remove.Label = "Delete cue…"
}

// cueFacts counts the lines of facts about a cue above its button: its ID,
// category and recordings.
const cueFacts = 4

// drawCue draws the selected cue's inspector: its name, ID, category,
// recordings, where it is used and its buttons.
func (s *State) drawCue(screen *ebiten.Image, cue protocol.AudioCue, left int) {
	s.drawHeader(screen, ui.IconSound, "Cue", cue.Name, left)
	top := headerRule + ui.Inset
	s.theme.DrawMono(screen, cue.ID, left, top, ui.ColorSubdued())
	top += ui.LineHeight
	category := "SFX"
	for _, named := range audioCategories() {
		if named[0] == cue.Category {
			category = named[1]
		}
	}
	s.theme.DrawText(screen, category+" · "+plural(len(cue.Recordings), "1 recording",
		strconv.Itoa(len(cue.Recordings))+" recordings, in turn"), left, top, ui.ColorMuted())
	top += ui.LineHeight
	if detail := audioDetail(cue); detail != "" && detail != strconv.Itoa(len(cue.Recordings))+" in turn" {
		s.theme.DrawText(screen, strings.ToUpper(detail[:1])+detail[1:], left, top, ui.ColorWarn())
	}
	s.audio.open.Draw(screen, s.theme)
	top = s.audio.open.Rect.Max.Y + ui.Inset
	s.theme.Heading(screen, "USED BY", left, top)
	top += ui.LineHeight
	if len(s.audio.uses) == 0 {
		s.theme.DrawText(screen, "Nothing uses this cue yet.", left, top, ui.ColorSubdued())
	}
	width := frame.Inspector(s.size).Dx() - ui.Half*ui.Inset
	for _, use := range s.audio.uses {
		s.theme.DrawText(screen, s.theme.FitText(use.Name+" · "+use.Label, width), left, top, ui.ColorInk())
		top += ui.LineHeight
	}
	s.remove.Draw(screen, s.theme)
}
