# Second build: walk the map

The second implementation step towards the
[first useful authoring milestone](EDITOR_FEATURES.md#first-useful-authoring-milestone):
build a map, place NPCs with basic logic and schedule loops, then walk around
it and talk to them. It starts with walking, as a preview of a zone in the Map
window (D15), so each later piece can be tried by walking up to it. It is held
to the same strict checks as [the first build](FIRST_BUILD.md).

## What it proves

1. **A zone can be played inside the editor** without changing the project
   (D15).
2. **Live state reaches every view of the zone** through the orchestrator,
   latest value wins (D7, D13).
3. **Movement is precise**: a tap moves exactly one cell.
4. **Editing during a preview applies at once.**

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. Walking | Stairs stand-ins; the engine's `play` package with the movement rules | A run repeats exactly; every rule tested | Done (PR #6) |
| 2. Live state | Protocol version 3: preview start, state and stop through the orchestrator | One preview at a time; the zone's other views see the player; a closing window ends its preview | Done (PR #6) |
| 3. Preview in the Map window | `P` and a start cell; arrow keys walk; the view follows and changes floor on stairs; other views draw the player | Walking judged by its author | Done (PR #6) |

Later builds of the milestone (placing NPCs with basic logic and schedule
loops, conversations) are not planned yet.

## Decisions

Settled by its author before building:

- The next build starts with walking the map, before objects and placements or
  workflows and NPCs.
- **Doors stay passable for now.** Opening and closing come later ("we are
  focused on basics").
- **Stairs:** stairs up and stairs down are stand-ins. Walking onto them takes
  the player to the same cell one floor up or down, and the Map window
  switches to that floor.
- **Movement:** a tap moves exactly one cell and holding keeps walking; the
  player slides smoothly between cells.
- **Eight directions.** The player cannot squeeze diagonally between two cells
  that block.

Chosen while building, for its author to check:

- The arrow keys walk, since letter keys choose Map tools. A second arrow key
  pressed within 4 ticks of the first turns the step into a diagonal.
- A step takes 12 ticks: a fifth of a second, five cells a second.
- A diagonal into a wall does not slide along it; the player stays put.
- Passing one blocking corner diagonally is allowed; only two blocking cells
  stop a diagonal.
- Walls and water block. Unassigned ground and tiles no definition has do not.
- Arriving on stairs does not take them again; the player steps off and back
  on.
- While the player walks, the view follows, keeping three cells from its edge;
  the author can pan freely while the player stands still.
- The player is a blue disc with a white dot towards where they face.

## Step results

- **Engine:** stairs up and stairs down join the stand-ins (`engine/tiles`),
  with `Passable` and `Climb`. The `play` package runs a zone: a run changes
  only when stepped, by its input and the zone's floors on that tick, read
  through a small `Zone` interface one floor at a time. Tests cover a tap, a
  hold, the slide, a tap during a step, walls, water and the floor edge, doors
  and pickups, diagonals, a second key joining a diagonal (and one arriving
  too late), squeezing, opposite keys, stairs both ways, stairs to a missing
  floor or a blocked cell, edits during a run, a lost floor, and two runs with
  the same input ending identically.
- **Live state:** protocol version 3 adds three preview messages (D7). The
  orchestrator allows one preview at a time and gives it a run ID; it passes
  states only to the zone's other Map windows, keeps the latest for a view
  that opens later, skips a state for a window that is behind, ignores states
  from any other window or run, and ends the preview when its window closes.
  Tests cover each of these with scripted windows.
- **Map window:** `P` or the Preview button, then a click on a cell the player
  can stand on, starts a preview; the window runs it and sends its state at
  most every second tick, and only when it changes. View tests cover walking
  and walls, edits during a preview, stairs switching the floor shown, the
  start cell, a refused start, and another view's preview shown and ignored
  when out of date.
- **Real windows:** `make gui-test` previews the sample zone: the player walks
  one cell, walks onto the stairs and arrives upstairs with the view switched,
  and a second view of the zone shows the player walking. The saved floor is
  unchanged afterwards (revision 0). Screenshots of all three were checked by
  eye.

- **First try by its author:** walking worked, but nothing showed that a
  click on the start cell was needed after `P`, so the arrow keys seemed to do
  nothing. Now, while the author chooses, the player shows faintly on the cell
  under the cursor wherever they can stand, and arrow keys pressed before a
  preview runs say what to do in the status bar. Its author then judged
  walking and the cues good.

## Not built yet

From D15: keeping preview state until Reset (there is nothing to keep yet:
doors stay open and nothing can be picked up), test profiles, the clock,
conversations over the map, and the world-state view. Leaving the zone cannot
happen yet, since zones have no exits.
