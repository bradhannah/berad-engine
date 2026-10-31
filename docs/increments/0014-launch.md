# Increment 14: Launch

**Status:** Built

The fourteenth increment lets an author play their game, not only preview a
map. **Launch** opens a **Play tab** that runs the project as a game would,
with nothing of the editor around it: the game's screen of a fixed number of
cells, filling the tab while keeping its shape, with the player centred and
starting at the **player start**. Everything shows its sprite, walls and
characters included, and everything is heard: the sounds NPC workflows ask
for, ambient sounds placed on the map, louder as the player comes closer, and
the sounds a zone plays throughout. A new engine package, the **game runtime**, runs it,
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
   cells, as large as the tab allows while keeping its shape, with the player
   in the middle cell and darkness past the floor's edge.
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

Chosen while building, for its author to check:

- **The runtime reports each ambient sound's distance,** and the Play tab
  turns it into a gain on berad-audio's curve, the one the Audio window
  draws, since the engine module has no audio dependency.
- **The Play tab plays at full monitor levels.** Monitor levels belong to
  each tab's own player, so the Audio window's are not shared; this departs
  from the decision above.
- **A game behind another tab stops and goes quiet,** and carries on when its
  tab comes back to the front.
- **Cmd+R launches from any tab,** the project browser included, and Cmd+W
  closes the Play tab, as in every tab. The status bar stays under the Play
  tab, as under every tab.
- **Leaving the zone stops the game,** which says to press Cmd+R.
- **The sample's hum is heard within 5 cells,** where it was heard
  everywhere, so it can stand on the map; and the sample gains a clerk, an
  NPC with no behaviour standing by the desk.

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. Wall sprites | A wall stand-in can show a sprite, drawn in every wall cell on the Map | Headless tests: Shows offers walls, the Map draws the sprite in each wall cell and undo takes it away | Built |
| 2. Character sprites | An NPC's sprite, chosen in its inspector; the player's, from Shows; the Map and Preview draw them in place of discs | Headless tests for the NPC's format version, the choice and its undo; a real-window screenshot of Preview with both sprites | Built |
| 3. Game settings and the player start | The game settings document with the screen size; "Set player start here" on the Map's right-click menu, drawn as a marker | Headless tests: the document's format, setting, moving and undoing the start; the marker shows on its floor only | Built |
| 4. Ambient sounds on the map | Placing, moving and removing an ambient sound on a cell, with its marker and distance ring | Headless tests for the floor's new format version, each edit and its undo; only ambient cues are offered | Built |
| 5. A zone's sounds | The zone lists cues that play throughout, added and removed beside its palettes | Headless tests for the zone's new format version and the list's edits | Built |
| 6. The game runtime | An engine package that runs a snapshot of a project's zone: the player, NPCs, what is drawn around the player and what is heard, with each ambient sound's gain by distance | Headless tests that step a sample zone and check positions, drawn cells, cues asked for and ambient distances | Built |
| 7. Launch and the Play tab | Launch, the Play tab drawing the screen to fill the tab with the player centred, sounds through the pane's player and soundscape, relaunch | A real-window test that launches the sample, walks, takes stairs and checks what is drawn and heard, with screenshots; editing the map changes nothing until relaunch | Built |
| 8. The sample plays | The sample project gets a player start, a wall sprite, character sprites, an ambient sound and a zone sound, so `make demo` can launch | The sample's headless test checks each; `make check` and the real-window suite pass | Built |

## Step results

- **Wall sprites:** every built-in stand-in can now show a sprite, walls
  included, so Shows offers the four walls; the Map already drew a sprite in
  place of whatever stands on a cell, so a wall's sprite fills each wall cell
  without arms towards its neighbours. Tests: Shows offers a wall family, the
  stand-in record and its edit accept walls and still refuse unknown
  stand-ins, and family A's walls show Lantern's tile until it shows nothing.
- **Character sprites:** the NPC format goes to version 2, adding the sprite
  an NPC shows and reading version 1 unchanged, with the edit
  `npc.sprite.set`; the NPC's inspector gains a Sprite section, with Choose…
  and Open, under Behaviour. The stand-in sprites record keeps the player's
  sprite under `player`, which no stand-in uses, so Shows offers Player above
  the stand-ins. The Map follows the NPCs placed on its zone's floors and
  draws each, and the player in a preview, as its sprite filling a cell, with
  the selection or watched ring round it. Tests: the NPC format, its upgrade,
  the edit's undo and references, the inspector's choice and Open, Shows
  offering the player, and the Map drawing both. The real-window screenshot
  moves to step 7, where the Play tab draws both.
- **Game settings and the player start:** a new engine package, `game`,
  holds the game settings: the screen's columns and rows, odd numbers from 3
  to 63, 19 by 11 for a new project, and the player start, a cell of a floor
  of a zone. Every project holds them as a document with a fixed ID, in the
  folder `game-settings`, edited by `game.settings.screen` and
  `game.settings.start` from a new editor plug-in, `game`. The browser's
  Settings inspector shows the screen's two fields below the tile size and
  says where the player starts; the Map's right-click menu offers "Set player
  start here" on any cell the player can stand on, and "The player starts
  here" on the start, which it outlines in green and labels Start on its
  floor only. Tests: the settings' format and limits, the edits' undo and
  references, the screen fields refusing an even size, and the Map setting
  the start, marking it, and refusing a wall.
