package npcrun_test

import (
	"encoding/json"
	"image"
	"slices"
	"strconv"
	"testing"

	"github.com/bradhannah/berad-engine/engine/clock"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/npcrun"
	"github.com/bradhannah/berad-engine/engine/play"
	"github.com/bradhannah/berad-engine/engine/tilefloor"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// Declared values: a shop template of whether an NPC is waiting and served,
// a zone's own count, an NPC's own patience, and two workflows' locals.
const (
	waitingID  = 60
	servedID   = 61
	countID    = 62
	customerL  = 63
	tallyL     = 64
	shopID     = 65
	patienceID = 66
	zoneID     = 70
	// loner is placed with seed 140 and has no shop template.
	loner = 140
)

// catalog declares the shop template, applied to every NPC placed with
// seeds 100 to 130, NPC 141 without it but with patience from 0 to 3 of its
// own, and the zone with its own count.
func catalog(t *testing.T) *worldstate.Catalog {
	t.Helper()
	state := worldstate.NewDeclarations(worldstate.HoldsWorldState)
	inShop := func(made worldstate.Declaration) worldstate.Declaration {
		made.Parent = id(shopID)

		return made
	}
	if err := state.InsertAll([]worldstate.Declaration{
		worldstate.New(id(shopID), "shop", worldstate.ScopeNPC, worldstate.TypeTemplate),
		inShop(worldstate.New(id(waitingID), "waiting", worldstate.ScopeNPC, worldstate.TypeBool)),
		inShop(worldstate.New(id(servedID), "served", worldstate.ScopeNPC, worldstate.TypeBool)),
	}); err != nil {
		t.Fatal(err)
	}
	owners := map[ids.ULID]worldstate.Owner{}
	for seed := byte(101); seed <= 131; seed += 10 {
		owned := worldstate.NewOwned(worldstate.ScopeNPC)
		if err := owned.Apply(id(shopID)); err != nil {
			t.Fatal(err)
		}
		owners[id(seed)] = worldstate.Owner{
			Scope: worldstate.ScopeNPC, Path: "npc_" + strconv.Itoa(int(seed)), Name: "NPC " + strconv.Itoa(int(seed)),
			Owned: owned,
		}
	}
	patience := worldstate.New(id(patienceID), "patience", worldstate.ScopeNPC, worldstate.TypeInteger)
	patience.Min, patience.Max = worldstate.Limit(0), worldstate.Limit(3)
	alone := worldstate.NewOwned(worldstate.ScopeNPC)
	office := worldstate.NewOwned(worldstate.ScopeZone)
	if alone.Values.Insert(patience) != nil ||
		office.Values.Insert(worldstate.New(id(countID), "count", worldstate.ScopeZone, worldstate.TypeInteger)) != nil {
		t.Fatal("declare")
	}
	owners[id(loner+1)] = worldstate.Owner{Scope: worldstate.ScopeNPC, Path: "loner", Name: "Loner", Owned: alone}
	owners[id(zoneID)] = worldstate.Owner{Scope: worldstate.ScopeZone, Path: "office", Name: "Office", Owned: office}

	return worldstate.NewCatalog(state, owners)
}

func ref(scope worldstate.Scope, value byte) flow.ValueRef {
	return flow.ValueRef{Scope: scope, Value: id(value), Owner: ids.ULID{}, Found: ids.ULID{}}
}

// withLocal declares a local value on a workflow.
func withLocal(t *testing.T, workflow *flow.Workflow, seed byte, name string, kind worldstate.Type) {
	t.Helper()
	if err := workflow.Locals().Insert(worldstate.New(id(seed), name, worldstate.ScopeLocal, kind)); err != nil {
		t.Fatal(err)
	}
}

// town is NPCs sharing one zone, one store and one clock, stepped in
// placement order, each seeing the others as the tick began.
type town struct {
	npcs      []*npcrun.NPC
	workflows []*flow.Workflow
	state     *worldstate.Store
	zone      play.Levels
	clock     *clock.Clock
	tick      uint64
}

func newTown(t *testing.T, state *worldstate.Store) *town {
	t.Helper()

	return &town{
		npcs: nil, workflows: nil, state: state, zone: play.Levels{{ID: id(groundID), Floor: office(t)}},
		clock: clock.New(eight, clock.MinutePerSecond), tick: 0,
	}
}

// place adds an NPC, its placement and NPC IDs from seed and seed+1.
func (w *town) place(seed byte, at image.Point, workflow *flow.Workflow) *npcrun.NPC {
	placement := tilefloor.Placement{ID: id(seed), NPC: id(seed + 1), Col: at.X, Row: at.Y}
	made := npcrun.Start(placement, id(groundID), uint64(seed))
	w.npcs, w.workflows = append(w.npcs, made), append(w.workflows, workflow)

	return made
}

func (w *town) run(ticks int) {
	for range ticks {
		w.tick++
		passed := w.clock.Advance()
		others := make([]npcrun.Other, 0, len(w.npcs))
		for _, each := range w.npcs {
			others = append(others, npcrun.Other{Placement: each.Placement, NPC: each.NPC, Floor: each.Floor, Cell: each.Cell})
		}
		for index, each := range w.npcs {
			each.Step(npcrun.World{
				Zone: w.zone, Workflow: w.workflows[index], Clock: w.clock, Passed: passed, State: w.state,
				Here: id(zoneID), Others: others,
			}, w.tick)
		}
	}
}

func (w *town) value(t *testing.T, value byte, owner ids.ULID) worldstate.Value {
	t.Helper()
	declared, _, ok := w.state.Catalog().Declared(id(value))
	if !ok {
		t.Fatalf("no value %d", value)
	}

	return w.state.Get(declared, owner)
}

// TestSetAndCheck counts up a local value each loop and adds to the zone's
// count; once the local reaches three, the NPC marks itself served. The
// local clears when the workflow restarts.
func TestSetAndCheck(t *testing.T) {
	t.Parallel()
	nodes := []flow.Node{
		node(t, 2, flow.KindSetValue, map[string]any{flow.ParamValue: ref(worldstate.ScopeLocal, tallyL), "how": flow.HowAdd,
			"to": json.RawMessage("1")}),
		node(t, 3, flow.KindSetValue, map[string]any{flow.ParamValue: ref(worldstate.ScopeZone, countID), "how": flow.HowAdd,
			"to": json.RawMessage("2")}),
		node(t, 4, flow.KindCheckValue, map[string]any{flow.ParamValue: ref(worldstate.ScopeLocal, tallyL),
			"compare": flow.CompareLeast, "against": json.RawMessage("3")}),
		node(t, 5, flow.KindSetValue, map[string]any{flow.ParamValue: ref(worldstate.ScopeNPC, servedID),
			"to": json.RawMessage("true")}),
		node(t, 6, flow.KindWait, map[string]any{"duration": waitFor(1, flow.UnitSeconds, flow.ClockGame)}),
	}
	workflow := workflowOf(t, nodes, link(1, flow.OutputNext, 2), link(2, flow.OutputNext, 3),
		link(3, flow.OutputNext, 4), link(4, flow.OutputYes, 5), link(4, flow.OutputOtherwise, 6), link(5, flow.OutputNext, 6))
	withLocal(t, workflow, tallyL, "tally", worldstate.TypeInteger)
	town := newTown(t, worldstate.NewStore(catalog(t)))
	clerk := town.place(100, image.Pt(2, 2), workflow)
	town.run(1)
	if tally := clerk.Locals()[id(tallyL)]; tally.Integer != 1 || town.value(t, servedID, clerk.NPC).Bool {
		t.Fatalf("one loop: tally %+v", tally)
	}
	town.run(3 * 70)
	if tally := clerk.Locals()[id(tallyL)]; tally.Integer < 3 || !town.value(t, servedID, clerk.NPC).Bool {
		t.Fatalf("three loops mark it served: tally %+v", tally)
	}
	if count := town.value(t, countID, id(zoneID)); count.Integer != 2*clerk.Locals()[id(tallyL)].Integer {
		t.Fatalf("the zone's count goes up by two a loop: %+v", count)
	}
	clerk.Restart()
	if len(clerk.Locals()) != 0 {
		t.Fatal("a restart clears the local values")
	}
	if !town.value(t, servedID, clerk.NPC).Bool {
		t.Fatal("a restart keeps the NPC's own values")
	}
}

// Locals for the comparison tests: a flag the yes branch sets, an integer
// limit, a mood of three choices, a name and a readiness.
const (
	flagL  = 67
	limitL = 68
	moodL  = 69
	nameL  = 71
	readyL = 72
)

// checks runs one Check value on the loner, after the setup nodes have set
// its values, and reports whether the check took yes: the yes branch sets a
// local flag, the otherwise branch only waits.
func checks(t *testing.T, declare func(*flow.Workflow), setup []flow.Node, check map[string]any) bool {
	t.Helper()
	nodes := append(slices.Clone(setup),
		node(t, 8, flow.KindCheckValue, check),
		setLocal(t, 9, flagL, "true"),
		node(t, 10, flow.KindWait, map[string]any{"duration": waitFor(1, flow.UnitSeconds, flow.ClockGame)}),
	)
	links := []flow.Link{link(8, flow.OutputYes, 9), link(8, flow.OutputOtherwise, 10), link(9, flow.OutputNext, 10)}
	previous := byte(1)
	for _, made := range setup {
		links = append(links, link(previous, flow.OutputNext, made.ID[15]))
		previous = made.ID[15]
	}
	links = append(links, link(previous, flow.OutputNext, 8))
	workflow := workflowOf(t, nodes, links...)
	withLocal(t, workflow, flagL, "flag", worldstate.TypeBool)
	declare(workflow)
	town := newTown(t, worldstate.NewStore(catalog(t)))
	npc := town.place(loner, image.Pt(2, 2), workflow)
	town.run(2)

	return npc.Locals()[id(flagL)].Bool
}

// setLocal makes a Set value node that sets a local to a saved value.
func setLocal(t *testing.T, seed, local byte, to string) flow.Node {
	t.Helper()

	return node(t, seed, flow.KindSetValue, map[string]any{
		flow.ParamValue: ref(worldstate.ScopeLocal, local), "to": json.RawMessage(to),
	})
}

// quoted makes a list operand of the given choices.
func quoted(items ...string) json.RawMessage {
	raws := make([]json.RawMessage, 0, len(items))
	for _, item := range items {
		raws = append(raws, json.RawMessage(strconv.Quote(item)))
	}

	return flow.OperandList(raws)
}

// TestComparisonsByType checks the loner's patience, an integer from 0 to
// 3, with every integer comparison, against a number and against another
// value; a bool; a choice against a list; and text.
func TestComparisonsByType(t *testing.T) {
	t.Parallel()
	patience := ref(worldstate.ScopeNPC, patienceID)
	setPatience := func(to string) []flow.Node {
		return []flow.Node{node(t, 2, flow.KindSetValue, map[string]any{flow.ParamValue: patience, "to": json.RawMessage(to)})}
	}
	none := func(*flow.Workflow) {}
	for _, tc := range []struct {
		compare, against, to string
		want                 bool
	}{
		{flow.CompareIs, "2", "", true}, {flow.CompareNot, "2", "", false}, {flow.CompareBelow, "3", "", true},
		{flow.CompareBelow, "2", "", false}, {flow.CompareMost, "2", "", true}, {flow.CompareAbove, "2", "", false},
		{flow.CompareAbove, "1", "", true}, {flow.CompareLeast, "3", "", false}, {flow.CompareBetween, "1", "2", true},
		{flow.CompareBetween, "3", "5", false}, {flow.CompareMax, "", "", false},
	} {
		check := map[string]any{flow.ParamValue: patience, flow.ParamCompare: tc.compare}
		if tc.against != "" {
			check[flow.ParamAgainst] = json.RawMessage(tc.against)
		}
		if tc.to != "" {
			check[flow.ParamTo] = json.RawMessage(tc.to)
		}
		if got := checks(t, none, setPatience("2"), check); got != tc.want {
			t.Fatalf("patience 2 %s %s %s: %v", tc.compare, tc.against, tc.to, got)
		}
	}
	if !checks(t, none, setPatience("3"), map[string]any{flow.ParamValue: patience, flow.ParamCompare: flow.CompareMax}) {
		t.Fatal("patience 3 is at its maximum")
	}
	withLimit := func(workflow *flow.Workflow) { withLocal(t, workflow, limitL, "limit", worldstate.TypeInteger) }
	limit := flow.RefOperand(ref(worldstate.ScopeLocal, limitL))
	if !checks(t, withLimit, append(setPatience("2"), setLocal(t, 3, limitL, "2")),
		map[string]any{flow.ParamValue: patience, flow.ParamCompare: flow.CompareIs, flow.ParamAgainst: limit}) {
		t.Fatal("patience 2 is the limit 2, read from another value")
	}
	if checks(t, withLimit, append(setPatience("2"), setLocal(t, 3, limitL, "3")),
		map[string]any{flow.ParamValue: patience, flow.ParamCompare: flow.CompareLeast, flow.ParamAgainst: limit}) {
		t.Fatal("patience 2 is not at least the limit 3")
	}
	withReady := func(workflow *flow.Workflow) { withLocal(t, workflow, readyL, "ready", worldstate.TypeBool) }
	ready := ref(worldstate.ScopeLocal, readyL)
	if checks(t, withReady, setPatience("2"), map[string]any{
		flow.ParamValue: patience, flow.ParamCompare: flow.CompareIs, flow.ParamAgainst: flow.RefOperand(ready),
	}) {
		t.Fatal("a reference of another type takes otherwise")
	}
	isTrue := map[string]any{flow.ParamValue: ready, flow.ParamCompare: flow.CompareIs, flow.ParamAgainst: json.RawMessage("true")}
	if !checks(t, withReady, []flow.Node{setLocal(t, 2, readyL, "true")}, isTrue) {
		t.Fatal("a bool is true")
	}
	isTrue[flow.ParamCompare] = flow.CompareLeast
	if checks(t, withReady, []flow.Node{setLocal(t, 2, readyL, "true")}, isTrue) {
		t.Fatal("a bool cannot be at least anything")
	}
	withMood := func(workflow *flow.Workflow) {
		mood := worldstate.New(id(moodL), "mood", worldstate.ScopeLocal, worldstate.TypeChoice)
		mood.Choices = []worldstate.Choice{{ID: "happy", Name: "happy"}, {ID: "calm", Name: "calm"}, {ID: "grumpy", Name: "grumpy"}}
		mood.Default = mood.Empty().Encode()
		if err := workflow.Locals().Insert(mood); err != nil {
			t.Fatal(err)
		}
	}
	mood, calm := ref(worldstate.ScopeLocal, moodL), []flow.Node{setLocal(t, 2, moodL, `"calm"`)}
	for _, tc := range []struct {
		compare string
		among   json.RawMessage
		want    bool
	}{
		{flow.CompareAnyOf, quoted("happy", "calm"), true}, {flow.CompareNoneOf, quoted("happy", "calm"), false},
		{flow.CompareNoneOf, quoted("grumpy"), true}, {flow.CompareAnyOf, quoted("grumpy"), false},
		{flow.CompareAnyOf, flow.NoOperand(), false},
	} {
		got := checks(t, withMood, calm, map[string]any{flow.ParamValue: mood, flow.ParamCompare: tc.compare, flow.ParamAmong: tc.among})
		if got != tc.want {
			t.Fatalf("calm %s %s: %v", tc.compare, tc.among, got)
		}
	}
	if !checks(t, withMood, calm, map[string]any{
		flow.ParamValue: mood, flow.ParamCompare: flow.CompareIs, flow.ParamAgainst: json.RawMessage(`"calm"`),
	}) {
		t.Fatal("a choice checked with is, from before any of, still matches")
	}
	withName := func(workflow *flow.Workflow) { withLocal(t, workflow, nameL, "name", worldstate.TypeText) }
	name, bob := ref(worldstate.ScopeLocal, nameL), []flow.Node{setLocal(t, 2, nameL, `"Bob"`)}
	stranger := map[string]any{flow.ParamValue: name, flow.ParamCompare: flow.CompareNot, flow.ParamAgainst: json.RawMessage(`"Stranger"`)}
	if !checks(t, withName, bob, stranger) {
		t.Fatal("text is not a stranger")
	}
	stranger[flow.ParamCompare] = flow.CompareIs
	if checks(t, withName, bob, stranger) {
		t.Fatal("text is not a stranger, so is fails")
	}
}

// shopWorkflows are a customer, who says it is waiting until a clerk
// nearby is serving, and a clerk, who finds a waiting customer within two
// tiles, reads that it is waiting and marks itself serving.
func shopWorkflows(t *testing.T) (*flow.Workflow, *flow.Workflow) {
	t.Helper()
	pause := map[string]any{"duration": waitFor(1, flow.UnitSeconds, flow.ClockGame)}
	customer := workflowOf(t, []flow.Node{
		node(t, 2, flow.KindFindNPC, map[string]any{"within": 2, "where": ref(worldstate.ScopeNPC, servedID),
			"is": json.RawMessage("true"), "into": ref(worldstate.ScopeLocal, customerL)}),
		node(t, 3, flow.KindSetValue, map[string]any{flow.ParamValue: ref(worldstate.ScopeNPC, waitingID),
			"to": json.RawMessage("false")}),
		node(t, 4, flow.KindSetValue, map[string]any{flow.ParamValue: ref(worldstate.ScopeNPC, waitingID),
			"to": json.RawMessage("true")}),
		node(t, 5, flow.KindWait, pause),
	}, link(1, flow.OutputNext, 2), link(2, flow.OutputFound, 3), link(2, flow.OutputNone, 4), link(4, flow.OutputNext, 5))
	withLocal(t, customer, customerL, "clerk", worldstate.TypeNPC)
	found := flow.ValueRef{Scope: worldstate.ScopeNPC, Value: id(waitingID), Owner: ids.ULID{}, Found: id(customerL)}
	clerk := workflowOf(t, []flow.Node{
		node(t, 2, flow.KindFindNPC, map[string]any{"within": 2, "where": ref(worldstate.ScopeNPC, waitingID),
			"is": json.RawMessage("true"), "into": ref(worldstate.ScopeLocal, customerL)}),
		node(t, 3, flow.KindCheckValue, map[string]any{flow.ParamValue: found, "against": json.RawMessage("true")}),
		node(t, 4, flow.KindSetValue, map[string]any{flow.ParamValue: ref(worldstate.ScopeNPC, servedID),
			"to": json.RawMessage("true")}),
		node(t, 5, flow.KindSetValue, map[string]any{flow.ParamValue: found, "to": json.RawMessage("false")}),
		node(t, 6, flow.KindWait, pause),
	}, link(1, flow.OutputNext, 2), link(2, flow.OutputFound, 3), link(2, flow.OutputNone, 6), link(3, flow.OutputYes, 4),
		link(4, flow.OutputNext, 5), link(5, flow.OutputNext, 6))
	withLocal(t, clerk, customerL, "customer", worldstate.TypeNPC)

	return customer, clerk
}

// TestFindNPCNearby has a clerk find the customer waiting within two tiles
// and serve it, and the customer then see a clerk serving and stop
// waiting. The clerk can read the customer's value, never set it, and a
// customer three tiles away is never found.
func TestFindNPCNearby(t *testing.T) {
	t.Parallel()
	customerFlow, clerkFlow := shopWorkflows(t)
	town := newTown(t, worldstate.NewStore(catalog(t)))
	clerk := town.place(100, image.Pt(2, 2), clerkFlow)
	near := town.place(110, image.Pt(3, 4), customerFlow)
	far := town.place(120, image.Pt(2, 5), customerFlow)
	town.run(2)
	if found := clerk.Locals()[id(customerL)]; found.NPC != near.NPC {
		t.Fatalf("the clerk keeps the waiting customer within two tiles: %+v", found)
	}
	if !town.value(t, servedID, clerk.NPC).Bool {
		t.Fatal("the clerk read that it is waiting and serves")
	}
	if clerk.Last.Status != npcrun.StatusNoValue {
		t.Fatalf("setting another NPC's value carries on without it: %q", clerk.Last.Status)
	}
	town.run(70)
	if town.value(t, waitingID, near.NPC).Bool {
		t.Fatal("the near customer saw a clerk serving and stopped waiting itself")
	}
	if !town.value(t, waitingID, far.NPC).Bool {
		t.Fatal("the far customer, three tiles away, still waits")
	}
}

// TestValuesRepeat runs the shop twice: the same values every time.
func TestValuesRepeat(t *testing.T) {
	t.Parallel()
	outcome := func() []bool {
		customerFlow, clerkFlow := shopWorkflows(t)
		town := newTown(t, worldstate.NewStore(catalog(t)))
		var npcs []*npcrun.NPC
		for index, at := range []image.Point{{2, 2}, {3, 3}, {1, 3}, {2, 4}} {
			flow := customerFlow
			if index == 0 {
				flow = clerkFlow
			}
			npcs = append(npcs, town.place(byte(100+10*index), at, flow))
		}
		town.run(500)
		var seen []bool
		for _, each := range npcs {
			seen = append(seen, town.value(t, waitingID, each.NPC).Bool, town.value(t, servedID, each.NPC).Bool)
		}

		return seen
	}
	first, second := outcome(), outcome()
	for index := range first {
		if first[index] != second[index] {
			t.Fatalf("runs differ: %v %v", first, second)
		}
	}
}

// TestNamedNPC reads another NPC's value by naming it.
func TestNamedNPC(t *testing.T) {
	t.Parallel()
	named := flow.ValueRef{Scope: worldstate.ScopeNPC, Value: id(servedID), Owner: id(111), Found: ids.ULID{}}
	checker := workflowOf(t, []flow.Node{
		node(t, 2, flow.KindCheckValue, map[string]any{flow.ParamValue: named, "against": json.RawMessage("true")}),
		node(t, 3, flow.KindSetValue, map[string]any{flow.ParamValue: ref(worldstate.ScopeNPC, waitingID),
			"to": json.RawMessage("true")}),
		node(t, 4, flow.KindWait, map[string]any{"duration": waitFor(1, flow.UnitSeconds, flow.ClockGame)}),
	}, link(1, flow.OutputNext, 2), link(2, flow.OutputYes, 3), link(2, flow.OutputOtherwise, 4))
	state := worldstate.NewStore(catalog(t))
	served, _, _ := state.Catalog().Declared(id(servedID))
	yes := worldstate.NoValue()
	yes.Type, yes.Bool = worldstate.TypeBool, true
	if err := state.Set(served, id(111), yes); err != nil {
		t.Fatal(err)
	}
	town := newTown(t, state)
	reader := town.place(100, image.Pt(2, 2), checker)
	town.run(1)
	if !town.value(t, waitingID, reader.NPC).Bool {
		t.Fatal("it read that the NPC it names is served, wherever that NPC is")
	}
}

// TestLimitsAndTemplates adds to an integer held at its maximum and asks
// whether it is there, then sets a template's value its NPC lacks; and has
// an NPC with the template read a named zone's value, set its own and
// rename itself.
func TestLimitsAndTemplates(t *testing.T) {
	t.Parallel()
	pause := map[string]any{"duration": waitFor(1, flow.UnitSeconds, flow.ClockGame)}
	impatient := workflowOf(t, []flow.Node{
		node(t, 2, flow.KindSetValue, map[string]any{flow.ParamValue: ref(worldstate.ScopeNPC, patienceID), "how": flow.HowAdd,
			"to": json.RawMessage("2")}),
		node(t, 3, flow.KindCheckValue, map[string]any{flow.ParamValue: ref(worldstate.ScopeNPC, patienceID),
			"compare": flow.CompareMax}),
		node(t, 4, flow.KindSetValue, map[string]any{flow.ParamValue: ref(worldstate.ScopeNPC, waitingID),
			"to": json.RawMessage("true")}),
		node(t, 5, flow.KindWait, pause),
	}, link(1, flow.OutputNext, 2), link(2, flow.OutputNext, 3), link(3, flow.OutputYes, 4), link(3, flow.OutputOtherwise, 5),
		link(4, flow.OutputNext, 5))
	town := newTown(t, worldstate.NewStore(catalog(t)))
	alone := town.place(loner, image.Pt(2, 2), impatient)
	town.run(1)
	if patience := town.value(t, patienceID, alone.NPC); patience.Integer != 2 || alone.Last.Status != "" {
		t.Fatalf("one loop adds two, not yet at its maximum: %+v %q", patience, alone.Last.Status)
	}
	town.run(70)
	if patience := town.value(t, patienceID, alone.NPC); patience.Integer != 3 {
		t.Fatalf("held at its maximum of three: %+v", patience)
	}
	if alone.Last.Status != npcrun.StatusNoValue {
		t.Fatalf("at its maximum it sets npc.shop.waiting, which it lacks, and carries on: %q", alone.Last.Status)
	}
	name := flow.ValueRef{Scope: worldstate.ScopeNPC, Value: worldstate.NameID(), Owner: ids.ULID{}, Found: ids.ULID{}}
	reader := workflowOf(t, []flow.Node{
		node(t, 2, flow.KindCheckValue, map[string]any{flow.ParamValue: flow.ValueRef{
			Scope: worldstate.ScopeZone, Value: id(countID), Owner: id(zoneID), Found: ids.ULID{},
		}, "against": json.RawMessage("0")}),
		node(t, 3, flow.KindSetValue, map[string]any{flow.ParamValue: ref(worldstate.ScopeNPC, waitingID),
			"to": json.RawMessage("true")}),
		node(t, 4, flow.KindSetValue, map[string]any{flow.ParamValue: name, "to": json.RawMessage(`"Old Tom"`)}),
		node(t, 5, flow.KindWait, pause),
	}, link(1, flow.OutputNext, 2), link(2, flow.OutputYes, 3), link(2, flow.OutputOtherwise, 5), link(3, flow.OutputNext, 4),
		link(4, flow.OutputNext, 5))
	shopper := town.place(100, image.Pt(4, 4), reader)
	town.run(1)
	declared, _, _ := town.state.Catalog().Declared(worldstate.NameID())
	if !town.value(t, waitingID, shopper.NPC).Bool || town.state.Get(declared, shopper.NPC).Text != "Old Tom" ||
		town.state.Get(declared, alone.NPC).Text != "Loner" {
		t.Fatal("it read the office's count by name, set its own template value and renamed itself, and only itself")
	}
}

// TestGoToAPlaceValue goes to where a place local starts, and carries on
// when it holds no place.
func TestGoToAPlaceValue(t *testing.T) {
	t.Parallel()
	goTo := node(t, 2, flow.KindGoTo, map[string]any{
		flow.ParamGoes: flow.GoesToValue, flow.ParamPlace: ref(worldstate.ScopeLocal, 20),
	})
	waits := node(t, 3, flow.KindWait, map[string]any{"duration": waitFor(1, flow.UnitHours, flow.ClockGame)})
	workflow := workflowOf(t, []flow.Node{goTo, waits}, link(1, flow.OutputNext, 2), link(2, flow.OutputNext, 3))
	place := worldstate.New(id(20), "place", worldstate.ScopeLocal, worldstate.TypePlace)
	start := worldstate.NoValue()
	start.Type, start.Place = worldstate.TypePlace, worldstate.Place{Floor: id(groundID), Anchor: id(deskID)}
	place.Default = start.Encode()
	if err := workflow.Locals().Insert(place); err != nil {
		t.Fatal(err)
	}
	r := newRunner(t, workflow, image.Pt(2, 1), eight, clock.Real)
	r.run(40 * play.StepTicks)
	if r.npc.Cell != image.Pt(8, 1) || r.npc.Node != id(3) {
		t.Fatalf("went to the desk the place holds: %v on %v (%q)", r.npc.Cell, r.npc.Node, r.npc.Status)
	}
	empty := workflowOf(t, []flow.Node{goTo, waits}, link(1, flow.OutputNext, 2), link(2, flow.OutputNext, 3))
	withLocal(t, empty, 20, "place", worldstate.TypePlace)
	r = newRunner(t, empty, image.Pt(2, 1), eight, clock.Real)
	r.run(1)
	if r.npc.Node != id(3) || r.npc.Last.Status != npcrun.StatusNoAnchor {
		t.Fatalf("with no place it carries on: %v %q", r.npc.Node, r.npc.Last.Status)
	}
}
