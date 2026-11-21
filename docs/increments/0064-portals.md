# Increment 64: Portals, the way from one zone to another

**Status:** Built

A game can leave its first zone. [Increment
63](0063-preview-on-the-runtime.md) taught the game runtime to move a
running game into another zone, but only an author's tool used it: Move the
player here. Nothing an author put in a project sent the player anywhere.
This increment adds the **portal**: a cell that leads to an anchor, in
another zone or in its own. Stepping onto it takes the player there, as
stairs take them to another floor. An author makes one from a cell's menu in
the Map, choosing the zone and then clicking the anchor in that zone's Map.
Launch and a game's own window travel by them; a Map's Preview stays in its
zone. It is held to the same strict checks as [increment
1](0001-foundation.md).

## What it proves

1. **A floor holds portals:** each on a cell of its own, leading to an
   anchor, with how the screen changes; they save, load, undo and go with a
   floor's edge as its other marks do.
2. **A step onto a portal takes the player where it leads:** into another
   zone, where the zone left stops and the one entered starts, the values
   and the clock carrying over; or to a cell of the zone they are in, where
   nothing starts again.
3. **The change is the portal's own:** one cuts at once; another fades to
   black and in again, the keys waiting meanwhile.
4. **Only a step sets one off:** arriving on a portal's cell does nothing,
   so the way back can stand where the player arrives; stairs on a portal's
   cell win.
5. **An author makes one without leaving the Map,** and sees on the Map
   where each leads, and which lead nowhere.
6. **Launch, a game's window and `berad-play` travel alike,** by the same
   runtime; a Preview stays in its zone and says where the portal leads.
7. **Nothing used goes unseen:** deleting an anchor, a floor or a zone a
   portal leads to names the portal first.

## Decisions

Agreed by its author on 2026-11-21, before building:

- **A cell that leads somewhere, made in the Map.** (Not now: a workflow
  node that moves the player. Workflows cannot yet notice the player or be
  started by them, so little could use it.)
- **It is called a portal,** the word the design notes already used for
  doorways between maps. **Network** keeps its meaning in [the
  vocabulary](../VOCABULARY.md): the travel layer between zones, still to be
  designed.
- **Only what the author makes.** Each portal leads one way; the way back is
  another portal the author makes. Arriving on a cell never sets a portal
  off; only stepping onto it does. (Rejected: offering to make the way back,
  as two-way stairs place both ends.)
- **The change is a choice, and the list can grow:** a cut at once, or a
  short fade through black.
- **The change is chosen on each portal,** a cut unless changed, so a door
  can cut and a train fade. It is saved as a named kind, so another kind is
  an addition.
- **A fade is 20 ticks out and 20 in,** a third of a second each, fixed by
  the engine for now. The keys wait; the world carries on.
- **Preview stays in its zone.** Stepping onto a portal in a Map's preview
  says where it leads and leaves the player there. This follows increment
  63, which rejected a preview that hands over to another Map.
- **A portal that leads nowhere leaves the player where they are** and says
  why; the Map draws it flagged.
- **NPCs do not use portals;** they live in one zone ([increment
  22](0022-npc-zones.md)).
- **No time passes in travel,** and stairs win over a portal on their cell.

Chosen while building, for its author to check:

- **A portal may lead to an anchor of its own zone.** The picker offers
  every zone, this one first, so the case had to mean something. The player
  is put on the anchor and nothing of the zone starts again, where entering
  a zone starts its NPCs afresh. It is the elevator the vocabulary allowed
  for, which teleports where stairs do not.
- **The zone is chosen first, then the anchor is clicked in that zone's
  Map.** "Add portal here…" asks "Lead to an anchor in which zone?". A Map
  already showing that zone takes the pick, this Map before another; with
  none open, one opens for it. A zone needs an anchor to lead to: the pick
  chooses among anchors and adds none.
- **A portal's menu:** Open where the portal leads, naming the anchor, its
  floor and its zone; Lead the portal somewhere else…; Change the portal to
  a fade through black, or to a cut; Move portal; Remove portal. Each change
  is one floor edit with undo.
- **On the Map a portal is a teal diamond,** with "→" and its anchor's name
  beside its cell, and "· fade" after it when it fades. One that leads
  nowhere is red and says "→ nowhere"; its menu says why, and offers no
  opening. Teal is the marker's own: anchors are blue rings, ambient sounds
  purple discs and the player start green.
- **The game holds the player; whoever runs it takes them through.** A step
  onto a portal starts the player *leaving*: the runtime holds them on the
  cell and fades the screen, and waits to be told where they arrive, since
  another zone's documents must be gathered first and only the window knows
  where they are. A game's window reads them from its folder the same tick.
  The Play tab asks for them as the player steps on, and the player waits on
  the portal, the screen staying black for a fade, until all have arrived.
- **The darkness covers the whole of the game's screen,** bubbles and boxes
  included, and is drawn by the engine's player, so the Play tab and a
  game's window fade alike.
