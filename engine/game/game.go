package game

import (
	"errors"
	"fmt"
	"hash/fnv"
	"image"
	"math"
	"slices"

	"github.com/bradhannah/berad-engine/engine/clock"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/npc"
	"github.com/bradhannah/berad-engine/engine/npcrun"
	"github.com/bradhannah/berad-engine/engine/play"
	"github.com/bradhannah/berad-engine/engine/show"
	"github.com/bradhannah/berad-engine/engine/tilefloor"
	flow "github.com/bradhannah/berad-engine/engine/workflow"
	"github.com/bradhannah/berad-engine/engine/worldstate"
	"github.com/bradhannah/berad-engine/engine/zone"
)

// ErrNoStart reports a project with no player start.
var ErrNoStart = errors.New("no player start")

// ErrStart reports a player start the game cannot begin at.
var ErrStart = errors.New("the player cannot start there")

// Project is what a game plays, as it was when the game started: the game
// settings, the start's zone with its floors, the NPCs placed on them with
// their behaviours, the zone's loop, and the values the project declares.
// Nothing in it changes while the game runs.
type Project struct {
	Settings  Settings
	Zone      *zone.Zone
	Floors    map[ids.ULID]*tilefloor.Floor
	NPCs      map[ids.ULID]npc.NPC
	Workflows map[ids.ULID]*flow.Workflow
	// Values are the project's declared values; nil when it declares none,
	// so the value nodes carry on.
	Values *worldstate.Catalog
	// Seconds is the time of day the clock starts at.
	Seconds float64
	// Seed fixes the NPCs' random choices, so a run repeats.
	Seed uint64
	// Names names documents and places for values' text forms; nil leaves
	// them as IDs.
	Names npcrun.Names
	// Signals are the signals the project declares.
	Signals []worldstate.Signal
}

// Game is a project being played: the player at the start, the NPCs on
// their placements, the clock, and the values as they are now, each its
// default when the game starts.
type Game struct {
	settings Settings
	levels   play.Levels
	sounds   []string
	run      *play.Run
	npcs     []*npcrun.NPC
	flows    map[ids.ULID]*flow.Workflow
	// behaviours names each NPC's behaviour, and workflows holds every
	// workflow, for Calls to call.
	behaviours map[ids.ULID]ids.ULID
	workflows  map[ids.ULID]*flow.Workflow
	clock      *clock.Clock
	values     *worldstate.Store
	// loop runs the zone's loop, loopFlow, nil when it has none that fits
	// it; loopID is the loop's workflow.
	loop     *npcrun.NPC
	loopFlow *flow.Workflow
	loopID   ids.ULID
	names    npcrun.Names
	// stage is the message boxes and the shake every workflow shares.
	stage *show.Stage
	// cueTicks says how long each cue plays, for a Play sound that waits.
	cueTicks func(cue string) int
	// signals are the project's signals, and sent those sent on the latest
	// tick, which land on the next.
	signals []worldstate.Signal
	sent    []npcrun.Sent
}

// Character is an NPC as the game shows it: which NPC, the floor it is on,
// where it is in cells, between two cells during a step, and the line it
// is saying, if any.
type Character struct {
	NPC      ids.ULID
	Floor    ids.ULID
	X, Y     float64
	Line     show.Line
	Speaking bool
}

// Heard is an ambient sound on the player's floor and how far from the
// player it is, in cells; a player of sounds turns the distance into a gain
// on the cue's own curve.
type Heard struct {
	Sound    ids.ULID
	Cue      string
	Distance float64
}

// Step is what one tick did: the player's outcome, and the cues the NPCs
// asked for, each once.
type Step struct {
	Outcome play.Outcome
	Cues    []npcrun.Sound
}

// View is what the screen shows: the floor the player is on, and its cell
// rectangle in cells, the player's cell in the middle. Left and Top are
// fractional while the player is between cells.
type View struct {
	Floor         ids.ULID
	Left, Top     float64
	Columns, Rows int
}

