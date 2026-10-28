# Increment 11: Drawing tiles

**Status:** Planned

The eleventh increment is the first slice of the pixel editor. An author
sets the game's tile size, builds the shared palette from ramps named for
their use, draws a tile in a new Pixel editor view, and sees it on the map:
a one-frame sprite shown in place of a stand-in's flat colour, drawn live as
it is painted. A colour can be see-through, like glass. It builds on the
pixel editor's design ([editor features](../EDITOR_FEATURES.md#pixel-editor))
and on increment 10's tabs, and leaves animation, bigger sprites, further
palettes and the other drawing tools for later increments. It is held to the
same strict checks as [increment 1](0001-foundation.md).

## What it proves

1. **A tile is drawn at the game's tile size:** a project's tile size is a
   setting, and a new sprite's tile is that size. The Pixel editor shows it
   magnified by whole numbers, with a pixel grid, and paints it with a
   pencil, an eraser, a fill and an eyedropper.
2. **Colours are named, not numbered:** a pixel refers to a shade of a ramp
   in the shared palette. Changing a shade's colour recolours every tile that
   uses it, including on an open map; renaming a ramp changes nothing drawn.
3. **Glass is see-through:** a shade with an opacity shows the checkerboard
   through it in the Pixel editor and the ground through it on the map; the
   clear colour erases.
4. **A sprite reaches the map:** a stand-in other than a wall can be shown by
   a one-frame sprite. The Map draws the sprite instead of the flat colour,
   uniformly scaled, and a preview walks over it.
5. **Drawing is live and undoable:** a stroke is one undo step in every view
   of the tile, and another view of the tile, or a Map showing it, updates as
   the stroke is drawn.
6. **Art files stay small and readable:** each tile, sprite and palette is a
   versioned document of its own, and a tile's pixels are saved row by row, so
   changing one pixel changes one line of its file.

## Decisions

Asked for by its author on 2026-10-26, paraphrased: draft the first slice of
the pixel editor as a Planned increment, to review before anything is built.

Agreed by its author on 2026-10-25, recorded in [the editor
features](../EDITOR_FEATURES.md#pixel-editor), that this increment relies on:

- **A tile is one cell-sized piece of art; a sprite is one or more tiles,**
  what a definition shows; the grid position is a cell.
- **Art is drawn only in the built-in pixel editor;** nothing is imported.
- **A pixel names its colour by scope** (`shared / grass 2`), never by
  number or value, and colours come in **ramps** named for their use,
  numbered dark to light.
- **A colour can carry its own opacity,** and one colour is fully clear.
- **No sheets:** art is found with folders and tags like other documents, and
  how it is packed on disk is the editor's business.
- **No colour budget.**
- **Tile size is a setting each game chooses;** tiles of another size are
  flagged.

Proposed while planning and agreed by its author on 2026-10-26, before
building:

- **The first slice is one tile, one frame, one palette.** A sprite has one
  frame of one tile, one cell in size; the shared palette is the only
  palette; the tools are the pencil, the eraser, the fill and the eyedropper.
  Animation, bigger sprites, further palettes and the other tools follow in
  their own increments (below).
- **Until the definition editor exists, a stand-in can be given a sprite.**
  The stand-ins are still built in. A small project document records which
  sprite shows each stand-in, chosen in the Pixel editor under "Shows"; the
  definition editor later takes this over. Walls are left out: a wall family
  needs a sprite for each neighbour arrangement.
- **A tile's pixels are saved as rows of text, not as PNG.** A tile document
  holds its size, a table of the colours it uses (each a palette, ramp and
  shade ID) and one line of two hexadecimal digits per pixel for each row.
  PNG cannot hold a colour's name, and rows diff line by line. This departs
  from D11's "art as PNG", which changes with this increment. (Rejected: an
  indexed PNG beside a table of colours, which git shows only as a changed
  binary file.)
- **Names sit on permanent IDs.** Each ramp and shade has an ID that pixels
  refer to, so renaming a ramp or reordering its shades never breaks a tile.
  (This was a candidate on 2026-10-25.)
