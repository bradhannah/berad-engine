// Package gameload gathers what a game plays from a project's documents
// (increment 62): the game settings, the start's zone and its floors, the
// NPCs placed on them, their behaviours and the workflows those call, the
// values and signals the project declares, and the art they show. It reads
// through a Source, which says of each document whether it has it yet, so the
// editor's Play tab, whose documents arrive one at a time, and a game reading
// a project folder gather the same thing by the same rules.
package gameload

import (
	"encoding/json"
	"slices"

	"github.com/bradhannah/berad-engine/engine/art"
	"github.com/bradhannah/berad-engine/engine/game"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/npc"
	"github.com/bradhannah/berad-engine/engine/tilefloor"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/worldstate"
	"github.com/bradhannah/berad-engine/engine/zone"
)

// State says whether a source has a document.
type State int

// What a source says of a document: the project holds none such, it holds
// one that has not arrived yet, or here it is.
const (
	Missing State = iota
	Waiting
	Held
)

// Document is a document as a source shows it: its name, and its content as
// the engine reads it, such as a *zone.Zone or an npc.NPC.
type Document struct {
	Name  string
	Value any
}

// Source is a project's documents. View shows one for as long as fn runs,
// and nothing of it may be kept past the call without copying it. Name names
// a document, "" for one the project lacks. List lists the documents of a
// format, in a steady order.
type Source interface {
	View(id ids.ULID, fn func(Document)) State
	Name(id ids.ULID) string
	List(format string) []ids.ULID
}

// Loaded is what a game plays and shows.
type Loaded struct {
	// Project is what the game runtime plays. Its clock's start and its seed
	// are left for whoever starts the game.
	Project game.Project
	// TileSize is the project's tile size, in pixels.
	TileSize int
	// Tiles is the tile that shows each stand-in, the player, each NPC under
	// its NPCKey, and each sprite a workflow switches an NPC to under its
	// SpriteKey.
	Tiles map[string]ids.ULID
	// NPCSprites is the sprite each NPC shows as its details name it.
	NPCSprites map[ids.ULID]ids.ULID
	// Cues are the cues the workflows' Play sounds play, to have ready before
	// an NPC reaches them.
	Cues []string
}

// NPCKey is what an NPC's art is kept under.
func NPCKey(npcID ids.ULID) string { return "npc:" + npcID.String() }

// SpriteKey is what a sprite a workflow switches an NPC to is kept under.
func SpriteKey(sprite ids.ULID) string { return "sprite:" + sprite.String() }

// loader gathers a game's documents; ready turns false when one the project
// holds has not arrived.
type loader struct {
	source   Source
	ready    bool
	standIns map[string]ids.ULID
}

// Load gathers what a game plays. It reports false while a document the
// project holds has not arrived; what it returns then is not complete. A
// document deleted since something named it is skipped. from, when not nil,
// puts the player there in place of the player start.
func Load(source Source, from *game.Start) (Loaded, bool) {
	loaded := Loaded{
		Project: game.Project{
			Settings: game.DefaultSettings(), Zone: nil, Floors: map[ids.ULID]*tilefloor.Floor{},
			NPCs: map[ids.ULID]npc.NPC{}, Workflows: map[ids.ULID]*flow.Workflow{}, Values: nil, Seconds: 0, Seed: 0,
			Names: NewNames(source), Signals: nil,
		},
		TileSize: art.DefaultTileSize, Tiles: map[string]ids.ULID{}, NPCSprites: map[ids.ULID]ids.ULID{}, Cues: nil,
	}
	load := &loader{source: source, ready: true, standIns: map[string]ids.ULID{}}
	load.settings(&loaded.Project, from)
	load.zone(&loaded.Project)
	load.npcs(&loaded)
	load.values(&loaded.Project)
	loaded.TileSize = load.art()
	loaded.Tiles = load.tiles(loaded)
	loaded.Cues = cuesOf(loaded.Project.Workflows)

	return loaded, load.ready
}

// need shows a document the project holds and reports whether it has
// arrived; one it does not hold is skipped.
func (l *loader) need(documentID ids.ULID, fn func(Document)) bool {
	if documentID.IsZero() {
		return false
	}
	switch l.source.View(documentID, fn) {
	case Held:
		return true
	case Waiting:
		l.ready = false
	case Missing:
	}

	return false
}

func (l *loader) settings(project *game.Project, from *game.Start) {
	l.need(game.SettingsID(), func(document Document) {
		if settings, ok := document.Value.(game.Settings); ok {
			project.Settings = settings
		}
	})
	if from != nil {
		project.Settings.Start = *from
	}
}

