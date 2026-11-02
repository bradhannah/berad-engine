package stateview_test

import (
	"image"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/model"
	"github.com/bradhannah/berad-engine/editor/plugin"
	npcplugin "github.com/bradhannah/berad-engine/editor/plugins/npc"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	stateplugin "github.com/bradhannah/berad-engine/editor/plugins/worldstate"
	zoneplugin "github.com/bradhannah/berad-engine/editor/plugins/zone"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
	"github.com/bradhannah/berad-engine/editor/views/stateview"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/npc"
	"github.com/bradhannah/berad-engine/engine/worldstate"
	"github.com/bradhannah/berad-engine/engine/zone"
)

var (
	questsID  = frametest.ID(40)
	slainID   = frametest.ID(41)
	guardsID  = frametest.ID(42)
	arrestsID = frametest.ID(43)
	secretID  = frametest.ID(44)
	moodID    = frametest.ID(45)
	georgeID  = frametest.ID(50)
	guardID   = frametest.ID(51)
	marketID  = frametest.ID(52)
)

type fixture struct {
	t        *testing.T
	state    *stateview.State
	docs     *frametest.Docs
	requests *frametest.Requests
	ctx      *frame.Context
	registry *plugin.Registry
	applied  int
	nextID   byte
}

// in puts a declaration inside a group or template.
func in(declared worldstate.Declaration, parent ids.ULID) worldstate.Declaration {
	declared.Parent = parent

	return declared
}

// newFixture is a project whose world has quests.dragon_slain and a mood
// to choose, with a guards template; George owns a secret, Guard 1 has
// the guards template, and the market is a zone.
func newFixture(t *testing.T) *fixture {
	t.Helper()
	theme, err := ui.NewTheme()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(theme.Close)
	registry, err := plugin.NewRegistry(stateplugin.Plugin(), npcplugin.Plugin(), zoneplugin.Plugin())
	if err != nil {
		t.Fatal(err)
	}
	document := stateplugin.NewDocument()
	mood := worldstate.New(moodID, "mood", worldstate.ScopeWorld, worldstate.TypeChoice)
	mood.Choices = []worldstate.Choice{{ID: "c1", Name: "calm"}, {ID: "c2", Name: "cross"}}
	if err := document.State.Values.InsertAll([]worldstate.Declaration{
		worldstate.New(questsID, "quests", worldstate.ScopeWorld, worldstate.TypeGroup),
		in(worldstate.New(slainID, "dragon_slain", worldstate.ScopeWorld, worldstate.TypeBool), questsID), mood,
		worldstate.New(guardsID, "guards", worldstate.ScopeNPC, worldstate.TypeTemplate),
		in(worldstate.New(arrestsID, "arrests", worldstate.ScopeNPC, worldstate.TypeInteger), guardsID),
	}); err != nil {
		t.Fatal(err)
	}
	george, guard, market := npcplugin.NewDocument(), npcplugin.NewDocument(), zoneplugin.NewDocument()
	george.NPC.Path, guard.NPC.Path, market.Zone.Path = "george", "guard1", "market"
	if george.NPC.Owned.Values.Insert(worldstate.New(secretID, "secret", worldstate.ScopeNPC, worldstate.TypeBool)) != nil ||
		guard.NPC.Owned.Apply(guardsID) != nil {
		t.Fatal("owners")
	}
	docs, requests := frametest.NewDocs(), &frametest.Requests{}
	docs.Put(worldstate.ID(), worldstate.Name, document)
	docs.Put(georgeID, "George", george)
	docs.Put(guardID, "Guard 1", guard)
	docs.Put(marketID, "Market", market)
	docs.Put(project.IndexID(), "Project", project.NewIndex([]project.Entry{
		{ID: georgeID, Name: "George", Format: npc.FormatName}, {ID: guardID, Name: "Guard 1", Format: npc.FormatName},
		{ID: marketID, Name: "Market", Format: zone.FormatName},
	}))
	ctx := frametest.Context(docs, requests, image.Pt(1400, 900))
	f := &fixture{t: t, state: stateview.New(frametest.ID(99), worldstate.ID(), theme), docs: docs,
		requests: requests, ctx: ctx, registry: registry, applied: 0, nextID: 60}
	f.state.Start(ctx)
	f.frame(ui.NewInput(image.Pt(0, 0)))

	return f
}

// frame runs one frame and draws it, then carries out every new command as
// the orchestrator would: prepared, applied and answered.
func (f *fixture) frame(input ui.Input) {
	f.t.Helper()
	f.state.Update(&input, f.ctx)
	f.state.Draw(ebiten.NewImage(1400, 900), f.ctx.Docs)
	for f.applied < len(f.requests.Commands) {
		command := f.requests.Commands[f.applied]
		f.applied++
		f.carryOut(command)
	}
}

