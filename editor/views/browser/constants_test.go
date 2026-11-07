package browser_test

import (
	"encoding/json"
	"image"
	"testing"

	"github.com/bradhannah/berad-engine/editor/client"
	npcplugin "github.com/bradhannah/berad-engine/editor/plugins/npc"
	stateplugin "github.com/bradhannah/berad-engine/editor/plugins/worldstate"
	zoneplugin "github.com/bradhannah/berad-engine/editor/plugins/zone"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

var (
	shopTemplateID = frametest.ID(80)
	greetingID     = frametest.ID(81)
	serveID        = frametest.ID(82)
)

// withShop gives the project a shop template holding a greeting and how
// long a shopkeeper serves, and puts it on Ada Clerk.
func (f *fixture) withShop() {
	f.t.Helper()
	state := stateplugin.NewDocument()
	greeting := worldstate.NewConstant(greetingID, "greeting", worldstate.ScopeNPC, worldstate.TypeText)
	greeting.Parent, greeting.Default = shopTemplateID, json.RawMessage(`"Welcome in!"`)
	serve := worldstate.NewConstant(serveID, "serve_minutes", worldstate.ScopeNPC, worldstate.TypeInteger)
	serve.Parent, serve.Default = shopTemplateID, json.RawMessage(`3`)
	if err := state.State.Values.InsertAll([]worldstate.Declaration{
		worldstate.New(shopTemplateID, "shop", worldstate.ScopeNPC, worldstate.TypeTemplate), greeting, serve,
	}); err != nil {
		f.t.Fatal(err)
	}
	f.docs().Put(worldstate.ID(), worldstate.Name, state)
	f.docs().View(clerkID, func(document client.Document) {
		if err := document.Model.(*npcplugin.Document).NPC.Owned.Apply(shopTemplateID); err != nil {
			f.t.Fatal(err)
		}
	})
	f.docs().Touch(clerkID)
	f.frame(ui.NewInput(image.Pt(0, 0)))
}

// lastCommand returns the last command sent and its arguments.
func (f *fixture) lastCommand() (string, stateplugin.ConstantArgs) {
	f.t.Helper()
	if len(f.requests.Commands) == 0 {
		f.t.Fatal("no command sent")
	}
	command := f.requests.Commands[len(f.requests.Commands)-1].Command
	var args stateplugin.ConstantArgs
	if err := json.Unmarshal(command.Args, &args); err != nil {
		f.t.Fatal(err)
	}

	return command.Edit, args
}

// TestAnNPCsConstants lists the constants of the shop template on Ada's
// inspector with the defaults, writes her own greeting in the writing
// sheet, types her own serving time, and gives the greeting back its
// default.
func TestAnNPCsConstants(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.withShop()
	f.click("row:Ada Clerk", false)
	if !f.has(`constant:npc.shop.greeting="Welcome in!"`) || !f.has("constant:npc.shop.serve_minutes=3") {
		t.Fatal("her inspector lists the shop's constants with their defaults")
	}
	f.click("ui:constant-write-npc.shop.greeting", false)
	if !f.has("sheet") || !f.has("sheet-holder=Ada Clerk's own") {
		t.Fatal("Write… opens the sheet on her own greeting")
	}
	f.press("Cmd+A")
	f.press("Backspace")
	f.typeText("Fresh bread, [i]still warm[/i]!")
	f.click("ui:sheet-done", false)
	edit, args := f.lastCommand()
	if f.has("sheet") || edit != npcplugin.SetConstantEdit || args.Constant != greetingID ||
		string(args.Value) != `"Fresh bread, [i]still warm[/i]!"` {
		t.Fatalf("Done saves her own greeting: %s %+v", edit, args)
	}
	f.click("ui:constant-number-npc.shop.serve_minutes", false)
	f.press("Cmd+A")
	f.typeText("5")
	f.press("Enter")
	if edit, args = f.lastCommand(); edit != npcplugin.SetConstantEdit || args.Constant != serveID ||
		string(args.Value) != "5" {
		t.Fatalf("Enter saves her own serving time: %s %+v", edit, args)
	}
	f.docs().View(clerkID, func(document client.Document) {
		document.Model.(*npcplugin.Document).NPC.Owned.SetConstant(greetingID, json.RawMessage(`"Fresh bread!"`))
	})
	f.docs().Touch(clerkID)
	f.frame(ui.NewInput(image.Pt(0, 0)))
	f.click("ui:constant-default-npc.shop.greeting", false)
	if edit, args = f.lastCommand(); edit != npcplugin.SetConstantEdit || args.Constant != greetingID ||
		len(args.Value) != 0 {
		t.Fatalf("Default clears her own greeting: %s %+v", edit, args)
	}
}

// TestAZonesConstants lists the constants of the signs template on Town's
// inspector, its own value first.
func TestAZonesConstants(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.withShop()
	signs, welcome := frametest.ID(83), frametest.ID(84)
	f.docs().View(worldstate.ID(), func(document client.Document) {
		said := worldstate.NewConstant(welcome, "welcome", worldstate.ScopeZone, worldstate.TypeText)
		said.Parent, said.Default = signs, json.RawMessage(`"Welcome, traveller."`)
		if err := document.Model.(*stateplugin.Document).State.Values.InsertAll([]worldstate.Declaration{
			worldstate.New(signs, "signs", worldstate.ScopeZone, worldstate.TypeTemplate), said,
		}); err != nil {
			t.Fatal(err)
		}
	})
	f.docs().View(zoneID, func(document client.Document) {
		owned := &document.Model.(*zoneplugin.Document).Zone.Owned
		if err := owned.Apply(signs); err != nil {
			t.Fatal(err)
		}
		owned.SetConstant(welcome, json.RawMessage(`"Saltmere Harbour. Mind the nets."`))
	})
	f.docs().Touch(worldstate.ID())
	f.docs().Touch(zoneID)
	f.click("row:Town", false)
	if !f.has(`constant:zone.signs.welcome="Saltmere Harbour. Mind the nets."`) {
		t.Fatal("a zone's inspector lists its templates' constants, with its own value")
	}
}
