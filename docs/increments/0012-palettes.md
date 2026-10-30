# Increment 12: Palettes

**Status:** Built

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

Asked of its author on 2026-10-28, before building, and answered:

- **The old shared palette goes:** nothing drawn before matters, so it is not
  converted; a project that still has one keeps it as an ordinary palette.
- **Default's nine unfinished ramps are generated in the style of the
  others:** amber, yellow, lime, moss, teal, cyan, sky, violet and magenta.
- **A colour is also chosen with a colour picker (2026-10-30),** beside its
  hex field, drawn by the editor so it is the same on every system.
- **Added after trying it (2026-10-30):** a slider beside the Opacity field,
  keeping the picker's opacity strip; a colour moves to another ramp by
  dropping it on the ramp's name or with Move to ramp ▾; "+ Colour after" is
  now Duplicate colour; and the gradient has the same picker, in its panel,
  for whichever end is chosen.

Chosen while building, for its author to check:

- **Default's ramps,** in order: grey, red, orange, amber, yellow, lime,
  green, moss, teal, cyan, sky, blue, violet, magenta, skin, earth and slate.
  The nine generated ramps step evenly in OKLCH lightness, from 0.17 to 0.94,
  their colour strongest in the middle and their hue turning slightly warmer
  as they lighten, clipped to what a screen shows.
- **Default is made again whenever a project opens,** from the engine's own
  data, so a changed copy on disk is put back; its folder and tags are kept.
  It has fixed IDs, as the old shared palette had.
- **A pixel's name uses its palette's name as written:** `Default / grey 3`,
  `gloomy / smoke 1`.
- **Palettes and tiles are saved at version 2,** which reads version 1 as it
  is, since version 2 only added labels, shadow records and loose colours.
- **A palette is not deleted while something refers to it,** a new seam
  (`model.KeptWhileUsed`) the orchestrator checks; deleting a palette that is
  used from the project browser opens its Palette Editor at Delete palette….
- **Ramps are reordered by dragging their names,** and colours by dragging
  them; no copying by Alt-drag.
- **A shadow record stays in the palette's file,** so undoing a repaint still
  finds the colour; Moved colours shows it while a tile refers to both the
  ramp the colour left and the colour, and hides it after.
- **A labelled colour has a dot under its swatch,** and every swatch's tip
  shows its name, label, value and opacity.
- **The gradient starts mixing the colour only;** Use chosen takes either end
  from the chosen colour, and choosing an existing ramp starts it from that
  ramp's ends and length. Its settings never wait for Save.
- **Copy colours from… takes a whole ramp by its name,** and new copies go to
  a new ramp named after their source ramp, or the end of a ramp chosen.
  From Default's window, Copy to palette sends each source ramp's colours to
  the ramp of that name, or a new one.
- **A new palette is made two ways:** the Palettes section's [+] makes an
  empty one, "Palette 1", and opens it; New palette from these, in Default's
  window, makes one from the picked colours. There is no dialog in the
  project browser.
- **A sprite's inspector lists the palettes its tile uses,** Default aside,
  each with Remove….
- **A loose colour draws as its value;** the Pixel editor warns about it and
  about colours that have moved, Show them outlines those pixels, and the
  eyedropper does not pick a loose colour.
- **A sprite is placed in a zone** when a stand-in it shows is on a floor of
  that zone. The panel's folds belong to the window and are not saved.
- **A zone's palettes show in the Map's inspector under the zone's name**
  while nothing is selected, and in the project browser's under the zone's
  buttons; + Palette never offers Default.
- **The window bar names the kind Palette Editor,** after Sprites, and the
  windows of one palette share an undo list.
- **The colour picker** is a square of saturation and brightness over a hue
  strip and an opacity strip, under the colour and opacity fields. What it
  picks goes into those fields and waits for Save, as typing does; what is
  typed moves it. It keeps its hue while a colour is grey or black.
- **A colour dropped on a ramp's name goes to that ramp's end,** the name
  outlined while it is over it; Move to ramp ▾ does the same and offers every
  ramp but the colour's own. Both leave a shadow record, as any move does.
