package stateview

import (
	"image"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	stateplugin "github.com/bradhannah/berad-engine/editor/plugins/worldstate"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/ui/textedit"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/signals"
	"github.com/bradhannah/berad-engine/editor/views/valuetree"
	"github.com/bradhannah/berad-engine/engine/ids"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// The SIGNALS section: its heading, each signal's row, Add signal, and a
// selected signal's name, line and Delete.
const (
	signalsSection  = sectionPrefix + "signals"
	signalPrefix    = "signal:"
	addSignalID     = "add-signal"
	signalNameID    = "signal-name"
	signalAboutID   = "signal-about"
	deleteSignalID  = "delete-signal"
	signalLocate    = "signal"
	stateSignals    = "signals="
	newSignalName   = "signal"
	signalAboutRows = 3
)

// signalFields are a selected signal's name, its line and Delete.
type signalFields struct {
	name, about *ui.TextField
	remove      *ui.Button
	// of is the signal the fields hold, and deleting the one a dialog asks
	// about deleting.
	of       ids.ULID
	deleting ids.ULID
}

func newSignalFields(theme *ui.Theme) signalFields {
	remove := ui.NewButton(deleteSignalID, "Delete signal")
	remove.Danger = true

	return signalFields{
		name:   ui.NewTextField(signalNameID, textedit.NewField("", false, worldstate.MaxName), false, theme),
		about:  ui.NewTextField(signalAboutID, textedit.NewField("", true, worldstate.MaxText), true, theme),
		remove: remove, of: ids.ULID{}, deleting: ids.ULID{},
	}
}

// signalRows adds the SIGNALS section: each declared signal, by name, with
// how many workflows send and answer it.
func (s *State) signalRows(built *tree, docs frame.Docs) {
	heading := ui.NewHeading(signalsSection, "Signals")
	heading.Section, heading.Toggle = true, s.disclosure(signalsSection)
	built.rows = append(built.rows, heading)
	if !s.open(signalsSection) {
		return
	}
	declared, _ := signals.Declared(docs)
	uses := signals.Uses(docs)
	for _, signal := range declared {
		row := ui.NewRow(signalPrefix+signal.ID.String(), signal.Name)
		row.Detail = signalUses(uses[signal.ID])
		built.add(row, item{
			kind: itemSignal, scope: worldstate.ScopeWorld,
			list:     valuetree.List{Document: worldstate.ID(), Edit: stateplugin.SignalsEdit, Values: nil},
			declared: worldstate.New(ids.ULID{}, "", "", ""), owner: noOwner(), ownerID: signal.ID, path: signal.Name,
		})
	}
}

// signalUses says how many workflows send and answer a signal.
func signalUses(uses []signals.Use) string {
	sent, answered := 0, 0
	for _, use := range uses {
		if use.Kind == flow.KindSendSignal {
			sent++
		} else {
			answered++
		}
	}

	return "sent " + strconv.Itoa(sent) + " · answered " + strconv.Itoa(answered)
}

// addSignal declares a signal with a name no other has, and selects it with
// its name ready to type.
func (s *State) addSignal(ctx *frame.Context) {
	declared, _ := signals.Declared(ctx.Docs)
	taken := map[string]bool{}
	for _, signal := range declared {
		taken[signal.Name] = true
	}
	name := worldstate.UniqueName(newSignalName, taken)
	added := worldstate.Signal{ID: ids.ULID{}, Name: name, About: ""}
	s.send(ctx, worldstate.ID(), stateplugin.SignalsEdit,
		stateplugin.SignalsArgs{Add: []worldstate.Signal{added}, Set: nil, Remove: nil},
		func(result protocol.Result, err error) {
			if err != nil {
				ctx.Frame.Report("")(result, err)

				return
			}
			s.toggled[signalsSection] = false
			s.list.Selected = signalPrefix + result.Created.String()
			ctx.Frame.Status("Signal " + name + " added. Name it, and send it with Send signal.")
		})
}

// showSignal puts a selected signal into its fields, unless they hold it
// already.
func (s *State) showSignal(docs frame.Docs, selected item) {
	if s.signal.of == selected.ownerID {
		return
	}
	declared, _ := signals.Declared(docs)
	for _, signal := range declared {
		if signal.ID == selected.ownerID {
			s.focus = ui.Focus{}
			s.focus.Register(s.signal.name, s.signal.about)
			s.signal.of = signal.ID
			s.signal.name.Field.SetSaved(signal.Name)
			s.signal.about.Field.SetSaved(signal.About)
		}
	}
}

// saveSignal saves the selected signal's name and line, or says what is
// wrong with the name.
func (s *State) saveSignal(ctx *frame.Context) bool {
	name := strings.TrimSpace(s.signal.name.Field.Value())
	if err := worldstate.CheckName(name); err != nil {
		s.focus.Set(s.signal.name)
		ctx.Frame.Status("A signal's name is lower-case letters, digits and underscores, starting with a letter.")

		return false
	}
	set := worldstate.Signal{ID: s.signal.of, Name: name, About: strings.TrimSpace(s.signal.about.Field.Value())}
	s.signal.name.Field.Commit()
	s.signal.about.Field.Commit()
	s.send(ctx, worldstate.ID(), stateplugin.SignalsEdit,
		stateplugin.SignalsArgs{Add: nil, Set: []worldstate.Signal{set}, Remove: nil},
		func(result protocol.Result, err error) {
			if err != nil {
				s.signal.name.Field.Failed()
				ctx.Frame.Report("")(result, err)

				return
			}
			s.focus.Set(nil)
			ctx.Frame.Status("Saved signal " + name)
		})

	return true
}

// startSignalDelete deletes a signal no workflow uses, or asks first,
// listing the workflows that send or answer it.
func (s *State) startSignalDelete(ctx *frame.Context, selected item) {
	uses := signals.Uses(ctx.Docs)[selected.ownerID]
	if len(uses) == 0 {
		s.deleteSignal(ctx, selected.ownerID, selected.path)

		return
	}
	lines := []string{"Workflows send or answer it. Their nodes will name a missing signal:"}
	for _, use := range uses {
		lines = append(lines, use.Name+" · "+kindLabel(use.Kind))
	}
	s.signal.deleting = selected.ownerID
	s.dialog = ui.NewDialog("Delete "+selected.path+"?", lines, dangerButton(deleteID, "Delete"),
		ui.NewButton("cancel", "Cancel"))
}

// kindLabel names a kind of node.
func kindLabel(kind string) string {
	declared, _ := flow.KindOf(kind)

	return declared.Label
}

// dangerButton is a button drawn as a destructive action.
func dangerButton(id, label string) *ui.Button {
	button := ui.NewButton(id, label)
	button.Danger = true

	return button
}

// deleteSignal deletes a signal; undo brings it back.
func (s *State) deleteSignal(ctx *frame.Context, signal ids.ULID, name string) {
	s.send(ctx, worldstate.ID(), stateplugin.SignalsEdit,
		stateplugin.SignalsArgs{Add: nil, Set: nil, Remove: []ids.ULID{signal}},
		func(result protocol.Result, err error) {
			if err != nil {
				ctx.Frame.Report("")(result, err)

				return
			}
			s.signal.of = ids.ULID{}
			ctx.Frame.Status(name + " deleted. Undo brings it back.")
		})
}

// layoutSignal places a selected signal's name, line and Delete.
func (s *State) layoutSignal(panel image.Rectangle) {
	left, width := panel.Min.X+ui.Inset, panel.Dx()-ui.Half*ui.Inset
	top := inspectorTop(panel) + ui.LineHeight
	s.signal.name.Rect = image.Rect(left, top, left+width, top+ui.ButtonH)
	top += ui.ButtonH + ui.Gap + ui.LineHeight
	s.signal.about.Rect = image.Rect(left, top, left+width, top+signalAboutRows*ui.LineHeight)
	bottom := panel.Max.Y - ui.Inset
	s.signal.remove.Rect = image.Rect(left, bottom-ui.ButtonH, left+width, bottom)
}

// drawSignal draws a selected signal's name, line, where it is used and
// Delete.
func (s *State) drawSignal(screen *ebiten.Image, docs frame.Docs, panel image.Rectangle) {
	left, width := panel.Min.X+ui.Inset, panel.Dx()-ui.Half*ui.Inset
	s.theme.DrawStrong(screen, "Name", left, s.signal.name.Rect.Min.Y-ui.LineHeight, ui.ColorInk())
	s.signal.name.Draw(screen)
	s.theme.DrawStrong(screen, "About", left, s.signal.about.Rect.Min.Y-ui.LineHeight, ui.ColorInk())
	s.signal.about.Draw(screen)
	top := s.signal.about.Rect.Max.Y + ui.Inset
	s.theme.DrawLabel(screen, "SENT AND ANSWERED BY", left, top, ui.ColorMuted())
	top += ui.LineHeight
	uses := signals.Uses(docs)[s.signal.of]
	if len(uses) == 0 {
		s.theme.DrawText(screen, "No workflow yet.", left, top, ui.ColorMuted())
	}
	for _, use := range uses {
		s.theme.DrawText(screen, s.theme.FitText(use.Name+" · "+kindLabel(use.Kind), width), left, top, ui.ColorInk())
		top += ui.LineHeight
	}
	s.signal.remove.Draw(screen, s.theme)
}

// hasSignalState answers "signals=<count>", how many signals are declared.
func hasSignalState(docs frame.Docs, state string) (bool, bool) {
	count, asked := strings.CutPrefix(state, stateSignals)
	if !asked {
		return false, false
	}
	declared, _ := signals.Declared(docs)

	return strconv.Itoa(len(declared)) == count, true
}

// locateSignal finds a signal's row, "signal:<name>".
func (s *State) locateSignal(name string) (image.Point, bool) {
	for _, row := range s.list.Rows {
		if meaning, found := s.shown.items[row.ID]; found && meaning.kind == itemSignal && meaning.path == name {
			return s.list.Locate(row.ID)
		}
	}

	return image.Pt(0, 0), false
}
