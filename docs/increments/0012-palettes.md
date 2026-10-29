# Increment 12: Palettes

**Status:** Planned

The twelfth increment gives palettes a window of their own. The engine ships
one built-in palette, Default, of 256 colours in named ramps, the same in
every game and never changed; a game adds palettes of its own beside it, such
as a "gloomy" one, made empty or from colours chosen out of Default. A new
window kind, the Palette Editor, does all palette editing: ramps added,
renamed, reordered and removed, colours moved within and between ramps, a
colour's value, opacity and short label, a gradient that fills a ramp, and
colours copied from one palette to another. The Pixel editor's palette panel
becomes pick-only. A zone lists the palettes it uses, so the picker offers
them first. It builds on [increment 11](0011-drawing-tiles.md)'s palettes,
tiles and Pixel editor, and is held to the same strict checks as
[increment 1](0001-foundation.md).

## What it proves

1. **Default is built in and static:** every project holds Default, with the
   same 256 colours and IDs, and every change to it, its name included, is
   refused; its colours can only be copied.
2. **A game makes palettes of its own:** a new palette starts empty or from
   ramps and single colours of Default, is renamed and deleted from the
   project browser or the Palette Editor, and a tile painted with it shows
   its colours.
3. **All palette editing is in the Palette Editor:** ramps are added,
   renamed, reordered and removed; colours are added, moved, recoloured,
   given an opacity and labelled; a gradient fills a new or existing ramp;
   colours are copied between palettes. Nothing is duplicated in the Pixel
   editor.
4. **Moving a colour never breaks a tile:** a tile painted with a moved colour
   still shows it, through a shadow record of the colour's old place, and
   says so until the pixels are repainted.
5. **Deleting a palette is safe:** it is refused while a zone or a tile refers
   to it, and removing a palette from a tile keeps every pixel's colour, as
   Default's exact match or as a loose colour that the tile warns about.
6. **The picker offers the right palettes first:** the palettes of the zones
   a sprite is placed in, then Default, then the rest.

## Decisions

Asked for by its author on 2026-10-28, paraphrased: move palette editing out
of the Pixel editor's panel, which was confusing, into an editor of its own,
so nothing is duplicated; then, after reviewing wireframes over three rounds
of questions, build it.

Agreed by its author on 2026-10-28, before building:

- **One built-in palette, Default, of 256 colours of the engine's own
  making,** not a licensed palette such as Aurora: sixteen greys and sixteen
  ramps of fifteen colours, dark to light. Underneath it is a list of
  built-in palettes, but there is only one and nothing offers a choice.
- **Default is static:** read only in every game, the same everywhere. Its
  name, ramps and colours cannot be changed, it cannot be removed, and its
  colours can be copied out. Every other palette's ramps and colours can be
  edited.
- **A game's palettes live with the game, beside Default.** Palettes are
  added and removed, except Default.
- **Terms follow common pixel-art use:** a palette holds named **ramps**, a
  ramp is a named list of colours, and a **gradient** is a tool that fills a
  ramp from a left colour, a right colour and a number of steps, after which
  each colour can still be edited. A pixel names `palette / ramp n`, such as
  `gloomy / smoke 1`. "Mood palette" is only how a palette is used; every
  palette but Default is the same kind of document.
- **A colour can carry a short label,** at most 15 characters, unique within
  its palette and stored with that palette's colour. It shows when the colour
  is chosen and on hover.
- **The Palette Editor is a window kind of its own,** named Palette Editor,
  one window per palette, opened from the project browser's Palettes section.
- **The Pixel editor's panel is pick-only:** choosing a colour, its name and
  label, and Edit palette…, which opens the Palette Editor at that colour.
  Shows stays in the Pixel editor.
- **A palette is renamed and deleted** in the project browser and in the
  Palette Editor.
- **Moving a colour to another ramp keeps a shadow record** of its old place
  in the palette, so tiles painted with it still find it; opening such a tile
  warns until those pixels are repainted. A shadow record goes by itself once
  no tile names the old place, and the Palette Editor lists them under Moved
  colours.
- **The gradient mixes the colour, the opacity, or both,** in steps that look
  evenly spaced (OKLab), and fills a new ramp or an existing one, setting its
  colours 1 to n and adding or removing at the end.
- **Gradient, copy and move apply at once,** and Cmd+Z undoes them, as adding
  and removing do; typed fields wait for Save.
- **A copied colour is independent:** its colour and opacity are copied once,
  its label never is.
- **A new palette starts empty or from colours chosen out of Default,** whole
  ramps or single colours. Whole ramps keep their names; single colours go
  together into the first ramp, named "default".
- **Deleting a palette lists every zone and tile that refers to it,** and each
  reference is removed first. Removing a palette from a tile keeps its
  pixels' colours: a colour that exactly matches one of Default's becomes
  Default's, and the rest become **loose colours**, a value that names no
  palette. A tile with loose colours warns about them and marks them until
  they are repainted. Exact matches are looked for only in Default.
- **A zone lists the palettes it uses,** per zone, edited in the Map's
  inspector and in the project browser's. It is a convenience, not a limit.
- **The picker's order is alphabetical:** the palettes of every zone a sprite
  is placed in, merged, A–Z; then Default; then every other palette A–Z,
  folded. A sprite placed in no zone shows Default first.
