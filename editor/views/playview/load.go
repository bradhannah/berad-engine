package playview

import (
	"encoding/json"

	"github.com/bradhannah/berad-engine/editor/client"
	artplugin "github.com/bradhannah/berad-engine/editor/plugins/art"
	gameplugin "github.com/bradhannah/berad-engine/editor/plugins/game"
	npcplugin "github.com/bradhannah/berad-engine/editor/plugins/npc"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	workflowplugin "github.com/bradhannah/berad-engine/editor/plugins/workflow"
	zoneplugin "github.com/bradhannah/berad-engine/editor/plugins/zone"
	"github.com/bradhannah/berad-engine/editor/views/frame"
	"github.com/bradhannah/berad-engine/editor/views/places"
	"github.com/bradhannah/berad-engine/editor/views/signals"
	"github.com/bradhannah/berad-engine/editor/views/tileart"
	"github.com/bradhannah/berad-engine/editor/views/valuetree"
	"github.com/bradhannah/berad-engine/engine/art"
	"github.com/bradhannah/berad-engine/engine/game"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/npc"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/worldstate"
)

// A launch reads, one document at a time: the game settings, the start's
// zone and its floors, the NPCs placed on them, their behaviours and the
// workflows those call, the documents the project's values are declared in,
// and the art they show. It subscribes to each as it finds it, and launches
// once all have loaded. A document deleted since something named it is skipped.

// loader gathers a launch's documents.
type loader struct {
	state *State
	ctx   *frame.Context
	index *project.Index
	ready bool
}

// prepare reads what a launch needs, subscribing to what has not loaded, and
// returns the project with the tile size once everything has.
func (s *State) prepare(ctx *frame.Context) (game.Project, int, bool) {
	s.subscribe(project.IndexID(), ctx)
	snapshot := game.Project{
		Settings: game.DefaultSettings(), Zone: nil, Floors: map[ids.ULID]*floor.Floor{},
		NPCs: map[ids.ULID]npc.NPC{}, Workflows: map[ids.ULID]*flow.Workflow{}, Values: nil, Seconds: 0, Seed: 0,
		Names: places.NewNames(ctx.Docs), Signals: nil,
	}
	load := &loader{state: s, ctx: ctx, index: nil, ready: true}
	ctx.Docs.View(project.IndexID(), func(document client.Document) {
		if found, ok := document.Model.(*project.Index); ok {
			load.index = found
		}
	})
	if load.index == nil {
		return snapshot, 0, false
	}
	load.settings(&snapshot)
	load.zone(&snapshot)
	load.npcs(&snapshot)
	load.values(&snapshot)
	tile := load.art()
	s.art.want = load.sprites(snapshot.Workflows)

	return snapshot, tile, load.ready
}

// need subscribes to a document the project holds and reports whether it has
// loaded; one it does not hold is skipped.
func (l *loader) need(documentID ids.ULID) bool {
	if documentID.IsZero() {
		return false
	}
	if _, held := l.index.Entry(documentID); !held && !isRequired(documentID) {
		return false
	}
	l.state.subscribe(documentID, l.ctx)
	if !l.ctx.Docs.View(documentID, func(client.Document) {}) {
		l.ready = false

		return false
	}

	return true
}

// isRequired reports a document every project holds, which the index may not
// list.
func isRequired(documentID ids.ULID) bool {
	return documentID == game.SettingsID() || documentID == art.SettingsID() || documentID == art.StandInsID()
}

func (l *loader) settings(project *game.Project) {
	if !l.need(game.SettingsID()) {
		return
	}
	l.ctx.Docs.View(game.SettingsID(), func(document client.Document) {
		if settings, ok := document.Model.(*gameplugin.SettingsDocument); ok {
			project.Settings = settings.Settings
		}
	})
	if l.state.from != nil {
		project.Settings.Start = *l.state.from
	}
}

// zone reads the start's zone and its floors.
func (l *loader) zone(project *game.Project) {
	start := project.Settings.Start
	if start.IsZero() || !l.need(start.Zone) {
		return
	}
	l.ctx.Docs.View(start.Zone, func(document client.Document) {
		if zoneDoc, ok := document.Model.(*zoneplugin.Document); ok {
			project.Zone = zoneDoc.Zone.Clone()
		}
	})
	if project.Zone == nil {
		return
	}
	for _, floorID := range project.Zone.Floors() {
		if !l.need(floorID) {
			continue
		}
		l.ctx.Docs.View(floorID, func(document client.Document) {
			if floorDoc, ok := document.Model.(*tilefloor.Document); ok {
				project.Floors[floorID] = floorDoc.Floor.Clone()
			}
		})
	}
}

