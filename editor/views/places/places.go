// Package places lists the places NPCs can go to, the anchors on the floors
// of a project's zones, and has the box that shows a place in an inspector
// and the list that chooses one by name. A place is an anchor on a map, and
// both are marked by the ring the map draws for an anchor.
package places

import (
	"cmp"
	"slices"
	"strings"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	zoneplugin "github.com/bradhannah/berad-engine/editor/plugins/zone"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/ids"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
	"github.com/bradhannah/berad-engine/engine/zone"
)

// whereSeparator joins a zone's name to a floor's.
const whereSeparator = " · "

// Place is one anchor: where it is, its name, and its zone and floor, as
// "Sample zone · Ground floor".
type Place struct {
	Ref   floor.AnchorRef
	Name  string
	Where string
}

// ID names a place for a list row, as its floor and anchor.
func ID(ref floor.AnchorRef) string { return ref.Floor.String() + ":" + ref.Anchor.String() }

// Parse reads a place's ID back.
func Parse(id string) (floor.AnchorRef, bool) {
	floorPart, anchorPart, cut := strings.Cut(id, ":")
	floorID, floorErr := ids.Parse(floorPart)
	anchorID, anchorErr := ids.Parse(anchorPart)
	if !cut || floorErr != nil || anchorErr != nil {
		return floor.AnchorRef{Floor: ids.ULID{}, Anchor: ids.ULID{}}, false
	}

	return floor.AnchorRef{Floor: floorID, Anchor: anchorID}, true
}

// All lists every place: zones by name, each zone's floors from the bottom
// up, and each floor's anchors by name. subscribe is asked for every zone
// and floor, so the list fills in as they load.
func All(docs frame.Docs, subscribe func(ids.ULID)) []Place {
	index := indexOf(docs)
	var found []Place
	for _, entry := range zonesByName(index) {
		subscribe(entry.ID)
		for _, floorID := range floorsOf(docs, entry.ID) {
			subscribe(floorID)
			where := entry.Name + whereSeparator + index.Name(floorID, "a floor")
			anchors := anchorsOf(docs, floorID)
			slices.SortStableFunc(anchors, func(a, b floor.Anchor) int { return cmp.Compare(a.Name, b.Name) })
			for _, anchor := range anchors {
				found = append(found, Place{
					Ref: floor.AnchorRef{Floor: floorID, Anchor: anchor.ID}, Name: anchor.Name, Where: where,
				})
			}
		}
	}

	return found
}

// Where names a floor's zone and the floor, as "Sample zone · Ground
// floor", or the floor alone while its zone is not loaded.
func Where(docs frame.Docs, floorID ids.ULID) string {
	index := indexOf(docs)
	floorName := index.Name(floorID, "")
	for _, entry := range zonesByName(index) {
		if slices.Contains(floorsOf(docs, entry.ID), floorID) {
			return entry.Name + whereSeparator + floorName
		}
	}

	return floorName
}

// indexOf returns a copy of the project index.
func indexOf(docs frame.Docs) *project.Index {
	found := project.NewIndex(nil)
	docs.View(project.IndexID(), func(document client.Document) {
		if index, ok := document.Model.(*project.Index); ok {
			found = project.NewIndex(index.Entries())
		}
	})

	return found
}

// zonesByName lists the project's zones by name.
func zonesByName(index *project.Index) []project.Entry {
	zones := index.OfFormat(zone.FormatName)
	slices.SortStableFunc(zones, func(a, b project.Entry) int { return cmp.Compare(a.Name, b.Name) })

	return zones
}

// floorsOf lists a loaded zone's floors from the bottom up.
func floorsOf(docs frame.Docs, zoneID ids.ULID) []ids.ULID {
	var floors []ids.ULID
	docs.View(zoneID, func(document client.Document) {
		if zoneDoc, ok := document.Model.(*zoneplugin.Document); ok {
			floors = zoneDoc.Zone.Floors()
		}
	})

	return floors
}

// anchorsOf lists a loaded floor's anchors.
func anchorsOf(docs frame.Docs, floorID ids.ULID) []floor.Anchor {
	var anchors []floor.Anchor
	docs.View(floorID, func(document client.Document) {
		if floorDoc, ok := document.Model.(*tilefloor.Document); ok {
			anchors = floorDoc.Floor.Anchors()
		}
	})

	return anchors
}
