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
- **Format names:** `palette`, `tile`, `sprite`, `art-settings` and
  `standin-sprites`.

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
| 2. Art plug-in | `editor/plugins/art`: the document types and edit kinds (add, rename and remove a ramp; add, set and remove a shade; paint pixels; set the tile size; set a stand-in's sprite); the shared palette and settings added on open; removal refused while in use | Every edit kind passes the shared apply-then-undo test; headless orchestrator tests of opening an older project and of a refused removal | Planned |
| 3. Project browser | Sprites and Palettes sections, the Settings row and its tile size field; a new sprite with its tile; icons | View tests of the sections, and a real-window test that makes a sprite in a folder | Planned |
| 4. Pixel editor | `editor/views/pixelview`: the canvas, whole-number zoom, pixel grid and checkerboard; pencil, eraser, fill and eyedropper; the palette panel and its save bar; the Sprites window kind | View tests of each tool and of fill's bounds; a real-window test that paints a stroke and undoes it | Planned |
| 5. On the map | "Shows" in the Pixel editor; the Map drawing a stand-in's sprite, zoom steps from the tile size, and tiles of another size flagged | A real-window test: a stroke in the Pixel editor shows in a Map's screenshot as it is drawn, a shade's new colour recolours it, and a glass pixel shows the ground through it | Planned |
| 6. Trying it | The demo project's sample zone with one sprite shown, for its author to try | Judged by its author | Planned |

Where each step changes code:

- **Art model:** a new `engine/art` package.
- **Art plug-in:** a new `editor/plugins/art`, registered in
  `editor/cmd/berad-editor`'s plug-in list, and the orchestrator's project
  opening, to add the shared palette and settings.
- **Project browser:** `editor/views/browser` (the tree's sections) and
  `editor/ui` (icons).
- **Pixel editor:** a new `editor/views/pixelview`, and the window kinds in
  `editor/window/kinds.go`.
- **On the map:** `editor/views/mapview` (drawing a cell, and the zoom steps
  in `camera.go`).

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
