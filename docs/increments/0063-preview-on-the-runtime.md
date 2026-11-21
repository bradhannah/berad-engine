# Increment 63: Preview on the game runtime, and moving the player

**Status:** Built

One simulation runs Preview, Launch and a game. [Increment
62](0062-a-game-runs-on-its-own.md) put Launch and a game's own window on the
engine's game runtime, but the Map's Preview still ran a copy of its own: its
own player, clock, NPC runners, values, zone loop and signals, so every rule
for a tick was written twice. This increment has Preview run the game runtime
and drops the copy; Preview keeps its place in the Map, its live edits, and
Watch's pause, step, trail and logs. The runtime learns what Preview needs of
it: to walk and to tick apart, to show each NPC as it runs, and to take edits
while it runs. It also learns to **move the player** while a game runs, into
another zone if need be, keeping its values and clock. It is held to the same
strict checks as [increment 1](0001-foundation.md).

## What it proves

1. **Preview is the game:** a preview in a Map runs the engine's game
   runtime. The Map's own simulation is gone, and every test and walk of
   Preview passes as it did.
2. **Edits reach a running preview as before:** an NPC whose behaviour was
   edited starts again, saying why; one inside a call to a workflow that was
   edited starts again; a placement added starts running and one removed
   stops; a floor painted is walked as painted; values declared anew keep
   what they held.
3. **Pause and step work as before:** with the NPCs paused the player still
   walks and the clock and the boxes wait; a step runs the NPCs until the
   watched one changes node.
4. **The player can be moved within a zone:** during a preview, a cell's
   menu moves the player there, and nothing else changes.
5. **The player can be moved to another zone:** while Launch's game runs, a
   cell's menu on any Map moves the player there. The zone left stops; the
   zone entered starts its NPCs at their placements; the values and the
   clock carry over.
6. **A scripted run repeats:** with its seed fixed, a walk sees the same
   random choices every time.

## Decisions

Agreed by its author on 2026-11-16, before building:

- **Preview stays in the Map and runs on the game runtime,** in place of the
  Map's own copy of the simulation. (Rejected: dropping Preview for the game
  view alone, which would lose walking inside the Map while editing it.)

Agreed by its author on 2026-11-21, before building:

- **Moving the player is part of this increment.** While Launch's game runs
  in the Play tab, a cell's menu on any Map has **Move the player here**:
  the game jumps there, into another zone if need be, keeping its values and
  its clock, where Play from here starts the game again. During a preview
  the same item moves the player within the zone that Map shows. (Rejected:
  a preview that hands over to another zone's Map; moving within a zone
  only.)
- **The zone left stops.** Its NPCs stop, and start afresh at their
  placements if the player returns, as when a game begins. Values and the
  clock carry over, so what is recorded as a value is remembered. This
  follows [the vocabulary](../VOCABULARY.md): entities outside the current
  zone are not simulated. (Rejected: keeping the zone left as it was, to
  carry on from there on return.)
- **Preview's sound stays as it is:** the cues NPCs ask for, without the
  zone's sounds or ambient sounds, which Launch plays.
- **The walk that fails by chance is fixed here:** a scripted run can fix
  the seed, so `TestClerkCallsFetchStock` always sees the clerk fetch stock.

Chosen while building, for its author to check:

- **An edit reaches a running preview by gathering the project again.** The
  Map no longer reads each document every tick: it notes what the documents
  it gathered from were, and when one changes or arrives it gathers again,
  as Launch gathers, and hands the result to the game. The effect is the
  same tick as before.
- **Moving a node on the canvas still starts its NPC again,** as it did:
  the runtime compares a workflow whole, layout included. Leaving layout out
  of that would be a change of its own.
- **A preview's world waits while a document the Map asked for is on its
  way.** Documents arrive in no fixed order, and a zone loop whose workflow
  arrived a moment before the world state ran its first node before the
  value it sets was declared. The player walks meanwhile; the wait is a few
  frames when a preview starts or a new document is first needed.
- **An NPC inside a call to a workflow that is deleted starts again,** as
  one inside a workflow that was edited does. Before, it carried on in the
  copy the Map had kept.
- **An edit made while the NPCs are paused is applied at once,** so an NPC
  it starts again is at its start when the NPCs resume, where before it
  started again as they resumed.
- **Move the player here shows only while there is a player to move:**
  while this Map runs a preview, or Launch's game runs in the Play tab. On a
  cell the player cannot stand on it is disabled.
- **With a preview in this Map and Launch's game both running,** the menu
  moves the preview's player.
- **The Play tab tells the Maps while a game runs,** as shared session
  state. A Play tab closed in a way that does not say so leaves the item
  showing; chosen then, the Play tab opens, says no game is running and
  launches nothing, since nobody asked for a launch.
- **Each move is said in the status bar,** naming the cell, and the zone
  when it changed: "The zone left has stopped; the values and the clock go
  on."
- **Entering a zone clears the message boxes and the shake,** and drops
  signals not yet landed: they belonged to the zone left. Its NPCs start at
  their placements with the clock as it is.
