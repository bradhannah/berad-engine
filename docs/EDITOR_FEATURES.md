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

## Maps and tiles

- **Agreed:** a visual map editor.
- **Agreed:** **stand-in tiles** let an author lay out a map before its art
  exists. Stand-ins are typed (for example ground, generic wall, generic door,
  stairs up and down, small or medium pickup) so their role is recognisable. Default behaviour and
  how stand-ins are replaced are open.
- **Agreed:** a tile definition editor, separate as a function from drawing art.
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
- **Agreed:** walls **occupy whole tiles** (a wall is one tile thick and a
  doorway is a tile), not tile edges.
- **Agreed:** a cell has a **ground** tile and at most **one thing on top** (a
  wall, a door, stairs or a pickup stand-in). Objects and placements come
  later.
- **Agreed:** **stairs** are stand-ins, up or down. Walking onto them takes the
  player to the same cell one floor up or down.
- **Agreed:** doors are simply passable for now; opening and closing come
  later.
- **Agreed:** stand-ins are **built in** for now; editable definitions arrive
  with the definition editor.
- **Agreed:** a Map window opens a **zone** and shows one floor at a time, with
  the **floor below shown faintly** (it can be turned off). All views of a zone
  share one undo list.
- **Agreed:** pan and zoom the map; integer cell sizes keep pixel art uniformly
  scaled once real tiles replace stand-ins.
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
- **Candidate:** tile discovery through an atlas, a tree, keyword search, packs,
  or a combination, with the same tile possibly appearing in several groups.
- **Agreed:** tile size and base resolution are settings each game chooses, not
  hard-coded. A game decides its tile size up front and redraws its art if that
  changes; tiles of an incompatible size are flagged.

## Objects and definitions

- **Tentative:** **composition, not inheritance.** Walls, floors, roads, doors and
  items share one basic object type and differ by their characteristics (a wall
  is placed and static; a door is similar but changes state). One definition
  editor therefore covers what might otherwise be separate tile and item editors.
- **Agreed:** definitions can grow new properties as a game needs them. Example
  properties: gettable, weight, passable, blocks sight (independent of
  passable), emits light (with an amount and a type whose meaning is open),
  moveable, strength needed to move, and whether other objects can sit on top.
- **Agreed:** background tiles sit on the bottom layer, under other tiles, and
  cannot be picked up.
- **Agreed:** an object can cover several tiles while staying one entity, with
  its look combining several sprite cells.
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
- **Candidate:** per-map palettes for ambience. *Open:* whether art is stored as
  palette indices so a palette recolours it.
- **Candidate:** editing an item's art and definition together.
- *Open:* importing from or round-tripping with other tools such as Aseprite.

## World state, quests and rules

- **Agreed:** tracked state (flags), including state set by conversations, can
  change NPC behaviour, which objects appear and where, which variant of a tile
  or object is shown, and access to parts of a map. Logic flags work for access
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

## Preview, testing and validation

- **Agreed:** the editor plays a map: walking around, NPCs running their
  routines, interactions, and talking to NPCs (D15 covers preview behaviour).
- **Agreed:** a global clock for testing routines; on a jump, NPCs adapt to the
  new time immediately rather than simulating the gap: each restarts its
  workflow from the start and walks to wherever its new branch sends it.
- **Agreed (early compromise):** editing during preview restarts affected NPCs'
  behaviour rather than preserving their place in a changed workflow.
- **Agreed:** a workflow view can show the step each NPC is on and its inputs,
  ideally with the surrounding steps visible.
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
  engine's `play` package; see [the second build](SECOND_BUILD.md)).
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
catalogue. Many overlap: object and tile definitions are one editor; schedules
are part of workflows; quests and rules are views of world state. Audio authoring
is a separate tool.

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
