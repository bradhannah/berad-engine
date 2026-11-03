# Increment 22: NPCs live in one zone

**Status:** Building

The twenty-second increment gives each NPC the zone it lives in. Today an
NPC's zone is only where it happens to be placed, the place lists offer
every place in the project, and a Go to whose place is on another floor,
even of the same zone, is skipped without a word. Now an NPC's zone is set
in its details and binds it there: it is placed only on that zone's maps,
it can go to a place on any of the zone's floors by taking the stairs, and
a place it can never reach is flagged. Changing its zone asks first,
saying what would break. Zones are chosen with a **zone picker** over the
window, built to serve whatever asks for a zone. It builds on the places
of [increment 21](0021-flow-modules.md) and the picker of [increment
19](0019-value-picker.md), and is held to the same strict checks as
[increment 1](0001-foundation.md).

## What it proves

1. **An NPC lives in one zone:** its zone is part of the NPC, chosen in its
   details, and it is placed only on a map of that zone.
2. **A zone is chosen from many:** the zone picker finds a zone by name,
   folder or tag among many, and shows enough of it to choose well.
3. **One picker serves every asker:** the asker says what the zone is for,
   the zone it is now, the zones it can't take and why, and whether No
   zone is offered.
4. **Changing a zone says what breaks:** moving an NPC that has places in
   its workflows lists them, and the other NPCs that share them, before it
   moves; Undo puts it back.
5. **A place it can't reach says so:** a place in another zone, or on a
   floor no stairs lead to, is a problem on the node and in the Problems
   window, naming the NPC it is for.
6. **NPCs take the stairs:** a Go to finds its way across the zone's floors
   by their stairs, as the player walks them, and Watch shows the floors
   on the way.

## Decisions

Asked for by its author on 2026-11-02, after trying increment 21: the
place lists offered places in any zone, though an NPC can't go to another
zone, and an NPC should belong to a zone.

Agreed by its author on 2026-11-02, before building, in answer to the
planning questions:

- **An NPC's zone is a setting of the NPC,** set in its details as its
  behaviour is, and binds it to that zone.
- **An NPC lives in one zone and only one,** and can change floors within
  it.
- **Changing its zone asks first, then flags:** the move is never refused;
  what breaks is listed before and flagged after.
- **It is placed only on its zone's maps.** Changing its zone takes it off
  its old map, until it is placed in the new zone.
- **A workflow's place list puts its NPCs' zone first,** every floor of
  it; other zones are greyed, can still be chosen, and are flagged when
  they are. A module, which doesn't know who calls it, lists every place.
- **A character in two zones is two NPCs,** or one module called by both;
  one behaviour is not expected to serve NPCs in different zones.
- **Duplicate NPC comes later.**

Agreed by its author on 2026-11-02, before building, from the wireframes
on the canvas "Workflow canvas wireframes", row "NPCs live in one zone":

- **The Zone box** in an NPC's details, above Behaviour, shows the zone's
  name, or No zone yet, grey and dashed, and opens the zone picker. Under
  it, Placed on names the floor and cell, with Show on map, or nowhere yet.
- **The zone picker** opens over the window, as the value picker does:
  Choose a zone with what it is for and what it is now; a search by name,
  folder or tag, with the project's tags as chips; a rail of All zones,
  Recent and the project browser's zone folders, with counts; a list
  giving each zone's folder, floors, NPCs living there and places; and a
  look at the zone under the cursor: its ground floor drawn small, its
  floors, size, who lives there, its places, tags and notes. Enter or the
  Choose button picks it; Esc closes.
- **Moving an NPC** to another zone shows Move Clerk to Harbour?: that it
  comes off its map, each place its workflows go to that it could no longer
  reach, and the other NPCs that run those workflows, with Cancel and Move
  to Harbour. Each place is flagged until it has one in the new zone.
- **The place list** heads its NPCs' zone, Where Clerk and Customer live,
  over a group per floor, then Other zones: Clerk and Customer can't go
  there, greyed.
- **The map's Place NPC tool** lists the NPCs that live in its zone, those
  with no zone yet, and, greyed, those that live elsewhere, with the zone
  they live in; the status bar says to change an NPC's zone in its details
  to place it there.
- **A place it can't reach** shows its zone on the node in red, and the
  node's red mark, and Problems names the NPC each problem is for.
- **Watch shows the floors** of a walk across them, as Ground floor →
  stairs down at 12, 4 → Cellar → Shelf.

Chosen while planning, for its author to check:

- **An NPC with no zone takes the zone of the first map it is placed on;**
  choosing a zone for an NPC that has none asks nothing.
- **Once an NPC has a zone it keeps one:** the picker offers no No zone
  for it.
- **Zones get notes,** private to the author as an NPC's are, shown in the
  zone's details and in the picker.
- **Recent** is the zones last chosen in the picker while the editor is
  open.
- **The picker's look at a zone draws its ground floor only.**
- **Floors with no stairs between them are flagged in the editor:** a Go
  to on one floor whose place is on a floor no stairs lead to. A way
  blocked by something that moves is still found only in the game.
