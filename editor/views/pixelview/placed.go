package pixelview

import (
	"slices"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	zoneplugin "github.com/bradhannah/berad-engine/editor/plugins/zone"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/ids"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
	"github.com/bradhannah/berad-engine/engine/zone"
)

// zonesAndFloors returns every zone of the project and each one's floors, so
// the window can hold them to see where the sprite is placed.
func zonesAndFloors(docs frame.Docs) []ids.ULID {
	var held []ids.ULID
	docs.View(project.IndexID(), func(document client.Document) {
		if index, ok := document.Model.(*project.Index); ok {
			for _, entry := range index.OfFormat(zone.FormatName) {
				held = append(held, entry.ID)
			}
		}
	})
	for _, zoneID := range slices.Clone(held) {
		docs.View(zoneID, func(document client.Document) {
			if zoneDoc, ok := document.Model.(*zoneplugin.Document); ok {
				held = append(held, zoneDoc.Zone.Floors()...)
			}
		})
	}

	return held
}

// placedPalettes returns the palettes listed by every zone in which a
// stand-in the sprite shows is placed, merged: they come first in the
// panel. A sprite placed in no zone has none.
func placedPalettes(docs frame.Docs, standIn string) []ids.ULID {
	if standIn == "" {
		return nil
	}
	var zones []ids.ULID
	docs.View(project.IndexID(), func(document client.Document) {
		if index, ok := document.Model.(*project.Index); ok {
			for _, entry := range index.OfFormat(zone.FormatName) {
				zones = append(zones, entry.ID)
			}
		}
	})
	var listed []ids.ULID
	for _, zoneID := range zones {
		var floors, palettes []ids.ULID
		docs.View(zoneID, func(document client.Document) {
			if zoneDoc, ok := document.Model.(*zoneplugin.Document); ok {
				floors, palettes = zoneDoc.Zone.Floors(), zoneDoc.Zone.Palettes()
			}
		})
		if !slices.ContainsFunc(floors, func(floorID ids.ULID) bool { return floorShows(docs, floorID, standIn) }) {
			continue
		}
		for _, paletteID := range palettes {
			if !slices.Contains(listed, paletteID) {
				listed = append(listed, paletteID)
			}
		}
	}

	return listed
}

// floorShows reports whether any cell of a floor holds a stand-in, on the
// ground or on top.
func floorShows(docs frame.Docs, floorID ids.ULID, standIn string) bool {
	shows := false
	docs.View(floorID, func(document client.Document) {
		floorDoc, ok := document.Model.(*tilefloor.Document)
		if ok {
			shows = holds(floorDoc.Floor, standIn)
		}
	})

	return shows
}

func holds(grid *floor.Floor, standIn string) bool {
	bounds := grid.Bounds()
	for row := bounds.Min.Y; row < bounds.Max.Y; row++ {
		for col := bounds.Min.X; col < bounds.Max.X; col++ {
			ground, groundErr := grid.Ground(col, row)
			top, topErr := grid.Top(col, row)
			if (groundErr == nil && ground == standIn) || (topErr == nil && top == standIn) {
				return true
			}
		}
	}

	return false
}
