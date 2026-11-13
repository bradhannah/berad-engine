package pixelview

import (
	"strconv"
	"strings"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/views/frame"
)

// The status bar names what is in hand in the Pixel editor (increment 49),
// as it does in a Map and a Workflow: the sprite, the tool, the colour it
// paints with, and the pixel the pointer is over with its colour.

// spriteName is the sprite's name, or "Sprite" until the project's index
// has it.
func (s *State) spriteName(docs frame.Docs) string {
	name := "Sprite"
	docs.View(project.IndexID(), func(document client.Document) {
		if index, ok := document.Model.(*project.Index); ok {
			name = index.Name(s.sprite, name)
		}
	})

	return name
}

// toolLabel is a tool's name, as the toolbar shows it.
func toolLabel(tool Tool) string {
	for _, info := range tools() {
		if info.tool == tool {
			return info.label
		}
	}

	return ""
}

// handLine is what the status bar says is in hand.
func (s *State) handLine(docs frame.Docs) string {
	parts := []string{s.spriteName(docs), toolLabel(s.tool)}
	if s.tool == ToolPencil || s.tool == ToolFill {
		if name, named := s.colorName(docs, s.color); named {
			parts = append(parts, name)
		}
	}
	if s.hovering {
		over := "pixel " + strconv.Itoa(s.hover.X) + ", " + strconv.Itoa(s.hover.Y)
		if ref, read := s.pixelColor(docs, s.hover); read {
			if name, named := s.colorName(docs, ref); named {
				over += ": " + name
			}
		}
		parts = append(parts, over)
	}

	return strings.Join(parts, " · ")
}
