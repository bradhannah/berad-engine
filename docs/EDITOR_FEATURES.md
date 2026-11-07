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
  style of the editor wireframes for every view: IBM Plex Sans and Mono,
  dark outlines and rounded boxes, small uppercase labels, blue for selection
  and orange for anything live.
- **Agreed:** typed changes wait for an explicit save: every inspector with
  fields to type into has a save bar with Revert and Save, and Cmd+S saves
  in any pane. Enter saves a one-line field and
  starts a new line in a field of several lines. A Say's and a Show
  message's line is one field that may break: Enter saves it, and
  Shift+Enter starts a new line (2026-11-06). What is put in a field
  another way waits for Save as typing does: a colour picked, a value or
  constant inserted, a constant chosen for a number, a line written in the
  writing sheet (2026-11-06, [increment 33](increments/0033-constants.md)).
  Buttons, choices and pick
  on map still apply at once and can be undone. Selecting something else or
  closing a tab with unsaved changes asks: Save, Don't save or Cancel.
- **Agreed (2026-10-30):** the editor is **one window**, the editor window,
  in one process ([increment 13](increments/0013-single-window.md)). Its
  window bar has a tab for each kind of view open, the project browser's
  first; a second row lists the open documents of the kind in front when
  there are several; the pinned clock sits at its right end; and Watch opens
  beside a Map of its zone, in that Map's tab. Each view runs in a pane of a
  tab. There are no separate windows, and nothing pops out into one.
  (Rejected: a window process for each view, as the editor ran until
  increment 12.)
- **Agreed (2026-11-04):** **the inspector folds.** An Inspector toggle in
  the status bar folds every tab's inspector to a strip at the right edge,
  and clicking the strip opens it again. (Rejected: an inspector floating
  beside the selection, quick settings on the node, one docked along the
  bottom, auto-hide, and a layout fitted to the window's width; each meant
  another layout to keep in step.)
- **Agreed (2026-11-04):** on a wide window **a list keeps a readable
  width,** each row's detail in a column close to its name, and the
  inspector stands beside the list instead of at the far edge. World
  state's Values and Signals do so first. [Increment
  29](increments/0029-lines-state-and-picking.md) builds the readable lists
  and the fold.
