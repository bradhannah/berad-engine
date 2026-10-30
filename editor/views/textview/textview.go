// Package textview is the Text window: a placeholder document with a body of
// text and a location, used to judge text editing and to prove pick on map.
// The body keeps a draft; Save or Cmd+S saves it, Enter starts a new line and
// Escape reverts. The location is chosen by picking an anchor in a Map window
// (D13).
package textview

import (
	"encoding/json"
	"image"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/plugins/textdoc"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/ui/textedit"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/picker"
	"github.com/bradhannah/berad-engine/engine/ids"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
)

// ViewName is the Text window's view name; its target is a text document.
const ViewName = "text"

// cancelledHere is the reason given when a pick is cancelled in this window.
const cancelledHere = "cancelled in the text"

const (
	bodyTop       = ui.Inset + ui.LineHeight + 4
	locationSpace = 4*ui.LineHeight + ui.ButtonH
	pickWidth     = 120
	buttonGap     = 8
	shortID       = 6
)

// State is one Text window.
type State struct {
	window   ids.ULID
	document ids.ULID
	theme    *ui.Theme
	size     image.Point
	body     *ui.TextField
	focus    ui.Focus
	save     *ui.SaveBar
	guard    frame.Guard
	pickBtn  *ui.Button
	clearBtn *ui.Button
	subs     map[ids.ULID]bool
	picker   *picker.Picker
}

// New returns the Text window for a document.
func New(window, document ids.ULID, theme *ui.Theme) *State {
	view := &State{
		window: window, document: document, theme: theme, size: image.Pt(0, 0),
		body:     ui.NewTextField("body", textedit.NewField("", true, textdoc.MaxBody), true, theme),
		focus:    ui.Focus{},
		save:     ui.NewSaveBar(),
		guard:    frame.Guard{},
		pickBtn:  ui.NewButton("pick", "Pick on map"),
		clearBtn: ui.NewButton("clear", "Clear"),
		subs:     map[ids.ULID]bool{},
		picker:   picker.New(window),
	}
	view.body.Placeholder = "Write here. Save or Cmd+S saves; Escape reverts."
	view.focus.Register(view.body)

	return view
}

// Title is the window title.
func (s *State) Title(docs frame.Docs) string {
	name := "Text"
	docs.View(project.IndexID(), func(document client.Document) {
		if index, ok := document.Model.(*project.Index); ok {
			name = index.Name(s.document, name)
		}
	})

	return "Text: " + name + "  (view " + s.window.Short(shortID) + ")"
}

// Focused returns the focused text field.
func (s *State) Focused() *ui.TextField { return s.focus.Current() }

// Picking reports whether a pick is in progress.
func (s *State) Picking() bool { return s.picker.Picking() }

// Body returns the body field.
func (s *State) Body() *ui.TextField { return s.body }

// Start subscribes to the project index, for names.
func (s *State) Start(ctx *frame.Context) {
	s.subscribe(project.IndexID(), ctx)
}

func (s *State) subscribe(document ids.ULID, ctx *frame.Context) {
	if s.subs[document] {
		return
	}
	s.subs[document] = true
	ctx.Requests.Subscribe(document, func(err error) {
		if err != nil {
			delete(s.subs, document)
		}
	})
}

func (s *State) text(docs frame.Docs) (textdoc.Document, bool) {
	var found textdoc.Document
	loaded := docs.View(s.document, func(document client.Document) {
		if textDoc, ok := document.Model.(*textdoc.Document); ok {
			found = *textDoc
		}
	})

	return found, loaded
}

// Changed reacts to picks and keeps the location's floor subscribed.
func (s *State) Changed(update client.Update, ctx *frame.Context) {
	s.picker.Changed(update, ctx)
	if document, ok := s.text(ctx.Docs); ok && !document.Location.IsZero() {
		s.subscribe(document.Location.Floor, ctx)
	}
}

func (s *State) answered(answer protocol.PickAnswer, ctx *frame.Context) {
	anchor, err := ids.Parse(answer.Item)
	if err != nil {
		ctx.Frame.Status("The map answered with something that is not an anchor")

		return
	}
	s.subscribe(answer.Floor, ctx)
	s.command(ctx, textdoc.SetLocationEdit, textdoc.SetLocationArgs{
		Location: floor.AnchorRef{Floor: answer.Floor, Anchor: anchor},
	}, "Location set")
}

func (s *State) command(ctx *frame.Context, edit string, args any, success string) {
	data, err := json.Marshal(args)
	if err != nil {
		ctx.Frame.Status(err.Error())

		return
	}
	command := protocol.Command{Document: s.document, Edit: edit, Gesture: "", Args: data}
	ctx.Requests.Command(command, ctx.Frame.Report(success))
}

