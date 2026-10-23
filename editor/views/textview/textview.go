// Package textview is the Text window: a placeholder document with a body of
// text and a location, used to judge text editing and to prove pick on map.
// The body keeps a draft; Enter saves it, Cmd+Enter or Ctrl+J adds a new line
// and Escape reverts. The location is chosen by picking an anchor in a Map
// window (D13).
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
	"github.com/bradhannah/berad-engine/engine/ids"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
	"github.com/bradhannah/berad-engine/engine/zone"
)

// ViewName is the Text window's view name; its target is a text document.
const ViewName = "text"

const (
	bodyTop       = ui.Inset + ui.LineHeight + 4
	locationSpace = 4*ui.LineHeight + ui.ButtonH
	pickWidth     = 120
	buttonGap     = 8
	shortID       = 6
)

type dialogKind int

const (
	dialogNone dialogKind = iota
	dialogChooseMap
	dialogChooseZone
)

// State is one Text window.
type State struct {
	window   ids.ULID
	document ids.ULID
	theme    *ui.Theme
	size     image.Point
	body     *ui.TextField
	focus    ui.Focus
	pickBtn  *ui.Button
	clearBtn *ui.Button
	subs     map[ids.ULID]bool

	picking    string
	picks      int
	dialog     *ui.Dialog
	dialogKind dialogKind
}

// New returns the Text window for a document.
func New(window, document ids.ULID, theme *ui.Theme) *State {
	view := &State{
		window: window, document: document, theme: theme, size: image.Pt(0, 0),
		body:     ui.NewTextField("body", textedit.NewField("", true, textdoc.MaxBody), true, theme),
		focus:    ui.Focus{},
		pickBtn:  &ui.Button{ID: "pick", Label: "Pick on map", Rect: ui.Zero(), Disabled: false, Primary: false},
		clearBtn: &ui.Button{ID: "clear", Label: "Clear", Rect: ui.Zero(), Disabled: false, Primary: false},
		subs:     map[ids.ULID]bool{},
		picking:  "", picks: 0, dialog: nil, dialogKind: dialogNone,
	}
	view.body.Placeholder = "Write here. Enter saves; Cmd+Enter or Ctrl+J starts a new line."
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

	return "Text: " + name + "  (window " + s.window.Short(shortID) + ")"
}

// Focused returns the focused text field.
func (s *State) Focused() *ui.TextField { return s.focus.Current() }

