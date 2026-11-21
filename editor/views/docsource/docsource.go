// Package docsource shows a window's documents to the engine's gameload
// package as its Source (increment 62), so the views that gather what a game
// plays, the values a project declares and the names of its places do it by
// the engine's rules, the ones a game reading a project folder follows.
package docsource

import (
	"slices"

	"github.com/bradhannah/berad-engine/editor/client"
	"github.com/bradhannah/berad-engine/editor/model"
	artplugin "github.com/bradhannah/berad-engine/editor/plugins/art"
	gameplugin "github.com/bradhannah/berad-engine/editor/plugins/game"
	npcplugin "github.com/bradhannah/berad-engine/editor/plugins/npc"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	stateplugin "github.com/bradhannah/berad-engine/editor/plugins/worldstate"
	zoneplugin "github.com/bradhannah/berad-engine/editor/plugins/zone"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/engine/art"
	"github.com/bradhannah/berad-engine/engine/game"
	"github.com/bradhannah/berad-engine/engine/gameload"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// Source is a window's documents as gameload reads them. A document the
// project index does not list is missing, unless every project holds it; one
// it lists that the window has not been sent yet is waited for. Until the
// index itself arrives nothing is missing, and nothing is listed.
type Source struct {
	docs frame.Docs
}

// New shows a window's documents as a source.
func New(docs frame.Docs) Source { return Source{docs: docs} }

// Indexed reports whether the project index has arrived.
func (s Source) Indexed() bool {
	return s.docs.View(project.IndexID(), func(client.Document) {})
}

// View shows a document the window holds for as long as show runs.
func (s Source) View(documentID ids.ULID, show func(gameload.Document)) gameload.State {
	listed, name := true, ""
	s.index(func(index *project.Index) {
		entry, held := index.Entry(documentID)
		listed, name = held || everyProjectHolds(documentID), entry.Name
	})
	if !listed {
		return gameload.Missing
	}
	shown := s.docs.View(documentID, func(document client.Document) {
		if name == "" {
			name = document.Name
		}
		show(gameload.Document{Name: name, Value: valueOf(document.Model)})
	})
	if !shown {
		return gameload.Waiting
	}

	return gameload.Held
}

// Name names a document as the project index does, "" for one it lacks.
func (s Source) Name(documentID ids.ULID) string {
	name := ""
	s.index(func(index *project.Index) { name = index.Name(documentID, "") })

	return name
}

// List lists the documents of a format, ordered by name.
func (s Source) List(format string) []ids.ULID {
	var listed []ids.ULID
	s.index(func(index *project.Index) {
		for _, entry := range index.OfFormat(format) {
			listed = append(listed, entry.ID)
		}
	})

	return listed
}

// index shows the project index, once it has arrived.
func (s Source) index(fn func(*project.Index)) {
	s.docs.View(project.IndexID(), func(document client.Document) {
		if index, ok := document.Model.(*project.Index); ok {
			fn(index)
		}
	})
}

// everyProjectHolds reports a document every project holds, which the index
// may not list.
func everyProjectHolds(documentID ids.ULID) bool {
	return documentID == game.SettingsID() || documentID == art.SettingsID() || documentID == art.StandInsID() ||
		documentID == worldstate.ID()
}

// valueOf is a document's content as the engine reads it, or nil for a
// document a game does not read.
func valueOf(document model.Document) any {
	switch held := document.(type) {
	case *gameplugin.SettingsDocument:
		return held.Settings
	case *zoneplugin.Document:
		return held.Zone
	case *tilefloor.Document:
		return held.Floor
	case *npcplugin.Document:
		return held.NPC
	case *workflowplugin.Document:
		return held.Workflow
	case *stateplugin.Document:
		return held.State
	}

	return artOf(document)
}

// artOf is an art document's content as the engine reads it.
func artOf(document model.Document) any {
	switch held := document.(type) {
	case *artplugin.SettingsDocument:
		return held.Settings
	case *artplugin.StandInsDocument:
		return held.Sprites
	case *artplugin.SpriteDocument:
		return held.Sprite
	case *artplugin.TileDocument:
		return held.Tile
	case *artplugin.PaletteDocument:
		return held.Palette
	}

	return nil
}

// Asked is a window's documents for something that gathers a game from
// them: each document asked for that the project holds is subscribed to, so
// it arrives, and noted as it was when first asked for, so that a change to
// it, or its arrival, can be seen later with Stamp. A window's documents
// arrive while it works, so what a gathering saw is noted as it looks, not
// read again afterwards.
type Asked struct {
	Source

	subscribe func(ids.ULID)
	seen      map[ids.ULID]uint64
	index     uint64
}

// Asking shows a window's documents, subscribing to each one asked for
// that the project holds.
func (s Source) Asking(subscribe func(ids.ULID)) Asked {
	return Asked{Source: s, subscribe: subscribe, seen: map[ids.ULID]uint64{}, index: s.mark(project.IndexID())}
}

// mark is a document as it is now, for telling when it changes: zero while
// the window does not hold it, and otherwise one more than its revision.
func (s Source) mark(documentID ids.ULID) uint64 {
	mark := uint64(0)
	s.docs.View(documentID, func(document client.Document) { mark = document.Revision + 1 })

	return mark
}

// View shows a document, subscribing to it when the project holds it.
func (a Asked) View(documentID ids.ULID, show func(gameload.Document)) gameload.State {
	// The mark is taken before the document is shown, so a document that
	// arrives in between is seen as changed, never missed.
	mark := a.mark(documentID)
	held := a.Source.View(documentID, show)
	if held != gameload.Missing {
		a.subscribe(documentID)
		if _, noted := a.seen[documentID]; !noted {
			a.seen[documentID] = mark
		}
	}

	return held
}

// Seen lists the documents asked for so far that the project holds, in
// order, and a stamp of what they and the project index were when first
// asked for: Stamp gives another once any of them has changed or arrived.
func (a Asked) Seen() ([]ids.ULID, uint64) {
	read, stamp := make([]ids.ULID, 0, len(a.seen)), a.index
	for documentID, mark := range a.seen {
		read = append(read, documentID)
		stamp += mark
	}
	slices.SortFunc(read, func(a, b ids.ULID) int { return a.Compare(b) })

	return read, stamp
}

// Stamp is a stamp of some documents and the project index as they are
// now, to compare with the one a gathering saw.
func (s Source) Stamp(documents []ids.ULID) uint64 {
	stamp := s.mark(project.IndexID())
	for _, documentID := range documents {
		stamp += s.mark(documentID)
	}

	return stamp
}
