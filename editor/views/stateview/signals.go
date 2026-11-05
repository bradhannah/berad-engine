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

// The SIGNALS section: its heading, each signal's and group's row, Add
// signal, and a selected signal's or group's name, description and Delete.
const (
	signalsSection  = sectionPrefix + "signals"
	signalPrefix    = "signal:"
	addSignalID     = "add-signal"
	signalNameID    = "signal-name"
	signalDescID    = "signal-description"
	deleteSignalID  = "delete-signal"
	signalLocate    = "signal"
	stateSignals    = "signals="
	newSignalName   = "signal"
	signalDescRows  = 3
	signalsTopLabel = "Signals"
)

// signalFields are a selected signal's or group's name, its description and
// Delete.
type signalFields struct {
	name, description *ui.TextField
	remove            *ui.Button
	// of is the signal or group the fields hold; deleting is what a dialog
	// asks about deleting, and deletingName what it is called; focusName
	// gives the name the focus once the fields hold the signal, one just
	// added or being renamed.
	of           ids.ULID
	deleting     []ids.ULID
	deletingName string
	focusName    bool
}

func newSignalFields(theme *ui.Theme) signalFields {
	remove := ui.NewButton(deleteSignalID, "Delete signal")
	remove.Danger = true

	return signalFields{
		name:        ui.NewTextField(signalNameID, textedit.NewField("", false, worldstate.MaxName), false, theme),
		description: ui.NewTextField(signalDescID, textedit.NewField("", true, worldstate.MaxText), true, theme),
		remove:      remove, of: ids.ULID{}, deleting: nil, deletingName: "", focusName: false,
	}
}

// isSignal reports whether a row is a signal or a group of them.
func isSignal(kind itemKind) bool { return kind == itemSignal || kind == itemSignalGroup }

// signalItem is what a signal's or group's row means.
func signalItem(kind itemKind, id ids.ULID, path string) item {
	return item{
		kind: kind, scope: worldstate.ScopeWorld,
		list:     valuetree.List{Document: worldstate.ID(), Edit: stateplugin.SignalsEdit, Values: nil},
		declared: worldstate.New(ids.ULID{}, "", "", ""), owner: noOwner(), ownerID: id, path: path,
	}
}

// signalRows adds the SIGNALS section: its groups and signals, as value
// groups are shown, each signal with how many workflows send and answer
// it.
func (s *State) signalRows(built *tree, docs frame.Docs) {
	heading := ui.NewHeading(signalsSection, signalsTopLabel)
	heading.Section, heading.Toggle = true, s.disclosure(signalsSection)
	built.rows = append(built.rows, heading)
	if !s.open(signalsSection) {
		return
	}
	declared, _ := signals.Declared(docs)
	built.signals = declared
	s.signalTreeRows(built, declared, signals.Uses(docs), ids.ULID{}, 0)
}

// signalTreeRows lists what a group of signals holds, depth first.
func (s *State) signalTreeRows(
	built *tree, declared []worldstate.Signal, uses map[ids.ULID][]signals.Use, parent ids.ULID, depth int,
) {
	for _, signal := range worldstate.SignalChildren(declared, parent) {
		rowID := signalPrefix + signal.ID.String()
		row := ui.NewRow(rowID, signal.Name)
		row.Indent, row.Detail = depth, signals.Counted(uses[signal.ID])
		kind := itemSignal
		if signal.Group {
			kind, row.Icon, row.Strong, row.Detail = itemSignalGroup, ui.IconFolder, true, newGroupName
			if len(worldstate.SignalChildren(declared, signal.ID)) > 0 {
				row.Toggle = s.disclosure(rowID)
			}
		}
		built.add(row, signalItem(kind, signal.ID, worldstate.SignalPath(declared, signal.ID)))
		if signal.Group && s.open(rowID) {
			s.signalTreeRows(built, declared, uses, signal.ID, depth+1)
		}
	}
}

// signalParent is the group the toolbar's adds go into while a signal or
// group is selected: the selected group, or the selected signal's group;
// the top otherwise.
func (s *State) signalParent(docs frame.Docs) ids.ULID {
	selected, found := s.selected()
	if !found || !isSignal(selected.kind) {
		return ids.ULID{}
	}
	if selected.kind == itemSignalGroup {
		return selected.ownerID
	}
	declared, _ := signals.Declared(docs)
	for _, signal := range declared {
		if signal.ID == selected.ownerID {
			return signal.Parent
		}
	}

	return ids.ULID{}
}

// signalWhere says where a signal or group goes: in a group, or at the top
// of Signals.
func signalWhere(declared []worldstate.Signal, parent ids.ULID) string {
	if parent.IsZero() {
		return "at the top of " + signalsTopLabel
	}

	return "in " + worldstate.SignalPath(declared, parent)
}