- **No migrations:** projects made before this increment are not upgraded.
  Their tiles and the old shared palette may be discarded.

## Wireframes

The Palette Editor, as a tab, editing a palette of its own. Letters in
brackets stand for each kind's icon.

```text
┌──────────────────────────────────────────────────────────────────────┐
│ [P] Project │ [S] Chimney top │ [C] gloomy ⧉ ×        │      ◷ 08:00 │
├──────────────────────────────────────────────────────────────────────┤
│ Palette [gloomy    ]  + Ramp  Gradient…  Copy colours from…  Delete… │
├──────────────────────────────────────────────┬───────────────────────┤
│ clear  ▢                                     │ COLOUR · smoke 2      │
│ smoke  ■ ■ ■ ■                               │ Label  [chimney] 7/15 │
│ murk   ■ ■ ■                                 │ Colour  [#5D5A63]     │
│ ember  ■ ■ ■ ■•                              │ Opacity [100%   ]     │
│ fog    ▣ ▣                                   │ Used by Chimney top   │
│                                              │ [+ Colour] [Remove]   │
│ MOVED COLOURS                                │ RAMP                  │
│ ember 1 → murk 4 · named by Chimney top      │ Name [smoke   ]       │
│                                              │ [Remove ramp]         │
│ GRADIENT  ☑ Colour #2E2C33 → #E1E3E6         ├───────────────────────┤
│           ☐ Opacity  Steps 5  ■ ■ ■ ■ ■      │ 2 unsaved Revert Save │
│           Into [New ramp ▾] [ash  ] [Apply]  │                       │
└──────────────────────────────────────────────┴───────────────────────┘
```

Default in the Palette Editor shows the same ramps with no fields: its
colours can be chosen and copied, and nothing else.

The Pixel editor's panel, pick-only:

```text
│ PALETTE                    │
│ clear ▢                    │
│ ▾ gloomy                   │
│ smoke ■ ■ ■ ■              │
│ ▾ Default                  │
│ grey  ■ ■ ■ ■ ■ ■ ■ ■ …    │
│ ▸ sunny                    │
│ CHOSEN                     │
│ ■ gloomy / smoke 2         │
│   chimney · #5D5A63 · 100% │
│ [Edit palette…]            │
│ SHOWS  [Small pickup ▾]    │
```

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. Art model | `engine/art`: Default's 256 colours with fixed IDs; labels; moving ramps and colours, with shadow records; loose colours in tiles; a palette dropped from a tile; the gradient's OKLab steps | Unit tests of Default's size, IDs and order; label length and uniqueness; a moved colour found through its shadow record; loose colours saved and drawn; a dropped palette mapped to Default's exact matches; gradient end points and steps | Planned |
| 2. Plug-ins and orchestrator | Default as the palette every project holds, read only; edit kinds to move a ramp and a colour; a shade's label; a zone's palettes and their references; the sample's palette | Every edit kind passes the shared apply-then-undo test; headless orchestrator tests that Default is added, refused every edit, rename and delete, and that a palette a zone lists is not deleted | Planned |
| 3. Palette Editor | `editor/views/paletteview`: the window kind; ramps and colours; the inspector with its save bar; adding, renaming, reordering, moving and removing; labels; Moved colours; Default read only; renaming and deleting the palette, with its references | View tests of each action and of Default's refusals; a real-window test that adds a ramp, moves a colour and saves a label | Planned |
| 4. Gradient and copying | The gradient panel; Copy colours from…; New palette from Default's colours | View tests of a gradient into a new and an existing ramp and of copied colours without labels | Planned |
| 5. Pick-only Pixel editor | The panel grouped by palette, in the agreed order, folding; the chosen colour's name and label; hover; Edit palette…; warnings for moved and loose colours | View tests of the order, of Edit palette… and of each warning; the real-window tests updated | Planned |
| 6. Browser and zones | Palettes [+] and its New palette dialog; Default marked; opening a palette; a zone's palettes in the Map's and the browser's inspectors; a sprite's palettes and Remove… | View tests of the dialog, of a zone's palettes and of a palette removed from a tile | Planned |
| 7. Trying it | The demo project with Default, a palette of its own and a zone listing it, for its author to try | Judged by its author | Planned |

Where each step changes code:

- **Art model:** `engine/art`.
- **Plug-ins and orchestrator:** `editor/plugins/art`, `engine/zone` and
  `editor/plugins/zone`, `editor/orchestrator/required.go`, and the sample in
  `editor/cmd/berad-editor`.
- **Palette Editor:** a new `editor/views/paletteview`, and the window kinds
  in `editor/window`.
- **Gradient and copying:** `editor/views/paletteview` and
  `editor/views/browser`.
- **Pick-only Pixel editor:** `editor/views/pixelview`.
- **Browser and zones:** `editor/views/browser` and `editor/views/mapview`.

## Step results

## Not in this increment

- **A choice of built-in palettes:** there is one, Default.
- **Palette cycling,** still a candidate in
  [the editor features](../EDITOR_FEATURES.md#palettes-and-transparency).
- **Alt-drag to copy a colour** within a palette, shown in the wireframes but
  not agreed.