- **Agreed (2026-11-05):** the Project window is sliced by **facets**:
  Kind, Zone, Runs as, Folder, Tags, Changed and Problems, each value with
  how many things ticking it would list, so no tick leads to nothing. The
  list stays the tree until something is typed or ticked, then shows a
  table of what is left. A set of ticks is saved with the project as a
  view. Every document records when it last changed, and every workflow's
  problems are known without opening it. (Set aside, from the [research
  notes](RESEARCH_NOTES.md#the-project-window-at-scale-researched-2026-11-04):
  a working set, stepping along relations, an overview, columns and quick
  open; the atlas is kept as an idea, with a graph of how things relate,
  for when the strong associations between them are settled.) [Increment
  30](increments/0030-project-facets.md) builds it.
- **Agreed (2026-11-06):** after trying the facets, **the Project is Home,
  then folders beside what they hold.** Home, at the top left and what the
  window opens on, lists recent things across every kind, pinned things,
  what needs attention and saved views; recents and pins are kept per
  person. A tree of folders on the left holds folders, never things; the
  main pane shows the folder picked in a view that suits it: pictures for
  sprites, tiles and palettes; rows with columns for workflows; cards for
  NPCs and zones; waveforms with loudness for sounds; and a preview, one
  large with the rest in a strip. The facets become a bar of dropdowns that
  narrows the tree to the folders holding matches. (Replaced: increment
  30's facets column, and the table a tick switched to, since a flat table
  does not scale and a tick changed what the list was. Set aside: a
  narrowing tree beside the facets, grouping by any facet, and working
  inside one place at a time.) [Increment
  32](increments/0032-project-by-folders.md) builds it.
- **Agreed:** Cmd+W closes the focused pane, and a tab's close box closes its
  tab; Cmd+Q, or closing the project browser's tab or the editor window, asks
  whether the author is done and, if so, closes the editor. With unsaved
  changes in any tab it names them and offers Save and quit, Don't save or
  Cancel.
- **Agreed:** document types are told apart by an icon, always with a muted
  colour beside it, never by colour alone: zones and floors green, workflows
  purple, NPCs slate. Blue and orange stay for selection and live.
- **Agreed:** the editor window can be kept above every other application's
  windows with the On top toggle at the right of any pane's status bar.
- **Agreed:** a view's keys are listed on a sheet opened with its Keys
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
  pans the Map, Workflow and Watch the same way.
- **Agreed (2026-10-30):** a two-finger scroll on a trackpad moves the Map,
  Watch, Workflow and Pixel editor canvases, and the palette swatch lists,
  with the fingers, as macOS scrolls pages; it had moved them the other way.
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
- **Agreed (2026-10-30):** until wall families exist, a wall stand-in can show
  **one sprite**, the same in every cell, with no corner or edge pieces.
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
- **Agreed:** a Map opens a **zone** and shows one floor at a time, with
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
- **Agreed:** floors are **resized** in the Map, adding or removing rows
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
  Aseprite. (Changed on 2026-11-01: art may be **imported one way**, such
  as a bought pack or a stand-in, and is never exported or round-tripped.
  How it comes in, sprite sheets or separate files, is worked out with its
  author once there is a real pack to bring in, together with each tile's
  origin and whether it was made with AI.)
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
  name**, such as `default / grass 2` or `gloomy / smoke 1`, never by number or
  value, so changing a palette colour changes every tile that uses it.
- **Candidate:** each name sits on a permanent ID, as anchors do, so renaming a
  ramp never breaks a tile.
- **Agreed (2026-10-25):** colours come in **ramps** named for their use, not
  their hue, numbered dark to light ("grass" 1 to 6); in its author's words,
  "It's not green - it's grassColor1". This is how pixel art is shaded, and
  keeps names few. (Rejected: a name on every colour.)
- **Agreed (2026-10-25):** **palettes** are documents of their own: one
  palette for the game's look, and further palettes named for a feeling rather
  than a place, such as "gloomy". (Changed on 2026-10-28: the palette for the
  game's look is Default, below.) A further palette adds colours; it never
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

- **Agreed (2026-10-28):** the engine ships one built-in palette, **Default**,
  of 256 colours of its own making, the same in every game and read only: its
  colours can be copied, never changed. A game adds palettes of its own beside
  it, made empty or from Default's ramps and single colours, and every palette
  but Default can be edited, renamed and deleted.
- **Agreed (2026-10-28):** all palette editing is in the **Palette Editor**, a
  view of its own; the Pixel editor only picks colours. A colour can
  carry a short **label**, at most 15 characters and unique within its
  palette. A **gradient** fills a ramp from a left colour, a right colour and
  a number of steps, mixing the colour, the opacity or both.
- **Agreed (2026-10-28):** moving a colour keeps a shadow record of its old
  place, so tiles still find it and warn until repainted. Deleting a palette
  is refused while a zone or tile refers to it; removing a palette from a tile
  keeps each pixel's colour, as Default's exact match or as a **loose colour**
  that names no palette and is flagged.
- **Agreed (2026-10-28):** a zone lists the palettes it uses, and the Pixel
  editor offers the palettes of the zones a sprite is placed in first, A–Z,
  then Default, then the rest. The details are in
  [increment 12](increments/0012-palettes.md).
- **Agreed (2026-10-30):** a colour is chosen with a colour picker as well as
  by its hex value; the editor draws the picker, so it is the same on every
  system.

### Drawing

- **Agreed (2026-10-25):** a **template** can keep part of a tile clear, where
  the pencil cannot paint, so the floor below shows through stairs or a hole,
  besides the guides it gives to draw to, such as a step angle or a floor
  height. Guides are never part of the tile.
- **Agreed (2026-11-01):** a template marks pixels **not allowed** (its
  author's words, for "kept clear") or **discouraged**: an item that never
  uses its outside border pixels, a character that keeps three pixels clear
  at each side. Not allowed is absolute: the tools never paint there, and
  breaking it means changing or removing the template. Discouraged pixels
  can be painted, and are shown and counted. Templates are documents of
  their own, edited in a **Template editor** tab; the Pixel editor chooses a
  sprite's template, and New ▾ makes a sprite already drawn to one
  ([increment 16](increments/0016-templates.md)).
- **Candidate:** a folder names the template its new sprites are drawn to,
  once folders can carry settings; and families of pieces, such as a wall
  family made into a folder of its pieces, each with a template made from
  the family's guides.
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
- **Agreed (2026-11-02):** in the World state window a value or group moves
  into a group by a right-click, Move into, or by dragging it onto the group,
  and out of one by dragging it onto its section's heading; it never moves
  between the world, zones and NPCs. A dotted name in the Name box moves it
  too, which a line under the box says. Workflows name a value by itself, not
  by its path, so a move rewrites nothing; a value something uses asks first,
  listing each use as it will read, and a move is undone as one change.
- **Agreed (2026-11-02):** a row's right-click menu in the World state
  window adds inside it: a group's, a template's, the World heading's, and
  an NPC's or zone's own, Add value by type and Add group; the add buttons
  say where they add. A value anything uses keeps its type: its other types
  are greyed, and a new value is declared for another type. (Rejected:
  changing a used value's type after asking, leaving its uses broken.)
- **Agreed (2026-11-02):** **Find a value**, by its button or Cmd+F, opens the
  workflow's value picker over the World state window, with the window's own
  scopes, every type and how many places use each value; a pick shows the
  value in the tree, and a name that matches nothing is declared from it in
  the world, a zone, an NPC or a template, then shown.
- **Agreed (2026-11-04):** **World state is a section** of the Project
  window holding **Values** and **Signals**, each opening its own window,
  so later kinds of world state join the section rather than crowding one
  window. Both windows are filed under World state on the window bar.
  (Rejected: one World state window whose top level is VALUES and
  SIGNALS.) [Increment 29](increments/0029-lines-state-and-picking.md)
  builds it.

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
- **Agreed (2026-11-02):** any workflow can be called from another by a
  **Call workflow** node in the Flow palette, which shows the module's name
  and inputs and has one output per way out, then done. Its inspector
  chooses the workflow, with Open, fills each input with a fixed value,
  Pick on map or another value, and keeps each output in a value chosen
  with the picker, or leaves it; a double-click opens the module's tab,
  whose strip names who calls it. A **Return** node at a module's top level
  names a way out; renaming it keeps its callers' lines. A local's role is
  Local, Input or Output, and **place** joins the types for locals, so Go
  to can go to a place value. A Call that no longer fits is flagged on the
  node, and Watch shows the path down through the calls.
  [Increment 21](increments/0021-flow-modules.md) builds it.
- **Agreed (2026-11-02):** a place is an anchor on a map, marked wherever
  it appears by the ring the map draws for one. Every place control is
  one box naming the place, its zone and floor, which opens a list of
  every place by name, with Pick on map; picking makes the map a mode of
  its own, saying what the place is for and what it is now, and brings the
  asking tab back with the place. [Increment
  21](increments/0021-flow-modules.md) builds it, after its author tried
  picking.
- **Agreed:** custom behaviours compose logic, randomness and built-in
  behaviours such as wandering.
- **Agreed:** editing flow matters more than arranging boxes: layout should mostly
  take care of itself, with manual arrangement available. Optional colours on
  nodes and containers, annotations and note nodes keep large workflows readable;
  layout is saved apart from logic (D17).
- **Agreed (2026-11-01):** right-click on a Workflow node or line selects it
  and opens a menu that deletes it, as Backspace does. The start node cannot
  be deleted.
- **Agreed (2026-10-31):** the Workflow canvas zooms with the Map's floating
  zoom control and keys, and Recentre in its toolbar brings a workflow panned
  out of sight back to the middle.
- **Agreed (2026-11-01):** every interior leg of a line can be dragged: a
  forward line's upright part, as today, and a back line's drop, level run
  and end legs. Each is one offset saved with the layout, measured from the
  line's own node, so the line keeps its shape when its nodes move; Reset
  line puts it back where the editor would run it. No free waypoints: lines
  stay level and upright. *Candidate:* an Add bend menu item that splits a
  leg in three, if a real workflow ever needs a line to go round something.
- **Agreed (2026-11-01):** the status bar names the line under the pointer:
  where it leaves, by which outcome, where it enters, whether it loops back
  and whether it was moved by hand. A Legend toggle in the toolbar says what
  each look means: solid goes forward, dashed loops back, grey is a time
  switch's otherwise, blue is lit, orange is live.
- **Agreed (2026-11-01):** colour is organisation, not meaning. The editor
  offers seven fixed tints, none blue and none red, so selection and
  problems stay unmistakable; the project declares no colours of its own. A
  node, a container or an outcome (one output of a node) can be given one. A
  line's colour is unset, from its outcome, or set by hand, and the
  inspector says which; selected and live always win over it. Colours are
  layout, saved with positions and bends, and mean nothing to the engine.
- **Agreed (2026-11-01):** a long line shows its outcome's name again near
  its arrowhead, small and muted; a short line does not, since the outcome's
  own row is in view.
- **Agreed (2026-11-01):** several nodes and containers can be selected,
  with Cmd+click or a box dragged from empty canvas, and moved, coloured,
  duplicated (Cmd+D, or copied and pasted into another workflow) or deleted
  together, each one undo step. Lines are never part of a group. A
  container brings its contents, and a dragged group stops at a container's
  wall, as one node does. Arrow keys nudge the selection, and Arrange can
  lay out the selection alone. (Changed on 2026-11-06 from Shift-click:
  Shift suggests a range, a path from one node to another, which a graph
  does not have; and Ctrl+click is a Mac's right-click.)
  [Increment 34](increments/0034-multi-select.md) builds it. A copy keeps
  the lines between its nodes and their colours, a paste declares the
  locals it needs, and the inspector shows how many are selected, with
  Colour and Delete (2026-11-06). The right-click menu offers Duplicate,
  Copy and Delete, and Auto-arrange lays out the selection alone while
  several are selected.
- **Replaced (2026-11-04):** dropping a node onto a line, from the palette
  or from the canvas, put it in the flow: the line ended at the new node,
  and the node's first output continued to the old target. In use it kept
  catching lines whose ends were far away.
- **Agreed (2026-11-04):** the author draws the lines. A node dropped from
  the palette lands where it falls, connected to nothing, even on a line;
  Add after connects the new node to a free output of the selected one, or,
  with none free, places it beside, connected to nothing. A back line runs
  just under its own two nodes and dips below a node only where it would
  run through it, and only for nodes in its own container. [Increment
  29](increments/0029-lines-state-and-picking.md) builds it.
- **Agreed (2026-11-05):** a node over a line shows it is not joined to it.
  A line that crosses a node is drawn over it at 40% strength, through its
  dots, and a node nothing links to rings its loose dots red and says "Not
  linked: never runs" under itself. (Rejected: solid lines over nodes,
  which strike through them; the line showing through as dashes, which
  mean a loop; a gap in the line; a shadow under the node.) [Increment
  30](increments/0030-project-facets.md) builds it.
- **Agreed (2026-11-03):** the palette says what a node does before it is
  placed: a search matches what each node does as well as its name,
  hovering an entry gives its one line, and clicking it shows its help page
  in the inspector, with Add after. Only dragging places a node; a click no
  longer adds one after the selected node. [Increment
  25](increments/0025-palette.md) builds it.
- **Agreed (2026-11-01):** Check value compares by its value's type, and
  shows only the comparisons that type allows. An integer compares with
  `=`, `≠`, `<`, `≤`, `>` and `≥`, as symbols, or is between two numbers,
  both included, or at its maximum or minimum; its right side is a typed
  number or another integer value the node may read, and a line under the
  row reads the chosen symbol back in words. A bool has no comparison to
  choose, only true or false. A choice is any of, or none of, a ticked list
  of its choices. Text is, or is not, typed text. A bool is true or false
  in every word of the editor, never yes or no; a Check value's outcomes
  stay yes and otherwise. *Candidate:* a Switch on a value, one outcome per
  choice, when a workflow wants a fan-out; Check value stays a yes or no
  question.
- **Agreed (2026-11-01):** a value is chosen in a **picker**, a dialog over
  the window, not a list inside the inspector, which gets too cramped as
  values multiply. The box in the inspector opens it, by click or Enter,
  with its search focused so typing starts at once; the search matches any
  part of a path. Type chips narrow the list and are locked to the type an
  operand must match; a scope rail, Everywhere, Local, This NPC, This zone,
  World, Other NPCs and Zones, shows how many match in each and narrows to
  one. Results sit under headings with path, type and description; values
  of another type are counted and hidden, shown on request. With nothing
  typed, groups fold as the World state tree does, arrows open them, and
  the last group opened is remembered. A name that matches nothing can be
  declared from the picker, with where, type and starting value, and is
  chosen at once. Arrows move, Enter picks, Esc closes. The inspector keeps
  only the box, showing the path and one line of type, limits and
  description. Every chooser uses it: Set value's and Check value's Value,
  Find NPC nearby's Only one whose and Keep it in, and an integer's Another
  value. (Rejected: a popover anchored to the box; shorter for a short
  list, cramped again for a long one.)
- **Agreed (2026-11-04):** a signal is chosen in a **signal picker**, a
  dialog over the window like the value picker, as a project soon has too
  many signals for a button each. Its search matches any part of a path.
  With nothing typed, the signals this workflow sends or answers come
  first, then every signal under folding groups, each saying how often it
  is sent and answered. Send signal's and On signal's inspectors keep only
  the box. (Rejected: columns of groups and signals, and type-ahead in the
  box.) [Increment 29](increments/0029-lines-state-and-picking.md) builds
  it.
- **Agreed (2026-11-06):** reusable text, its author's "static strings",
  becomes **constants**: a list of its own beside values, of strings, from a
  sign to long dialogue, and whole numbers, which the designer sets and a
  game never changes. Constants are scoped as values are, to the world, a
  zone or an NPC. Messages, Check value and Set value can use them, and they
  are deleted under the same rules as values. Their text takes markup for
  colour, bold and italic, and nothing that changes the flow, such as a
  pause. Long text is not paged for now. They are built once the Project
  window's new design is settled.
- **Agreed (2026-11-06):** a constant belongs wherever a designer might
  change a typed-in value later, rather than hunt for where it was typed.
  Paths say which they are, as `const.` and `var.`, wherever the two mix,
  and not inside their own lists. An NPC's or a zone's constant can hold a
  value of its own for each one, edited on its inspector and in the
  Constants window, which is World state's third part. Markup comes with
  them. [Increment 33](increments/0033-constants.md) builds them.
- **Agreed (2026-11-06):** constants follow templates, as values do. A
  template holds constants beside its values, and every NPC or zone it is
  on holds them, with a value of its own kept in its own file; an NPC or a
  zone can also have constants of its own. Long text is written in a sheet
  over the window, beside a preview of how the game draws it. Making a
  constant from a typed-in value lists the other places typed the same, to
  tick and switch, with one Undo for all of it.
- **Removed (2026-11-06):** Text documents, a placeholder kept to prove the
  text field and picking a place, are gone, as constants will hold a game's
  text ([increment 31](increments/0031-no-text-documents.md)).
- **Candidate:** cut scenes as workflows that block player input.

## NPCs

- **Agreed:** selecting an NPC on the map gives immediate access to its details
  and workflows. NPC flags grow as a game needs them.
- **Agreed:** each NPC can carry private author notes, never shown to players.
  **Candidate:** description fields usable in an in-game guide.
- **Agreed (2026-11-02):** an NPC lives in one zone and only one, set in its
  details as its behaviour is, and is placed only on that zone's maps; it
  can change floors within the zone by the stairs. A character that seems
  to live in two zones is two NPCs. Changing an NPC's zone asks first,
  listing the places it could no longer reach, then flags them. Zones are
  chosen with a zone picker over the window, with search, folders, tags
  and a look at each zone, serving whatever asks for a zone. This replaces
  an NPC keeping one identity across zones with placements in several.
  [Increment 22](increments/0022-npc-zones.md) builds it.
- **Agreed (2026-11-03):** a workflow runs in one zone or in any zone, set
  on the workflow with the zone picker, and its places, the NPCs that
  follow it and the zones that loop it are checked against that. Where a
  workflow, a behaviour or a place is chosen, what fits comes first and
  the rest is greyed with the reason; changing where a workflow runs asks
  first, listing what breaks, then flags it. [Increment
  23](increments/0023-workflow-kinds.md) builds it.
- **Agreed:** NPCs travel to destinations by pathfinding and perform behaviour
  there, looping within scheduled times.
- **Candidate:** positions entered by picking on the map rather than typing
  coordinates, using anchors and areas.

## Sound

- **Agreed:** audio editing moves into the editor. The berad-sdk terminal audio
  editor stays for Ultima V Redux. Both read and write berad-audio's formats
  only through its authoring package, and those formats change only through a
  new version, so Ultima V Redux stays compatible. Choosing cues for workflows
  comes first ([increment 4](increments/0004-sounds-in-workflows.md)); Audio follows
  ([increment 5](increments/0005-audio-window.md)), then trimming and levels ([increment 6](increments/0006-trimming-and-levels.md)), then ambient distance, profiles and families, and
  the soundscape ([increment 7](increments/0007-ambient-and-soundscape.md)).
- **Agreed:** Audio does everything the terminal audio editor does
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
  Audio's panel, as the terminal editor shows it in its details,
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
- **Agreed (2026-11-03):** the zone loop is a kind of workflow, set in a
  zone's details, beside NPC behaviour; it sets zone and world values and
  plays sounds across the zone. [Increment
  23](increments/0023-workflow-kinds.md) builds it; music and ambient
  nodes, handlers and a loop by turn wait.
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
- **Agreed (2026-10-30):** **Launch** plays the project as a game would, in
  a **Play tab** with nothing of the editor around it, beside the Map's
  Preview, which stays for quick checks. It starts at the **player start**,
  placed once per project, and plays the project as it was when launched;
  relaunching picks up edits. The screen shows a fixed number of cells set
  per game, filling the tab while keeping its shape (2026-10-31), with the
  player always centred. NPC cues, ambient sounds placed on the map and a
  zone's own sounds are heard. The player and each NPC show one sprite. See
  [increment 14](increments/0014-launch.md).
- **Agreed:** a global clock for testing routines; on a jump, NPCs adapt to the
  new time immediately rather than simulating the gap: each restarts its
  workflow from the start and walks to wherever its new branch sends it.
- **Agreed:** the clock is pinned to the window bar: its readout shows the
  preview's time on every tab, and its controls open in a panel under it.
  Before a preview it sets the time the next one starts at; during one it
  runs the preview's clock: an hour back or on, a jump to a time, stopping
  and starting it, and its rate. Its panel opens by itself when a preview
  starts.
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
- **Agreed (2026-11-03):** a workflow's **Log** node writes a line of text
  and values, each value in its text form, to a log in the editor's Watch
  window, for its author. [Increment 24](increments/0024-built-ins-and-log.md)
  builds it.

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
  A step is taken the moment its key goes down and only drawn sliding, as
  Ultima V Redux does; a held key steps every 10 ticks, the slide taking 8
  ([increment 14](increments/0014-launch.md)). *Open:* whether a game chooses
  these timings or the engine fixes them.
- **Agreed:** uniform pixel scaling is guaranteed (no stretching), and layout
  supports several popular aspect ratios rather than one fixed layout. Text uses
  TTF fonts.
- *Candidate (2026-11-06):* a project chooses its own fonts, as TTF files,
  for its game's text. Until then a game's text uses the editor's faces,
  which are free to use in a commercial game: IBM Plex Sans and Mono, under
  the SIL Open Font License. [Increment 33](increments/0033-constants.md)
  adds Plex Sans's italics for markup.
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

**Views (current wireframes):** project browser, Map, Workflow (including
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