- **Art travels as ordinary documents.** Palettes, tiles, sprites, the
  settings and the stand-in sprites are documents a window subscribes to, and
  a stroke's pixels are an ordinary command, grouped into one undo step as a
  map stroke is. Other views apply the same edit to their copy (D8's patches).
  No new protocol messages are expected.
- **Where things live.** The art's data and drawing a tile as an image are in
  a new engine package, `engine/art`, so a game draws sprites the same way;
  its edit kinds are a new `art` plug-in in `editor/plugins/art`; the view is
  `editor/views/pixelview`. The engine's `tiles` package keeps the stand-ins.
- **The project browser gets Sprites and Palettes sections,** with folders,
  tags and search like the others, and a Settings row for the tile size. A
  new sprite makes its tile with it. Their icons get a muted teal.
- **A project without a shared palette or settings gets them when it is
  opened,** before any window connects: the shared palette with the clear
  colour and one ramp, "grey", of five shades; the settings with a tile size
  of 32. A new project starts the same way.
- **Removing a ramp or a shade that a tile uses is refused,** naming the tiles
  that use it, as deleting a document checks where it is used.
- **Typed fields wait for a save.** A shade's colour (as hexadecimal), its
  opacity (a percentage) and a ramp's name are typed in the palette panel and
  saved with its save bar, as every inspector does. Adding and removing ramps
  and shades apply at once and can be undone.
- **The map's zoom follows the tile size** once sprites show: its steps are a
  quarter, a half, and one to four times the tile size, so art is always
  scaled by the same whole amount, or uniformly reduced. A stand-in without a
  sprite is still a flat colour.
- **A tile of another size,** after the tile size changes, says so in its
  Pixel editor, and the Map draws its stand-in's flat colour instead.
- **The Pixel editor is a window kind of its own,** Sprites, after Audio on the
  window bar, and opens as a tab or, without the shell, as a window process.

Asked of its author on 2026-10-27, before step 5, and answered:

- **The Map opens at a sprite's actual size:** 100% is one screen pixel for
  each of a sprite's pixels, so a 32-pixel tile fills a 32-pixel cell, where
  the Map's cells were 24 pixels before. Its zoom steps are a quarter, a
  half, and one to four times the tile size.
- **A sprite shows one stand-in,** chosen from a dropdown under Shows, as the
  wireframe has it, and each stand-in has at most one sprite.
- **Step 6's sprite is a lantern drawn for it,** shown as the small pickup,
  in grey with an amber glass ramp at 40%.

Chosen while building, for its author to check:

- **Clear is the empty colour,** not a ramp: a pixel that names no shade is
  clear, saved as 00, and every palette offers it.
- **A shade's opacity runs from 1% to 100%;** 0% is refused, since that is
  what clear is for.
- **Ramp names are unique within a palette,** so `shared / grey 3` always
  means one colour, and colours are saved as `#RRGGBB`.
- **Tiles are square, from 8 to 128 pixels,** and one tile uses at most 255
  colours besides clear, since a saved pixel is two hexadecimal digits. A
  palette holds up to 256 ramps of up to 64 shades.
- **A pixel whose colour cannot be found is drawn clear,** and the count of
  such pixels is reported, for the Pixel editor to say so. Removing a shade a
  tile uses is refused, so this happens only with a missing palette.
- **The starter ramp, "grey", runs from #202020 to #DCDCDC** in five even
  steps.
- **Format names:** `palette`, `tile`, `sprite`, `art-setting` and
  `standin-sprite`, since a document's folder is its format's name with an s
  (`art-settings/`, `standin-sprites/`).

- **Documents every project holds** are a new part of a plug-in. The art
  plug-in names three, with fixed IDs whose time part is zero: the shared
  palette ("Shared"), the art settings ("Art settings") and the stand-in
  sprites ("Stand-in sprites"). The orchestrator adds any a project lacks
  when it opens, before replaying the journal, and refuses to delete or
  rename them. The starter ramp and its shades have fixed IDs too, so the
  shared palette is the same every time it is made, and a project reopened
  after a crash replays its strokes onto the same colours.
- **An edit that removes something names it** (a new seam, `model.Remover`),
  and the orchestrator refuses it while another document refers to it,
  naming those documents: a tile refers to each ramp and shade it uses, and
  is named by its sprite. Undo and redo check before changing anything, so a
  refused undo stays on the list and works once nothing uses the colour.
- **A sprite's tile is its part,** as a zone's floors are, so its name is
  unique only within the sprite.
- **Renaming a ramp and changing its shades touch different items,** so
  they never clash in undo across windows.
- **A project holding only the documents every project holds counts as
  empty,** so `--sample` still makes the sample there.

- **The Settings row is the list's first row,** above Zones, showing the tile
  size. Selecting it shows the tile size in the inspector, typed and saved
  with the save bar; a size outside 8 to 128 is refused there, and stays to
  correct.
- **Sprites and Palettes come after Audio.** Palettes have no [+], since the
  shared palette is the only palette, but can be put in folders and tagged.
- **A new sprite's tile is made first,** named "Tile 1" at the project's tile
  size, then the sprite naming it, as one undo step. If the sprite is
  refused, the tile is deleted again, so no tile is left without a sprite.
  Deleting a sprite deletes its tile with it, in one step.
- **The shared palette shows its name without a field,** and its Rename and
  Delete are disabled, since the orchestrator refuses them.
- **A sprite opens in the Pixel editor,** from Open, a double-click or its
  right-click menu. A palette opens nothing, since its panel is part of the
  Pixel editor.
- **Icons:** a sprite is a heart drawn in big pixels and a palette a ramp of
  three swatches, dark to light, both in a muted teal (#267378); the Settings
  row has two sliders, in ink.

- **The Pixel editor's view is `sprite`,** its target a sprite, and views of
  one sprite share an undo list, as views of a map do. The window bar names
  the kind Sprites, with the sprite's icon, and a tab shows the sprite's
  name.
- **Tool keys are Aseprite's:** B for the pencil, E the eraser, G the fill and
  I the eyedropper. Cmd+= and Cmd+− step the zoom and Cmd+0 fits it; the
  wheel pans and Cmd with the wheel zooms.
- **Zoom steps** are 1, 2, 3, 4, 6, 8, 10, 12, 16, 20, 24, 32, 40, 48 and 64
  times. Fit chooses the largest whole number that fits, so a 32-pixel tile
  opens at 19 times in a window of the usual size. The pixel grid shows from
  4 times.
- **The checkerboard follows the tile's pixels:** from 8 times each pixel
  holds two by two checks, so the grid never cuts a check.
- **The pencil paints only pixels whose colour changes,** and fills in the
  pixels between the positions a fast drag passes.
- **Fill joins pixels side by side, never at a corner,** stays within the
  tile, and is one undo step.
- **Painting needs a shade.** Choosing a ramp by its name chooses its first
  shade, or the ramp alone when it has none, which paints nothing and says
  so.
- **The panel shows the shared palette only.** A colour of another palette,
  picked with the eyedropper, paints but shows no fields; no other palette
  exists yet.
- **+ Ramp adds "new ramp"** (then "new ramp 2"…) with one shade of mid grey,
  #808080, and chooses it. + Shade copies the chosen shade into the place
  after it, or adds mid grey at the end of a ramp chosen alone. Removing a
  shade chooses its neighbour; removing a ramp chooses clear.
- **A shade's colour and opacity save together,** as one edit, and the
  opacity can be typed with or without "%". With a typed change unsaved,
  choosing another colour asks Save, Don't save or Cancel first.
- **The readout is below the canvas, not in the status bar,** which shows
  messages: the pixel under the cursor and its colour, or the tool's help,
  with the tile's size at the right. A second line, in red, flags a tile of
  another size than the project's, or pixels whose colour cannot be found.

- **Shows sits at the foot of the palette panel,** above the save bar. Its
  menu offers Nothing and every stand-in but the walls, naming the sprite
  that already shows one. Choosing a stand-in takes it over, and the
  stand-in the sprite showed before goes back to its flat colour, in one
  undo step.
- **A sprite fills its cell on the Map,** whether it shows ground or what
  sits on it: a pickup is no longer drawn smaller than its cell, a door has
  no wall arms and stairs no chevron. A ground sprite's see-through pixels
  show white; anything else's show the cell's ground.
- **The quarter and half zooms are smoothed;** the others are drawn without
  smoothing. A tile size that does not divide by four rounds the quarter
  down, to at least 2 pixels.
- **The zoom shows a percentage of the tile size,** so 100% is actual size,
  and a new tile size keeps the zoom level.
- **The floor below shows its sprites faintly,** as it shows stand-ins.
- **Deleting a sprite leaves its stand-in naming it,** as deleting any
  document leaves references to it; the Map draws the stand-in's flat colour
  until undo brings the sprite back.
- **The Pixel editor and the Map draw a tile the same way,** with a new
  package, `editor/views/tileart`, that keeps a tile's picture until the
  tile or one of its palettes changes.
- **Real-window tests that paint the far side of the sample floor fit it
  first,** since at 100% its cells are bigger and some are out of view.

## Wireframes

The Pixel editor in the first slice, as a tab. Letters in brackets stand for
each kind's icon.

```text
┌─────────────────────────────────────────────────────────────────────┐
│ [P] Project │ [M] Sample zone │ [S] Lantern ⧉ × │         ◷ 08:00 ▾ │
├─────────────────────────────────────────────────────────────────────┤
│ Pencil  Eraser  Fill  Eyedropper              Zoom 800%  [Grid]     │
├───────────────────────────────────────────────┬─────────────────────┤
│                                               │ PALETTE · Shared    │
│        ┌───────────────────────────┐          │ clear  ▢            │
│        │ checkerboard under clear  │          │ grey   ■ ■ ■ ■ ■    │
│        │ pixels, a grid of the     │          │ glass  ▣ 40%        │
│        │ tile's pixels, magnified  │          │ [+ Ramp]            │
│        │ by a whole number         │          │                     │
│        └───────────────────────────┘          │ SHADE · grey 3      │
│                                               │ Colour  #8A919C     │
│                                               │ Opacity 100%        │
│                                               │ [+ Shade] [Remove]  │
│                                               │ SHOWS               │
│                                               │ Small pickup ▾      │
│                                               ├─────────────────────┤
│                                               │2 unsaved Revert Save│
├───────────────────────────────────────────────┴─────────────────────┤
│ Pixel 12, 7 · 32 × 32 · grey 3                                      │
└─────────────────────────────────────────────────────────────────────┘
```

The project browser's new sections:

```text
SETTINGS          Tile size 32
SPRITES     3  +
  Lantern
  Doors ▸
PALETTES    1
  Shared
```

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. Art model | `engine/art`: palette, tile, sprite, settings and stand-in sprite formats at version 1; a tile drawn as an image from its palette, opacity included | Unit tests of the formats, of colours resolved by ID after a rename, and of an image drawn with a see-through shade over a known ground | Built |
| 2. Art plug-in | `editor/plugins/art`: the document types and edit kinds (add, rename and remove a ramp; add, set and remove a shade; paint pixels; set the tile size; set a stand-in's sprite); the shared palette and settings added on open; removal refused while in use | Every edit kind passes the shared apply-then-undo test; headless orchestrator tests of opening an older project and of a refused removal | Built |
| 3. Project browser | Sprites and Palettes sections, the Settings row and its tile size field; a new sprite with its tile; icons | View tests of the sections, and a real-window test that makes a sprite in a folder | Built |
| 4. Pixel editor | `editor/views/pixelview`: the canvas, whole-number zoom, pixel grid and checkerboard; pencil, eraser, fill and eyedropper; the palette panel and its save bar; the Sprites window kind | View tests of each tool and of fill's bounds; a real-window test that paints a stroke and undoes it | Built |
| 5. On the map | "Shows" in the Pixel editor; the Map drawing a stand-in's sprite, zoom steps from the tile size, and tiles of another size flagged | A real-window test: a stroke in the Pixel editor shows in a Map's screenshot as it is drawn, a shade's new colour recolours it, and a glass pixel shows the ground through it | Built |
| 6. Trying it | The demo project's sample zone with one sprite shown, for its author to try | Judged by its author | Planned |

Where each step changes code:

- **Art model:** a new `engine/art` package.
- **Art plug-in:** a new `editor/plugins/art`, registered in
  `editor/cmd/berad-editor`'s plug-in list, and the orchestrator's project
  opening, to add the shared palette and settings.
- **Project browser:** `editor/views/browser` (the tree's sections) and
  `editor/ui` (icons).
- **Pixel editor:** a new `editor/views/pixelview`, the window kinds in
  `editor/window/kinds.go` and `editor/window/window.go`, and the project
  browser's Open.
- **On the map:** `editor/views/mapview` (drawing a cell, and the zoom steps
  in `camera.go`), Shows in `editor/views/pixelview`, and a new
  `editor/views/tileart` that both draw tiles with.

## Step results

- **Art model:** a new `engine/art` package holds palettes (ramps of shades,
  each with a colour and an opacity), tiles (square, each pixel naming a
  palette, ramp and shade by ID, saved as a table of colours and rows of
  hexadecimal digits), sprites (one tile for now), the art settings (the tile
  size, 32 to start) and the stand-in sprites (which sprite shows each
  stand-in, walls refused). Each has a format at version 1. A tile draws as
  an image from the palettes it names, with a see-through shade's opacity as
  alpha. Verified with unit tests: each format round-trips and refuses bad
  data; changing one pixel changes one saved row, and equal tiles save the
  same way; renaming a ramp draws the same, and changing a shade recolours
  the tile; a 40% blue over red composites to the expected colour; a tile
  holds 255 colours and refuses one more. `make check` passes: no lint
  findings, race tests passing.
- **Art plug-in:** `editor/plugins/art` holds the five document types and
  nine edit kinds: add, rename and remove a ramp; add, set and remove a
  shade; paint pixels; set the tile size; and choose a stand-in's sprite.
  Adding a ramp or shade gives it permanent IDs before it is applied. It is
  registered in the editor's plug-in list, and the orchestrator adds the
  shared palette, art settings and stand-in sprites to a project that lacks
  them (`editor/orchestrator/required.go`). Verified with unit tests: every
  edit kind passes the shared apply-then-undo test, including a pixel
  painted twice in one command; preparing gives new IDs once and leaves
  complete arguments alone; a tile refers to its ramps and shades; refused
  edits leave their document unchanged; and the registry refuses a required
  document of an unknown format or listed twice. Headless orchestrator
  tests: a project made without art gains the three documents, once, saved
  in `palettes/`, `art-settings/` and `standin-sprites/`, and reopening
  adds nothing more; removing grey 3 or the grey ramp while the sprite
  Lantern uses it is refused with "used by Lantern", and an unused shade
  goes; deleting the shared palette or renaming the settings is refused; an
  undo that would remove a ramp another window has painted with is refused,
  and the same undo works once the pixel is erased. `make check` shows no
  lint findings in either module and every package passing except the
  orchestrator's audio tests, which time out under load: unchanged `main`
  fails the same tests in two runs of three on the same machine, and both
  pass with `go test -parallel 2`.
- **Project browser:** the browser lists a Settings row first, showing the
  tile size, and Sprites and Palettes sections after Audio, with folders,
  tags and search like the others. The Settings row's inspector types the
  tile size and saves it with the save bar. A new sprite, from the Sprites
  [+], New ▾ or a right-click menu, comes with one tile at the tile size, as
  one undo step, and deleting a sprite deletes its tile. Sprites and palettes
  have new icons in a muted teal, and the Settings row its own. Verified
  with view tests: the Settings row comes first with the tile size; Sprites
  has a [+] and Palettes none; a sprite's tile, the settings and the
  stand-in sprites are not listed as documents; a tag search finds a sprite,
  and searching for "tile" finds the Settings row; a new sprite makes its
  tile at the tile size, then the sprite naming it in the selected folder, in
  one gesture, and a refused sprite deletes its tile in the same gesture; the
  tile size is saved with Save, and 7, 129 and "big" are refused and kept to
  correct; deleting a sprite asks where it and its tile are used, then
  deletes both in one step; the shared palette has no name field, and Delete,
  Cmd+Backspace and Rename leave it alone. A real-window test sets the tile
  size to 24, makes a sprite folder from the Sprites heading's right-click
  menu, then a sprite inside it from the folder's menu: the saved sprite is in
  `sprites/Lights/` and names its tile, which is 24 pixels square; all 30
  real-window tests pass. `make check` shows no lint findings, and every
  package passes except the orchestrator's audio test that times out under
  load, which passes with `go test -parallel 2`.
- **Pixel editor:** a new `editor/views/pixelview` shows a sprite's tile
  magnified by a whole number over a checkerboard, with a pixel grid, and
  paints it with a pencil, an eraser, a fill and an eyedropper. Its palette
  panel shows the shared palette's ramps, chooses the colour to paint with,
  adds and removes ramps and shades, and types a ramp's name and a shade's
  colour and opacity, saved with its save bar. Below the canvas it names the
  pixel under the cursor and its colour, and flags a tile of another size or
  a colour that cannot be found. It is a window kind of its own, Sprites,
  after Audio on the window bar, and the project browser opens a sprite in
  it. Verified with view tests: opening subscribes to the tile, the shared
  palette and the settings, and fits the tile; a pencil drag paints the
  pixels between frames in one gesture, skipping pixels already that colour,
  and the next stroke has its own gesture; the eraser, and the pencil with
  clear, clear pixels; a fill paints the 24 pixels left of a wall as one
  step, never passes through a corner, and sends nothing over its own
  colour; the eyedropper picks a colour, or clear, and changes nothing; the
  zoom steps through whole numbers, Fit restores it, and pixels are found at
  any zoom; + Ramp and + Shade send a unique name and a copy and choose what
  they add; a typed colour and opacity wait for Save, refuse "#12" and 0%,
  and save together; Enter renames a ramp; a refused removal says why;
  removing a ramp's last shade leaves the ramp chosen, and removing the ramp
  chooses clear; a typed change asks before another colour is chosen; the
  tile's colours are premultiplied for drawing, so a see-through shade shows
  its true colour. A browser view test opens a sprite from Open and a
  double-click, and a palette not at all. Real-window tests: in a window of
  its own, the Pixel editor paints a stroke in grey 3 and another in grey 5,
  and Cmd+Z undoes the second in one step; then it adds a ramp, names it
  glass, types #3060C0 at 40%, saves, and paints with it. The saved tile
  holds the first stroke and the glass, and the shared palette the glass
  ramp. In the shell, the sprite opens as a tab under Sprites and paints
  there, with no other window process; all 32 real-window tests pass.
  `make check` shows no lint findings, with every package passing.
- **On the map:** the Pixel editor's Shows chooses the stand-in a sprite
  replaces on every map, and the Map draws that sprite's tile to fill each
  cell of the stand-in, live as it is painted, at zoom steps from the tile
  size: a quarter, a half, and one to four times, with 100% the actual
  size. A tile of another size keeps the stand-in's flat colour. Verified
  with view tests: Shows sends the sprite for the small pickup, then takes
  the medium pickup over from Chest in one step that puts the small pickup
  back, then nothing, and offers no wall; the Map subscribes to the art
  settings, the stand-in sprites and each sprite, tile and palette, draws
  the door with its sprite and leaves grass, whose tile is another size,
  flat; a painted pixel and a shade's new colour show at once, and taking
  the sprite away brings the flat colour back; the zoom steps for a
  24-pixel tile are 6, 12, 24, 48, 72 and 96. A real-window test makes a
  sprite show the small pickup and draws it while the sample's Map shows
  the pickup at 200%: in the Map's screenshots a magenta stroke shows as it
  is drawn, giving its shade a new colour turns it cyan with no magenta
  left, and a fill of 40% blue glass shows the pickup's floor through it,
  matched to the composited colour; the saved stand-in sprites name the
  sprite. All 33 real-window tests pass.

## Changed after its author tried it

None yet.

## Not in this increment

Each of these is agreed in
[the editor features](../EDITOR_FEATURES.md#pixel-editor) and waits for an
increment of its own:

- **Animation:** frames with durations, sequences and the weighted choice of
  what plays next, and placements in step or offset by cell.
- **Bigger sprites:** a grid of tiles drawn on one canvas, base rows and the
  draw order, and reusing tiles by copying or pasting linked.
- **Alternatives** that vary a sprite from cell to cell.
- **Further palettes,** such as "gloomy", and a zone referring to one.
- **The other drawing tools:** line, rectangle, circle and ellipse, with
  filled and outline modes; select; the dither pencil; drawing layers; and
  templates that keep part of a tile clear.
- **Definitions that need sprites:** the definition editor, door states,
  overlays, a character's facings, and art for wall families.
- **Stairs** that are one-way or two-way, with the other end placed for you.
- A Tiles section of their own in the project browser: in this increment a
  sprite's tile is reached through its sprite.