// npcs reads the NPCs placed on the zone's floors, their behaviours, the
// zone's loop and the workflows those call, in turn.
func (l *loader) npcs(project *game.Project) {
	for _, placed := range project.Floors {
		for _, placement := range placed.Placements() {
			if _, read := project.NPCs[placement.NPC]; read || !l.need(placement.NPC) {
				continue
			}
			l.ctx.Docs.View(placement.NPC, func(document client.Document) {
				if npcDoc, ok := document.Model.(*npcplugin.Document); ok {
					project.NPCs[placement.NPC] = npcDoc.NPC
				}
			})
		}
	}
	var waiting []ids.ULID
	for _, definition := range project.NPCs {
		waiting = append(waiting, definition.Behaviour)
	}
	if project.Zone != nil {
		waiting = append(waiting, project.Zone.Loop)
	}
	for len(waiting) > 0 {
		workflowID := waiting[0]
		waiting = waiting[1:]
		if _, read := project.Workflows[workflowID]; read || !l.need(workflowID) {
			continue
		}
		l.ctx.Docs.View(workflowID, func(document client.Document) {
			if workflowDoc, ok := document.Model.(*workflowplugin.Document); ok {
				project.Workflows[workflowID] = workflowDoc.Workflow.Clone()
			}
		})
		if loaded, ok := project.Workflows[workflowID]; ok {
			waiting = append(waiting, loaded.Callees()...)
		}
	}
	l.state.npcSprites = map[ids.ULID]ids.ULID{}
	for npcID, definition := range project.NPCs {
		if !definition.Sprite.IsZero() {
			l.state.npcSprites[npcID] = definition.Sprite
		}
	}
}

// values reads the declared values: the world state's, and every NPC's and
// zone's own and the templates applied to it.
func (l *loader) values(project *game.Project) {
	l.state.subscribe(worldstate.ID(), l.ctx)
	for _, document := range valuetree.Documents(l.ctx.Docs) {
		if document != worldstate.ID() {
			l.need(document)
		}
	}
	if catalog, ready := valuetree.Catalog(l.ctx.Docs); ready {
		project.Values = catalog
	} else {
		l.ready = false
	}
	project.Signals, _ = signals.Declared(l.ctx.Docs)
}

// art reads the tile size and the record of which sprite shows each
// stand-in and the player, and returns the tile size.
func (l *loader) art() int {
	tile := art.DefaultTileSize
	if l.need(art.SettingsID()) {
		l.ctx.Docs.View(art.SettingsID(), func(document client.Document) {
			if settings, ok := document.Model.(*artplugin.SettingsDocument); ok {
				tile = settings.Settings.TileSize
			}
		})
	}
	l.state.standIns = map[string]ids.ULID{}
	if l.need(art.StandInsID()) {
		l.ctx.Docs.View(art.StandInsID(), func(document client.Document) {
			if standIns, ok := document.Model.(*artplugin.StandInsDocument); ok {
				for _, standIn := range standIns.Sprites.StandIns() {
					l.state.standIns[standIn], _ = standIns.Sprites.Sprite(standIn)
				}
			}
		})
	}

	return tile
}

// sprites returns the tile each stand-in, the player and each NPC shows,
// and each sprite the workflows switch an NPC to, once its sprite, tile and
// palettes have loaded.
func (l *loader) sprites(workflows map[ids.ULID]*flow.Workflow) map[string]ids.ULID {
	shows := map[string]ids.ULID{}
	for standIn, sprite := range l.state.standIns {
		shows[standIn] = sprite
	}
	for npcID, sprite := range l.state.npcSprites {
		shows[npcKey(npcID)] = sprite
	}
	for _, workflow := range workflows {
		for _, sprite := range switchedTo(workflow) {
			shows[spriteKey(sprite)] = sprite
		}
	}
	tiles := map[string]ids.ULID{}
	for key, sprite := range shows {
		if !l.need(sprite) {
			continue
		}
		var tileID ids.ULID
		l.ctx.Docs.View(sprite, func(document client.Document) {
			if spriteDoc, ok := document.Model.(*artplugin.SpriteDocument); ok {
				tileID = spriteDoc.Sprite.Tile
			}
		})
		if !l.need(tileID) {
			continue
		}
		for _, paletteID := range tileart.Palettes(l.ctx.Docs, tileID) {
			l.need(paletteID)
		}
		tiles[key] = tileID
	}

	return tiles
}

// npcKey is what an NPC's art is kept under.
func npcKey(npcID ids.ULID) string { return "npc:" + npcID.String() }

// spriteKey is what a sprite a workflow switches an NPC to is kept under.
func spriteKey(sprite ids.ULID) string { return "sprite:" + sprite.String() }

// switchedTo lists the sprites a workflow's Set values set its NPC's
// sprite to, so their art is ready before they show.
func switchedTo(workflow *flow.Workflow) []ids.ULID {
	var found []ids.ULID
	for _, node := range workflow.Nodes() {
		ref, err := node.Ref(flow.ParamValue)
		if node.Kind != flow.KindSetValue || err != nil || ref.Value != worldstate.SpriteID() {
			continue
		}
		raw, err := node.Operand("to")
		var saved string
		if err != nil || json.Unmarshal(raw, &saved) != nil {
			continue
		}
		if sprite, err := ids.Parse(saved); err == nil && !sprite.IsZero() {
			found = append(found, sprite)
		}
	}

	return found
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

// workflowCues lists the cues the behaviours' Play sounds play, so they can
// be fetched before an NPC reaches them.
func (s *State) workflowCues(project game.Project) []string {
	var cues []string
	for _, workflow := range project.Workflows {
		for _, node := range workflow.Nodes() {
			if cue, err := node.Cue("cue"); node.Kind == flow.KindPlaySound && err == nil && cue != "" {
				cues = append(cues, cue)
			}
		}
	}

	return cues
}