func (f *fixture) carryOut(command frametest.Command) {
	f.t.Helper()
	kind, ok := f.registry.Edit(command.Command.Edit)
	if !ok {
		f.t.Fatalf("unknown edit %s", command.Command.Edit)
	}
	args := command.Command.Args
	var created ids.ULID
	var failed error
	f.docs.View(command.Command.Document, func(document client.Document) {
		if preparer, prepares := kind.(model.Preparer); prepares {
			prepared, err := preparer.Prepare(document.Model, args, func() (ids.ULID, error) {
				created = frametest.ID(f.nextID)
				f.nextID++

				return created, nil
			})
			if err != nil {
				f.t.Fatal(err)
			}
			args = prepared
		}
		_, failed = kind.Apply(document.Model, args)
	})
	f.docs.Touch(command.Command.Document)
	if failed != nil {
		command.Done(protocol.Result{OK: false, Reason: failed.Error(), Revision: 0, Created: ids.ULID{}, Item: ""}, failed)

		return
	}
	command.Succeed(created)
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

func (f *fixture) typeInto(field, text string) {
	f.t.Helper()
	f.click("ui:" + field)
	f.keys("Cmd+A", "Backspace")
	input := ui.NewInput(image.Pt(0, 0))
	input.Text = text
	f.frame(input)
	f.keys("Enter")
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

// world reads the world state's declarations.
func (f *fixture) world() *worldstate.Declarations {
	var values *worldstate.Declarations
	f.docs.View(worldstate.ID(), func(document client.Document) {
		values = document.Model.(*stateplugin.Document).State.Values.Clone()
	})

	return values
}

// npcDoc reads an NPC.
func (f *fixture) npcDoc(id ids.ULID) npc.NPC {
	var found npc.NPC
	f.docs.View(id, func(document client.Document) { found = document.Model.(*npcplugin.Document).NPC.Clone() })

	return found
}

// answerUses answers every where-used question still waiting: uses for the
// items given, nothing for the rest.
func (f *fixture) answerUses(uses map[string][]protocol.Use) {
	for index, done := range f.requests.Uses {
		if done != nil {
			done(uses[f.requests.UsesOf[index].Item], nil)
			f.requests.Uses[index] = nil
		}
	}
	f.frame(ui.NewInput(image.Pt(0, 0)))
}

// TestTheTree lists the world's values in their groups, each scope's
// templates, and each NPC and zone with its own values, its name and the
// templates applied to it.
func TestTheTree(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	if _, found := f.state.Locate("row:npc.george.secret", f.ctx.Docs); found {
		t.Fatal("an NPC starts closed")
	}
	f.click("toggle:npc.george")
	f.click("toggle:npc.guard1")
	for _, row := range []string{
		"world.quests", "world.quests.dragon_slain", "world.mood", "npc.guards", "npc.guards.arrests", "npc.george",
		"npc.george.name", "npc.george.secret", "npc.guard1.guards", "npc.guard1.guards.arrests", "zone.market",
	} {
		if _, found := f.state.Locate("row:"+row, f.ctx.Docs); !found {
			t.Errorf("no row %s", row)
		}
	}
	f.click("toggle:world.quests")
	if _, found := f.state.Locate("row:world.quests.dragon_slain", f.ctx.Docs); found {
		t.Fatal("a closed group hides what it holds")
	}
	f.click("toggle:world.quests")
	f.click("row:npc.george.name")
	if !f.has("selected=npc.george.name") || !f.has(`value=npc.george.name:text:"George"`) {
		t.Fatal("an NPC's name starts as its display name")
	}
}

// TestGroupsAndDeleting renames a value into a new group by its dotted
// name, deletes the group it left after asking, and refuses to delete a
// template applied to an NPC, saying where.
func TestGroupsAndDeleting(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.click("row:world.quests.dragon_slain")
	f.typeInto("value-name", "george's")
	if !strings.Contains(f.status(), "lower-case") {
		t.Fatalf("punctuation is refused, saying why: %q", f.status())
	}
	f.typeInto("value-name", "story.dragon_slain")
	if world := f.world(); world.Path(slainID) != "world.story.dragon_slain" {
		t.Fatalf("moved into a new group: %s", world.Path(slainID))
	}
	f.click("row:world.quests")
	f.click("ui:value-delete")
	f.click("dialog:delete")
	if _, found := f.world().Value(questsID); found {
		t.Fatal("the empty group was deleted after asking")
	}
	f.click("row:npc.guards")
	f.click("ui:value-delete")
	f.answerUses(map[string][]protocol.Use{guardsID.String(): {{
		Document: guardID, Name: "Guard 1", Format: npc.FormatName, Label: "applied template",
	}}})
	if _, found := f.state.Locate("dialog:delete", f.ctx.Docs); found {
		t.Fatal("a template in use cannot be deleted")
	}
	f.click("dialog:cancel")
	if _, found := f.world().Value(guardsID); !found {
		t.Fatal("the template stays")
	}
	f.click("row:npc.guards.arrests")
	f.click("ui:value-delete")
	f.answerUses(nil)
	if _, found := f.world().Value(arrestsID); found {
		t.Fatal("a value nothing uses is deleted at once")
	}
}

// TestTemplatesAndOwnValues applies the guards template to George, adds a
// value of his own and a new template.
func TestTemplatesAndOwnValues(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.click("row:npc.george")
	f.click("ui:template-guards")
	if !f.has("applied=npc.george:guards") || len(f.npcDoc(georgeID).Owned.Templates) != 1 {
		t.Fatal("the template is applied to George")
	}
	f.click("ui:template-guards")
	if len(f.npcDoc(georgeID).Owned.Templates) != 0 {
		t.Fatal("and taken off again")
	}
	f.click("row:npc.george")
	f.click("ui:add-value")
	if own := f.npcDoc(georgeID).Owned.Values; own.Len() != 2 || !f.has("selected=npc.george.value") {
		t.Fatalf("Add value declares one of George's own: %d", own.Len())
	}
	f.click("ui:add-template")
	if template, found := f.world().Find(worldstate.ScopeNPC, ids.ULID{}, "template"); !found ||
		template.Type != worldstate.TypeTemplate {
		t.Fatal("Add template makes an NPC template")
	}
}

// TestIntegerLimits makes a value an integer from 0 to 3 and refuses a
// default beyond them.
func TestIntegerLimits(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.click("row:world.quests.dragon_slain")
	f.answerUses(nil)
	f.click("ui:value-type-integer")
	f.typeInto("value-max", "3")
	if slain, _ := f.world().Value(slainID); slain.Max == nil || *slain.Max != 3 {
		t.Fatalf("a maximum: %+v", slain)
	}
	f.typeInto("value-default", "5")
	if !strings.Contains(f.status(), "from") {
		t.Fatalf("a default beyond the limits is refused: %q", f.status())
	}
	f.keys("Cmd+A", "Backspace")
	input := ui.NewInput(image.Pt(0, 0))
	input.Text = "2"
	f.frame(input)
	f.keys("Enter")
	if !f.has("value=world.quests.dragon_slain:integer:2") {
		t.Fatal("a default within them is saved")
	}
}

// TestChoices adds a choice and makes a choice the default.
func TestChoices(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.click("row:world.mood")
	f.click("ui:value-add-choice")
	if mood, _ := f.world().Value(moodID); len(mood.Choices) != 3 || mood.Choices[2].ID != "c3" {
		t.Fatalf("add choice %+v", mood.Choices)
	}
	f.click("ui:value-default-c2")
	if mood, _ := f.world().Value(moodID); string(mood.Default) != `"c2"` {
		t.Fatalf("default %s", mood.Default)
	}
	f.click("ui:value-choice-1-remove")
	if mood, _ := f.world().Value(moodID); len(mood.Choices) != 2 || mood.Choices[0].ID != "c2" ||
		string(mood.Default) != `"c2"` {
		t.Fatalf("remove choice %+v %s", mood.Choices, mood.Default)
	}
}

// TestRenamedTemplateApplies renames a new template, then applies it to
// George: the form it was named in takes no clicks once George is shown. A
// template named as one of George's own values cannot be applied to him.
func TestRenamedTemplateApplies(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.click("row:npc.george")
	f.click("ui:add-template")
	f.typeInto("value-name", "stall")
	if !f.has("selected=npc.stall") {
		t.Fatal("the template is named stall")
	}
	f.click("row:npc.george")
	f.click("ui:template-stall")
	if !f.has("applied=npc.george:stall") {
		t.Fatal("the renamed template is applied to George")
	}
	f.click("ui:add-template")
	f.typeInto("value-name", "secret")
	f.click("row:npc.george")
	f.click("ui:template-secret")
	if f.has("applied=npc.george:secret") || !strings.Contains(f.status(), "has its own secret") {
		t.Fatalf("George's own secret stops a secret template: %q", f.status())
	}
}

// TestPathNames renames the market zone's path name in its inspector, and
// refuses punctuation.
func TestPathNames(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.click("row:zone.market")
	f.typeInto("owner-path", "town's")
	if !strings.Contains(f.status(), "lower-case") {
		t.Fatalf("punctuation is refused: %q", f.status())
	}
	f.typeInto("owner-path", "town_square")
	var path string
	f.docs.View(marketID, func(document client.Document) { path = document.Model.(*zoneplugin.Document).Zone.Path })
	if path != "town_square" || !f.has("selected=zone.town_square") {
		t.Fatalf("the zone is renamed: %q", path)
	}
}
