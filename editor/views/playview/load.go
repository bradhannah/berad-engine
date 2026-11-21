package playview

import (
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/views/docsource"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/game"
	"github.com/bradhannah/berad-engine/engine/gameload"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// A launch reads what the engine's gameload package gathers for a game: the
// game settings, the start's zone and its floors, the NPCs placed on them,
// their behaviours and the workflows those call, the documents the project's
// values are declared in, and the art they show (increment 62). The tab
// subscribes to each document as the launch asks for it, and launches once
// all have loaded. A document deleted since something named it is skipped.

// prepare gathers what a game needs with the player at the player start, or
// at from when it is given, subscribing to what has not loaded, and reports
// whether everything has.
func (s *State) prepare(ctx *frame.Context, from *game.Start) (gameload.Loaded, bool) {
	s.subscribe(project.IndexID(), ctx)
	s.subscribe(worldstate.ID(), ctx)
	docs := docsource.New(ctx.Docs)
	loaded, ready := gameload.Load(docs.Asking(func(documentID ids.ULID) { s.subscribe(documentID, ctx) }), from)
	// The game names things from the window's documents as they are when it
	// asks, without subscribing to any.
	loaded.Project.Names = gameload.NewNames(docs)

	return loaded, ready && docs.Indexed()
}

func (s *State) subscribe(documentID ids.ULID, ctx *frame.Context) {
	if s.subs[documentID] {
		return
	}
	s.subs[documentID] = true
	ctx.Requests.Subscribe(documentID, func(err error) {
		if err != nil {
			s.subs[documentID] = false
		}
	})
}