- **The Map follows a watched NPC** down the stairs to the floor it is on.
- **An NPC's locals start fresh** when its zone changes, as agreed in
  [workflow design](../WORKFLOW_DESIGN.md#values).
- **Projects made before this increment** give each placed NPC its
  placement's zone when they open; an NPC placed nowhere has no zone yet.
- **The zone picker is its own package,** `editor/views/zonepicker`, as
  the value picker is.
- **Locating for tests:** the Zone box as "ui:npc-zone", the picker's
  search as "zone-search", a zone's row as "zone:<name>", Choose as
  "ui:zone-choose", the move dialog's buttons as "ui:zone-move" and
  "ui:zone-cancel".

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. An NPC's zone | The zone in the NPC's file, its next format version; older projects giving placed NPCs their placement's zone; notes on a zone | Engine tests save, load and upgrade an NPC's zone and a zone's notes | Built |
| 2. The zone picker | The picker package: the asker's purpose, now, zones it can't take and No zone; the search, tag chips, rail of All zones, Recent and folders, the list and the look at a zone | View tests search by name, folder and tag, narrow by a folder, grey a zone with its reason and choose one | Built |
| 3. The Zone box | The Zone box and Placed on in an NPC's details, opening the picker; choosing a zone for an NPC with none; a zone's notes in its details | View tests choose a first zone, read Placed on and save a zone's notes | Planned |
| 4. Placing in its zone | The Place NPC tool's groups, those living elsewhere greyed; a first placement giving its zone | View tests place an NPC with no zone, find one from another zone refused with the status line | Planned |
| 5. Moving zone | The move dialog: off its map, the places it could not reach, the other NPCs that share them; the move and its Undo | View tests move an NPC with places and without, and undo a move | Planned |
| 6. Places by zone | The place list headed by its NPCs' zone, every floor, other zones greyed; a module listing every place | View tests read the order for a behaviour, a shared behaviour and a module | Planned |
| 7. Reach problems | A place in another zone, on a node, a Call's input and a place local's start, and a floor no stairs lead to, each for the NPC it is for | View tests raise and mend each problem for an NPC, and for two NPCs sharing a behaviour | Planned |
| 8. Taking the stairs | A Go to's route across the zone's floors by their stairs; the NPC changing floor in the game | Runner tests walk down and back up two-way stairs, and down a one-way hole | Planned |
| 9. Watching floors | Watch's floors on a walk; the Map following a watched NPC to its floor | A view test reads the floors in Watch and finds the Map on the new floor | Planned |
| 10. Checks | `make check` and the real-window suite; a real-window test moves an NPC to another zone, mends its flagged place there, and watches it take the stairs to it | The checks pass with no lint findings | Planned |

## Step results

- **An NPC's zone:** an NPC names the zone it lives in, and NPCs are
  version 5; a version 4 NPC reads unchanged, living in no zone. The NPC
  refers to its zone, labelled lives here, so a zone's uses list the NPCs
  that live in it. Zones keep notes, private to the author, and are
  version 5. Opening a project gives each NPC that lives in no zone but is
  placed on a zone's floor that zone, saved at the next save as an
  upgrade is; an NPC placed nowhere keeps no zone. The sample's clerk and
  customer live in the sample zone. Verified: engine tests save, read and
  upgrade an NPC's zone and a zone's notes and refuse notes too long;
  plug-in tests meet the undo contract for setting a zone and notes; an
  orchestrator test reopens a project saved before NPCs had zones and
  finds the placed NPC living in its zone and the other in none; the
  headless sample test finds both NPCs in the sample zone.
- **The zone picker:** `editor/views/zonepicker` opens over the window
  with Choose a zone and what it is for and now; a search by a zone's
  name, folder or tag, the project's zone tags as chips that narrow to
  the zones carrying them; a rail of All zones, Recent and the zone
  folders, a folder holding the folders inside it, each with how many
  match; the zones with their folder, or now, or why they can't be chosen,
  and their floors, NPCs and places in columns; and a look at the zone
  under the cursor, else the one selected: its ground floor drawn a pixel
  a cell in its tiles' stand-in colours, its floors, size, who lives
  there, its places, tags and notes. A click selects, and a double-click,
  Enter or Choose chooses, the button naming the zone, with an ellipsis
  when choosing goes on to ask more. A refused zone stays, its reason at
  the foot. No zone shows only when the asker offers it. Verified: view
  tests search by name, folder and tag, narrow by a folder and a tag
  chip, read the look, mark the zone it is now, refuse a zone with its
  reason, choose by Enter, choose No zone and close with Escape.

## Not in this increment

- **Duplicate NPC,** for a character that seems to live in two zones; the
  zone picker is built to choose the copy's zone.
- **Doors between zones,** a door leading to another zone, chosen with the
  zone picker.
- **NPCs that travel between zones** in the game.
- **Place values beyond locals,** in world, zone and NPC values, and in
  nodes other than Go to.
- **A behaviour as a module with its inputs filled in per NPC.**
