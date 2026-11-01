// Package templateview is the Template editor: a template's rules and guides,
// with a magnified preview of the marks they make, optionally over a sprite
// (increment 16).
//
// Every button applies at once as one undo step, shared by every view of the
// template; a guide's name is typed, and waits for Save, Enter or Cmd+S.
package templateview

import (
	"encoding/json"
	"image"
	"slices"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/client"
	artplugin "github.com/bradhannah/berad-engine/editor/plugins/art"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/ui/textedit"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/tileart"
	"github.com/bradhannah/berad-engine/engine/art"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// ViewName is the Template editor's view name; its target is a template.
const ViewName = "template"

// UndoGroup is the undo list the Template editors of one template share.
func UndoGroup(template ids.ULID) string { return "template:" + template.String() }

// shortID is how much of the window's ID its title shows.
const shortID = 6

// List and preview geometry, in screen pixels.
const (
	listWidth   = 470
	rowStep     = 34
	edgeWidth   = 76
	stepWidth   = 26
	valueWidth  = 52
	markWidth   = 104
	nameWidth   = 120
	lineWidth   = 70
	fromWidth   = 128
	addWidth    = 96
	showWidth   = 180
	headGap     = 6
	maxZoom     = 64
	fitMargin   = 48
	gridAlpha   = 0x30
	minGridZoom = 4
	nameBytes   = 4 * artplugin.MaxGuideNameLength
)

// showItem prefixes a sprite's ID in the Show on menu; the item for nothing
// is the prefix alone.
const showItem = "sprite:"

// ruleRow is one rule's widgets: its edge, its width with a step either side,
// its mark and Remove.
type ruleRow struct {
	edge, narrower, wider, mark, remove *ui.Button
	width                               image.Rectangle
}

// guideRow is one guide's widgets: its name, its line, where it is measured
// from, its distance with a step either side, and Remove.
type guideRow struct {
	name                           *ui.TextField
	line, from, less, more, remove *ui.Button
	at                             image.Rectangle
}

// State is one Template editor window.
type State struct {
	window   ids.ULID
	template ids.ULID
	theme    *ui.Theme
	size     image.Point
	subs     map[ids.ULID]bool
	rules    []ruleRow
	guides   []guideRow
	addRule  *ui.Button
	addGuide *ui.Button
	// showOn opens showMenu, which chooses the sprite the preview shows
	// under the template; shown is never saved.
	showOn   *ui.Button
	showMenu ui.Menu
	shown    ids.ULID
	picture  *tileart.Picture
	focus    ui.Focus
	save     *ui.SaveBar
	guard    frame.Guard
	// gestures numbers the window's changes, so each undoes as one step.
	gestures int
	headings [2]int
	preview  image.Rectangle
}

// New returns the Template editor for a template.
func New(window, template ids.ULID, theme *ui.Theme) *State {
	view := &State{
		window: window, template: template, theme: theme, size: image.Pt(0, 0), subs: map[ids.ULID]bool{},
		rules: nil, guides: nil, addRule: ui.NewButton("add-rule", "+ Rule"),
		addGuide: ui.NewButton("add-guide", "+ Guide"), showOn: ui.NewButton("show-on", "Nothing"),
		showMenu: ui.Menu{Items: nil, Width: 0}, shown: ids.ULID{}, picture: tileart.NewPicture(),
		focus: ui.Focus{}, save: ui.NewSaveBar(), guard: frame.Guard{}, gestures: 0, headings: [2]int{},
		preview: ui.Zero(),
	}
	view.showOn.Caret = true

	return view
}

// Title is the window title: the template's name.
func (s *State) Title(docs frame.Docs) string {
	return "Template: " + name(docs, s.template, "Template") + "  (view " + s.window.Short(shortID) + ")"
}

// Focused returns the focused guide name, if any.
func (s *State) Focused() *ui.TextField { return s.focus.Current() }

// Start subscribes to the project index, for names, and the art settings,
// for the tile size the preview shows.
func (s *State) Start(ctx *frame.Context) {
	s.subscribe(project.IndexID(), ctx)
	s.subscribe(art.SettingsID(), ctx)
}

func (s *State) subscribe(document ids.ULID, ctx *frame.Context) {
	if s.subs[document] || document.IsZero() {
		return
	}
	s.subs[document] = true
	ctx.Requests.Subscribe(document, func(err error) {
		if err != nil {
			delete(s.subs, document)
			ctx.Frame.Status(err.Error())
		}
	})
}

// Changed keeps the shown sprite, its tile and the palettes it is painted
// with subscribed.
func (s *State) Changed(_ client.Update, ctx *frame.Context) { s.syncShown(ctx) }

// syncShown subscribes to the shown sprite, its tile, every palette of the
// project and every palette the tile names.
func (s *State) syncShown(ctx *frame.Context) {
	if s.shown.IsZero() {
		return
	}
	s.subscribe(s.shown, ctx)
	tileID, ok := shownTile(ctx.Docs, s.shown)
	if !ok {
		return
	}
	s.subscribe(tileID, ctx)
	palettes := []ids.ULID{art.DefaultPaletteID()}
	ctx.Docs.View(project.IndexID(), func(document client.Document) {
		if index, isIndex := document.Model.(*project.Index); isIndex {
			for _, entry := range index.OfFormat(art.PaletteFormatName) {
				palettes = append(palettes, entry.ID)
			}
		}
	})
	ctx.Docs.View(tileID, func(document client.Document) {
		if tileDoc, isTile := document.Model.(*artplugin.TileDocument); isTile {
			for _, ref := range tileDoc.Tile.Colors() {
				if !ref.IsLoose() {
					palettes = append(palettes, ref.Palette)
				}
			}
		}
	})
	for _, palette := range palettes {
		s.subscribe(palette, ctx)
	}
}

// shownTile returns a sprite's tile.
func shownTile(docs frame.Docs, sprite ids.ULID) (ids.ULID, bool) {
	var tile ids.ULID
	docs.View(sprite, func(document client.Document) {
		if spriteDoc, ok := document.Model.(*artplugin.SpriteDocument); ok {
			tile = spriteDoc.Sprite.Tile
		}
	})

	return tile, !tile.IsZero()
}

// name names a document from the project index.
func name(docs frame.Docs, document ids.ULID, fallback string) string {
	found := fallback
	docs.View(project.IndexID(), func(index client.Document) {
		if held, ok := index.Model.(*project.Index); ok {
			found = held.Name(document, fallback)
		}
	})

	return found
}

// current returns the template as saved.
func (s *State) current(docs frame.Docs) (artplugin.Template, bool) {
	var found artplugin.Template
	loaded := false
	docs.View(s.template, func(document client.Document) {
		if templateDoc, ok := document.Model.(*artplugin.TemplateDocument); ok {
			found, loaded = templateDoc.Template.Clone(), true
		}
	})

	return found, loaded
}

// tileSize returns the project's tile size, which the preview shows.
func tileSize(docs frame.Docs) int {
	size := art.DefaultTileSize
	docs.View(art.SettingsID(), func(document client.Document) {
		if settings, ok := document.Model.(*artplugin.SettingsDocument); ok {
			size = settings.Settings.TileSize
		}
	})

	return size
}

// syncRows keeps a row of widgets for each rule and guide, and each guide's
// name field showing its saved name.
func (s *State) syncRows(template artplugin.Template) {
	for len(s.rules) < len(template.Rules) {
		prefix := "rule-" + strconv.Itoa(len(s.rules)+1) + "-"
		s.rules = append(s.rules, ruleRow{
			edge: ui.NewButton(prefix+"edge", ""), narrower: ui.NewButton(prefix+"narrower", "−"),
			wider: ui.NewButton(prefix+"wider", "+"), mark: ui.NewButton(prefix+"mark", ""),
			remove: ui.NewButton(prefix+"remove", "×"), width: ui.Zero(),
		})
	}
	s.rules = s.rules[:len(template.Rules)]
	rebuilt := len(s.guides) != len(template.Guides)
	for len(s.guides) < len(template.Guides) {
		prefix := "guide-" + strconv.Itoa(len(s.guides)+1) + "-"
		field := ui.NewTextField(prefix+"name", textedit.NewField("", false, nameBytes), false, s.theme)
		s.guides = append(s.guides, guideRow{
			name: field, line: ui.NewButton(prefix+"line", ""), from: ui.NewButton(prefix+"from", ""),
			less: ui.NewButton(prefix+"less", "−"), more: ui.NewButton(prefix+"more", "+"),
			remove: ui.NewButton(prefix+"remove", "×"), at: ui.Zero(),
		})
	}
	s.guides = s.guides[:len(template.Guides)]
	if rebuilt {
		current := s.focus.Current()
		s.focus = ui.Focus{}
		for _, row := range s.guides {
			s.focus.Register(row.name)
		}
		if current != nil && !slices.ContainsFunc(s.guides, func(row guideRow) bool { return row.name == current }) {
			current = nil
		}
		s.focus.Set(current)
	}
	for index, guide := range template.Guides {
		s.guides[index].name.Field.SetSaved(guide.Name)
	}
}

// edgeLabel, markLabel, lineLabel and fromLabel name a rule's and a guide's
// parts as the editor shows them.
func edgeLabel(edge artplugin.Edge) string {
	switch edge {
	case artplugin.EdgeSides:
		return "Sides"
	case artplugin.EdgeTop:
		return "Top"
	case artplugin.EdgeBottom:
		return "Bottom"
	case artplugin.EdgeBorder:
	}

	return "Border"
}

func markLabel(mark artplugin.Mark) string {
	if mark == artplugin.MarkNotAllowed {
		return "Not allowed"
	}

	return "Discouraged"
}

func lineLabel(line artplugin.Line) string {
	if line == artplugin.LineDown {
		return "Down"
	}

	return "Across"
}

func fromLabel(guide artplugin.Guide) string {
	switch {
	case guide.From == artplugin.FromMiddle:
		return "from the middle"
	case guide.From == artplugin.FromEnd && guide.Line == artplugin.LineDown:
		return "from the right"
	case guide.From == artplugin.FromEnd:
		return "from the bottom"
	case guide.Line == artplugin.LineDown:
		return "from the left"
	}

	return "from the top"
}

// next returns the item after one in a list, round to the first.
//
//nolint:ireturn // T is the caller-chosen concrete type.
func next[T comparable](items []T, item T) T {
	return items[(slices.Index(items, item)+1)%len(items)]
}

// layout places the rules and guides in the list at the left, with the save
// bar at its foot, and the preview at the right.
func (s *State) layout(template artplugin.Template, docs frame.Docs) {
	content := frame.Content(s.size)
	s.headings[0] = ui.Inset
	top := s.layoutRules(template, ui.Inset+ui.LineHeight+headGap)
	s.headings[1] = top
	s.layoutGuides(template, top+ui.LineHeight+headGap)
	barHeight := ui.SaveBarHeight(listWidth)
	s.save.Rect = image.Rect(ui.Inset, content.Max.Y-ui.Inset-barHeight, ui.Inset+listWidth, content.Max.Y-ui.Inset)
	right := content.Max.X - ui.Inset
	s.showOn.Rect = image.Rect(right-showWidth, ui.Inset, right, ui.Inset+ui.ButtonH)
	s.showOn.Label = "Nothing"
	if !s.shown.IsZero() {
		s.showOn.Label = name(docs, s.shown, "a sprite")
	}
	previewLeft := ui.Inset + listWidth + ui.Half*ui.Inset
	s.preview = image.Rect(previewLeft, s.showOn.Rect.Max.Y+ui.Inset, right, content.Max.Y-ui.Inset-ui.LineHeight)
}

// row places a row's widgets from the left, each a width wide, with a gap
// between.
type row struct {
	across, top int
}

func (r *row) place(button *ui.Button, width int) {
	button.Rect = image.Rect(r.across, r.top, r.across+width, r.top+ui.ButtonH)
	r.across += width + ui.Gap
}

func (r *row) skip(width int) image.Rectangle {
	rect := image.Rect(r.across, r.top, r.across+width, r.top+ui.ButtonH)
	r.across += width + ui.Gap

	return rect
}

// layoutRules places a row for each rule from a top, and + Rule below them.
// It returns the top of the guides' heading.
func (s *State) layoutRules(template artplugin.Template, top int) int {
	for index, rule := range template.Rules {
		widgets := &s.rules[index]
		line := row{across: ui.Inset, top: top}
		line.place(widgets.edge, edgeWidth)
		line.place(widgets.narrower, stepWidth)
		widgets.width = line.skip(valueWidth)
		line.place(widgets.wider, stepWidth)
		line.place(widgets.mark, markWidth)
		line.place(widgets.remove, stepWidth)
		widgets.edge.Label, widgets.mark.Label = edgeLabel(rule.Edge), markLabel(rule.Mark)
		widgets.narrower.Disabled = rule.Width <= 1
		widgets.wider.Disabled = rule.Width >= artplugin.MaxRuleWidth
		top += rowStep
	}
	s.addRule.Rect = image.Rect(ui.Inset, top, ui.Inset+addWidth, top+ui.ButtonH)
	s.addRule.Disabled = len(template.Rules) >= artplugin.MaxRules

	return top + rowStep + ui.Inset
}

// layoutGuides places a row for each guide from a top, and + Guide below
// them.
func (s *State) layoutGuides(template artplugin.Template, top int) {
	for index, guide := range template.Guides {
		widgets := &s.guides[index]
		line := row{across: ui.Inset, top: top}
		widgets.name.Rect = line.skip(nameWidth)
		line.place(widgets.line, lineWidth)
		line.place(widgets.from, fromWidth)
		line.place(widgets.less, stepWidth)
		widgets.at = line.skip(stepWidth)
		line.place(widgets.more, stepWidth)
		line.place(widgets.remove, stepWidth)
		widgets.line.Label, widgets.from.Label = lineLabel(guide.Line), fromLabel(guide)
		widgets.less.Disabled = guide.At <= -art.MaxTileSize
		widgets.more.Disabled = guide.At >= art.MaxTileSize
		top += rowStep
	}
	s.addGuide.Rect = image.Rect(ui.Inset, top, ui.Inset+addWidth, top+ui.ButtonH)
	s.addGuide.Disabled = len(template.Guides) >= artplugin.MaxGuides
}

// Update handles one frame of input.
func (s *State) Update(input *ui.Input, ctx *frame.Context) {
	s.size = ctx.Size
	template, ok := s.current(ctx.Docs)
	if !ok {
		return
	}
	s.syncRows(template)
	s.layout(template, ctx.Docs)
	s.save.Unsaved = s.Unsaved(ctx.Docs)
	if s.guard.Active() {
		s.guard.Update(input, s.size, func() bool { return s.Save(ctx) }, s.revert)

		return
	}
	if s.showMenu.IsOpen() {
		s.updateShowMenu(input, ctx)

		return
	}
	if field, outcome := s.focus.Update(input, ctx.Clipboard); field != nil && outcome == textedit.OutcomeCommit {
		s.Save(ctx)
	}
	frame.CommonKeys(input, ctx)
	switch s.save.Update(input) {
	case ui.SaveID:
		s.Save(ctx)
	case ui.RevertID:
		s.revert()
	}
	if s.showOn.Update(input) {
		s.openShowMenu(ctx.Docs)

		return
	}
	if changed, status, pressed := s.updateRows(input, template); pressed {
		s.apply(ctx, changed, status)
	}
}

// updateRows handles the buttons of the rules and guides, and + Rule and
// + Guide. It returns the changed template, and what to say once it is
// saved, when one was pressed.
func (s *State) updateRows(input *ui.Input, template artplugin.Template) (artplugin.Template, string, bool) {
	changed := template.Clone()
	switch {
	case s.addRule.Update(input):
		changed.Rules = append(changed.Rules, artplugin.Rule{
			Edge: artplugin.EdgeBorder, Width: 1, Mark: artplugin.MarkNotAllowed,
		})

		return changed, "Rule added", true
	case s.addGuide.Update(input):
		changed.Guides = append(changed.Guides, artplugin.Guide{
			Name: "Guide " + strconv.Itoa(len(changed.Guides)+1), Line: artplugin.LineAcross,
			From: artplugin.FromMiddle, At: 0,
		})

		return changed, "Guide added", true
	}
	for index := range changed.Rules {
		if status, pressed := s.updateRule(input, index, &changed); pressed {
			return changed, status, true
		}
	}
	for index := range changed.Guides {
		if status, pressed := s.updateGuide(input, index, &changed); pressed {
			return changed, status, true
		}
	}

	return changed, "", false
}

// updateRule handles one rule's buttons.
func (s *State) updateRule(input *ui.Input, index int, changed *artplugin.Template) (string, bool) {
	row, rule := s.rules[index], &changed.Rules[index]
	switch {
	case row.edge.Update(input):
		rule.Edge = next(artplugin.Edges(), rule.Edge)
	case row.narrower.Update(input):
		rule.Width--
	case row.wider.Update(input):
		rule.Width++
	case row.mark.Update(input):
		rule.Mark = next([]artplugin.Mark{artplugin.MarkNotAllowed, artplugin.MarkDiscouraged}, rule.Mark)
	case row.remove.Update(input):
		changed.Rules = slices.Delete(changed.Rules, index, index+1)

		return "Rule removed", true
	default:
		return "", false
	}

	return "", true
}

// updateGuide handles one guide's buttons.
func (s *State) updateGuide(input *ui.Input, index int, changed *artplugin.Template) (string, bool) {
	row, guide := s.guides[index], &changed.Guides[index]
	switch {
	case row.line.Update(input):
		guide.Line = next([]artplugin.Line{artplugin.LineAcross, artplugin.LineDown}, guide.Line)
	case row.from.Update(input):
		guide.From = next([]artplugin.From{artplugin.FromStart, artplugin.FromEnd, artplugin.FromMiddle}, guide.From)
	case row.less.Update(input):
		guide.At--
	case row.more.Update(input):
		guide.At++
	case row.remove.Update(input):
		changed.Guides = slices.Delete(changed.Guides, index, index+1)

		return "Guide removed", true
	default:
		return "", false
	}

	return "", true
}

// apply sends a changed template as one undo step.
func (s *State) apply(ctx *frame.Context, changed artplugin.Template, status string) {
	s.send(ctx, changed, func(result protocol.Result, err error) { ctx.Frame.Report(status)(result, err) })
}

// send sends a template as one undo step.
func (s *State) send(ctx *frame.Context, changed artplugin.Template, done func(protocol.Result, error)) {
	s.gestures++
	data, err := json.Marshal(artplugin.TemplateArgs{Template: changed})
	if err != nil {
		ctx.Frame.Status(err.Error())

		return
	}
	command := protocol.Command{
		Document: s.template, Edit: artplugin.TemplateEdit, Gesture: s.gesture(), Args: data,
	}
	ctx.Requests.Command(command, done)
}

// gesture names the window's latest change, so it undoes as one step.
func (s *State) gesture() string { return s.window.String() + "-" + strconv.Itoa(s.gestures) }

// Unsaved counts the guide names typed and not saved.
func (s *State) Unsaved(frame.Docs) int {
	count := 0
	for _, row := range s.guides {
		count += frame.UnsavedFields(row.name)
	}

	return count
}

// UnsavedName names the template.
func (s *State) UnsavedName(docs frame.Docs) string { return name(docs, s.template, "this template") }

// Leave asks about unsaved guide names before then, such as closing the
// window.
func (s *State) Leave(ctx *frame.Context, then func()) {
	s.guard.Ask(s.Unsaved(ctx.Docs), s.UnsavedName(ctx.Docs), then)
}

// revert drops every typed guide name.
func (s *State) revert() {
	s.focus.Set(nil)
	for _, row := range s.guides {
		row.name.Field.Revert()
	}
}

// Save saves the typed guide names, from the save bar, Enter or Cmd+S, as
// one undo step. A name is trimmed; an empty one is refused.
func (s *State) Save(ctx *frame.Context) bool {
	template, ok := s.current(ctx.Docs)
	if !ok || s.Unsaved(ctx.Docs) == 0 {
		return ok
	}
	changed := template.Clone()
	for index, row := range s.guides {
		if index >= len(changed.Guides) || !row.name.Field.Dirty() {
			continue
		}
		typed := strings.TrimSpace(row.name.Field.Value())
		if typed == "" {
			ctx.Frame.Status("A guide needs a name")

			return false
		}
		changed.Guides[index].Name = typed
	}
	if err := changed.Validate(); err != nil {
		ctx.Frame.Status("A guide's name is at most " + strconv.Itoa(artplugin.MaxGuideNameLength) + " characters")

		return false
	}
	committed := make([]*textedit.Field, 0, len(s.guides))
	for _, row := range s.guides {
		if _, dirty := row.name.Field.Commit(); dirty {
			committed = append(committed, row.name.Field)
		}
	}
	s.send(ctx, changed, func(result protocol.Result, err error) {
		if err != nil {
			for _, field := range committed {
				field.Failed()
			}
			ctx.Frame.Report("")(result, err)

			return
		}
		ctx.Frame.Status("Saved")
	})

	return true
}

// sprites lists the project's sprites, A–Z, as the index orders them.
func sprites(docs frame.Docs) []project.Entry {
	var entries []project.Entry
	docs.View(project.IndexID(), func(document client.Document) {
		if index, ok := document.Model.(*project.Index); ok {
			entries = index.OfFormat(art.SpriteFormatName)
		}
	})

	return entries
}

// openShowMenu offers nothing and every sprite, to show under the template.
func (s *State) openShowMenu(docs frame.Docs) {
	entries := sprites(docs)
	items := make([]ui.MenuItem, 0, 1+len(entries))
	items = append(items, ui.MenuItem{ID: showItem, Label: "Nothing", Icon: ui.IconNone, Disabled: false,
		Danger: false, Rule: false})
	for index, entry := range entries {
		items = append(items, ui.MenuItem{ID: showItem + entry.ID.String(), Label: entry.Name, Icon: ui.IconSprite,
			Disabled: false, Danger: false, Rule: index == 0})
	}
	window := image.Rectangle{Min: image.Pt(0, 0), Max: s.size}
	s.showMenu.Items = items
	s.showMenu.Width = min(ui.MenuWidth(items, s.theme), window.Dx()-ui.Half*ui.Inset)
	s.showMenu.OpenAt(image.Pt(s.showOn.Rect.Min.X, s.showOn.Rect.Max.Y+ui.Gap), window)
}

// updateShowMenu handles the open Show on menu, which takes all input.
func (s *State) updateShowMenu(input *ui.Input, ctx *frame.Context) {
	chosen := s.showMenu.Update(input)
	input.ClearAll()
	if !strings.HasPrefix(chosen, showItem) {
		return
	}
	s.shown = ids.ULID{}
	if value := strings.TrimPrefix(chosen, showItem); value != "" {
		parsed, err := ids.Parse(value)
		if err != nil {
			ctx.Frame.Status(err.Error())

			return
		}
		s.shown = parsed
	}
	s.syncShown(ctx)
}

// Draw renders the window.
func (s *State) Draw(screen *ebiten.Image, docs frame.Docs) {
	s.size = screen.Bounds().Size()
	screen.Fill(ui.ColorBackground())
	template, ok := s.current(docs)
	if !ok {
		s.theme.DrawText(screen, "Loading the template…", ui.Inset, ui.Inset, ui.ColorMuted())

		return
	}
	s.syncRows(template)
	s.layout(template, docs)
	s.drawList(screen, template)
	s.drawPreview(screen, docs, template)
	s.showMenu.Draw(screen, s.theme)
	s.guard.Draw(screen, s.theme)
}

// drawList draws the rules and guides, + Rule and + Guide, and the save bar.
func (s *State) drawList(screen *ebiten.Image, template artplugin.Template) {
	s.theme.Heading(screen, "RULES · MEASURED FROM THE TILE'S EDGES", ui.Inset, s.headings[0])
	for index, rule := range template.Rules {
		row := s.rules[index]
		for _, button := range []*ui.Button{row.edge, row.narrower, row.wider, row.mark, row.remove} {
			button.Draw(screen, s.theme)
		}
		s.drawValue(screen, strconv.Itoa(rule.Width)+" px", row.width)
	}
	if len(template.Rules) == 0 {
		s.theme.DrawText(screen, "No rules: every pixel is open.", s.addRule.Rect.Max.X+ui.Inset,
			s.addRule.Rect.Min.Y+headGap, ui.ColorMuted())
	}
	s.addRule.Draw(screen, s.theme)
	s.theme.Heading(screen, "GUIDES · LINES TO DRAW TO, NEVER PART OF THE ART", ui.Inset, s.headings[1])
	for index, guide := range template.Guides {
		row := s.guides[index]
		row.name.Draw(screen)
		for _, button := range []*ui.Button{row.line, row.from, row.less, row.more, row.remove} {
			button.Draw(screen, s.theme)
		}
		s.drawValue(screen, strconv.Itoa(guide.At), row.at)
	}
	s.addGuide.Draw(screen, s.theme)
	s.save.Draw(screen, s.theme)
}

// drawValue writes a number centred in its place between two steps.
func (s *State) drawValue(screen *ebiten.Image, value string, rect image.Rectangle) {
	s.theme.DrawMono(screen, value, rect.Min.X+(rect.Dx()-s.theme.MonoWidth(value))/ui.Half,
		rect.Min.Y+(rect.Dy()-ui.MonoSize)/ui.Half-1, ui.ColorInk())
}

// drawPreview draws the template over a tile of the project's size,
// magnified to fit, over the shown sprite if any, with its guides.
func (s *State) drawPreview(screen *ebiten.Image, docs frame.Docs, template artplugin.Template) {
	s.theme.Heading(screen, "PREVIEW · SHOWN ON", s.preview.Min.X, s.showOn.Rect.Min.Y+headGap)
	s.showOn.Draw(screen, s.theme)
	ui.FillRect(screen, s.preview, ui.ColorCanvas())
	size := tileSize(docs)
	zoom := max(1, min(maxZoom, (min(s.preview.Dx(), s.preview.Dy())-fitMargin*ui.Half)/size))
	side := size * zoom
	corner := ui.Center(s.preview).Sub(image.Pt(side/ui.Half, side/ui.Half))
	rect := image.Rectangle{Min: corner, Max: corner.Add(image.Pt(side, side))}
	area, clipped := screen.SubImage(s.preview).(*ebiten.Image)
	if !clipped {
		return
	}
	ui.FillRect(area, rect, ui.ColorBackground())
	if tileID, ok := shownTile(docs, s.shown); ok && s.picture.Refresh(docs, tileID) && s.picture.Size() == size {
		var magnify ebiten.DrawImageOptions
		magnify.GeoM.Scale(float64(zoom), float64(zoom))
		magnify.GeoM.Translate(float64(rect.Min.X), float64(rect.Min.Y))
		area.DrawImage(s.picture.Image(), &magnify)
	}
	if zoom >= minGridZoom {
		ink := ui.HexAlpha(0, gridAlpha)
		for line := 1; line < size; line++ {
			x, y := rect.Min.X+line*zoom, rect.Min.Y+line*zoom
			ui.Line(area, image.Pt(x, rect.Min.Y), image.Pt(x, rect.Max.Y), ui.Hairline, ink)
			ui.Line(area, image.Pt(rect.Min.X, y), image.Pt(rect.Max.X, y), ui.Hairline, ink)
		}
	}
	tileart.DrawMarks(area, template, rect, size, zoom)
	tileart.DrawGuides(area, s.theme, template, rect, size, zoom)
	ui.StrokeRect(area, rect, ui.Hairline, ui.ColorRule())
	caption := strconv.Itoa(size) + " × " + strconv.Itoa(size) + " pixels · where rules overlap, the strictest wins"
	s.theme.DrawText(screen, caption, s.preview.Min.X, s.preview.Max.Y+ui.Gap, ui.ColorMuted())
}

// Locate finds a named part of the window for input scripts: "ui:<id>" for
// a button, a guide's name or the save bar, "menu:<item>" in the open Show on
// menu, and "state:…" for scripts waiting for the window: "loaded",
// "rules:<count>", "guides:<count>" and "shown:<sprite>" ("shown:" for
// nothing).
func (s *State) Locate(locate string, docs frame.Docs) (image.Point, bool) {
	kind, value, _ := strings.Cut(locate, ":")
	if template, ok := s.current(docs); ok {
		s.syncRows(template)
		s.layout(template, docs)
	}
	switch kind {
	case "menu":
		return s.showMenu.Locate(value)
	case "state":
		return image.Pt(0, 0), s.hasState(value, docs)
	case "ui":
		if point, ok := s.save.Locate(value); ok {
			return point, true
		}
		if point, ok := s.guard.Locate(value, s.size); ok {
			return point, true
		}

		return s.locateWidget(value)
	}

	return image.Pt(0, 0), false
}

func (s *State) locateWidget(widget string) (image.Point, bool) {
	buttons := []*ui.Button{s.addRule, s.addGuide, s.showOn}
	for _, row := range s.rules {
		buttons = append(buttons, row.edge, row.narrower, row.wider, row.mark, row.remove)
	}
	for _, row := range s.guides {
		if row.name.ID == widget {
			return row.name.Center(), true
		}
		buttons = append(buttons, row.line, row.from, row.less, row.more, row.remove)
	}
	for _, button := range buttons {
		if button.ID == widget && !button.Rect.Empty() {
			return ui.Center(button.Rect), true
		}
	}

	return image.Pt(0, 0), false
}

func (s *State) hasState(state string, docs frame.Docs) bool {
	kind, value, _ := strings.Cut(state, ":")
	template, loaded := s.current(docs)
	switch kind {
	case "loaded":
		return loaded
	case "rules":
		return loaded && strconv.Itoa(len(template.Rules)) == value
	case "guides":
		return loaded && strconv.Itoa(len(template.Guides)) == value
	case "shown":
		return (s.shown.IsZero() && value == "") || (!s.shown.IsZero() && name(docs, s.shown, "") == value)
	}

	return false
}
