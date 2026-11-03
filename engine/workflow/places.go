package workflow

import (
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/tilefloor"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// PlaceUse is a fixed place a workflow names itself: a Go to's place on a
// map, a place a Call gives one of its module's place inputs, or a place
// local's start.
type PlaceUse struct {
	// Node is the Go to or Call naming the place; zero for a local's start.
	Node ids.ULID
	// Called is the module a Call gives the place to, and Input the name of
	// the input it fills; Local names the local the place starts.
	Called ids.ULID
	Input  string
	Local  string
	Place  tilefloor.AnchorRef
}

// Modules finds a workflow a Call calls, when it is at hand.
type Modules func(ids.ULID) (*Workflow, bool)

// PlacesIn lists the fixed places a workflow names itself: each Go to's
// place on a map, each place a Call gives its module's place inputs, when
// the module is at hand, and each place local's start. A place reached
// through a place value is its local's start, listed once; a module's own
// places are the module's.
func PlacesIn(workflow *Workflow, modules Modules) []PlaceUse {
	var uses []PlaceUse
	for _, node := range workflow.Nodes() {
		switch node.Kind {
		case KindGoTo:
			goes, err := node.Choice(ParamGoes)
			if err == nil && goes == GoesToValue {
				continue
			}
			if destination, err := node.Anchor("destination"); err == nil && !destination.IsZero() {
				uses = append(uses, PlaceUse{Node: node.ID, Called: ids.ULID{}, Input: "", Local: "", Place: destination})
			}
		case KindCall:
			uses = append(uses, callPlaces(node, modules)...)
		}
	}
	for _, declared := range workflow.Locals().Values() {
		if place, isPlace := placeOf(declared, declared.Default); isPlace {
			uses = append(uses, PlaceUse{Node: ids.ULID{}, Called: ids.ULID{}, Input: "", Local: declared.Name, Place: place})
		}
	}

	return uses
}

// callPlaces lists the places a Call gives its module's place inputs.
func callPlaces(node Node, modules Modules) []PlaceUse {
	called, named := node.Called()
	if !named {
		return nil
	}
	module, loaded := modules(called)
	if !loaded {
		return nil
	}
	var uses []PlaceUse
	for _, input := range module.Inputs() {
		if place, isPlace := placeOf(input, node.Inputs()[input.ID]); isPlace {
			uses = append(uses, PlaceUse{Node: node.ID, Called: called, Input: input.Name, Local: "", Place: place})
		}
	}

	return uses
}

// placeOf reads a place a declaration of the place type holds or is given.
func placeOf(declared worldstate.Declaration, raw []byte) (tilefloor.AnchorRef, bool) {
	none := tilefloor.AnchorRef{Floor: ids.ULID{}, Anchor: ids.ULID{}}
	if declared.Type != worldstate.TypePlace || len(raw) == 0 {
		return none, false
	}
	value, err := declared.Parse(raw)
	if err != nil || value.Place.IsZero() {
		return none, false
	}

	return tilefloor.AnchorRef{Floor: value.Place.Floor, Anchor: value.Place.Anchor}, true
}