// addSignal declares a signal, or a group of them, in a group or at the
// top, with a name nothing beside it has, and selects it with its name
// ready to type.
func (s *State) addSignal(ctx *frame.Context, parent ids.ULID, group bool) {
	declared, _ := signals.Declared(ctx.Docs)
	taken := map[string]bool{}
	for _, held := range worldstate.SignalChildren(declared, parent) {
		taken[held.Name] = true
	}
	base, what := newSignalName, "Signal "
	if group {
		base, what = newGroupName, "Group "
	}
	name := worldstate.UniqueName(base, taken)
	added := worldstate.Signal{ID: ids.ULID{}, Name: name, Description: "", Parent: parent, Group: group}
	where := signalWhere(declared, parent)
	s.send(ctx, worldstate.ID(), stateplugin.SignalsEdit,
		stateplugin.SignalsArgs{Add: []worldstate.Signal{added}, Set: nil, Remove: nil},
		func(result protocol.Result, err error) {
			if err != nil {
				ctx.Frame.Report("")(result, err)

				return
			}
			s.toggled[signalsSection] = false
			if !parent.IsZero() {
				s.toggled[signalPrefix+parent.String()] = false
			}
			s.list.Selected, s.signal.focusName = signalPrefix+result.Created.String(), true
			ctx.Frame.Status(what + name + " added " + where + ". Type its name.")
		})
}

// showSignal puts a selected signal or group into its fields, unless they
// hold it already, and gives the name the focus when asked to.
func (s *State) showSignal(docs frame.Docs, selected item) {
	if s.signal.of == selected.ownerID {
		s.focusSignalName()

		return
	}
	declared, _ := signals.Declared(docs)
	for _, signal := range declared {
		if signal.ID != selected.ownerID {
			continue
		}
		s.focus = ui.Focus{}
		s.focus.Register(s.signal.name, s.signal.description)
		s.signal.of = signal.ID
		s.signal.name.Field.SetSaved(signal.Name)
		s.signal.description.Field.SetSaved(signal.Description)
		s.signal.remove.Label = "Delete signal"
		if signal.Group {
			s.signal.remove.Label = "Delete group"
		}
		s.focusSignalName()
	}
}

// focusSignalName gives the name field the focus, once, when asked to.
func (s *State) focusSignalName() {
	if s.signal.focusName && !s.signal.of.IsZero() {
		s.signal.focusName = false
		s.focus.Set(s.signal.name)
	}
}

// renameSignal gives the selected signal's or group's name the focus.
func (s *State) renameSignal() {
	s.signal.focusName = true
	s.focusSignalName()
}

// saveSignal saves the selected signal's or group's name and description,
// or says what is wrong with the name.
func (s *State) saveSignal(ctx *frame.Context) bool {
	name := strings.TrimSpace(s.signal.name.Field.Value())
	if err := worldstate.CheckName(name); err != nil {
		s.focus.Set(s.signal.name)
		ctx.Frame.Status("A signal's name is lower-case letters, digits and underscores, starting with a letter.")

		return false
	}
	declared, _ := signals.Declared(ctx.Docs)
	var set worldstate.Signal
	for _, signal := range declared {
		if signal.ID == s.signal.of {
			set = signal
		}
	}
	set.Name, set.Description = name, strings.TrimSpace(s.signal.description.Field.Value())
	s.signal.name.Field.Commit()
	s.signal.description.Field.Commit()
	s.send(ctx, worldstate.ID(), stateplugin.SignalsEdit,
		stateplugin.SignalsArgs{Add: nil, Set: []worldstate.Signal{set}, Remove: nil},
		func(result protocol.Result, err error) {
			if err != nil {
				s.signal.name.Field.Failed()
				ctx.Frame.Report("")(result, err)

				return
			}
			s.focus.Set(nil)
			ctx.Frame.Status("Saved " + name)
		})

	return true
}