- **Travel is said in the Play tab's status bar:** "The portal led to
  Cellar. The zone left has stopped; the values and the clock go on." A
  portal that cannot take the player is refused there, with why. A game's
  window reports it once for each portal and reason.
- **A shrinking floor lists the portals it gives up,** with its NPCs, sounds
  and stairs ([increment 58](0058-nothing-used-goes-unseen.md)), and undo
  puts them back.
- **Deleting an anchor also names the portals of its own floor** that lead
  to it. The project's answer to where an anchor is used leaves out the
  anchor's own floor, which no other reference needed.
- **The Places list is unchanged.** The plan had it count portals as uses;
  it turned out to be the list a workflow chooses a place from, not a list
  of uses. Where-used, which the questions on deleting ask, lists portals.
- **Scripts can wait on more states:** a Map's `portal:C,R`, with `:cut`,
  `:fade`, `:to:<anchor>` or `:nowhere`, and a dialog's row by what it says,
  `ui:row:<label>`; a game's `leaving`, `dark` and `light`; and a game's
  window's `zone:<name>`, as the Play tab already had.
- **The floor format is version 6,** and reads version 5 unchanged. A game
  pinned to an earlier engine cannot read a floor saved by this editor
  until its pin moves.

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. The portal in a floor | The floor's portals and format 6; the editor's edits for them | Tests: round trip, upgrade, undo, resize | Built |
| 2. The runtime | Leaving, the darkness, staying and arriving; finding where a portal leads | Tests, tick by tick: a cut and a fade, staying, stairs, arriving | Built |
| 3. A game's window | The window travels and draws the darkness | A project of two zones played by script, there and back | Built |
| 4. The Map | The menu, the pick, the marker and its flag; where-used and a floor's edge | Tests of each choice and its edit | Built |
| 5. Launch and Preview | The Play tab travels; Preview stays and says | Tests, with a zone whose documents arrive late | Built |
| 6. Walks and record | A walk in the real window, and this record | The checks below | Built |

## Step results

Not in a pull request yet.

- **The portal in a floor:** `tilefloor.Portal`, with `ChangeCut` and
  `ChangeFade`, and `InsertPortal`, `RemovePortal`, `UpdatePortal` and
  `PortalAt`; format version 6. In the editor, the edits
  `tilefloor.portal.add`, `.remove` and `.update`, a resize that restores
  the portals it removed, and each portal listed as a reference to its
  anchor. Verified by the floor's tests and by the shared test that every
  edit undoes exactly.
- **The runtime:** `game.Leaving`, `Dark`, `Darkness`, `Stay` and
  `FadeTicks`; `Place` and `Enter` end a leaving; `gameload.Destination`
  finds the zone, floor and cell of a portal's anchor, waits for documents
  not yet arrived, or says why the portal leads nowhere. Verified by tests
  that count the darkness tick by tick, and by one that plays the same keys
  twice through a fade and finds every tick the same.
- **A game's window:** `player.Window` travels, and `Screen.DrawShown`
  darkens the game's screen. Verified by a test that walks a project of two
  zones by script: a fade into the second, a cut within it, a cut back, and
  a portal that leads nowhere, reported once. `berad-play` played the same
  project in its real window by script, with a picture half way through the
  fade and one on each side.
- **The Map:** `mapview`'s portals, and `picker.StartIn` for a pick in a
  zone already chosen. Verified by tests of adding a portal with no Map of
  its zone open and with one open, of each menu choice and its edit, of a
  portal that leads nowhere, and of the questions a shrinking floor and a
  deleted anchor ask.
- **Launch and Preview:** `playview`'s travel, sharing `enter` with Move the
  player here. Verified by tests of a fade into another zone and a cut
  back, a portal within its zone, one that leads nowhere, and one whose
  zone's floor arrives late; and of a preview that stays and says where the
  portal leads.
- **Walks and record:** `TestAPortalLeadsToAnotherZone` in the real window:
  a zone is made with an anchor, each Map adds a portal to the other's
  anchor by its menu, one is told to fade, and Launch's game walks through
  both, launched once.

- **Checks:** `make check` passes: formatting, the strict lint with no
  findings, vet, and every test with the race detector. The real-window
  suite, run from a frozen copy of the final code, passed all 85 of its
  tests. `TestAPortalLeadsToAnotherZone` passed ten runs in ten on its own
  first.

## Changed after its author tried it

- None yet.

## Not in this increment

- **The network:** a travel map between zones, fares, journeys, and time
  passing in travel.
- **A workflow node that moves the player,** and a portal that leads only
  when a value holds. A portal always leads.
- **A sound for a portal,** other kinds of change, and a game choosing how
  long a fade lasts.
- **A preview that hands over to another Map,** and moving the player in
  `berad-play`.
- **Adding an anchor while picking one,** so a zone with none can be led to
  in one go.
- **Portals in Project search.**
- **The sample project** has no portal; the walk makes its own.
- **Saves, an export and packaging.**
