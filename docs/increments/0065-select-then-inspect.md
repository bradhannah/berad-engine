# Increment 65: Select a thing on the Map, then set it in the inspector

**Status:** Built

The Map gets one way to select what sits on a cell, and an inspector for
each kind of thing. Its author tried [portals](0064-portals.md) and found
the Map's way of setting things wrong: "an anchor should not be configured
entirely with right click menu - it should have an inspector after I
selected it". Until now an anchor or an NPC had an inspector only while the
tool that also *adds* that kind was in hand, and an ambient sound, a portal
and the player start could be set only from a cell's right-click menu. This
increment adds a **Select** tool for everything on a cell, inspector
sections for a sound, a portal and the player start, and a right-click menu
that only adds and removes. No document format and nothing in the engine
changes. It is held to the same strict checks as [increment
1](0001-foundation.md).

## What it proves

1. **One tool selects every kind of thing:** an anchor, an NPC, an ambient
   sound, a portal, the player start, and walls. One thing is selected at a
   time, and a click with Select never changes the project.
2. **A cell with several things asks which:** a menu at the cell lists
   them, and the one chosen is selected.
3. **The inspector sets what the menu used to:** a sound's cue; where a
   portal leads and whether it fades or cuts.
4. **Dragging moves the selected thing,** whatever its kind, in one edit.
5. **The tools that add only add,** and say so on a cell that is taken.
6. **A cell's menu only adds and removes.**

## Decisions

Agreed by its author on 2026-11-21, before building:

- **A Select tool.** A click selects one thing on a cell; the inspector
  shows it with its controls.
- **Several things on one tile:** "a select tool will pop up asking which
  to select if multiple things on single tile".
- **The right-click menu adds and removes only.** How a thing is set moves
  to the inspector.
- **Anchors and Place NPC only add.** Selecting, dragging to move and the
  inspector belong to Select. A click with them on a cell that already has
  one says so and changes nothing.
- **One Select tool,** which also takes over Select walls: a wall with
  nothing else on its cell selects its run. `S` stays its key, and Select
  is first in the list.
- **Its own increment, in one pull request with portals.**
- **What can be selected:** an anchor, an NPC placement, an ambient sound, a
  portal, the player start, and walls. A door, stairs or a pickup has
  nothing to set yet and is not selected.
- **A click on a cell with nothing to select selects nothing,** and so does
  Escape; the inspector then shows the cell under the pointer, as before.
- **Dragging moves.** Pressing on the selected thing, or on a cell that
  holds only one, and releasing on another cell moves it there. A cell that
  already holds one of the same kind refuses. Dragging from an empty cell
  still selects the walls in an area, and a double-click a room's.
- **Buttons and choices apply at once; the anchor's name waits for Save,**
  as the standing rule has it.
- **Cut or fade is two buttons side by side,** the one chosen filled.
- **A line in History that names an anchor or an NPC** selects it with
  Select in hand.
- **The tool in hand when a Map opens stays Ground.**

Chosen while building, for its author to check:

- **The menu that asks which** lists a cell's things in a fixed order, as
  "NPC: Customer", "Anchor: Door", "Portal to Counter", "Ambient sound:
  Hum", "Player start" and "The run of walls". The status bar then says
  what was selected.
- **On a cell with several things, a click always asks,** even when one of
  them is selected, so another can be chosen; a drag from that cell moves
  the one selected, and moves nothing when none is.
- **The selected thing's cell is outlined in blue,** and so is the cell a
  drag would land on. An anchor and an NPC keep the rings they had as
  well.
- **The portal's section** says where it leads on two lines, the anchor
  and then its floor and zone, or "Nowhere" in red with why. Open where it
  leads is disabled for a portal that leads nowhere.
- **Change sound opens the ambient cues under its button,** the sound's own
  ticked off, as the cell's menu did.
- **The player start can be dragged** to a cell the player can stand on; it
  has nothing else to set, and is not removed.
