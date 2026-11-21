# Increment 63: Preview on the game runtime, and moving the player

**Status:** Planned

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

- None yet.

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. Walk, tick, see | The runtime walks and ticks apart, and shows each NPC as it runs, with its workflow | Tests: the player walks while the NPCs are paused; the NPCs run many ticks while the player stands | Planned |
| 2. Edits | The runtime takes a project gathered again and applies what changed | Tests of each rule, with the reasons NPCs give for starting again | Planned |
| 3. Preview on it | The Map runs the game runtime; its own simulation is deleted | The Map's tests and the walks of Preview pass unchanged | Planned |
| 4. A fixed seed | A switch fixes the seed of a scripted run | `TestClerkCallsFetchStock` passes ten runs in ten | Planned |
| 5. Moving within a zone | The runtime places the player; Move the player here in a preview | Tests, and a walk that moves the player mid-preview with its values kept | Planned |
| 6. Moving between zones | The runtime enters a zone; the Play tab follows, with its art and the zone's sounds | Tests of what stops and what carries over; a walk that jumps Launch's game to a second zone and back | Planned |

## Step results

- None yet.

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