- **Ambient sounds on the map:** the floor format goes to version 5, adding
  ambient sounds, each an ambient cue on a cell with its own ID, one to a
  cell, and reading version 4 unchanged. The floor plug-in's
  `tilefloor.sound.add`, `.remove` and `.update` place, take off, move and
  recue them; a resize keeps them like anchors and placements, and undo puts
  back those it took off. The floor refers to each sound's cue, so deleting
  a cue shows where it is heard. The Map's right-click menu offers "Place
  ambient sound here…", then the library's ambient cues by name; on a sound,
  it offers changing its cue, moving it with a click on another cell, and
  removing it. Each is drawn as a purple marker with its cue's name and a
  faint ring at its distance. Tests: the floor's sounds and upgrade, the
  edits' undo, resize and references, and the Map placing, recueing, moving
  and removing a sound, with only ambient cues offered.
- **A zone's sounds:** the zone format goes to version 3, adding the cues it
  plays throughout, at most 16, and reading version 2 unchanged, with the
  edits `zone.sound.insert` and `zone.sound.remove`; the zone refers to each
  cue. The list under a zone's palettes, in the browser's inspector and the
  Map's, gains a second section, Sounds · played throughout, whose + Sound
  offers the library's music and ambient cues by name. Tests: the zone's
  sounds and upgrade, the edits' undo and references, and the Map's list
  adding the brook, a sound effect not offered, and taking it off.
- **The game runtime:** `game.New` starts a game from a project as it was
  when launched, the game settings, the start's zone with its floors, and the
  NPCs placed on them with their behaviours, and refuses one with no start or
  a start the player cannot stand on. Each `Step` walks the player, moves the
  clock on and steps every NPC through `npcrun`, as Preview does, returning
  the cues NPCs asked for, each once. `View` is the settings' screen of cells
  with the player in the middle; `Characters` are the NPCs where they are
  now; `Ambient` is each ambient sound on the player's floor with its
  distance from the player; and `ZoneSounds` the zone's cues. The engine
  module has no audio dependency, so the runtime reports distances and the
  Play tab turns them into gains on berad-audio's curve. Tests: the start and
  the view, refusing a bad start, walking into a wall, hearing the fountain
  and the clerk's bell once, climbing stairs, where the fountain is not
  heard, and the same input playing the same game.
- **Launch and the Play tab:** a new view, Play, whose target is the game
  settings, so there is only ever one. Launch, on the window bar left of the
  pinned clock, and Cmd+R in any tab, open it with the item `launch`; an
  open Play tab asked for that item launches again. A launch reads the game
  settings, the start's zone and floors, the NPCs placed on them and their
  behaviours, and the art they show, subscribing as it goes, and starts the
  game runtime once all have loaded; it copies what it reads, and draws the
  art once, so edits change nothing until the next launch. The tab draws the
  screen's cells around the player, ground then what stands on it, then the
  characters in order of their row, on a canvas of one pixel per tile pixel,
  scaled to fill the tab while keeping its shape, and centred on black. NPC
  cues play as asked; the zone's sounds and the ambient sounds within hearing
  play as layers of the tab's soundscape, each ambient sound's gain following
  the player. With no player start it says how to place one. Tests: headless,
  no start saying how, a launch reading every document, starting at the start
  with both sprites and a scale of 8, the music playing and the fountain
  coming within hearing, going quiet behind another tab, and an edit changing
  nothing until a second launch; and a real-window test that presses Cmd+R in
  the project browser, waits for the sample's game with brick walls, both
  figures, the tune and the hum, walks to the stairs and up them, and
  launches again at the start, with two screenshots.
- **The sample plays:** the sample gains a player start inside the door,
  Brick showing the walls of family A, Hero showing the player, a clerk with
  a figure of their own standing by the desk, the hum placed in the building
  and the tune played throughout, so `make demo` launches at once. Tests: the
  headless sample test checks each.

## Changed after its author tried it

Settled by its author on 2026-10-31, after trying it:

- **The screen fills the tab, keeping its shape,** with black bars where the
  tab's shape differs, in place of the largest whole-number scale, which left
  the game small in the editor window. A scale that is not whole is drawn by
  enlarging the screen by the next whole number with hard edges, then
  shrinking it to fit smoothly, so every pixel stays the same size and only
  their edges soften; a whole-number scale is drawn as it is.

## Not in this increment

- **Facings and walking frames** for characters, and animation in general.
- **Wall families,** which replace the one wall sprite.
- **Interaction:** opening doors, picking things up and talking to NPCs; the
  rest of the [first useful authoring milestone](../EDITOR_FEATURES.md#first-useful-authoring-milestone).
- **Modern Old Game's own program,** and a game's own modes such as a title
  screen.
- **Saving a game,** a HUD and anything else drawn over the map.
- **Hearing ambient sounds from other floors.**