func (s *State) layout() {
	content := frame.Content(s.size)
	width := content.Max.X - ui.Half*ui.Inset
	barHeight := ui.SaveBarHeight(width)
	bottom := content.Max.Y - locationSpace - ui.Inset - barHeight - ui.Gap
	s.body.Rect = image.Rect(ui.Inset, bodyTop, content.Max.X-ui.Inset, max(bottom, bodyTop+ui.ButtonH))
	barTop := s.body.Rect.Max.Y + ui.Gap
	s.save.Rect = image.Rect(ui.Inset, barTop, content.Max.X-ui.Inset, barTop+barHeight)
	buttons := content.Max.Y - ui.Inset - ui.ButtonH
	s.pickBtn.Rect = image.Rect(ui.Inset, buttons, ui.Inset+pickWidth, buttons+ui.ButtonH)
	clearLeft := s.pickBtn.Rect.Max.X + buttonGap
	s.clearBtn.Rect = image.Rect(clearLeft, buttons, clearLeft+pickWidth, buttons+ui.ButtonH)
	s.pickBtn.Label = "Pick on map"
	if s.picker.Picking() {
		s.pickBtn.Label = "Cancel pick"
	}
}

// Update handles one frame of input.
func (s *State) Update(input *ui.Input, ctx *frame.Context) {
	s.size = ctx.Size
	s.layout()
	if document, ok := s.text(ctx.Docs); ok {
		s.body.Field.SetSaved(document.Body)
		s.clearBtn.Disabled = document.Location.IsZero()
	}
	s.save.Unsaved = s.Unsaved(ctx.Docs)
	if s.guard.Active() {
		s.guard.Update(input, s.size, func() bool { return s.Save(ctx) }, s.body.Field.Revert)

		return
	}
	if s.picker.Dialog() != nil {
		s.picker.UpdateDialog(input, ctx, image.Rectangle{Min: image.Pt(0, 0), Max: s.size})

		return
	}
	s.focus.Update(input, ctx.Clipboard)
	if s.focus.Current() == nil && s.picker.Picking() && input.Consume(ui.KeyEscape, ui.Plain()) {
		s.picker.Cancel(ctx, cancelledHere)
	}
	frame.CommonKeys(input, ctx)
	switch s.save.Update(input) {
	case ui.SaveID:
		s.Save(ctx)
	case ui.RevertID:
		s.focus.Set(nil)
		s.body.Field.Revert()
	}
	switch {
	case s.pickBtn.Update(input):
		if s.picker.Picking() {
			s.picker.Cancel(ctx, cancelledHere)
		} else {
			label := "the location of “" + s.zoneNames(ctx.Docs).Name(s.document, "this text") + "”"
			s.picker.Start(ctx, label, s.answered)
		}
	case s.clearBtn.Update(input):
		cleared := textdoc.SetLocationArgs{Location: floor.AnchorRef{Floor: ids.ULID{}, Anchor: ids.ULID{}}}
		s.command(ctx, textdoc.SetLocationEdit, cleared, "Location cleared")
	}
}

// Unsaved counts the body's unsaved draft.
func (s *State) Unsaved(frame.Docs) int { return frame.UnsavedFields(s.body) }

// Save saves the body, from the save bar or Cmd+S.
func (s *State) Save(ctx *frame.Context) bool {
	s.saveBody(ctx)

	return true
}

// UnsavedName names the text document.
func (s *State) UnsavedName(docs frame.Docs) string {
	return s.zoneNames(docs).Name(s.document, "this text")
}

// Leave asks about an unsaved body before then, such as closing the window.
func (s *State) Leave(ctx *frame.Context, then func()) {
	s.guard.Ask(s.Unsaved(ctx.Docs), s.UnsavedName(ctx.Docs), then)
}

func (s *State) saveBody(ctx *frame.Context) {
	body, changed := s.body.Field.Commit()
	if !changed {
		return
	}
	data, err := json.Marshal(textdoc.SetBodyArgs{Body: body})
	if err != nil {
		s.body.Field.Failed()

		return
	}
	ctx.Requests.Command(protocol.Command{Document: s.document, Edit: textdoc.SetBodyEdit, Gesture: "", Args: data},
		func(result protocol.Result, err error) {
			if err != nil {
				s.body.Field.Failed()
				ctx.Frame.Report("")(result, err)

				return
			}
			ctx.Frame.Status("Saved")
		})
}

func (s *State) zoneNames(docs frame.Docs) *project.Index {
	found := project.NewIndex(nil)
	docs.View(project.IndexID(), func(document client.Document) {
		if index, ok := document.Model.(*project.Index); ok {
			found = project.NewIndex(index.Entries())
		}
	})

	return found
}

