package places

import (
	"strings"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// Names names zones, workflows and sprites, and places, from what a window
// holds, for values' text forms: a place as Counter, Ground floor, Market.
type Names struct {
	docs frame.Docs
}

// NewNames names from a window's documents.
func NewNames(docs frame.Docs) Names { return Names{docs: docs} }

// Document names a document, or returns "" for one the project lacks.
func (n Names) Document(documentID ids.ULID) string { return indexOf(n.docs).Name(documentID, "") }

// Place names a place by its anchor, floor and zone, or returns "" for one
// whose anchor the window does not have.
func (n Names) Place(place worldstate.Place) string {
	anchor := ""
	n.docs.View(place.Floor, func(document client.Document) {
		if floorDoc, ok := document.Model.(*tilefloor.Document); ok {
			if found, _, held := floorDoc.Floor.Anchor(place.Anchor); held {
				anchor = found.Name
			}
		}
	})
	if anchor == "" {
		return ""
	}
	index := indexOf(n.docs)
	parts := []string{anchor}
	if floorName := index.Name(place.Floor, ""); floorName != "" {
		parts = append(parts, floorName)
	}
	if zoneID, known := ZoneOf(n.docs, place.Floor); known {
		parts = append(parts, index.Name(zoneID, ""))
	}

	return strings.Join(parts, ", ")
}
