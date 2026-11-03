package zonepicker

import (
	"cmp"
	"image"
	"slices"

	"github.com/bradhannah/berad-engine/editor/client"
	npcplugin "github.com/bradhannah/berad-engine/editor/plugins/npc"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	zoneplugin "github.com/bradhannah/berad-engine/editor/plugins/zone"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/npc"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
	"github.com/bradhannah/berad-engine/engine/zone"
)

// Zone is one zone the picker lists: its name, the folder holding it, as
// "Coast/Caves", and its tags; its floors' names from the bottom up, the
// size of its largest floor in tiles, the NPCs that live in it and its
// places, each by name; its notes; and its ground floor, drawn small, with
// that floor's revision. Refused, when set, says why it cannot be chosen.
type Zone struct {
	ID      ids.ULID
	Name    string
	Folder  string
	Tags    []string
	Floors  []string
	Size    image.Point
	Living  []string
	Places  []string
	Notes   string
	Ground  *floor.Floor
	Drawn   uint64
	Refused string
}

// Folders are the folders of the project's zones, empty ones included, each
// before what it holds.
func Folders(docs frame.Docs) []string {
	var folders []string
	docs.View(project.IndexID(), func(document client.Document) {
		if index, ok := document.Model.(*project.Index); ok {
			folders = index.Folders(zone.FormatName)
		}
	})

	return folders
}

// Gather lists the project's zones by name, as far as the window has them.
// subscribe is asked for every zone, its floors and every NPC, so the list
// fills in as they load.
func Gather(docs frame.Docs, subscribe func(ids.ULID)) []Zone {
	index := project.NewIndex(nil)
	docs.View(project.IndexID(), func(document client.Document) {
		if held, ok := document.Model.(*project.Index); ok {
			index = project.NewIndex(held.Entries())
		}
	})
	living := livingIn(docs, index, subscribe)
	entries := index.OfFormat(zone.FormatName)
	slices.SortStableFunc(entries, func(a, b project.Entry) int { return cmp.Compare(a.Name, b.Name) })
	zones := make([]Zone, 0, len(entries))
	for _, entry := range entries {
		subscribe(entry.ID)
		gathered := Zone{
			ID: entry.ID, Name: entry.Name, Folder: entry.Folder, Tags: entry.Tags, Floors: nil, Size: image.Pt(0, 0),
			Living: living[entry.ID], Places: nil, Notes: "", Ground: nil, Drawn: 0, Refused: "",
		}
		var floors []ids.ULID
		docs.View(entry.ID, func(document client.Document) {
			if zoneDoc, ok := document.Model.(*zoneplugin.Document); ok {
				floors, gathered.Notes = zoneDoc.Zone.Floors(), zoneDoc.Zone.Notes
			}
		})
		for number, floorID := range floors {
			subscribe(floorID)
			gathered.Floors = append(gathered.Floors, index.Name(floorID, "a floor"))
			gatherFloor(docs, floorID, number == 0, &gathered)
		}
		slices.Sort(gathered.Places)
		zones = append(zones, gathered)
	}

	return zones
}

// gatherFloor adds a floor's size and places to its zone, and draws the
// ground floor.
func gatherFloor(docs frame.Docs, floorID ids.ULID, ground bool, gathered *Zone) {
	docs.View(floorID, func(document client.Document) {
		floorDoc, ok := document.Model.(*tilefloor.Document)
		if !ok {
			return
		}
		size := floorDoc.Floor.Bounds().Size()
		gathered.Size = image.Pt(max(gathered.Size.X, size.X), max(gathered.Size.Y, size.Y))
		for _, anchor := range floorDoc.Floor.Anchors() {
			gathered.Places = append(gathered.Places, anchor.Name)
		}
		if ground {
			gathered.Ground, gathered.Drawn = floorDoc.Floor, document.Revision
		}
	})
}

// livingIn maps each zone to the names of the NPCs that live in it.
func livingIn(docs frame.Docs, index *project.Index, subscribe func(ids.ULID)) map[ids.ULID][]string {
	living := map[ids.ULID][]string{}
	for _, entry := range index.OfFormat(npc.FormatName) {
		subscribe(entry.ID)
		docs.View(entry.ID, func(document client.Document) {
			if npcDoc, ok := document.Model.(*npcplugin.Document); ok && !npcDoc.NPC.Zone.IsZero() {
				living[npcDoc.NPC.Zone] = append(living[npcDoc.NPC.Zone], entry.Name)
			}
		})
	}
	for zoneID := range living {
		slices.Sort(living[zoneID])
	}

	return living
}
