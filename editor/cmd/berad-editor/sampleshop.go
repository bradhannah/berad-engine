package main

import (
	"encoding/json"
	"fmt"

	"github.com/bradhannah/berad-engine/editor/model"
	"github.com/bradhannah/berad-engine/editor/orchestrator"
	npcplugin "github.com/bradhannah/berad-engine/editor/plugins/npc"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	stateplugin "github.com/bradhannah/berad-engine/editor/plugins/worldstate"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/npc"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// The sample shop: a customer who walks up to the counter, waits for the
// clerk at the desk to serve them, leaves by the door and comes back a while
// later. It shows values and sensing: a shop template giving the clerk and
// the customer the same values, a world count in a group, local values and
// Find NPC nearby. Cells are from the zone origin; boxes are canvas columns
// and rows.
const (
	customerCol  = doorCol
	customerRow  = buildingBottom - 1
	shopWithin   = 2
	shopPause    = 1
	shopAway     = 10
	shopColumn   = 220
	shopRow      = 120
	shopLeft     = 40
	shopTop      = 80
	servedRow    = 0
	waitRow      = 1
	nobodyRow    = 2
	shopYes      = "true"
	shopNo       = "false"
	shopDuration = "duration"
)

// shopValues are what the sample shop declares: a shop template of whether
// an NPC is waiting, serving or served, and a shop group holding how many
// customers the world has served.
type shopValues struct {
	template, waiting, serving, served, group, count ids.ULID
}

// sampleCustomer adds the customer: their workflow, the NPC with the shop
// template applied, and their placement inside the door.
func sampleCustomer(orch *orchestrator.Orchestrator, anchors sampleAnchors, values shopValues) error {
	behaviour, err := customerWorkflow(orch, values, anchors)
	if err != nil {
		return err
	}
	customer := npc.New()
	customer.Behaviour, customer.Path = behaviour, "customer"
	if err := customer.Owned.Apply(values.template); err != nil {
		return fmt.Errorf("sample shop: %w", err)
	}
	npcID, err := orch.CreateDocument("Customer", &npcplugin.Document{NPC: customer})
	if err != nil {
		return fmt.Errorf("sample shop: %w", err)
	}
	placementID, err := orch.NewID()
	if err != nil {
		return fmt.Errorf("sample shop: %w", err)
	}

	return placeShop(orch, anchors.floor, floor.Placement{ID: placementID, NPC: npcID, Col: customerCol, Row: customerRow})
}

// declareShop declares the shop's template and group in the world state.
func declareShop(orch *orchestrator.Orchestrator) (shopValues, error) {
	var values shopValues
	declared := []struct {
		id, parent  *ids.ULID
		name        string
		scope       worldstate.Scope
		kind        worldstate.Type
		description string
	}{
		{&values.template, nil, "shop", worldstate.ScopeNPC, worldstate.TypeTemplate, "What a shop's NPCs share."},
		{&values.waiting, &values.template, "waiting", worldstate.ScopeNPC, worldstate.TypeBool,
			"Waiting at the desk to be served."},
		{&values.serving, &values.template, "serving", worldstate.ScopeNPC, worldstate.TypeBool,
			"Serving a customer at the desk."},
		{&values.served, &values.template, "served", worldstate.ScopeNPC, worldstate.TypeBool,
			"Served, and on the way out."},
		{&values.group, nil, "shop", worldstate.ScopeWorld, worldstate.TypeGroup, "The shop's own counts."},
		{&values.count, &values.group, "customers_served", worldstate.ScopeWorld, worldstate.TypeInteger,
			"Customers served so far."},
	}
	for _, each := range declared {
		made, err := orch.NewID()
		if err != nil {
			return values, fmt.Errorf("sample shop: %w", err)
		}
		*each.id = made
	}
	err := orch.ChangeDocument(worldstate.ID(), func(document model.Document) error {
		state, ok := document.(*stateplugin.Document)
		if !ok {
			return fmt.Errorf("%w: the world state", errSample)
		}
		changed := state.State.Values.Clone()
		for _, each := range declared {
			made := worldstate.New(*each.id, each.name, each.scope, each.kind)
			made.Description = each.description
			if each.parent != nil {
				made.Parent = *each.parent
			}
			if err := changed.Insert(made); err != nil {
				return fmt.Errorf("declare %s: %w", each.name, err)
			}
		}
		state.State.Values = changed

		return nil
	})
	if err != nil {
		return values, fmt.Errorf("sample shop: %w", err)
	}

	return values, nil
}

// placeShop places the customer on the ground floor, after the clerk.
func placeShop(orch *orchestrator.Orchestrator, floorID ids.ULID, placement floor.Placement) error {
	err := orch.ChangeDocument(floorID, func(document model.Document) error {
		floorDocument, ok := document.(*tilefloor.Document)
		if !ok {
			return fmt.Errorf("%w: the ground floor", errSample)
		}
		changed := floorDocument.Floor.Clone()
		if err := changed.InsertPlacement(len(changed.Placements()), placement); err != nil {
			return fmt.Errorf("place %s: %w", placement.NPC, err)
		}
		floorDocument.Floor = changed

		return nil
	})
	if err != nil {
		return fmt.Errorf("sample shop: %w", err)
	}

	return nil
}

// customerWorkflow walks to the counter and waits there until it finds a
// clerk serving within two tiles; then it is served, counts itself, leaves
// by the door, and some seconds later wants serving again.
func customerWorkflow(orch *orchestrator.Orchestrator, values shopValues, anchors sampleAnchors) (ids.ULID, error) {
	build, err := newShopFlow(orch)
	if err != nil {
		return ids.ULID{}, err
	}
	clerk := build.local("clerk", worldstate.TypeNPC)
	check := build.add(flow.KindCheckValue, build.workflow.Start(), waitRow, map[string]any{
		flow.ParamValue: npcValue(values.served), "against": json.RawMessage(shopYes),
	})
	away := build.add(flow.KindWait, check, servedRow, map[string]any{shopDuration: realSeconds(shopAway)})
	back := build.add(flow.KindSetValue, away, servedRow, setTo(npcValue(values.served), shopNo))
	walk := build.add(flow.KindGoTo, check, waitRow, map[string]any{
		paramDestination: floor.AnchorRef{Floor: anchors.floor, Anchor: anchors.counter},
	})
	wait := build.add(flow.KindSetValue, walk, waitRow, setTo(npcValue(values.waiting), shopYes))
	find := build.add(flow.KindFindNPC, wait, waitRow, map[string]any{
		"within": shopWithin, "where": npcValue(values.serving), "is": json.RawMessage(shopYes),
		"into": localValue(clerk),
	})
	pause := build.add(flow.KindWait, find, nobodyRow, map[string]any{shopDuration: realSeconds(shopPause)})
	done := build.add(flow.KindSetValue, find, waitRow, setTo(npcValue(values.waiting), shopNo))
	served := build.add(flow.KindSetValue, done, waitRow, setTo(npcValue(values.served), shopYes))
	count := build.add(flow.KindSetValue, served, waitRow, map[string]any{
		flow.ParamValue: worldValue(values.count), "how": flow.HowAdd, "to": json.RawMessage("1"),
	})
	leave := build.add(flow.KindGoTo, count, waitRow, map[string]any{
		paramDestination: floor.AnchorRef{Floor: anchors.floor, Anchor: anchors.door},
	})
	build.chain(build.workflow.Start(), check)
	build.link(check, flow.OutputYes, away)
	build.link(check, flow.OutputOtherwise, walk)
	build.chain(away, back)
	build.chain(walk, wait, find)
	build.link(find, flow.OutputFound, done)
	build.link(find, flow.OutputNone, pause)
	build.chain(done, served, count, leave)

	return build.create("Customer shops")
}

// shopFlow builds one sample workflow: nodes with new IDs, where each sits,
// in canvas columns from the start, and the links between them. Its first
// error stops the rest.
type shopFlow struct {
	orch     *orchestrator.Orchestrator
	workflow *flow.Workflow
	nodes    []flow.Node
	columns  map[ids.ULID]int
	boxes    map[ids.ULID]flow.Box
	links    []flow.Link
	err      error
}

func newShopFlow(orch *orchestrator.Orchestrator) (*shopFlow, error) {
	start, err := orch.NewID()
	if err != nil {
		return nil, fmt.Errorf("sample shop: %w", err)
	}
	workflow, err := flow.New(start)
	if err != nil {
		return nil, fmt.Errorf("sample shop: %w", err)
	}
	boxes := map[ids.ULID]flow.Box{start: shopBox(0, waitRow)}

	return &shopFlow{
		orch: orch, workflow: workflow, nodes: nil, columns: map[ids.ULID]int{start: 0}, boxes: boxes, links: nil, err: nil,
	}, nil
}

func shopBox(column, row int) flow.Box {
	return flow.Box{X: shopLeft + column*shopColumn, Y: shopTop + row*shopRow, W: 0, H: 0}
}

// local declares a local value and returns its ID.
func (b *shopFlow) local(name string, kind worldstate.Type) ids.ULID {
	made := b.newID()
	if b.err == nil {
		b.err = b.workflow.Locals().Insert(worldstate.New(made, name, worldstate.ScopeLocal, kind))
	}

	return made
}

// add adds a node of a kind with parameters set, in a row of the column
// after the node it follows.
func (b *shopFlow) add(kind string, after ids.ULID, row int, params map[string]any) ids.ULID {
	made := b.newID()
	if b.err != nil {
		return made
	}
	node, err := flow.NewNode(kind)
	node.ID = made
	for name, value := range params {
		if err != nil {
			break
		}
		node, err = node.WithParam(name, value)
	}
	if err != nil {
		b.err = err

		return made
	}
	b.nodes = append(b.nodes, node)
	b.columns[made] = b.columns[after] + 1
	b.boxes[made] = shopBox(b.columns[made], row)

	return made
}

func (b *shopFlow) link(from ids.ULID, output string, to ids.ULID) {
	b.links = append(b.links, flow.Link{From: from, Output: output, To: to})
}

// chain links each node's next output to the node after it.
func (b *shopFlow) chain(nodes ...ids.ULID) {
	for index := 1; index < len(nodes); index++ {
		b.link(nodes[index-1], flow.OutputNext, nodes[index])
	}
}

func (b *shopFlow) newID() ids.ULID {
	if b.err != nil {
		return ids.ULID{}
	}
	made, err := b.orch.NewID()
	b.err = err

	return made
}

// create saves the workflow as a document and returns its ID.
func (b *shopFlow) create(name string) (ids.ULID, error) {
	if b.err == nil {
		b.err = b.workflow.Insert(b.nodes, b.boxes, b.links)
	}
	if b.err != nil {
		return ids.ULID{}, fmt.Errorf("sample shop: %s: %w", name, b.err)
	}
	created, err := b.orch.CreateDocument(name, &workflowplugin.Document{Workflow: b.workflow})
	if err != nil {
		return ids.ULID{}, fmt.Errorf("sample shop: %w", err)
	}

	return created, nil
}

func npcValue(value ids.ULID) flow.ValueRef {
	return flow.ValueRef{Scope: worldstate.ScopeNPC, Value: value, Owner: ids.ULID{}, Found: ids.ULID{}}
}

func worldValue(value ids.ULID) flow.ValueRef {
	return flow.ValueRef{Scope: worldstate.ScopeWorld, Value: value, Owner: ids.ULID{}, Found: ids.ULID{}}
}

func localValue(value ids.ULID) flow.ValueRef {
	return flow.ValueRef{Scope: worldstate.ScopeLocal, Value: value, Owner: ids.ULID{}, Found: ids.ULID{}}
}

func setTo(ref flow.ValueRef, operand string) map[string]any {
	return map[string]any{flow.ParamValue: ref, "to": json.RawMessage(operand)}
}

func realSeconds(amount int) flow.Duration {
	return flow.Duration{Amount: amount, Unit: flow.UnitSeconds, Clock: flow.ClockReal}
}
