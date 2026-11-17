package places

import (
	"github.com/bradhannah/berad-engine/editor/views/docsource"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/gameload"
)

// Names names zones, workflows and sprites, and places, from what a window
// holds, for values' text forms: a place as Counter, Ground floor, Market.
// The engine names them, as it does for a game (increment 62).
type Names = gameload.Names

// NewNames names from a window's documents.
func NewNames(docs frame.Docs) Names { return gameload.NewNames(docsource.New(docs)) }