// startSignalDelete deletes a signal, or a group with everything in it, at
// once when no workflow uses them, or asks first, listing the workflows
// that send or answer them.
func (s *State) startSignalDelete(ctx *frame.Context, selected item) {
	declared, _ := signals.Declared(ctx.Docs)
	going := append([]ids.ULID{selected.ownerID}, worldstate.SignalsWithin(declared, selected.ownerID)...)
	everyUse := signals.Uses(ctx.Docs)
	var uses []signals.Use
	for _, signal := range going {
		uses = append(uses, everyUse[signal]...)
	}
	if len(uses) == 0 {
		s.deleteSignals(ctx, going, selected.path)

		return
	}
	lines := []string{"Workflows send or answer it. Their nodes will name a missing signal:"}
	for _, use := range uses {
		lines = append(lines, use.Name+" · "+kindLabel(use.Kind))
	}
	title := "Delete " + selected.path + "?"
	if len(going) > 1 {
		title = "Delete " + selected.path + " and everything in it?"
	}
	s.signal.deleting, s.signal.deletingName = going, selected.path
	s.dialog = ui.NewDialog(title, lines, dangerButton(deleteID, "Delete"), ui.NewButton("cancel", "Cancel"))
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

// deleteSignals deletes signals and groups as one change; undo brings
// them back.
func (s *State) deleteSignals(ctx *frame.Context, going []ids.ULID, name string) {
	s.send(ctx, worldstate.ID(), stateplugin.SignalsEdit,
		stateplugin.SignalsArgs{Add: nil, Set: nil, Remove: going},
		func(result protocol.Result, err error) {
			if err != nil {
				ctx.Frame.Report("")(result, err)

				return
			}
			s.signal.of = ids.ULID{}
			ctx.Frame.Status(name + " deleted. Undo brings it back.")
		})
}

// layoutSignal places a selected signal's or group's name, description and
// Delete.
func (s *State) layoutSignal(panel image.Rectangle) {
	left, width := panel.Min.X+ui.Inset, panel.Dx()-ui.Half*ui.Inset
	top := inspectorTop(panel) + ui.LineHeight
	s.signal.name.Rect = image.Rect(left, top, left+width, top+ui.ButtonH)
	top += ui.ButtonH + ui.Gap + ui.LineHeight
	s.signal.description.Rect = image.Rect(left, top, left+width, top+signalDescRows*ui.LineHeight)
	bottom := panel.Max.Y - ui.Inset
	s.signal.remove.Rect = image.Rect(left, bottom-ui.ButtonH, left+width, bottom)
}

// drawSignal draws a selected signal's name, description, where it is used
// and Delete; or a group's, with what it holds.
func (s *State) drawSignal(screen *ebiten.Image, docs frame.Docs, panel image.Rectangle, group bool) {
	left, width := panel.Min.X+ui.Inset, panel.Dx()-ui.Half*ui.Inset
	s.theme.DrawStrong(screen, "Name", left, s.signal.name.Rect.Min.Y-ui.LineHeight, ui.ColorInk())
	s.signal.name.Draw(screen)
	s.theme.DrawStrong(screen, "Description", left, s.signal.description.Rect.Min.Y-ui.LineHeight, ui.ColorInk())
	s.signal.description.Draw(screen)
	top := s.signal.description.Rect.Max.Y + ui.Inset
	label, lines, none := "SENT AND ANSWERED BY", s.signalUseLines(docs), "No workflow yet."
	if group {
		label, lines, none = "HOLDS", s.heldLines(docs), "Nothing yet."
	}
	s.theme.DrawLabel(screen, label, left, top, ui.ColorMuted())
	top += ui.LineHeight
	if len(lines) == 0 {
		s.theme.DrawText(screen, none, left, top, ui.ColorMuted())
	}
	for _, line := range lines {
		s.theme.DrawText(screen, s.theme.FitText(line, width), left, top, ui.ColorInk())
		top += ui.LineHeight
	}
	s.signal.remove.Draw(screen, s.theme)
}

// signalUseLines lists the workflows that send or answer the selected
// signal.
func (s *State) signalUseLines(docs frame.Docs) []string {
	uses := signals.Uses(docs)[s.signal.of]
	lines := make([]string, 0, len(uses))
	for _, use := range uses {
		lines = append(lines, use.Name+" · "+kindLabel(use.Kind))
	}

	return lines
}

// heldLines lists what the selected group holds.
func (s *State) heldLines(docs frame.Docs) []string {
	declared, _ := signals.Declared(docs)
	held := worldstate.SignalChildren(declared, s.signal.of)
	lines := make([]string, 0, len(held))
	for _, held := range held {
		line := held.Name
		if held.Group {
			line += " · group"
		}
		lines = append(lines, line)
	}

	return lines
}

// hasSignalState answers "signals=<count>", how many signals, not groups,
// are declared.
func hasSignalState(docs frame.Docs, state string) (bool, bool) {
	count, asked := strings.CutPrefix(state, stateSignals)
	if !asked {
		return false, false
	}
	declared, _ := signals.Declared(docs)

	return strconv.Itoa(len(signals.Sendable(declared))) == count, true
}

// locateSignal finds a signal's or group's row by its path,
// "signal:<path>".
func (s *State) locateSignal(path string) (image.Point, bool) {
	for _, row := range s.list.Rows {
		if meaning, found := s.shown.items[row.ID]; found && isSignal(meaning.kind) && meaning.path == path {
			return s.list.Locate(row.ID)
		}
	}

	return image.Pt(0, 0), false
}