- **The gradient's picker sits right of its fields,** or below them in a
  narrow window. Each end has a swatch beside its field; clicking the swatch
  or its fields chooses the end the picker sets, outlined. The picker shows
  opacity only while Opacity is mixed.
- **The sample paints its lantern in Default's grey 3, 5, 8, 11 and 14** and
  the glass of a palette of its own, "Sample", which the sample zone lists.
  (Changed in [increment 13](0013-single-window.md): the lantern is painted
  only from the sample's palette, now "gloomy".)

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
| 1. Art model | `engine/art`: Default's 256 colours with fixed IDs; labels; moving ramps and colours, with shadow records; loose colours in tiles; a palette dropped from a tile; the gradient's OKLab steps | Unit tests of Default's size, IDs and order; label length and uniqueness; a moved colour found through its shadow record; loose colours saved and drawn; a dropped palette mapped to Default's exact matches; gradient end points and steps | Built |
| 2. Plug-ins and orchestrator | Default as the palette every project holds, read only; edit kinds to move a ramp and a colour; a shade's label; a zone's palettes and their references; the sample's palette | Every edit kind passes the shared apply-then-undo test; headless orchestrator tests that Default is added, refused every edit, rename and delete, and that a palette a zone lists is not deleted | Built |
| 3. Palette Editor | `editor/views/paletteview`: the window kind; ramps and colours; the inspector with its save bar; adding, renaming, reordering, moving and removing; labels; Moved colours; Default read only; renaming and deleting the palette, with its references | View tests of each action and of Default's refusals; a real-window test that adds a ramp, moves a colour and saves a label | Built |
| 4. Gradient and copying | The gradient panel; Copy colours from…; New palette from Default's colours | View tests of a gradient into a new and an existing ramp and of copied colours without labels | Built |
| 5. Pick-only Pixel editor | The panel grouped by palette, in the agreed order, folding; the chosen colour's name and label; hover; Edit palette…; warnings for moved and loose colours | View tests of the order, of Edit palette… and of each warning; the real-window tests updated | Built |
| 6. Browser and zones | Palettes [+]; Default marked; opening a palette; a zone's palettes in the Map's and the browser's inspectors; a sprite's palettes and Remove… | View tests of the [+], of a zone's palettes and of a palette removed from a tile; a real-window test that makes a palette and edits it | Built |
| 7. Trying it | The demo project with Default, a palette of its own and a zone listing it, for its author to try | Judged by its author | Built |

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

- **Art model:** `engine/art` holds Default, 256 colours in seventeen ramps
  with fixed IDs whose time part is zero; shade labels of up to 15
  characters, unique within a palette; moving a ramp, and moving a colour
  within or between ramps with a shadow record of the place it left, which
  `Resolve` looks colours up through; loose colours in tiles; `Dropped`,
  what a tile's colours become when a palette is removed from it; and
  `Gradient`, steps mixed in OKLab. Zones list palettes (`engine/zone`,
  version 2). Verified with unit tests: Default's order, size, IDs and
  dark-to-light ramps, the same every time; labels refused when sixteen
  characters long, untrimmed or taken, counted in characters, and round
  tripped; a moved colour found through its record, the record dropped on
  moving back, and a move undone exactly with its records; loose colours
  saved, drawn as their value and refused when invalid; a dropped palette
  mapped to Default's exact match, opacity included, or loose; a gradient's
  ends and OKLab steps, by colour or opacity alone; version 1 palettes and
  tiles read as version 2; a zone's palettes inserted, removed and saved in
  order.
- **Plug-ins and orchestrator:** Default is the palette every project holds,
  read only (`model.RequiredDocument.ReadOnly`): the orchestrator refuses
  every edit to it and makes it again when a project opens. The art plug-in
  gains moving a ramp and a colour, labels in setting a shade, and IDs given
  to a new palette's ramps and colours; tiles' loose colours refer to no
  palette. The zone plug-in lists and unlists palettes, and a listed palette
  is a reference. A palette is kept while referred to (`model.KeptWhileUsed`).
  The sample's lantern uses Default and the sample's own palette, which the
  sample zone lists. Verified with every new edit kind passing the shared
  apply-then-undo test, and headless orchestrator tests: a project gains
  Default at the current version; a Default changed on disk is restored on
  opening; renaming a ramp, setting a shade and moving a ramp of Default are
  refused as read only, and deleting or renaming it as held by every project;
  a palette a tile is painted with is not deleted, naming the tile.
- **Palette Editor:** a new `editor/views/paletteview` and window kind,
  Palette Editor, after Sprites on the window bar. Verified with view tests:
  a click chooses, Shift-click picks a run, Cmd-click adds or takes away and
  a ramp's name picks it whole; + Ramp, renaming with Enter, Duplicate colour,
  Remove colour and Remove ramp; a label, colour and opacity wait for Save,
  sixteen characters are refused and kept, and choosing another colour asks
  first; dragging a colour to another ramp sends the move, Moved colours
  lists it while a tile names its old place, a move within a ramp counts the
  place without the colour, and dragging a ramp's name reorders it; Default
  shows no fields and nothing in it drags.
- **Gradient and copying:** verified with view tests: a new ramp of five
  OKLab steps; an existing ramp of four filled with six, as one step; a
  gradient that would remove a colour a tile uses refused; Default's slate 2
  to 6 copied into a new ramp, without labels; Default's colours copied into
  a palette of the game's own, and a new palette made from a whole ramp and a
  single colour, which go in "default" first; Delete palette… waiting while a
  zone and a tile refer to the palette, the zone's listing removed and the
  tile's pixels kept as Default's exact match and a loose colour, then the
  palette deleted.
- **Pick-only Pixel editor:** its panel picks colours only. Verified with
  view tests, each frame also drawn: palettes in the agreed order for a
  sprite placed in a zone that lists one and for a sprite placed nowhere,
  folded and opened by their headings; Edit palette… opening the chosen
  colour's palette at that colour, and doing nothing with clear; warnings
  for pixels that name a moved colour and for loose colours, Show them, the
  eyedropper picking a moved colour at its new place and refusing a loose
  one; a long palette scrolling.
- **Browser and zones:** the Palettes section has a [+] and New ▾ a Palette,
  which make an empty palette and open it; Default is marked "default · read
  only"; a palette opens in the Palette Editor; a used palette's Delete opens
  its Palette Editor at Delete palette…; a zone's inspector, in the browser
  and in the Map with nothing selected, lists its palettes
  (`editor/views/zonepalettes`); a sprite's inspector lists the palettes its
  tile uses, each with Remove…. Verified with view tests of each.
- **Real windows:** a new real-window test makes a palette from the Palettes
  section's [+], which opens in a Palette Editor, copies Default's slate
  ramp, labels a colour, adds a ramp, drags a colour into it and adds a
  gradient; the saved palette holds all of it and the shadow record. The
  map test now makes its ramps in a Palette Editor opened by Edit palette…,
  and the Pixel editor's tests paint with Default and the sample's palette.
  In a full run 34 of the 35 real-window tests passed; the new one failed
  because Copy colours from… did not scroll a ramp's name into view, and
  passes since that was fixed, with the other two palette tests run again.
  `make check` shows no lint findings, with every package passing.
- **Colour picker (2026-10-30):** `ui.ColorPicker`, used by the Palette
  Editor's inspector. Verified with unit tests that colours come back
  unchanged and that each part picks, with a view test that picks wait for
  Save and start from a typed colour, and with the real-window palette test,
  which now picks red at 50% opacity and saves it.
- **Slider, moving, Duplicate colour and the gradient's picker
  (2026-10-30):** `ui.Slider`, and `ui.ColorPicker` without its opacity
  strip. Verified with unit tests of the slider, view tests that the slider
  waits for Save, that a colour dropped on a ramp's name or moved with Move
  to ramp goes to that ramp's end, and that the gradient's picker sets the
  chosen end; the real-window palette test now drops a colour on a ramp's
  name.
- **Trying it:** the sample project has Default, its own palette "Sample"
  with the lantern's glass, listed by the sample zone:
  `make demo-shell DEMO_PROJECT=.cache/demo-palettes`.
## Not in this increment

- **A choice of built-in palettes:** there is one, Default.
- **Palette cycling,** still a candidate in
  [the editor features](../EDITOR_FEATURES.md#palettes-and-transparency).
- **Alt-drag to copy a colour** within a palette, shown in the wireframes but
  not agreed.
