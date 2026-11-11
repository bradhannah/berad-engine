package templateview_test

import (
	"image"
	"slices"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/client"
	artplugin "github.com/bradhannah/berad-engine/editor/plugins/art"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
	"github.com/bradhannah/berad-engine/editor/views/templateview"
	"github.com/bradhannah/berad-engine/engine/art"
	"github.com/bradhannah/berad-engine/engine/ids"
)

var (
	templateID = frametest.ID(1)
	spriteID   = frametest.ID(2)
	tileID     = frametest.ID(3)
)

type fixture struct {
	t        *testing.T
	state    *templateview.State
	docs     *frametest.Docs
	requests *frametest.Requests
	ctx      *frame.Context
	applied  int
	screen   *ebiten.Image
}

// newFixture opens the template Pickup, a border kept clear and one guide,
// in a project with the sprite Pouch and 8-pixel tiles.
func newFixture(t *testing.T) *fixture {
	t.Helper()
	theme, err := ui.NewTheme()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(theme.Close)
	tile, err := art.NewTile(8)
	if err != nil {
		t.Fatal(err)
	}
	docs := frametest.NewDocs()
	docs.Put(project.IndexID(), "Project", project.NewIndex([]project.Entry{
		{ID: templateID, Name: "Pickup", Format: artplugin.TemplateFormatName},
		{ID: spriteID, Name: "Pouch", Format: art.SpriteFormatName},
		{ID: tileID, Name: "Tile 1", Format: art.TileFormatName},
	}))
	docs.Put(art.SettingsID(), art.SettingsName, &artplugin.SettingsDocument{Settings: art.Settings{TileSize: 8}})
	docs.Put(templateID, "Pickup", &artplugin.TemplateDocument{Template: artplugin.Template{
		Rules:  []artplugin.Rule{{Edge: artplugin.EdgeBorder, Width: 1, Mark: artplugin.MarkNotAllowed}},
		Guides: []artplugin.Guide{{Name: "centre", Line: artplugin.LineDown, From: artplugin.FromMiddle, At: 0}},
	}})
	docs.Put(tileID, "Tile 1", &artplugin.TileDocument{Tile: tile})
	docs.Put(spriteID, "Pouch", &artplugin.SpriteDocument{Sprite: art.Sprite{Tile: tileID, Template: templateID}})
	requests := &frametest.Requests{}
	ctx := frametest.Context(docs, requests, image.Pt(1200, 800))
	state := templateview.New(frametest.ID(99), templateID, theme)
	state.Start(ctx)
	f := &fixture{t: t, state: state, docs: docs, requests: requests, ctx: ctx, screen: ebiten.NewImage(1200, 800)}
	f.frame(ui.NewInput(image.Pt(0, 0)))

	return f
}

func (f *fixture) frame(input ui.Input) {
	f.state.Update(&input, f.ctx)
	f.state.Draw(f.screen, f.docs)
}

func (f *fixture) at(name string) image.Point {
	f.t.Helper()
	point, ok := f.state.Locate(name, f.docs)
	if !ok {
		f.t.Fatalf("cannot locate %s", name)
	}

	return point
}

func (f *fixture) click(name string) {
	f.t.Helper()
	point := f.at(name)
	press := ui.NewInput(point)
	press.Pressed, press.Down = true, true
	f.frame(press)
	release := ui.NewInput(point)
	release.Released = true
	f.frame(release)
}

func (f *fixture) has(state string) bool {
	_, ok := f.state.Locate("state:"+state, f.docs)

	return ok
}

func (f *fixture) status() string {
	message, _ := f.ctx.Frame.Message()

	return message
}

// applyAll applies every command not yet applied, as the orchestrator would.
func (f *fixture) applyAll() {
	f.t.Helper()
	for ; f.applied < len(f.requests.Commands); f.applied++ {
		command := f.requests.Commands[f.applied]
		if command.Command.Edit != artplugin.TemplateEdit {
			f.t.Fatalf("sent %s", command.Command.Edit)
		}
		var err error
		f.docs.View(command.Command.Document, func(document client.Document) {
			_, err = artplugin.SetTemplate{}.Apply(document.Model, command.Command.Args)
		})
		if err != nil {
			f.t.Fatal(err)
		}
		f.docs.Touch(command.Command.Document)
		f.state.Changed(client.Update{Event: &protocol.Event{Document: command.Command.Document}}, f.ctx)
		command.Succeed(ids.ULID{})
	}
	f.frame(ui.NewInput(image.Pt(0, 0)))
}

func (f *fixture) template() artplugin.Template {
	f.t.Helper()
	var template artplugin.Template
	f.docs.View(templateID, func(document client.Document) {
		template = document.Model.(*artplugin.TemplateDocument).Template
	})

	return template
}

func TestOpensTheTemplate(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	if title := f.state.Title(f.docs); !strings.HasPrefix(title, "Template: Pickup  (view ") {
		t.Fatalf("title %q", title)
	}
	if !f.has("loaded") || !f.has("rules:1") || !f.has("guides:1") || !f.has("shown:") {
		t.Fatal("the template's rule and guide are listed, over no sprite")
	}
	if !slices.Contains(f.requests.Subscribed, art.SettingsID()) ||
		!slices.Contains(f.requests.Subscribed, project.IndexID()) {
		t.Fatal("the preview follows the project's tile size, and the index names the template")
	}
}

