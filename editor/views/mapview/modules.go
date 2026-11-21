package mapview

import (
	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/game"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// nameAnchors names the anchors of the zone's floors, one floor at a time.
func (s *State) nameAnchors(docs frame.Docs) {
	for _, floorID := range s.floors {
		docs.View(floorID, func(document client.Document) {
			if floorDoc, ok := document.Model.(*tilefloor.Document); ok {
				for _, anchor := range floorDoc.Floor.Anchors() {
					s.preview.anchors[anchor.ID] = anchor.Name
				}
			}
		})
	}
}

// running is the workflow an NPC is following now: its behaviour, or the
// workflow it is in a call to, nil when the game holds none.
func (s *State) running(run game.Runner) *flow.Workflow {
	if run.NPC.Module().IsZero() {
		return run.Workflow
	}
	held, _ := s.preview.game.Workflow(run.NPC.Module())

	return held
}
