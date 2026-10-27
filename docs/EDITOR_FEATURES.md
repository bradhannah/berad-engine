# Editor features

What the editor must let an author do, and the agreed directions for each
feature. [The architecture](ARCHITECTURE.md) covers how the editor is built;
this document covers what it offers. Status words: **agreed** (settled
direction), **tentative** (current leaning, may change), **candidate** (an idea
under consideration), **open** (undecided).

## Principles

- **Agreed:** an author must be able to build and revise a game world through
  the editor alone. Routine content (maps, objects, NPCs, workflows,
  conversations, art) never requires changing engine code or asking an AI.
- **Agreed:** all authoring happens inside the editor. There is no separate
  hand-editing workflow for definitions and no round-tripping of workflows
  through text.
- **Agreed:** the editor reuses the engine's own systems and widgets wherever it
  can, so authoring, testing and play stay consistent. Editor components are
  written to be reusable (for example a workflow view as a game's debug overlay),
  but not everything needs to be.
- **Agreed:** apply SOLID deliberately so views, layouts and configuration can
  change without large rewrites; avoid reuse infrastructure for hypothetical
  needs.
- **Agreed:** a plain white look, keeping attention on the content, in the
  style of the editor wireframes for every window: IBM Plex Sans and Mono,
  dark outlines and rounded boxes, small uppercase labels, blue for selection
  and orange for anything live.
- **Agreed:** typed changes wait for an explicit save: every inspector with
  fields to type into, and the Text window, has a save bar with Revert and
  Save, and Cmd+S saves in any window. Enter saves a one-line field and
  starts a new line in a field of several lines. Buttons, choices and pick
  on map still apply at once and can be undone. Selecting something else or
  closing a window with unsaved changes asks: Save, Don't save or Cancel.
- **Agreed:** Cmd+W closes a window; Cmd+Q in any window asks whether the
  author is done and, if so, closes every window. With unsaved changes in
  any window it names them and offers Save and quit, Don't save or Cancel.
- **Agreed:** document types are told apart by an icon, always with a muted
  colour beside it, never by colour alone: zones and floors green, workflows
  purple, NPCs slate, text brown. Blue and orange stay for selection and
  live.
- **Agreed:** any window can be kept on top of the others with the On top
  toggle at the right of its status bar. The Clock window starts on top.
- **Agreed:** a window's keys are listed on a sheet opened with its Keys
  button or ?, rather than always on screen.
- **Agreed:** the project browser keeps a growing project findable with
  folders, tags and a search box. Folders sit inside each type's section,
  hold documents of that type, nest to any depth, and are real folders on
  disk. Tags group documents across types. The search box matches names,
  folder names and tags, and "#" searches tags only. Documents and folders
  move by dragging or with Move to….
- **Agreed:** right-clicking in the project browser makes documents and
  folders where the click landed: a section heading makes them at the top
  of its section, a folder inside it, and a document beside it.

## Maps and tiles

- **Agreed:** a visual map editor, with a floating zoom control in the map's
  bottom-right corner (−, the zoom as a percentage, + and Fit). Scrolling
  pans the Map, Workflow and Watch windows the same way.
- **Agreed:** **stand-ins** let an author lay out a map before its art
  exists. Stand-ins are typed (for example ground, generic wall, generic door,
  stairs up and down, small or medium pickup) so their role is recognisable. Default behaviour and
  how stand-ins are replaced are open.
- **Agreed:** a definition editor, separate as a function from drawing art.
- **Agreed:** **wall families** (meta-tiles): the author picks a family such as
  "old brick" and the editor chooses each corner and edge variant from its
  neighbours, without routine manual tweaking; manual overrides remain possible.
  Where different families meet, a plain hard join is acceptable at first;
  blending is later.
- **Agreed:** **drawing walls** is a hybrid. Walls are drawn as structure with a
  current family, which defaults to unassigned (the stand-in) or the last family
  used. The family is a property that can be changed for a whole selection: a
  connected run, a room, or every wall in an area, plus a replace-everywhere
  command. Floors and roads can follow the same pattern.
- **Agreed:** walls **occupy whole cells** (a wall is one cell thick and a
  doorway is a cell), not cell edges.
- **Agreed:** a cell has its **ground** and at most **one thing on top** (a
  wall, a door, stairs or a pickup stand-in). Objects and placements come
  later.
- **Agreed:** **stairs** are stand-ins, up or down. Walking onto them takes the
  player to the same cell one floor up or down.
- **Agreed (2026-10-25):** stairs are **one-way** or **two-way**. One-way
  stairs, such as a hole, lead to the floor below and need only their down
  end. Two-way stairs record an up end and a down end: placing one end places
  the other on the floor above or below, in the same cell, and deleting one
  end asks about the other.
- **Agreed (2026-10-25):** the engine fixes the map's **draw order** for every
  game, with nothing to configure: the ground first, then what stands on the
  map, then whatever is marked to draw over everything, such as roof tops.
- **Agreed (2026-10-25):** what stands on the map, objects and characters
  alike, is drawn in order of its **base row**, the row where it touches the
  ground, so whatever is lower on screen is drawn in front. Walking behind a
  tree's trunk puts the player under its canopy; walking in front of it puts
  them over it. Each sprite has one base row, its bottom row unless its author
  chooses another. Flat things such as rugs and shadows stay with the ground.
  Fully top-down art loses nothing: every base row is then its sprite's only
  row. (Rejected: heights authored per tile or pixel, as too costly to author
  and to reason about; and marking each tile under or over characters, which
  breaks when sprites overlap neighbouring cells.)
- **Agreed:** doors are simply passable for now; opening and closing come
  later.
- **Agreed:** stand-ins are **built in** for now; editable definitions arrive
  with the definition editor.
- **Agreed:** a Map window opens a **zone** and shows one floor at a time, with
  the **floor below shown faintly** (it can be turned off). All views of a zone
  share one undo list.
- **Agreed:** pan and zoom the map; integer cell sizes keep pixel art uniformly
  scaled once sprites replace stand-ins.
- **Agreed:** beyond the floor bounds the map is shaded, with a fainter grid of
  the same cells, so the space around the floor can be pictured.
- **Agreed:** right-click on the map opens a menu of what can be done at that
  cell, acting on what is there (an anchor, an NPC placement, a wall) or on
  the cell itself (add an anchor, place the chosen NPC, remove what is on
  top, preview from here). On a cell beyond the floor bounds it offers
  Extend floor to here.
- **Agreed:** the **zone origin**, cell 0,0, is fixed and shown on the map, and
  every floor of a zone shares it, so stairs line up. Floors grow or shrink on
  any side without moving what is already placed; cells left of or above the
  origin have negative numbers. New floors start centred on the origin.
- **Agreed:** floors are **resized** in the Map window, adding or removing rows
  and columns on any side, each change one undo step.
- **Tentative:** keep the map view simple at first (no full lighting or
  visibility editing) and enrich it over time.
- **Agreed:** portal-linked maps do not need to be shown stitched together;
  convenient access through their doorways is enough.
- **Candidate:** finding what to paint through an atlas, a tree, keyword
  search, packs, or a combination, with the same definition possibly appearing
  in several groups.
- **Agreed:** tile size and base resolution are settings each game chooses, not
  hard-coded. A game decides its tile size up front and redraws its art if that
  changes; tiles of an incompatible size are flagged.

## Objects and definitions

- **Tentative:** **composition, not inheritance.** Walls, floors, roads, doors and
  items share one basic object type and differ by their characteristics (a wall
  is placed and static; a door is similar but changes state). One definition
  editor therefore covers what might otherwise be separate editors for walls,
  floors and items.
- **Agreed:** definitions can grow new properties as a game needs them. Example
  properties: gettable, weight, passable, blocks sight (independent of
  passable), emits light (with an amount and a type whose meaning is open),
  moveable, strength needed to move, and whether other objects can sit on top.
- **Agreed:** the ground sits on the bottom layer, under everything else, and
  cannot be picked up.
- **Agreed:** an object can cover several cells while staying one entity; its
  sprite is a grid of tiles (see [the pixel editor](#pixel-editor)).
- **Agreed (2026-10-25):** a definition's characteristics decide which sprites
  it needs, as a wall family needs one per neighbour arrangement. A door's
  state needs a closed and an open sprite, with optional opening and closing
  sprites that play once; two-way stairs need an up and a down sprite; a
  character's Moves characteristic needs a sprite for each facing and action
  (stand, walk), and walk frames advance one per step, so a slow walker never
  slides. Each game sets how many facings it uses. The game's state picks which
  sprite shows, and the sprite animates itself; one sprite can serve several
  definitions. (Rejected: one sprite with named sequences the game jumps
  between; separate definitions for each state.)
- **Agreed (2026-10-25):** moss, damage, snow and the like are **overlays**:
  a definition lists sprites drawn over its own, each shown when a state or
  flag is set, so they combine freely. A wall family's overlay is a family
  too.
- **Agreed:** interactions start as simple flags (can pick up, can move).
  **Candidate:** an optional workflow attached to using an object, which might
  occasionally pause for player input.
- **Candidate:** properties split into unalterable ones and defaults that a
  placement can change.
- **Candidate:** items sharing one sprite, told apart by generated borders.
- **Agreed:** a **placement** is a definition put on a map; it inherits from the
  definition and can override some properties.

## Pixel editor

- **Agreed:** a built-in pixel editor, despite the extra work, so art can be made
  with the game's own rules in mind: templates and guides (for example a stair
  angle that must line up with floor height), drawing variants in context, and
  control of palettes.
- **Agreed:** it understands transparency and the palette, and offers basic
  tools (pencil, line, rectangle, circle, ellipse, fill, select, eyedropper),
  with filled and outline modes. Gradients and similar come later.
- **Agreed:** no vector art; shape tools produce pixel art.
- **Agreed (2026-10-25):** art is drawn only in the built-in pixel editor.
  Nothing is imported from, or round-tripped with, other tools such as
  Aseprite.
- **Candidate:** editing an item's art and definition together.

Asked for by its author on 2026-10-25, paraphrased: an editor in the spirit of
Aseprite that understands the game (its tile size, what is a wall, what can be
picked up), restricts what may be painted where the game needs it (stairs),
treats palettes as central (a default for the game's look, others for a
feeling), understands transparency and semi-transparency (glass), and animates
with frames of varying length and optional parts, starting simple. Agreed by
its author on 2026-10-25, over several rounds of questions and wireframes:

### Tiles and sprites

- **Agreed (2026-10-25):** a **tile** is one cell-sized piece of art; a
  **sprite** is one or more tiles, and is what a definition shows. The grid
  position on a map is a **cell** (see [the vocabulary](VOCABULARY.md)).
- **Agreed (2026-10-25):** a sprite bigger than one cell has a grid of tiles in
  each frame: a 3 × 2 table has six, and a longer table repeats the middle
  ones. Overhang, such as a canopy above a trunk, is a tile in the
  neighbouring cell. A big sprite is drawn on one canvas, with its tile
  boundaries shown. (Rejected: one larger image per frame.)
- **Agreed (2026-10-25):** reusing a tile **copies** it by default. **Paste
  linked** shares one tile instead, so a change follows everywhere; a linked
  tile shows its other uses and can be made unique again.
- **Agreed (2026-10-25):** no sheets. Tiles are found with folders and tags
  like other documents; how they are packed into image files on disk is the
  editor's business.
- **Agreed (2026-10-25):** a sprite can list **alternatives** with weights,
  such as three grasses. Each cell picks one from its position, so the choice
  is stable and the same in the editor and the game, and a placement can pin
  one by hand.

### Animation

- **Agreed (2026-10-25):** a sprite animates as a graph of **sequences**. A
  sequence is frames in order, each shown for its own duration; when it ends,
  weighted choices pick the sequence that plays next, so a torch burns, now
  and then glints, and burns on. Sequences chain one into another. The
  smallest sprite is one sequence of one frame. (Rejected: marking single
  frames optional with a chance, since an optional moment can need several
  frames.)
- **Agreed (2026-10-25):** whether a sprite's placements animate **in step**
  or each from its own point is chosen per sprite: water in step, so
  neighbouring cells join up; torches offset, so they don't flicker in unison.
  The offset comes from the cell, so the editor and the game always agree.
- **Candidate:** palette cycling: a ramp turning its shades over time, so
  everything painted with it shimmers without extra frames.

### Palettes and transparency

- **Agreed (2026-10-25):** a pixel refers to a palette colour by its **scoped
  name**, such as `shared / grass 2` or `gloomy / smoke 1`, never by number or
  value, so changing a palette colour changes every tile that uses it.
- **Candidate:** each name sits on a permanent ID, as anchors do, so renaming a
  ramp never breaks a tile.
- **Agreed (2026-10-25):** colours come in **ramps** named for their use, not
  their hue, numbered dark to light ("grass" 1 to 6); in its author's words,
  "It's not green - it's grassColor1". This is how pixel art is shaded, and
  keeps names few. (Rejected: a name on every colour.)
- **Agreed (2026-10-25):** **palettes** are documents of their own: one shared
  palette for the game's look, and further palettes named for a feeling rather
  than a place, such as "gloomy". A further palette adds colours; it never
  recolours the shared ones. A tile can use colours from any palette and keeps
  them wherever it is placed, so a palette matters while drawing, not while
  placing. A zone can refer to a palette, so the editor offers it first while
  drawing that zone's art.
- **Agreed (2026-10-25):** no colour budget: the editor does not limit how
  many colours a palette holds or a zone shows.
- **Agreed (2026-10-25):** each palette colour can carry its own **opacity**
  ("glass 1, 40%"), so painting with it is see-through; one colour is fully
  clear. (Rejected: opacity steps per pixel, dither-only transparency, and one
  opacity for a whole sprite.)

### Drawing

- **Agreed (2026-10-25):** a **template** can keep part of a tile clear, where
  the pencil cannot paint, so the floor below shows through stairs or a hole,
  besides the guides it gives to draw to, such as a step angle or a floor
  height. Guides are never part of the tile.
- **Agreed (2026-10-25):** **drawing layers** (base, detail, shading) while
  painting a tile, kept by the editor for later edits and flattened into one
  tile for the game. (Moss and the like are overlays, not layers: see
  [objects and definitions](#objects-and-definitions).)
- **Agreed (2026-10-25):** the **dither** tool is a pencil with a pattern. It
  paints the two current colours at 25%, 50% or 75%; the second colour starts
  as the next shade of the ramp, and any colour can be chosen instead. Nothing
  cleverer.
- **Tentative:** characteristics such as wall or gettable belong to the
  definition, shown beside the art in the editor, and the art itself knows
  nothing of them, since one sprite can serve several definitions.

## World state, quests and rules

- **Agreed:** tracked state (flags), including state set by conversations, can
  change NPC behaviour, which objects appear and where, which sprite a
  definition shows, and access to parts of a map. Logic flags work for access
  rules, not only for dialogue.
- **Tentative:** placement conditions refer to flags, not to the conversations
  that set them, so any workflow can set the flag.
- **Agreed:** world state is organised as a tree of flags, stats, skills and
  capabilities.
- **Agreed:** games define their own numeric **skills** and **stats** and their
  own binary **capabilities**, evaluated dynamically.
- **Agreed:** quests are first-class, with workflow operations to give a quest,
  add or tick off tasks, and complete it explicitly. Finding something can give a
  quest, not only a conversation. *Open:* whether completing every task
  completes the quest.
- **Candidate:** workflows writing journal or quest-log notes.
- **Tentative:** control over when a conditional change becomes visible (for
  example on leaving a zone or area, immediately, or with a visual effect), so
  objects never appear inexplicably in view.

## Workflows

- **Agreed:** behaviour is authored as workflows in the editor, so routine
  behaviour never needs new engine code. Start simple and extend as worlds grow.
- **Agreed:** workflows are built from **typed nodes** (logic, movement, action,
  audio, effects) whose parameters have types, including enums such as slow and
  fast or and and or. Node and parameter structures must be designed carefully,
  because the editor builds its controls from them (an anchor parameter offers
  "Pick on map"). Start with basic nodes and extend.
- **Agreed:** conversations are workflow diagrams too, including optional skill
  or stat evaluations with more than two outcomes.
- **Agreed:** a conversation can be tested on its own, without a map, using a
  changeable test profile for the player (stats and skills to start with). Test
  profiles form a collection and are not game characters.
- **Agreed:** ambient NPC-to-NPC exchanges live in the NPCs' behaviour
  workflows, not in conversation workflows.
- **Tentative:** schedules and behaviour workflows are two parts of one thing. A
  workflow can have a hub that evaluates conditions and branches. Time enters
  through a dedicated time-switch node, or through general logic nodes where time
  is one comparison among many, or both.
- **Tentative:** a **time-bound container** of nodes has a known exit; when its
  time is up, flow leaves through the exit to wherever the author wires it.
  Explicit checks inside a branch also work. **Agreed:** the NPC leaves at once
  when the time is up, even partway through a node. *Open:* nesting
  containers.
- **Agreed:** when the flow reaches the end of a branch with nothing connected
  after it, it goes back to the start, so a time switch there picks the
  branch for the time now; inside a time-bound container it repeats the
  container's contents.
- **Agreed:** reusable workflows with input parameters, assigned to many NPCs.
  **Tentative:** changes to a shared workflow apply to every NPC using it; a
  derived workflow inherits parent steps it has not overridden. **Candidate:**
  showing which NPCs use a workflow, and duplicating one (an independent copy).
- **Agreed:** custom behaviours compose logic, randomness and built-in
  behaviours such as wandering.
- **Agreed:** editing flow matters more than arranging boxes: layout should mostly
  take care of itself, with manual arrangement available. Optional colours on
  nodes and containers, annotations and note nodes keep large workflows readable;
  layout is saved apart from logic (D17).
- **Candidate:** cut scenes as workflows that block player input.

## NPCs

- **Agreed:** selecting an NPC on the map gives immediate access to its details
  and workflows. NPC flags grow as a game needs them.
- **Agreed:** each NPC can carry private author notes, never shown to players.
  **Candidate:** description fields usable in an in-game guide.
- **Agreed:** an NPC keeps one identity across zones and may have placements in
  several; logic decides whether it appears at a placement.
- **Agreed:** NPCs travel to destinations by pathfinding and perform behaviour
  there, looping within scheduled times.
- **Candidate:** positions entered by picking on the map rather than typing
  coordinates, using anchors and areas.

## Sound

- **Agreed:** audio editing moves into the editor. The berad-sdk terminal audio
  editor stays for Ultima V Redux. Both read and write berad-audio's formats
  only through its authoring package, and those formats change only through a
  new version, so Ultima V Redux stays compatible. Choosing cues for workflows
  comes first ([increment 4](increments/0004-sounds-in-workflows.md)); the Audio window follows
  ([increment 5](increments/0005-audio-window.md)), then trimming and levels ([increment 6](increments/0006-trimming-and-levels.md)), then ambient distance, profiles and families, and
  the soundscape ([increment 7](increments/0007-ambient-and-soundscape.md)).
- **Agreed:** the Audio window does everything the terminal audio editor does
  except its Ultima V routing (sounds per tile type, footstep surfaces, named
  emitters, the music, dungeon and naval groups), in three increments: adding and
  editing sounds and cues, then trimming and levels, then ambient distance,
  fades and the soundscape.
- **Agreed:** audio is always saved and undone with Cmd+Z, like everything
  else; there is no Save button.
- **Agreed:** adding a sound copies its file into the project and asks
  straight away who made it, where it came from and its licence, with an
  "Allowed in a commercial game" checkbox and a reminder that some free
  licences forbid commercial use. The checkbox is berad-audio's schema 4.
- **Agreed (2026-10-25):** the attribution of the recording in focus shows in
  the Audio window's panel, as the terminal editor shows it in its details,
  with the commercial checkbox ticked there. Its source type and processing
  steps, which berad-audio already keeps, can be seen and edited.
- **Agreed (2026-10-25):** a recording's dB adjustment is its **level**;
  **gain** means only a cue's, or a soundscape layer's, percentage.
- **Agreed (2026-10-25):** on the waveform, only the region's two ends are
  dragged; a press elsewhere does nothing.
- **Agreed (2026-10-25):** recordings sound equally loud without their
  author judging them by ear: each is evened out to one reference loudness,
  measured automatically, and the editor shows it on a meter. How loud a cue
  is meant to sound is chosen on the cue, in a few named steps around the
  reference; a Play sound node may choose another step for one moment.
  Monitor levels and soundscape gains are for listening only. Five steps,
  from −12 to +8 dB; one reference for every recording with an adjustment per
  category; louder cues through berad-audio's schema 5 ([increment 8](increments/0008-loudness.md)).
- **Agreed (2026-10-25):** a file is kept apart from its uses: one file can
  be cut into many recordings, and one recording used by many cues, without
  copying the file. Evening out belongs to the recording; how loud it is meant
  to sound belongs to the cue. A file's pieces are seen together on its own
  screen ([increment 9](increments/0009-files-and-pieces.md)).
- **Agreed:** the editor understands a cue's category: music, SFX or ambient.
- **Agreed:** a Play sound node plays a cue once and the workflow carries on;
  most sounds can come from a node.
- **Candidate:** a sound that comes with a conversation, such as a "mumble".
- **Candidate:** ambient sounds that keep playing, so the world and its NPCs
  feel alive. An object's sound belongs to its definition, so every fountain
  gurgles, unless a placement overrides it.
- **Candidate:** NPCs have an optional ambient sound: a default on the NPC, or
  a node that sets it, which the engine then keeps playing. After a workflow
  restarts, authors tidy it up in their own workflows for now; convenience
  nodes can come later.
- **Candidate:** music changes without sounds disagreeing. Zones have handlers
  such as On enter and On exit that set the music; later, state machine nodes
  change it.
- **Candidate:** handlers, workflows that run when something happens, for each
  kind of thing: the game, zones, areas, object definitions and placements,
  NPCs, conversations and quests. "On create" runs each time something comes
  alive, for example when its zone loads, not once per game. It also runs when
  a placement appears partway through a visit because a flag changed, so the
  placement behaves the same as one that was there when the player arrived.
  The proposed term is *handler*, since *event* already means the
  orchestrator's notice of a change.
- **Candidate:** a zone loop that watches flags and other things affecting the
  zone, and can control ambient sounds as well as music. It is not based on
  turns and pauses when the game pauses; *open:* whether a second loop runs by
  turn.
- **Agreed:** an ambient sound is heard from a number of cells, N, growing
  louder as the player comes closer, with Ultima V Redux's curve
  (berad-audio's `QuadraticDistanceGain`): full volume within one cell, then
  1 − (distance ÷ N)², silent at N. Distance is a straight line between cells,
  and walls are ignored. Only the loudest source of each cue plays, the
  nearest when two are equally loud; different cues still play together.
- **Candidate:** N comes from the cue, and an object definition, placement or
  NPC can override it and the volume in the game's data, so the audio formats
  stay unchanged.
- **Candidate:** each floor between the player and a sound adds 6 cells to the
  distance before the curve applies, as one project-wide setting tuned by ear.
- Sound on the map, such as a layer showing where ambient sounds are heard,
  is set aside for later.

## Preview, testing and validation

- **Agreed:** the editor plays a map: walking around, NPCs running their
  routines, interactions, and talking to NPCs (D15 covers preview behaviour).
- **Agreed:** a global clock for testing routines; on a jump, NPCs adapt to the
  new time immediately rather than simulating the gap: each restarts its
  workflow from the start and walks to wherever its new branch sends it.
- **Agreed:** the clock's controls live in the Clock window, as in the
  wireframes. Before a preview it sets the time the next one starts at;
  during one it runs the preview's clock: an hour back or on, a jump to a
  time, stopping and starting it, and its rate. It opens by itself when a
  preview starts, and is on top by default.
- **Agreed (early compromise):** editing during preview restarts affected NPCs'
  behaviour rather than preserving their place in a changed workflow.
- **Agreed:** a workflow view can show the step each NPC is on and its inputs,
  ideally with the surrounding steps visible.
- **Agreed:** Step to next change goes straight to the watched NPC's next
  node, however long its node takes, and shows the way it went (each node
  passed, the choice taken, what was blocked) until the NPCs resume or step
  again.
- **Tentative:** loadable test scenarios set just enough state (conditions true
  or false) to test something without replaying the game. Later: prepared save
  states and jumping straight to a location.
- **Agreed:** built-in checks for common problems: broken references and
  unreferenced content. Unused content is not automatically an error.

## Games and debugging

- **Agreed:** games contain no editor functionality. Games may have debug tools
  that inspect and change state at run time, for the current session only, never
  the authored files; debug tools may be "a little unsafe" rather than heavily
  guarded. *Candidate:* a debug display as an in-game overlay or a separate
  application connected over a socket.

## Runtime directions

What the engine provides to games, as far as decided:

- **Agreed:** avoid large amounts of specialised game logic in compiled engine
  code. Workflows are the designer-authored layer above the engine; the engine
  provides the building blocks. The balance between engine logic and scripting is
  deliberate, and no scripting language is chosen.
- **Agreed:** the player walks one cell at a time: a tap moves exactly one cell,
  holding keeps walking, and the player slides smoothly between cells, in eight
  directions, never squeezing diagonally between two cells that block (the
  engine's `play` package; see [increment 2](increments/0002-walk-the-map.md)).
- **Agreed:** uniform pixel scaling is guaranteed (no stretching), and layout
  supports several popular aspect ratios rather than one fixed layout. Text uses
  TTF fonts.
- **Agreed:** OS-specific features stay behind abstractions; broad portability is
  intended, with development on macOS.
- *Open:* the game save format. JSON (readable, possibly large) and SQLite
  (compact, possibly too rigid while iterating) are candidates.

## Editor inventory

**Agreed candidates** for what the editor covers: maps and tiles, the pixel
editor, object and NPC definitions, behaviour and conversation workflows,
schedules, quests, world state (flags, stats, skills, capabilities) and the rules
catalogue. Many overlap: definitions of objects, walls and floors are one editor; schedules
are part of workflows; quests and rules are views of world state. Audio editing
moves into the editor (see [Sound](#sound)).

**Window set (current wireframes):** project browser, Map, Workflow (including
conversations), Pixel editor, World state, Conversation test, Clock and Problems,
each with an embedded inspector (D13).

## First useful authoring milestone

Build a map, place NPCs with basic logic and schedule loops, then walk around it
and interact:

- player movement and a basic command foundation (action first, then target);
- NPC schedules and custom behaviour workflows from the start, not only built-in
  behaviours;
- layered ground, furniture on top, and items on furniture that can be picked up;
- conversation workflows with standalone testing and test profiles;
- walking up to NPCs and talking to them in preview, with a simple provisional
  conversation interface.

## Open questions

- Which workflows should be visual, textual, or both.
- How much scripting beyond workflows, if any.
- Which kinds of changes should never need engine changes.
- What feedback makes conditions and interactions understandable.
- The smallest toolset that allows fully independent authoring.