- **Moving into the zone being played from another Map of it** places the
  player and starts nothing again.
- **`BERAD_SEED` fixes the seed** of the previews and launches a window
  starts, for an automated run.
- **An NPC is seeded by its place among those its zone has started,** in
  the order their placements were made, and no longer by its placement's
  ID. A walk makes its sample afresh every run, with new IDs, so a fixed
  seed fixed nothing while IDs went into it; now a project made again plays
  the same from the same seed. The zone's loop is seeded after every NPC.
  An NPC's draws differ from before for the same seed; nothing an author
  sees depends on which they are.
- **`TestClerkCallsFetchStock` runs with seed 4.** That walk failed now and
  then for a second reason, found by playing the sample without a window
  under each seed: with most seeds the clerk is inside her first call to
  Fetch stock for ticks 10 to 30 of a preview, a third of a second, before
  Watch can open. With seed 4 she stretches first and makes the call from
  tick 1940, half a minute in.
- **When a project is gathered, its values are asked for before the
  workflows that read them.**
- **Scripts can wait on two more states:** a Map's `launched`, while
  Launch's game runs, and the Play tab's `zone:<name>`.
- **A fault from increment 62 is fixed:** a game's script could end its
  next step before a screenshot it asked for was taken, and lose it.

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. Walk, tick, see | The runtime walks and ticks apart, and shows each NPC as it runs, with its workflow | Tests: the player walks while the NPCs are paused; the NPCs run many ticks while the player stands | Built |
| 2. Edits | The runtime takes a project gathered again and applies what changed | Tests of each rule, with the reasons NPCs give for starting again | Built |
| 3. Preview on it | The Map runs the game runtime; its own simulation is deleted | The Map's tests and the walks of Preview pass unchanged | Built |
| 4. A fixed seed | A switch fixes the seed of a scripted run | `TestClerkCallsFetchStock` passes ten runs in ten | Built |
| 5. Moving within a zone | The runtime places the player; Move the player here in a preview | Tests, and a walk that moves the player mid-preview | Built |
| 6. Moving between zones | The runtime enters a zone; the Play tab follows, with its art and the zone's sounds | Tests of what stops and what carries over; a walk that jumps Launch's game to a second zone and back | Built |

## Step results

Not in a pull request yet.

- **Walk, tick, see:** `game.Walk`, `game.Tick`, `game.Runners`,
  `game.LoopRunner` and `game.Workflow`; `Step` is the two together.
  Verified by the runtime's tests.
- **Edits:** `game.Update`, with `RestartedByEdit` and `RestartedByCall`.
  Verified by a test of each rule: nothing changed, a behaviour edited,
  taken away and given back, placements added and removed, a floor painted,
  values declared anew, and a called workflow edited with one NPC inside
  the call and one not.
- **Preview on it:** the Map's `previewRun` holds a `game.Game`, gathered by
  `gameload.Load` through `docsource`'s `Asking`, and its own player, clock,
  runners, values, zone loop, signals and copies of called workflows are
  gone: about 290 lines fewer in the Map. Verified by the Map's tests, of
  which one changed: `TestPreviewSeesEdits` edited a floor in place without
  raising its revision, which a real window always does, so it now does.
  Two faults found by the walks in the real window and fixed: a document
  that arrived between a gathering and its note of what it had read was
  never seen, and the wait described above.
- **A fixed seed:** `frame.Context.Seed`, from `BERAD_SEED`, and the
  runtime's seeding by order. Verified by
  `TestAProjectMadeAgainPlaysTheSame`, and by `TestClerkCallsFetchStock`
  passing ten runs in ten in the real window; with the seed fixed but NPCs
  still seeded by ID it had failed four in ten.
- **Moving within a zone:** `play.Run.Place` and `game.Place`, with
  `ErrPlace`; Move the player here in `mapview`. Verified by tests of the
  runtime and the Map.
- **Moving between zones:** `game.Enter`; `playview`'s `MoveToItem`, the
  session topic `TopicPlaying`, and `player.Sound` stopping the sounds of a
  zone left. Verified by tests of the runtime, the Play tab and the Map,
  and by `TestMovingThePlayer` in the real window: a move within a preview,
  then Launch's game moved within its zone, into a zone made for the walk
  and back, launched once throughout.

- **Checks:** `make check` passes: formatting, the strict lint with no
  findings, vet, and every test with the race detector. The real-window
  suite, run from a frozen copy of the final code, passed all 84 of its
  tests. `TestClerkCallsFetchStock`, which failed about two runs in five
  before this increment, passed ten runs in ten on its own, and
  `TestASampleZoneLoop`, which the faults above made fail now and then
  while this was built, passed twelve in twelve.

## Changed after its author tried it

- None yet.

## Not in this increment

- **How a player travels between zones in the game itself,** the network of
  the vocabulary: still open. Moving the player is a tool for trying a game,
  not travel.
- **Moving the player in `berad-play`,** and a preview that hands over to
  another zone's Map.
- **The zone's sounds and ambient sounds in Preview.**
- **A game's own program, an export, and saves.**
