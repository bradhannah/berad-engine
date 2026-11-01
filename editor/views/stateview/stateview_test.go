package stateview_test

import (
	"image"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/client"
	stateplugin "github.com/bradhannah/berad-engine/editor/plugins/worldstate"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
	"github.com/bradhannah/berad-engine/editor/views/stateview"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

var (
	moodID   = frametest.ID(40)
	robbedID = frametest.ID(41)
)

type fixture struct {
	t        *testing.T
	state    *stateview.State
	docs     *frametest.Docs
	requests *frametest.Requests
	ctx      *frame.Context
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	theme, err := ui.NewTheme()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(theme.Close)
	document := stateplugin.NewDocument()
	mood := worldstate.New(moodID, "mood", worldstate.ScopeNPC, worldstate.TypeChoice)
	mood.Choices = []worldstate.Choice{{ID: "c1", Name: "calm"}, {ID: "c2", Name: "cross"}}
	robbed := worldstate.New(robbedID, "shop_robbed", worldstate.ScopeWorld, worldstate.TypeYesNo)
	for _, value := range []worldstate.Declaration{mood, robbed} {
		if err := document.State.Values.Insert(value); err != nil {
			t.Fatal(err)
		}
	}
	docs, requests := frametest.NewDocs(), &frametest.Requests{}
	docs.Put(worldstate.ID(), worldstate.Name, document)
	ctx := frametest.Context(docs, requests, image.Pt(1200, 800))
	f := &fixture{t: t, state: stateview.New(frametest.ID(99), worldstate.ID(), theme), docs: docs,
		requests: requests, ctx: ctx}
	f.frame(ui.NewInput(image.Pt(0, 0)))

	return f
}

func (f *fixture) frame(input ui.Input) {
	f.state.Update(&input, f.ctx)
	f.state.Draw(ebiten.NewImage(1200, 800), f.ctx.Docs)
}

func (f *fixture) at(name string) image.Point {
	f.t.Helper()
	point, ok := f.state.Locate(name, f.ctx.Docs)
	if !ok {
		f.t.Fatalf("cannot locate %s", name)
	}

	return point
}

func (f *fixture) click(name string) {
	f.t.Helper()
	at := f.at(name)
	input := ui.NewInput(at)
	input.Pressed, input.Down = true, true
	f.frame(input)
	input = ui.NewInput(at)
	input.Released = true
	f.frame(input)
}

func (f *fixture) has(state string) bool {
	_, ok := f.state.Locate("state:"+state, f.ctx.Docs)

	return ok
}

func (f *fixture) typeText(text string) {
	input := ui.NewInput(image.Pt(0, 0))
	input.Text = text
	f.frame(input)
}

func (f *fixture) keys(names ...string) {
	input := ui.NewInput(image.Pt(0, 0))
	for _, name := range names {
		input.Presses = append(input.Presses, ui.ParsePress(name))
	}
	f.frame(input)
}

func (f *fixture) status() string {
	message, _ := f.ctx.Frame.Message()

	return message
}

// lastSet decodes the last change sent to a value.
func (f *fixture) lastSet() worldstate.Declaration {
	f.t.Helper()
	sent := f.requests.Of(stateplugin.SetValueEdit)
	if len(sent) == 0 {
		f.t.Fatal("nothing was changed")
	}
	var args stateplugin.ValueArgs
	if err := sent[len(sent)-1].Decode(&args); err != nil {
		f.t.Fatal(err)
	}

	return args.Value
}

// TestDeclaringValues lists each scope's values, adds one and selects it,
// renames it, changes its type and default, and deletes it.
func TestDeclaringValues(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	if !f.has("loaded") {
		t.Fatal("the world state is not loaded")
	}
	f.click("row:world.shop_robbed")
	if !f.has("selected=world.shop_robbed") {
		t.Fatal("a world value is listed under World")
	}
	f.click("scope:npc")
	f.click("row:npc.mood")
	f.click("ui:add-value")
	added := f.requests.Of(stateplugin.AddValueEdit)
	var args stateplugin.ValueArgs
	if len(added) != 1 || added[0].Decode(&args) != nil || args.Value.Path() != "npc.value" ||
		args.Value.Type != worldstate.TypeNumber {
		t.Fatalf("add %d %+v", len(added), args)
	}
	made := args.Value
	made.ID = frametest.ID(42)
	document := stateplugin.NewDocument()
	for _, id := range []byte{40, 41} {
		var found worldstate.Declaration
		f.docs.View(worldstate.ID(), func(each client.Document) {
			found, _ = each.Model.(*stateplugin.Document).State.Values.Value(frametest.ID(id))
		})
		if err := document.State.Values.Insert(found); err != nil {
			t.Fatal(err)
		}
	}
	if err := document.State.Values.Insert(made); err != nil {
		t.Fatal(err)
	}
	f.docs.Put(worldstate.ID(), worldstate.Name, document)
	added[0].Succeed(made.ID)
	f.frame(ui.NewInput(image.Pt(0, 0)))
	if !f.has("selected=npc.value") {
		t.Fatal("the new value is selected")
	}
	f.click("ui:value-name")
	f.keys("Cmd+A", "Backspace")
	f.typeText("george's")
	f.keys("Enter")
	if len(f.requests.Of(stateplugin.SetValueEdit)) != 0 || !strings.Contains(f.status(), "lower-case") {
		t.Fatalf("punctuation is refused, saying why: %q", f.status())
	}
	f.keys("Cmd+A", "Backspace")
	f.typeText("gold_owed")
	f.keys("Enter")
	if renamed := f.lastSet(); renamed.Path() != "npc.gold_owed" || renamed.ID != made.ID {
		t.Fatalf("renamed %+v", renamed)
	}
	f.click("ui:value-type-yes-no")
	if retyped := f.lastSet(); retyped.Type != worldstate.TypeYesNo || string(retyped.Default) != "false" {
		t.Fatalf("retyped %+v", retyped)
	}
	f.click("ui:value-delete")
	removed := f.requests.Of(stateplugin.RemoveValueEdit)
	var removal stateplugin.RemoveValueArgs
	if len(removed) != 1 || removed[0].Decode(&removal) != nil || removal.ID != made.ID {
		t.Fatalf("delete %d %+v", len(removed), removal)
	}
}

// TestChoices adds a choice and makes a choice the default.
func TestChoices(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.click("scope:npc")
	f.click("row:npc.mood")
	f.click("ui:value-add-choice")
	if added := f.lastSet(); len(added.Choices) != 3 || added.Choices[2].ID != "c3" {
		t.Fatalf("add choice %+v", added.Choices)
	}
	f.click("ui:value-default-c2")
	if chosen := f.lastSet(); string(chosen.Default) != `"c2"` {
		t.Fatalf("default %s", chosen.Default)
	}
	f.click("ui:value-choice-1-remove")
	if removed := f.lastSet(); len(removed.Choices) != 1 || removed.Choices[0].ID != "c2" ||
		string(removed.Default) != `"c2"` {
		t.Fatalf("remove choice %+v %s", removed.Choices, removed.Default)
	}
}