// zone reads the start's zone and its floors.
func (l *loader) zone(project *game.Project) {
	start := project.Settings.Start
	if start.IsZero() {
		return
	}
	l.need(start.Zone, func(document Document) {
		if held, ok := document.Value.(*zone.Zone); ok {
			project.Zone = held.Clone()
		}
	})
	if project.Zone == nil {
		return
	}
	for _, floorID := range project.Zone.Floors() {
		l.need(floorID, func(document Document) {
			if held, ok := document.Value.(*tilefloor.Floor); ok {
				project.Floors[floorID] = held.Clone()
			}
		})
	}
}

// npcs reads the NPCs placed on the zone's floors, their behaviours, the
// zone's loop and the workflows those call, in turn.
func (l *loader) npcs(loaded *Loaded) {
	project := &loaded.Project
	for _, placed := range project.Floors {
		for _, placement := range placed.Placements() {
			if _, read := project.NPCs[placement.NPC]; read {
				continue
			}
			l.need(placement.NPC, func(document Document) {
				if held, ok := document.Value.(npc.NPC); ok {
					project.NPCs[placement.NPC] = held
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
		if _, read := project.Workflows[workflowID]; read {
			continue
		}
		l.need(workflowID, func(document Document) {
			if held, ok := document.Value.(*flow.Workflow); ok {
				project.Workflows[workflowID] = held.Clone()
			}
		})
		if held, ok := project.Workflows[workflowID]; ok {
			waiting = append(waiting, held.Callees()...)
		}
	}
	for npcID, definition := range project.NPCs {
		if !definition.Sprite.IsZero() {
			loaded.NPCSprites[npcID] = definition.Sprite
		}
	}
}

// values reads the declared values: the world state's, and every NPC's and
// zone's own and the templates applied to it; and the signals.
func (l *loader) values(project *game.Project) {
	for _, owner := range Owners(l.source) {
		l.need(owner, func(Document) {})
	}
	if catalog, ready := Catalog(l.source); ready {
		project.Values = catalog
	} else {
		l.ready = false
	}
	project.Signals, _ = Signals(l.source)
}

// art reads the tile size and the record of which sprite shows each
// stand-in and the player, and returns the tile size.
func (l *loader) art() int {
	tile := art.DefaultTileSize
	l.need(art.SettingsID(), func(document Document) {
		if settings, ok := document.Value.(art.Settings); ok {
			tile = settings.TileSize
		}
	})
	l.need(art.StandInsID(), func(document Document) {
		if standIns, ok := document.Value.(*art.StandInSprites); ok {
			for _, standIn := range standIns.StandIns() {
				l.standIns[standIn], _ = standIns.Sprite(standIn)
			}
		}
	})

	return tile
}

// tiles returns the tile each stand-in, the player and each NPC shows, and
// each sprite the workflows switch an NPC to, once its sprite, its tile and
// the tile's palettes have arrived.
func (l *loader) tiles(loaded Loaded) map[string]ids.ULID {
	shows := map[string]ids.ULID{}
	for standIn, sprite := range l.standIns {
		shows[standIn] = sprite
	}
	for npcID, sprite := range loaded.NPCSprites {
		shows[NPCKey(npcID)] = sprite
	}
	for _, workflow := range loaded.Project.Workflows {
		for _, sprite := range switchedTo(workflow) {
			shows[SpriteKey(sprite)] = sprite
		}
	}
	tiles := map[string]ids.ULID{}
	for key, sprite := range shows {
		var tileID ids.ULID
		if !l.need(sprite, func(document Document) {
			if held, ok := document.Value.(art.Sprite); ok {
				tileID = held.Tile
			}
		}) {
			continue
		}
		if !l.need(tileID, func(Document) {}) {
			continue
		}
		for _, paletteID := range TilePalettes(l.source, tileID) {
			l.need(paletteID, func(Document) {})
		}
		tiles[key] = tileID
	}

	return tiles
}

// TilePalettes returns the palettes a tile is painted from: Default, and
// every other palette its colours name. A loose colour names none.
func TilePalettes(source Source, tileID ids.ULID) []ids.ULID {
	palettes := []ids.ULID{art.DefaultPaletteID()}
	source.View(tileID, func(document Document) {
		tile, ok := document.Value.(*art.Tile)
		if !ok {
			return
		}
		for _, ref := range tile.Colors() {
			if !ref.IsLoose() && !slices.Contains(palettes, ref.Palette) {
				palettes = append(palettes, ref.Palette)
			}
		}
	})

	return palettes
}

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

// cuesOf lists the cues the workflows' Play sounds play, each once, in
// order.
func cuesOf(workflows map[ids.ULID]*flow.Workflow) []string {
	var cues []string
	for _, workflow := range workflows {
		for _, node := range workflow.Nodes() {
			if cue, err := node.Cue("cue"); node.Kind == flow.KindPlaySound && err == nil && cue != "" {
				cues = append(cues, cue)
			}
		}
	}
	slices.Sort(cues)

	return slices.Compact(cues)
}
