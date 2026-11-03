package places

import (
	"slices"

	"github.com/bradhannah/berad-engine/editor/client"
	zoneplugin "github.com/bradhannah/berad-engine/editor/plugins/zone"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/ids"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
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
type Modules = flow.Modules

// UsesIn lists the places a workflow names itself, as flow.PlacesIn does,
// each saying what names it.
func UsesIn(workflow *flow.Workflow, modules Modules, names func(ids.ULID) string) []Use {
	found := flow.PlacesIn(workflow, modules)
	uses := make([]Use, 0, len(found))
	for _, each := range found {
		what := "Go to"
		switch {
		case each.Local != "":
			what = "local " + each.Local
		case !each.Called.IsZero():
			what = "Call " + names(each.Called) + " · " + each.Input
		}
		uses = append(uses, Use{Node: each.Node, What: what, Ref: each.Place})
	}

	return uses
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