func TestButtonsChangeTheRulesAsOneStepEach(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	for _, step := range []string{
		"ui:add-rule", "ui:rule-2-edge", "ui:rule-2-wider", "ui:rule-2-mark", "ui:rule-1-edge", "ui:rule-1-remove",
	} {
		f.click(step)
		f.applyAll()
	}
	want := []artplugin.Rule{{Edge: artplugin.EdgeSides, Width: 2, Mark: artplugin.MarkDiscouraged}}
	if got := f.template().Rules; !slices.Equal(got, want) {
		t.Fatalf("rules %+v", got)
	}
	gestures := map[string]bool{}
	for _, command := range f.requests.Commands {
		gestures[command.Command.Gesture] = true
	}
	if len(gestures) != 6 || f.status() != "Rule removed" {
		t.Fatalf("each button is its own undo step: %d, %q", len(gestures), f.status())
	}
	f.click("ui:rule-1-narrower")
	f.applyAll()
	if f.template().Rules[0].Width != 1 {
		t.Fatal("narrower")
	}
	before := len(f.requests.Commands)
	f.click("ui:rule-1-narrower")
	if len(f.requests.Commands) != before {
		t.Fatal("a rule is at least a pixel wide")
	}
}

func TestGuidesMoveAndAreNamed(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	for _, step := range []string{"ui:add-guide", "ui:guide-2-from", "ui:guide-2-more", "ui:guide-2-more", "ui:guide-1-line"} {
		f.click(step)
		f.applyAll()
	}
	guides := f.template().Guides
	if len(guides) != 2 || guides[1] != (artplugin.Guide{
		Name: "Guide 2", Line: artplugin.LineAcross, From: artplugin.FromStart, At: 2,
	}) || guides[0].Line != artplugin.LineAcross {
		t.Fatalf("guides %+v", guides)
	}
	f.click("ui:guide-2-name")
	typing := ui.NewInput(f.at("ui:guide-2-name"))
	typing.Presses = append(typing.Presses, ui.ParsePress("Cmd+A"))
	f.frame(typing)
	typing = ui.NewInput(f.at("ui:guide-2-name"))
	typing.Text = " feet "
	f.frame(typing)
	if f.state.Unsaved(f.docs) != 1 || len(f.requests.Commands) != f.applied {
		t.Fatal("a typed name waits for Save")
	}
	enter := ui.NewInput(f.at("ui:guide-2-name"))
	enter.Presses = append(enter.Presses, ui.ParsePress("Enter"))
	f.frame(enter)
	f.applyAll()
	if f.template().Guides[1].Name != "feet" || f.state.Unsaved(f.docs) != 0 {
		t.Fatalf("Enter saves the trimmed name: %+v", f.template().Guides[1])
	}
	f.click("ui:guide-1-name")
	clear := ui.NewInput(f.at("ui:guide-1-name"))
	clear.Presses = append(clear.Presses, ui.ParsePress("Cmd+A"), ui.ParsePress("Backspace"))
	f.frame(clear)
	before := len(f.requests.Commands)
	f.click("ui:save")
	if len(f.requests.Commands) != before || f.status() != "A guide needs a name" {
		t.Fatalf("an empty name is refused: %q", f.status())
	}
	f.click("ui:revert")
	if f.state.Unsaved(f.docs) != 0 {
		t.Fatal("Revert drops the typed name")
	}
	f.click("ui:guide-1-remove")
	f.applyAll()
	if !f.has("guides:1") || f.template().Guides[0].Name != "feet" {
		t.Fatal("removing the first guide keeps the second")
	}
}

func TestPreviewShowsASprite(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.click("ui:show-on")
	f.click("menu:sprite:" + spriteID.String())
	if !f.has("shown:Pouch") {
		t.Fatal("the preview shows Pouch")
	}
	for _, want := range []ids.ULID{spriteID, tileID, art.DefaultPaletteID()} {
		if !slices.Contains(f.requests.Subscribed, want) {
			t.Fatalf("subscribed to %v, not %v", f.requests.Subscribed, want)
		}
	}
	f.click("ui:show-on")
	f.click("menu:sprite:")
	if !f.has("shown:") {
		t.Fatal("the preview shows nothing again")
	}
}

// TestAnUnsavedGuideNameStaysWithItsGuide types a name for the second of
// three guides, then removes the first: the typed name moves up with its
// guide, and Save names that guide, not the one now in the second row.
func TestAnUnsavedGuideNameStaysWithItsGuide(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	for _, step := range []string{"ui:add-guide", "ui:add-guide"} {
		f.click(step)
		f.applyAll()
	}
	f.click("ui:guide-2-name")
	typing := ui.NewInput(f.at("ui:guide-2-name"))
	typing.Presses = append(typing.Presses, ui.ParsePress("Cmd+A"))
	f.frame(typing)
	typing = ui.NewInput(f.at("ui:guide-2-name"))
	typing.Text = "feet"
	f.frame(typing)
	f.click("ui:guide-1-remove")
	f.applyAll()
	f.frame(ui.NewInput(image.Pt(0, 0)))
	if !f.has("guides:2") || f.state.Unsaved(f.docs) != 1 {
		t.Fatalf("the first guide is gone and the typed name still waits: %d unsaved", f.state.Unsaved(f.docs))
	}
	f.click("ui:save")
	f.applyAll()
	guides := f.template().Guides
	if len(guides) != 2 || guides[0].Name != "feet" || guides[1].Name != "Guide 3" {
		t.Fatalf("the typed name went to the guide it was typed for: %+v", guides)
	}
}
