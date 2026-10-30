# Increment 14: Launch

**Status:** Planned

The fourteenth increment lets an author play their game, not only preview a
map. **Launch** opens a **Play tab** that runs the project as a game would,
with nothing of the editor around it: the game's screen of a fixed number of
cells, scaled up by whole numbers, with the player centred and starting at
the **player start**. Everything shows its sprite, walls and characters
included, and everything is heard: the sounds NPC workflows ask for, ambient
sounds placed on the map, louder as the player comes closer, and the sounds a
zone plays throughout. A new engine package, the **game runtime**, runs it,
so a game's own program can later run the same thing. It builds on
[increment 2](0002-walk-the-map.md)'s movement, [increment
3](0003-npc-workflows.md)'s NPCs, [increment
7](0007-ambient-and-soundscape.md)'s ambient distance and [increment
11](0011-drawing-tiles.md)'s sprites, and is held to the same strict checks
as [increment 1](0001-foundation.md).

## What it proves

1. **Launch plays the game:** pressing Launch opens the Play tab at the
   player start, with no editor chrome; the arrow keys walk, stairs change
   floor, NPCs follow their workflows, and closing the tab stops it.
2. **The screen is the game's:** the Play tab shows the game's screen size in
   cells, scaled by the largest whole number that fits, with the player in
   the middle cell and darkness past the floor's edge.
3. **Everything shows its sprite:** ground, walls, doors, stairs, pickups,
   NPCs and the player, each with the sprite chosen for it, or its flat colour
   or disc when it has none.
4. **Everything is heard:** NPC cues play as in Preview; an ambient sound
   placed on the map is heard within its cue's distance, louder as the player
   comes closer, on the curve the Audio window draws; a zone's sounds play
   while the player is in it.
5. **Launch plays what was there when it started:** editing while the Play
   tab runs changes nothing in it, and relaunching picks the edits up.
6. **Preview is unchanged:** it still starts from a clicked cell in the Map
   and shows edits live.

## Decisions

Asked for by its author on 2026-10-30, paraphrased: a Launch option beyond
the Map's Preview that shows the sprites and plays the sounds, to build
something for fun and try it; and the gaps that would stop that, such as
walls having no sprite.

Agreed by its author on 2026-10-30, before building:

- **Launch runs in a Play tab of the editor,** through a new engine package,
  the game runtime, that a game's own program can use later. Modern Old
  Game's own program is not part of this increment.
- **The player starts at a player start the author places,** once per
  project, on a map; Launch always begins there.
- **The player and each NPC show one sprite,** with no facings or walking
  frames yet. The player's sprite is chosen like a stand-in's; each NPC's in
  its inspector.
- **Sounds:** NPC cues, as now; **ambient sounds placed on the map**, each
  heard from its cue's distance; and **a zone's sounds**, such as music or a
  bed, which play the whole time the player is in the zone.
- **Walls show one sprite for now:** a wall stand-in can show a sprite, the
  same in every cell, with no corner or edge pieces, until wall families
  replace it.
- **A fixed screen of cells:** each game sets how many cells its screen
  shows. Modern Old Game's is **19 × 11** (608 × 352 pixels at its 32-pixel
  tiles), recorded in the research repository.
- **The view keeps the player centred,** always; past a floor's edge is dark.
- **Launch plays a snapshot:** it plays the project as it was when it
  started, as a game would; relaunching picks up edits.
- **Preview stays** in the Map, for quick checks: it starts anywhere and shows
  edits live.

Proposed by the assistant and accepted by its author on 2026-10-30, before building:

- **The Play tab shows the floor the player is on only,** not the floor
  below, and no grid, markers or editor overlays.
- **Launch is a button at the right of the window bar, beside the pinned
  clock, and Cmd+R,** which relaunches when a Play tab is open. Only one Play
  tab is open at a time.
- **A game settings document holds the screen size and the player start,**
  with a fixed ID beside the art settings. A new project's screen is 19 × 11
  until its author changes it, and with no player start Launch says so and
  offers the Map's "Set player start here".
- **The player's sprite is chosen from the Pixel editor's Shows menu,** which
  gains "Player" above the stand-ins.
- **An ambient sound is placed on a cell** of a floor from the Map's
  right-click menu, choosing among ambient cues, and is heard only on that
  floor. The Map draws it as a marker with a faint ring at its distance.
- **A zone's sounds are listed like its palettes,** in the zone's panel in
  the project browser and the Map, and play at the loudness their cues set.
- **Sounds play at the Audio window's monitor levels,** which stand in for a
  player's volume settings, as in Preview.

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. Wall sprites | A wall stand-in can show a sprite, drawn in every wall cell on the Map | Headless tests: Shows offers walls, the Map draws the sprite in each wall cell and undo takes it away | Built |
| 2. Character sprites | An NPC's sprite, chosen in its inspector; the player's, from Shows; the Map and Preview draw them in place of discs | Headless tests for the NPC's format version, the choice and its undo; a real-window screenshot of Preview with both sprites | Planned |
| 3. Game settings and the player start | The game settings document with the screen size; "Set player start here" on the Map's right-click menu, drawn as a marker | Headless tests: the document's format, setting, moving and undoing the start; the marker shows on its floor only | Planned |
| 4. Ambient sounds on the map | Placing, moving and removing an ambient sound on a cell, with its marker and distance ring | Headless tests for the floor's new format version, each edit and its undo; only ambient cues are offered | Planned |
| 5. A zone's sounds | The zone lists cues that play throughout, added and removed beside its palettes | Headless tests for the zone's new format version and the list's edits | Planned |
| 6. The game runtime | An engine package that runs a snapshot of a project's zone: the player, NPCs, what is drawn around the player and what is heard, with each ambient sound's gain by distance | Headless tests that step a sample zone and check positions, drawn cells, cues asked for and ambient gains | Planned |
| 7. Launch and the Play tab | Launch, the Play tab drawing the screen at a whole-number scale with the player centred, sounds through the pane's player and soundscape, relaunch | A real-window test that launches the sample, walks, takes stairs and checks what is drawn and heard, with screenshots; editing the map changes nothing until relaunch | Planned |
| 8. The sample plays | The sample project gets a player start, a wall sprite, character sprites, an ambient sound and a zone sound, so `make demo` can launch | The sample's headless test checks each; `make check` and the real-window suite pass | Planned |

## Step results

- **Wall sprites:** every built-in stand-in can now show a sprite, walls
  included, so Shows offers the four walls; the Map already drew a sprite in
  place of whatever stands on a cell, so a wall's sprite fills each wall cell
  without arms towards its neighbours. Tests: Shows offers a wall family, the
  stand-in record and its edit accept walls and still refuse unknown
  stand-ins, and family A's walls show Lantern's tile until it shows nothing.

## Not in this increment

- **Facings and walking frames** for characters, and animation in general.
- **Wall families,** which replace the one wall sprite.
- **Interaction:** opening doors, picking things up and talking to NPCs; the
  rest of the [first useful authoring milestone](../EDITOR_FEATURES.md#first-useful-authoring-milestone).
- **Modern Old Game's own program,** and a game's own modes such as a title
  screen.
- **Saving a game,** a HUD and anything else drawn over the map.
- **Hearing ambient sounds from other floors.**