// New starts a game at the project's player start.
func New(project Project) (*Game, error) {
	start := project.Settings.Start
	if start.IsZero() {
		return nil, ErrNoStart
	}
	if project.Zone == nil {
		return nil, fmt.Errorf("%w: its zone is not in the project", ErrStart)
	}
	levels := make(play.Levels, 0, len(project.Zone.Floors()))
	for _, floorID := range project.Zone.Floors() {
		levels = append(levels, play.Level{ID: floorID, Floor: project.Floors[floorID]})
	}
	cell := image.Pt(start.Col, start.Row)
	if !play.CanStand(levels, start.Floor, cell) {
		return nil, fmt.Errorf("%w: cell %d,%d of its floor", ErrStart, start.Col, start.Row)
	}
	game := &Game{
		settings: project.Settings, levels: levels, sounds: project.Zone.Sounds(), run: play.Start(start.Floor, cell),
		npcs: nil, flows: map[ids.ULID]*flow.Workflow{}, behaviours: map[ids.ULID]ids.ULID{},
		workflows: project.Workflows, clock: clock.New(project.Seconds, clock.MinutePerSecond), values: nil,
		loop: nil, loopFlow: nil, loopID: ids.ULID{}, names: project.Names, stage: show.NewStage(), cueTicks: nil,
		signals: project.Signals, sent: nil,
	}
	if project.Values != nil {
		game.values = worldstate.NewStore(project.Values)
	}
	for _, level := range levels {
		if level.Floor == nil {
			continue
		}
		for _, placement := range level.Floor.Placements() {
			game.npcs = append(game.npcs, npcrun.Start(placement, level.ID, seed(project.Seed, placement.ID)))
			if definition, ok := project.NPCs[placement.NPC]; ok && !definition.Behaviour.IsZero() {
				game.flows[placement.NPC] = project.Workflows[definition.Behaviour]
				game.behaviours[placement.NPC] = definition.Behaviour
			}
		}
	}
	slices.SortFunc(game.npcs, func(a, b *npcrun.NPC) int { return a.Placement.Compare(b.Placement) })
	game.startLoop(project)

	return game, nil
}

// startLoop starts the zone's loop, when it has one that is a zone loop
// running in the zone or in any zone. It runs while the zone is alive,
// which is the whole game while a game plays one zone.
func (g *Game) startLoop(project Project) {
	zoneID, loopID := project.Settings.Start.Zone, project.Zone.Loop
	loop := project.Workflows[loopID]
	if loopID.IsZero() || loop == nil || loop.Purpose() != flow.PurposeZoneLoop ||
		(!loop.RunsIn().IsZero() && loop.RunsIn() != zoneID) {
		return
	}
	g.loop, g.loopFlow, g.loopID = npcrun.StartLoop(zoneID, seed(project.Seed, zoneID)), loop, loopID
}

// Loop returns the zone's loop as it runs, nil when the zone has none.
func (g *Game) Loop() *npcrun.NPC { return g.loop }

// seed is an NPC's random seed: the game's and its placement's.
func seed(game uint64, placement ids.ULID) uint64 {
	hash := fnv.New64a()
	var bytes [8]byte
	for index := range bytes {
		bytes[index] = byte(game >> (8 * index)) //nolint:gosec,mnd // One byte of the seed at a time.
	}
	_, _ = hash.Write(bytes[:])
	_, _ = hash.Write(placement[:])

	return hash.Sum64()
}

// Step runs the game for one tick: the player walks, unless a message box
// holds them until they close it, the boxes and the shake count down, the
// clock moves on and the NPCs follow their behaviours, each seeing where the
// others were as the tick began.
func (g *Game) Step(input play.Input) Step {
	if g.stage.Holding() {
		input = play.Input{Held: 0, Pressed: 0}
	}
	outcome := g.run.Step(g.levels, input)
	g.stage.Tick()
	passed := g.clock.Advance()
	others := make([]npcrun.Other, 0, len(g.npcs))
	for _, character := range g.npcs {
		others = append(others, npcrun.Other{
			Placement: character.Placement, NPC: character.NPC, Floor: character.Floor, Cell: character.Cell,
		})
	}
	var cues []npcrun.Sound
	npcrun.Deliver(g.sent, g.runners(), g.settings.Start.Zone)
	g.sent = g.sent[:0]
	for _, character := range g.npcs {
		world := npcrun.World{
			Names: g.names, Zone: g.levels, Workflow: g.flows[character.NPC], Behaviour: g.behaviours[character.NPC],
			Workflows: g.workflow, Clock: g.clock, Passed: passed, State: g.values, Here: g.settings.Start.Zone,
			Others: others, Stage: g.stage, CueTicks: g.cueTicks, Signals: g.signals,
		}
		character.Step(world, g.run.Tick())
		cues = heard(cues, character.Sounds)
		g.sent = append(g.sent, character.Sent...)
	}
	if g.loop != nil {
		g.loop.Step(npcrun.World{
			Names: g.names, Zone: g.levels, Workflow: g.loopFlow, Behaviour: g.loopID, Workflows: g.workflow, Clock: g.clock,
			Passed: passed, State: g.values, Here: g.settings.Start.Zone, Others: others, Stage: g.stage,
			CueTicks: g.cueTicks, Signals: g.signals,
		}, g.run.Tick())
		cues = heard(cues, g.loop.Sounds)
		g.sent = append(g.sent, g.loop.Sent...)
	}

	return Step{Outcome: outcome, Cues: cues}
}