- **Backspace and Delete remove the selected sound or portal too.** The
  plan said removing would stay a button; the keys already removed a
  selected anchor or NPC, so they now do the same for the other kinds, each
  with undo. Removing is also in the inspector and in the cell's menu.
- **The wall families are listed only while walls are selected.** The left
  panel used to head them "Change selected walls to" whenever the tool was
  in hand, which read wrongly beside a selected portal.
- **Removals come first in a cell's menu,** then what can be added, then
  the game: Remove NPC from map, Delete anchor, Remove ambient sound, Remove
  portal; Add anchor here, Place the chosen NPC here, Remove what is on top,
  Place ambient sound here…, Add portal here…; Preview from here, Play from
  here, Move the player here, Set player start here.
- **In code, the selection is still a field for each kind,** with one
  function that selects a thing and clears the rest, where the plan had one
  field for all. The anchor's name field, its guard for unsaved names and
  the NPC's section depend on theirs, and behave as they did.
- **Scripts can wait on** `selected:<kind>:C,R` and `selected:none`, find
  the inspector's buttons by `ui:` and their IDs, and the rows of the menu
  that asks which as `menu:select:<kind>`.

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. One selection, one tool | The selection, Select first in the list, walls folded in, the outline, Escape; Anchors and Place NPC add only | Tests: each kind selects alone; a wall selects its run; the add tools refuse a taken cell | Built |
| 2. Which one? | The menu for a cell with several things | Tests: three things on a cell; Escape selects nothing; asking again | Built |
| 3. Dragging moves | One drag for every kind | Tests: each kind moves in one edit; a taken cell refuses | Built |
| 4. The inspector | Sections for a sound, a portal and the start | Tests of each button and its edit; screenshots of the real window | Built |
| 5. Right-click adds and removes | The menu trimmed | Tests of the menu on each kind of cell | Built |
| 6. Walks and record | A walk in the real window, and this record | The checks below | Built |

## Step results

Not in a pull request yet.

- **Steps 1 to 5** were built as one change, since each depends on the
  one selection: `mapview`'s `select.go` (what is on a cell, selecting, the
  menu that asks which, the drag) and `inspect.go` (the sections and their
  buttons), with `updateAnchor` and `updateNPCTool` cut down to adding and
  the cell menu to adding and removing. Verified by the Map's tests: four
  new ones, of selecting each kind alone, of a cell with three things, of
  dragging each kind and its refusals, and of the add tools; and nine
  moved from the old ways to the new, among them placing a sound, adding a
  portal, renaming an anchor and placing an NPC.
- **Walks and record:** `TestSelectThenInspect` in the real window selects
  the sample's hum and drags it; clicks the cell that holds the Door anchor
  and the customer and selects each from the menu; selects and drags the
  player start; adds a portal whose anchor is picked in the same Map, tells
  it to cut in the inspector, drags it and removes it from the menu. One
  earlier walk used a menu item that went: the portals walk now tells its
  portal to cut in the inspector. The plan expected nine walks to change;
  the others only add with the Anchors and Place NPC tools, which still
  add.

- **Checks:** `make check` passes: formatting, the strict lint with no
  findings, vet, and every test with the race detector. The real-window
  suite, run from a frozen copy with an empty screenshot folder, passed all
  86 of its tests; `TestSelectThenInspect` passed ten runs in ten on its own
  first, and the portals walk three in three. One change came after that
  run: a selected anchor's or NPC's cell is outlined in blue as the other
  kinds are, since one can stand over the other's ring on a cell. It is
  drawing only, and was checked by `make check`, not in the real window.

## Changed after its author tried it

- None yet.

## Not in this increment

- Selecting several things at once, and copying or pasting them.
- Anything to set on a door, stairs or a pickup.
- An inspector for the zone's loop, palettes or sounds beyond what is there.
- The other kinds of portal change, and showing an arrival, from increment
  64's list.
- The same rule on other canvases, such as the Workflow and the Pixel
  editor: its author is asked first.
