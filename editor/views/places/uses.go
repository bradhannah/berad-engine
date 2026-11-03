package places

import (
	"slices"

	"github.com/bradhannah/berad-engine/editor/client"
	zoneplugin "github.com/bradhannah/berad-engine/editor/plugins/zone"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/ids"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/worldstate"
	"github.com/bradhannah/berad-engine/engine/zone"
)

// Use is a place a workflow names: the node naming it, or zero for a place
// local's start; what names it, as "Go to", "Call Fetch stock · place" or
// "local home"; and the place.
type Use struct {
	Node ids.ULID
	What string
	Ref  floor.AnchorRef
}

// Modules finds a workflow a Call calls, when the window has it.
type Modules func(ids.ULID) (*flow.Workflow, bool)

// UsesIn lists the places a workflow names itself: each Go to's place on a
// map, each place a Call gives its module's place inputs, and each place
// local's start. A place reached through a place value is its local's
// start, listed once; a module's own places are the module's.
func UsesIn(workflow *flow.Workflow, modules Modules, names func(ids.ULID) string) []Use {
	var uses []Use
	for _, node := range workflow.Nodes() {
		switch node.Kind {
		case flow.KindGoTo:
			goes, err := node.Choice(flow.ParamGoes)
			if err == nil && goes == flow.GoesToValue {
				continue
			}
			if destination, err := node.Anchor("destination"); err == nil && !destination.IsZero() {
				uses = append(uses, Use{Node: node.ID, What: "Go to", Ref: destination})
			}
		case flow.KindCall:
			uses = append(uses, callUses(node, modules, names)...)
		}
	}
	for _, declared := range workflow.Locals().Values() {
		if ref, isPlace := placeOf(declared, declared.Default); isPlace {
			uses = append(uses, Use{Node: ids.ULID{}, What: "local " + declared.Name, Ref: ref})
		}
	}

	return uses
}

// callUses lists the places a Call gives its module's place inputs.
func callUses(node flow.Node, modules Modules, names func(ids.ULID) string) []Use {
	called, named := node.Called()
	if !named {
		return nil
	}
	module, loaded := modules(called)
	if !loaded {
		return nil
	}
	var uses []Use
	for _, input := range module.Inputs() {
		if ref, isPlace := placeOf(input, node.Inputs()[input.ID]); isPlace {
			uses = append(uses, Use{Node: node.ID, What: "Call " + names(called) + " · " + input.Name, Ref: ref})
		}
	}

	return uses
}

// placeOf reads a place a declaration of the place type holds or is given.
func placeOf(declared worldstate.Declaration, raw []byte) (floor.AnchorRef, bool) {
	if declared.Type != worldstate.TypePlace || len(raw) == 0 {
		return floor.AnchorRef{Floor: ids.ULID{}, Anchor: ids.ULID{}}, false
	}
	value, err := declared.Parse(raw)
	if err != nil || value.Place.IsZero() {
		return floor.AnchorRef{Floor: ids.ULID{}, Anchor: ids.ULID{}}, false
	}

	return floor.AnchorRef{Floor: value.Place.Floor, Anchor: value.Place.Anchor}, true
}

// ZoneOf returns the zone listing a floor, among the zones the window has.
func ZoneOf(docs frame.Docs, floorID ids.ULID) (ids.ULID, bool) {
	index := indexOf(docs)
	for _, entry := range index.OfFormat(zone.FormatName) {
		found := false
		docs.View(entry.ID, func(document client.Document) {
			if zoneDoc, ok := document.Model.(*zoneplugin.Document); ok {
				for _, listed := range zoneDoc.Zone.Floors() {
					found = found || listed == floorID
				}
			}
		})
		if found {
			return entry.ID, true
		}
	}

	return ids.ULID{}, false
}

// Callees lists the workflows a workflow runs: itself, then every module
// its Calls call, through any number of Calls, as far as the window has
// them; want is asked for each one not loaded yet.
func Callees(start ids.ULID, modules Modules, want func(ids.ULID)) []ids.ULID {
	runs := []ids.ULID{start}
	for index := 0; index < len(runs); index++ {
		workflow, loaded := modules(runs[index])
		if !loaded {
			want(runs[index])

			continue
		}
		for _, called := range workflow.Callees() {
			if !slices.Contains(runs, called) {
				runs = append(runs, called)
			}
		}
	}

	return runs
}