// KnowCues tells the game how many ticks each cue plays for, from its
// audio library, so a Play sound can wait until its sound has played.
func (g *Game) KnowCues(cueTicks func(cue string) int) { g.cueTicks = cueTicks }

// runners are the NPCs, then the zone's loop when it has one.
func (g *Game) runners() []*npcrun.NPC {
	if g.loop == nil {
		return g.npcs
	}

	return append(slices.Clip(g.npcs), g.loop)
}

// heard adds the cues asked for to those heard this tick, each once.
func heard(cues, asked []npcrun.Sound) []npcrun.Sound {
	for _, sound := range asked {
		if !slices.ContainsFunc(cues, func(other npcrun.Sound) bool { return other.Cue == sound.Cue }) {
			cues = append(cues, sound)
		}
	}

	return cues
}

// workflow finds a workflow a Call calls.
func (g *Game) workflow(target ids.ULID) (*flow.Workflow, bool) {
	found, ok := g.workflows[target]

	return found, ok && found != nil
}

// Stage returns the message boxes and the shake, for drawing them and for
// closing a box the player must close.
func (g *Game) Stage() *show.Stage { return g.stage }

// Player returns the player.
func (g *Game) Player() play.Player { return g.run.Player() }

// Clock returns the game's clock.
func (g *Game) Clock() *clock.Clock { return g.clock }

// Values returns the values as they are now; nil when the project declares
// none.
func (g *Game) Values() *worldstate.Store { return g.values }

// Floor calls fn with a floor of the zone, and reports whether it has one.
func (g *Game) Floor(floorID ids.ULID, fn func(*tilefloor.Floor)) bool {
	return g.levels.Floor(floorID, fn)
}

// Characters returns the NPCs where they are now, in placement order.
func (g *Game) Characters() []Character {
	characters := make([]Character, 0, len(g.npcs))
	for _, character := range g.npcs {
		x, y := character.At()
		line, speaking := character.Saying()
		characters = append(characters, Character{
			NPC: character.NPC, Floor: character.Floor, X: x, Y: y, Line: line, Speaking: speaking,
		})
	}

	return characters
}

// View returns what the screen shows now: the settings' columns and rows
// with the player in the middle.
func (g *Game) View() View {
	player := g.run.Player()
	x, y := player.At()
	columns, rows := g.settings.Columns, g.settings.Rows

	return View{
		Floor: player.Floor, Left: x - float64(columns/2), Top: y - float64(rows/2), //nolint:mnd // The middle cell.
		Columns: columns, Rows: rows,
	}
}

// Ambient returns the ambient sounds on the player's floor, each with its
// distance from the player in cells.
func (g *Game) Ambient() []Heard {
	player := g.run.Player()
	x, y := player.At()
	var heard []Heard
	g.levels.Floor(player.Floor, func(floor *tilefloor.Floor) {
		for _, sound := range floor.Sounds() {
			distance := math.Hypot(float64(sound.Col)-x, float64(sound.Row)-y)
			heard = append(heard, Heard{Sound: sound.ID, Cue: sound.Cue, Distance: distance})
		}
	})

	return heard
}

// ZoneSounds returns the cues the zone plays throughout.
func (g *Game) ZoneSounds() []string { return slices.Clone(g.sounds) }
