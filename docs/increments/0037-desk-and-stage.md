# Increment 37: A desk and a stage

**Status:** Planned

The editor's look changes from plain white to the desk and the stage agreed
on 2026-11-11 in [the editor features](../EDITOR_FEATURES.md#principles):
four tones with one job each, one job for each colour, waiting for Save in
ink instead of red, and the tints on every document's tab. Nothing an author
does changes; what changes is where the eye goes. White means a box the
author can type in or choose from. Panels, lists and a workflow's canvas are
the page. The window bar, the status bar and the split's divider are the
desk. The game is shown on a dark stage, so it is the brightest thing in the
window. It builds on [increment 36](0036-work-losing-bugs.md) and is held to
the same strict checks as [increment 1](0001-foundation.md).

## What it proves

1. **Every piece of text is at least 4.5 to 1 against its ground:** a test
   measures each pairing of text colour and ground the theme offers.
2. **The tones have one job each:** white is used only for fields, buttons,
   nodes, menus and what is drawn on a filled colour; screenshots of every
   view at 1512 × 916 show page, desk and stage where the decision puts them.
3. **The game stands on the stage:** the Map, the Pixel editor and Play draw
   behind the game in the stage's tone, and what is written over the stage
   stays readable.
4. **Waiting for Save is ink:** a field with a draft shows an ink dot and a
   heavier outline, and the save bar is solid ink while anything waits and
   quiet when nothing does.
5. **Nothing behaves differently:** the 61 real-window walks pass unchanged.

## Decisions

Agreed by its author on 2026-11-11, before building (see the editor
features, "a desk and a stage" and "a colour has one job"):

- **Four tones:** field `#ffffff`, page `#f4f2ec`, desk `#e7e4da`, stage
  `#35373b`.
- **Blue is where the author is, orange is running now, red is stop,** and
  waiting for Save is ink.
- **The seven tints stay organisation, not meaning,** and a document's tab
  keeps its tint.
- **Text at least 4.5 to 1,** and states told apart by tone or shape as well
  as hue.
- **Flat tones, hard edges,** and a hard shadow under what floats.

Chosen while building, for its author to check:

- **Regions meet at a hairline, not the dark outline.** The divider between
  panels was a 1.5-point ink line; it becomes a one-point line a step darker
  than the desk, `#b7b3a6`. The ink outline stays on what is pressed or
  typed in.
- **Every document's tab carries its tint, not only those on the second
  row.** A kind with one document open showed it on the top row untinted.
- **Open documents share one set of tints across the window.** Each kind
  used to count its own, so a single map and a single workflow were both
  green; now no two open documents share a tint while seven or fewer are
  open. The project browser's tab and Play's have none, since neither is a
  document.
- **The tab in front takes a stronger wash** (`#b5d9bd` for green, and so on
  for each tint) and a thicker band; tabs behind take a lighter wash, one
  step deeper than before so it holds against the desk.
- **A document's icon is drawn white on a filled chip** of its type's muted
  colour: zones and floors, workflows and zone loops, NPCs, sounds, sprites,
  palettes and templates. Folders, places, Home, the clock, Watch, Play and
  the project keep a plain mark.
- **Orange as text is `#a54c07`**, a step darker than `#b4540a`, which was
  4.45 to 1 on the page. The lighter grey for hints becomes `#5f646d`; the
  old one was 3.2 to 1 on white.
- **Dialogs, sheets and pickers are page tone with a hard shadow;** menus
  stay white, since an author chooses from them as from a field.
- **Watch's canvas is the page,** like a workflow's, since it shows a
  workflow; only the game is on the stage.
- **The stage is behind the Map, the Pixel editor, Play, the Template
  editor's preview and the writing sheet's "In the game".** A Log's line is
  read in Watch, so its preview stays on the page. The Project window's
  pictures stay on the page too: they show workflows and waveforms as well
  as art, and those read dark on light.
- **An empty cell of a floor is the page tone,** with its dot, so a floor
  stands out from the stage around it.
- **The save bar's quiet state is the page with a hairline,** and its
  waiting state solid ink with white text, a white dot and the Cmd+S hint
  in the stage's quieter ink.
- **A tab holding changes that wait for Save shows an ink dot,** read from
  what each pane last told the orchestrator; input scripts read it as
  `state:tab-unsaved:<title>`.
- **The clock's panel floats on a hard shadow,** as menus and dialogs do.

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. Tones and colours | The four tones, the hairline, the text colours and washes, the tints' two washes; every use of white named as a field, the page or reverse text | The contrast test passes; `make check` passes | Built |
| 2. The stage | The Map's canvas and what lies beyond a floor, the Pixel editor's canvas and Play on the stage; what is written over it in the stage's ink | Screenshots of a Map, a sprite and Play | Built |
| 3. Tabs and chips | The desk bar; every document tab tinted, the front one stronger; a dot on a tab with unsaved changes; document icons on filled chips | A script reads a tab's dot; screenshots of the bar | Built |
| 4. Waiting for Save | The field's ink dot and heavier outline; the save bar in ink | Screenshots of a draft and of a saved field | Built |
| 5. Hard shadows | Menus, dialogs, sheets and pickers float on a hard two-point shadow | Screenshots of a menu and a picker | Built |
| 6. Checks | `make check` and the real-window walks; screenshots of every view at 1512 × 916 and 2560 × 1380 | All pass | Planned |

## Step results

- **Tones and colours, the stage, waiting for Save and hard shadows:** built
  together in one commit, since every use of the old white had to be named
  at once. Each of the 90 places that drew white now says which job it
  does: a field, the page, or text on a filled colour. `ColorBackground`,
  `ColorPanel`, `ColorCanvas` and `ColorOutside` are gone, replaced by
  `ColorField`, `ColorPage`, `ColorDesk`, `ColorStage` and the stage's ink.
  `TestTextReadsOnEveryGround` measures 99 pairings of text and ground and
  `TestTonesStepApart` the four tones; both pass, and `make check` passes.
  Screenshots at 1512 × 916 of Home, the Project's lists and cards, a Map
  at rest and previewing, a sprite, Values, a workflow with a Say selected,
  a field with a draft, a menu and Play were looked at one by one.
- **Tabs and chips:** every document's tab tinted, the front one stronger,
  an ink dot on a tab with unsaved changes, and document icons on chips.
  `TestOpenDocumentsAreToldApart` replaces the test of tints by kind; the
  dot was read by an input script after typing a name in the Project's
  inspector, and seen to go when Esc reverted it.

## Not in this increment

The rest of what was agreed on 2026-11-11 changes how the editor behaves, not
only how it looks, and comes in increments of its own: one row of tabs with
keys and quick open; panels that fold and drag, kept per kind of view; a
history that can be read; lines that do only what their author did; a
preview that opens nothing by itself. Drawing at the screen's own resolution
and a scale for text and controls change every view's layout and come with
the panels.
