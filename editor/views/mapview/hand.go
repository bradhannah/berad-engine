package mapview

import (
	"fmt"
	"strings"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	"github.com/bradhannah/berad-engine/editor/views/frame"
)

// The status bar always names the tool in hand (increment 39): the floor,
// the tool and the tile it paints with, and what the pointer is over, so a
// folded panel never hides what the next click will do.

// handLine is what the status bar says is in hand.
func (s *State) handLine(docs frame.Docs) string {
	parts := []string{s.floorName(docs, s.floor)}
	switch {
	case s.preview != nil:
		parts = append(parts, "Previewing")
	case s.pick != nil:
		parts = append(parts, "Picking on the map")
	default:
		parts = append(parts, toolLabel(s.tool))
		if paints(s.tool) {
			parts = append(parts, tileName(s.tile[s.tool]))
		}
	}
	if over := s.pointerOver(docs); over != "" {
		parts = append(parts, over)
	}

	return strings.Join(parts, " · ")
}

// toolLabel is a tool's name, as the tool list shows it.
func toolLabel(tool Tool) string {
	for _, each := range tools() {
		if each.tool == tool {
			return each.label
		}
	}

	return ""
}

// paints reports whether a tool paints with a tile.
func paints(tool Tool) bool {
	return tool == ToolGround || tool == ToolWalls || tool == ToolObjects
}

// pointerOver says what the pointer is over: a cell and what shows there,
// or that it is outside the floor; "" off the canvas.
func (s *State) pointerOver(docs frame.Docs) string {
	if s.outside {
		return "outside the floor"
	}
	if !s.hovering {
		return ""
	}
	over := fmt.Sprintf("cell %d, %d", s.hover.X, s.hover.Y)
	docs.View(s.floor, func(document client.Document) {
		floorDoc, ok := document.Model.(*tilefloor.Document)
		if !ok {
			return
		}
		ground, _ := floorDoc.Floor.Ground(s.hover.X, s.hover.Y) //nolint:errcheck // The hover cell is on the floor.
		top, _ := floorDoc.Floor.Top(s.hover.X, s.hover.Y)       //nolint:errcheck // The hover cell is on the floor.
		shown := ground
		if top != "" {
			shown = top
		}
		over += ": " + tileName(shown)
	})

	return over
}