// location describes the document's location for display: the floor and
// anchor names, or what is wrong with the reference.
func (s *State) location(docs frame.Docs, ref floor.AnchorRef) string {
	if ref.IsZero() {
		return "none"
	}
	floorName := s.zoneNames(docs).Name(ref.Floor, "")
	if floorName == "" {
		return "broken: its floor was deleted"
	}
	anchorName := ""
	loaded := docs.View(ref.Floor, func(document client.Document) {
		if floorDoc, ok := document.Model.(*tilefloor.Document); ok {
			if anchor, _, found := floorDoc.Floor.Anchor(ref.Anchor); found {
				anchorName = anchor.Name
			}
		}
	})
	switch {
	case !loaded:
		return floorName + " / …"
	case anchorName == "":
		return floorName + " / broken: the anchor was deleted"
	default:
		return floorName + " / " + anchorName
	}
}

// Locate finds a named part of the window for input scripts: "ui:body",
// "ui:pick", "ui:clear", or a dialog button or list row by ID ("row:<n>" for
// the nth row).
func (s *State) Locate(name string, docs frame.Docs) (image.Point, bool) {
	s.layout()
	_, value, _ := strings.Cut(name, ":")
	if s.guard.Active() {
		return s.guard.Locate(value, s.size)
	}
	if point, found := s.save.Locate(value); found && strings.HasPrefix(name, "ui:") {
		return point, true
	}
	if s.picker.Dialog() != nil {
		return s.picker.Locate(value, image.Rectangle{Min: image.Pt(0, 0), Max: s.size})
	}
	if strings.HasPrefix(name, "state:") {
		return image.Pt(0, 0), s.hasState(value, docs)
	}
	switch value {
	case s.body.ID:
		return s.body.Center(), true
	case s.pickBtn.ID:
		return ui.Center(s.pickBtn.Rect), true
	case s.clearBtn.ID:
		return ui.Center(s.clearBtn.Rect), true
	}

	return image.Pt(0, 0), false
}

// hasState answers input scripts waiting for the window: "loaded", "picking",
// "located" once a location is set, and "saved" once the body has no unsaved
// draft.
func (s *State) hasState(state string, docs frame.Docs) bool {
	document, loaded := s.text(docs)
	switch state {
	case "loaded":
		return loaded
	case "picking":
		return s.picker.Picking()
	case "located":
		return loaded && !document.Location.IsZero()
	case "saved":
		return loaded && !s.body.Field.Dirty() && !s.body.Field.Pending()
	}

	return false
}

// Draw renders the window.
func (s *State) Draw(screen *ebiten.Image, docs frame.Docs) {
	s.size = screen.Bounds().Size()
	s.layout()
	screen.Fill(ui.ColorBackground())
	s.theme.Heading(screen, "BODY", ui.Inset, ui.Inset)
	s.body.Draw(screen)
	s.save.Draw(screen, s.theme)
	if s.body.Field.Conflict() {
		s.theme.DrawText(screen, "Changed in another tab since you started. Save keeps your version; Escape takes theirs.",
			ui.Inset, s.save.Rect.Max.Y+ui.Gap, ui.ColorWarn())
	}
	document, loaded := s.text(docs)
	locationTop := s.pickBtn.Rect.Min.Y - ui.LineHeight - ui.Gap
	s.theme.Heading(screen, "LOCATION", ui.Inset, locationTop-ui.LineHeight)
	if loaded {
		s.theme.DrawText(screen, s.location(docs, document.Location), ui.Inset, locationTop, ui.ColorInk())
	}
	s.pickBtn.Draw(screen, s.theme)
	s.clearBtn.Draw(screen, s.theme)
	s.drawInspector(screen, document, loaded)
	if dialog := s.picker.Dialog(); dialog != nil {
		dialog.Draw(screen, s.theme)
	}
	s.guard.Draw(screen, s.theme)
}

func (s *State) drawInspector(screen *ebiten.Image, document textdoc.Document, loaded bool) {
	frame.DrawInspector(screen)
	panel := frame.Inspector(s.size)
	textX, textY := panel.Min.X+ui.Inset, panel.Min.Y+ui.Inset
	s.theme.Heading(screen, "INSPECTOR", textX, textY)
	if !loaded {
		return
	}
	lines := []string{
		strconv.Itoa(len([]rune(document.Body))) + " characters, " +
			strconv.Itoa(strings.Count(document.Body, "\n")+1) + " lines",
		"ID " + s.document.String(),
	}
	if s.picker.Picking() {
		lines = append(lines, "", "Picking: click an anchor", "in a Map.")
	}
	for index, line := range lines {
		ink := ui.ColorInk()
		if strings.HasPrefix(line, "ID ") {
			ink = ui.ColorSubdued()
		}
		s.theme.DrawText(screen, line, textX, textY+(index+1)*ui.LineHeight, ink)
	}
}