// Picking reports whether a pick is in progress.
func (s *State) Picking() bool { return s.picking != "" }

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
	switch {
	case update.PickAnswer != nil && update.PickAnswer.Pick == s.picking:
		s.picking = ""
		s.answered(*update.PickAnswer, ctx)
	case update.PickCancel != nil && update.PickCancel.Pick == s.picking:
		s.picking = ""
		ctx.Frame.Status("Pick cancelled: " + update.PickCancel.Reason)
	}
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
	bottom := content.Max.Y - locationSpace - ui.Inset
	s.body.Rect = image.Rect(ui.Inset, bodyTop, content.Max.X-ui.Inset, max(bottom, bodyTop+ui.ButtonH))
	buttons := content.Max.Y - ui.Inset - ui.ButtonH
	s.pickBtn.Rect = image.Rect(ui.Inset, buttons, ui.Inset+pickWidth, buttons+ui.ButtonH)
	clearLeft := s.pickBtn.Rect.Max.X + buttonGap
	s.clearBtn.Rect = image.Rect(clearLeft, buttons, clearLeft+pickWidth, buttons+ui.ButtonH)
	s.pickBtn.Label = "Pick on map"
	if s.picking != "" {
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
	if s.dialog != nil {
		s.updateDialog(input, ctx)

		return
	}
	if field, outcome := s.focus.Update(input, ctx.Clipboard); field == s.body && outcome == textedit.OutcomeCommit {
		s.saveBody(ctx)
	}
	if s.focus.Current() == nil && s.picking != "" &&
		input.Consume(ui.KeyEscape, ui.Plain()) {
		s.cancelPick(ctx)
	}
	frame.CommonKeys(input, ctx)
	switch {
	case s.pickBtn.Update(input):
		if s.picking != "" {
			s.cancelPick(ctx)
		} else {
			s.askPick(ctx)
		}
	case s.clearBtn.Update(input):
		cleared := textdoc.SetLocationArgs{Location: floor.AnchorRef{Floor: ids.ULID{}, Anchor: ids.ULID{}}}
		s.command(ctx, textdoc.SetLocationEdit, cleared, "Location cleared")
	}
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

// askPick asks which maps could answer (D13): none open asks which zone to
// open, one open is used, several open asks which, marking the maps that
// hold anchors.
func (s *State) askPick(ctx *frame.Context) {
	ctx.Requests.PickAsk(protocol.PickAnchor, func(maps []protocol.PickMap, err error) {
		if err != nil {
			ctx.Frame.Status(err.Error())

			return
		}
		switch len(maps) {
		case 0:
			s.chooseZone(ctx)
		case 1:
			only := protocol.PickStart{
				Pick: "", What: "", Label: "", Window: maps[0].Window, Zone: maps[0].Zone, From: ids.ULID{},
			}
			s.startPick(ctx, only)
		default:
			s.chooseMap(ctx, maps)
		}
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

func (s *State) chooseZone(ctx *frame.Context) {
	index := s.zoneNames(ctx.Docs)
	zones := index.OfFormat(zone.FormatName)
	if len(zones) == 0 {
		ctx.Frame.Status("There are no zones to pick from. Create one in the project browser.")

		return
	}
	list := &ui.List{ID: "zones", Rect: ui.Zero(), Rows: nil, Selected: zones[0].ID.String()}
	for _, entry := range zones {
		list.Rows = append(list.Rows, ui.Row{
			ID: entry.ID.String(), Label: entry.Name, Detail: "", Indent: 0, Swatch: nil, Heading: false, Marked: false,
		})
	}
	s.dialog = ui.NewDialog("No map is open. Open which zone to pick from?", nil,
		ui.DialogButton("open", "Open map"), ui.DialogButton(ui.DialogCancel, "Cancel"))
	s.dialog.List = list
	s.dialogKind = dialogChooseZone
}

func (s *State) chooseMap(ctx *frame.Context, maps []protocol.PickMap) {
	index := s.zoneNames(ctx.Docs)
	list := &ui.List{ID: "maps", Rect: ui.Zero(), Rows: nil, Selected: ""}
	for number, option := range maps {
		detail := "view " + option.Window.Short(shortID)
		if !option.Shows {
			detail += ", no anchors"
		}
		list.Rows = append(list.Rows, ui.Row{
			ID: option.Window.String(), Label: strconv.Itoa(number+1) + ". " + index.Name(option.Zone, "zone"),
			Detail: detail, Indent: 0, Swatch: nil, Heading: false, Marked: option.Shows,
		})
		if list.Selected == "" && option.Shows {
			list.Selected = option.Window.String()
		}
	}
	if list.Selected == "" {
		list.Selected = list.Rows[0].ID
	}
	s.dialog = ui.NewDialog("Pick in which map?", []string{"● marks maps that have anchors."},
		ui.DialogButton("use", "Pick there"), ui.DialogButton(ui.DialogCancel, "Cancel"))
	s.dialog.List = list
	s.dialogKind = dialogChooseMap
}

func (s *State) updateDialog(input *ui.Input, ctx *frame.Context) {
	choice := s.dialog.Update(input, image.Rectangle{Min: image.Pt(0, 0), Max: s.size})
	if choice == "" {
		return
	}
	dialog, kind := s.dialog, s.dialogKind
	s.dialog, s.dialogKind = nil, dialogNone
	if choice == ui.DialogCancel || dialog.List == nil {
		return
	}
	chosen, err := ids.Parse(dialog.List.Selected)
	if err != nil {
		return
	}
	start := protocol.PickStart{Pick: "", What: "", Label: "", Window: ids.ULID{}, Zone: ids.ULID{}, From: ids.ULID{}}
	if kind == dialogChooseZone {
		start.Zone = chosen
	} else {
		start.Window = chosen
	}
	s.startPick(ctx, start)
}

func (s *State) startPick(ctx *frame.Context, start protocol.PickStart) {
	s.picks++
	start.Pick = s.window.String() + "-" + strconv.Itoa(s.picks)
	start.What = protocol.PickAnchor
	start.Label = "the location of “" + s.zoneNames(ctx.Docs).Name(s.document, "this text") + "”"
	s.picking = start.Pick
	ctx.Requests.PickStart(start, func(result protocol.Result, err error) {
		if err != nil {
			s.picking = ""
			ctx.Frame.Report("")(result, err)

			return
		}
		ctx.Frame.Status("Click an anchor in the map. Escape here or there cancels.")
	})
}

func (s *State) cancelPick(ctx *frame.Context) {
	ctx.Requests.PickCancel(protocol.PickCancel{Pick: s.picking, Reason: "cancelled in the text window"})
	s.picking = ""
	ctx.Frame.Status("Pick cancelled")
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
	if s.dialog != nil {
		if number, err := strconv.Atoi(strings.TrimPrefix(value, "row")); err == nil && s.dialog.List != nil &&
			number >= 1 && number <= len(s.dialog.List.Rows) {
			s.dialog.Locate("", image.Rectangle{Min: image.Pt(0, 0), Max: s.size})

			return s.dialog.List.Locate(s.dialog.List.Rows[number-1].ID)
		}

		return s.dialog.Locate(value, image.Rectangle{Min: image.Pt(0, 0), Max: s.size})
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
		return s.picking != ""
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
	hint := "Enter saves · Cmd+Enter or Ctrl+J: new line · Escape reverts"
	ink := ui.ColorMuted()
	switch {
	case s.body.Field.Conflict():
		hint = "Changed in another window since you started. Enter keeps your version; Escape takes theirs."
		ink = ui.ColorWarn()
	case s.body.Field.Dirty():
		hint, ink = "Unsaved. "+hint, ui.ColorWarn()
	}
	s.theme.DrawText(screen, hint, ui.Inset, s.body.Rect.Max.Y+ui.Gap, ink)
	document, loaded := s.text(docs)
	locationTop := s.pickBtn.Rect.Min.Y - ui.LineHeight - ui.Gap
	s.theme.Heading(screen, "LOCATION", ui.Inset, locationTop-ui.LineHeight)
	if loaded {
		s.theme.DrawText(screen, s.location(docs, document.Location), ui.Inset, locationTop, ui.ColorInk())
	}
	s.pickBtn.Draw(screen, s.theme)
	s.clearBtn.Draw(screen, s.theme)
	s.drawInspector(screen, document, loaded)
	if s.dialog != nil {
		s.dialog.Draw(screen, s.theme)
	}
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
	if s.picking != "" {
		lines = append(lines, "", "Picking: click an anchor", "in a Map window.")
	}
	for index, line := range lines {
		ink := ui.ColorInk()
		if strings.HasPrefix(line, "ID ") {
			ink = ui.ColorSubdued()
		}
		s.theme.DrawText(screen, line, textX, textY+(index+1)*ui.LineHeight, ink)
	}
}
