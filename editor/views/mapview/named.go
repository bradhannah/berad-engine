package mapview

import (
	"strings"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/ids"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
)

// A line in History selects what it names (increment 45): a Map finds the
// NPC placement or anchor a change to one of its zone's floors touched, and
// shows it when the line is clicked, on its floor with its tool in hand.

// AnchorItem prefixes the item a map is asked to show an anchor by, as
// "anchor:<floor>:<anchor>".
const AnchorItem = "anchor:"

// Named is the first NPC placement or anchor a change to one of the zone's
// floors touched that is still there, as the item that shows it, and its
// name.
func (s *State) Named(touched protocol.Touch, docs frame.Docs) (string, string, bool) {
	if indexOf(s.floors, touched.Document) < 0 {
		return "", "", false
	}
	item, name := "", ""
	docs.View(touched.Document, func(document client.Document) {
		if floorDoc, ok := document.Model.(*tilefloor.Document); ok {
			item, name = s.namedOn(floorDoc.Floor, touched)
		}
	})

	return item, name, item != ""
}

// namedOn finds the first placement or anchor of those touched still on a
// floor.
func (s *State) namedOn(floorData *floor.Floor, touched protocol.Touch) (string, string) {
	where := touched.Document.String() + ":"
	for _, key := range touched.Items {
		if placementID, ok := tilefloor.PlacementOfKey(key); ok {
			if placement, _, found := floorData.Placement(placementID); found {
				return PlacementItem + where + placementID.String(), s.names.Name(placement.NPC, "an NPC")
			}
		}
		if anchorID, ok := tilefloor.AnchorOfKey(key); ok {
			if anchor, _, found := floorData.Anchor(anchorID); found {
				return AnchorItem + where + anchorID.String(), anchorName(anchor.Name)
			}
		}
	}

	return "", ""
}

// anchorName is how a line names an anchor.
func anchorName(name string) string {
	if name == "" {
		return "an anchor"
	}

	return "the anchor " + name
}

// floorAndID reads the floor and the ID from an item shown with a prefix.
func (s *State) floorAndID(item, prefix string) (ids.ULID, ids.ULID, bool) {
	floorPart, idPart, _ := strings.Cut(strings.TrimPrefix(item, prefix), ":")
	floorID, floorErr := ids.Parse(floorPart)
	id, idErr := ids.Parse(idPart)

	return floorID, id, floorErr == nil && idErr == nil && indexOf(s.floors, floorID) >= 0
}

// showAnchor shows the floor an anchor is on and selects it, with the
// Anchors tool.
func (s *State) showAnchor(item string, ctx *frame.Context) {
	floorID, anchorID, ok := s.floorAndID(item, AnchorItem)
	if !ok {
		return
	}
	s.switchFloor(floorID, ctx)
	s.setTool(ToolAnchor)
	s.selectAnchor(anchorID)
}
