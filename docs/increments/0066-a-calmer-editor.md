# Increment 66: A calmer editor

**Status:** Built

The editor's shared controls get quieter strengths, and six places use
them. Its author found the Map's left panel "jarring and old looking". From
a canvas of five wireframes he chose the second, **grouped by job**, then
asked where else the same treatment would help, saw five more wireframes
and said to build "all of them". The fault they share is one the editor's
look already names: when every button is a white box with an ink outline
and every selected row has a heavy blue outline, the screen is stark and
nothing leads the eye. This increment stays inside the look agreed on
2026-11-11: light, four tones, blue for where the author is, tab tints
untouched. It is all in the editor module. No document format and nothing
in the engine changes. It is held to the same strict checks as [increment
1](0001-foundation.md).

## What it proves

1. **A button has three strengths.** *Main* is filled blue, at most one a
   panel. *Plain* is the white box with an ink outline, for a panel's few
   real choices. *Quiet* is the page's tone inside a hairline, for
   everything secondary.
2. **A selected row has two strengths.** *In hand*, filled blue with white
   text, is the one thing a panel is about: the tool in hand, the place
   shown. *Chosen*, a blue wash inside a thin ring, is a choice under it.
3. **Keys and counts are small caps** at a row's or a button's right edge,
   in the engine's own type.
4. **The Map's tools are grouped by job,** and what the tool in hand paints
   with is a set of swatches under its own heading.
5. **Buttons that are one choice stand together as one control:** a
   place's views, a portal's Fade and Cut, the clock's hour steps.
6. **Nothing a control does changes.** Every control keeps its ID, its key
   and what it does.

## Decisions

Agreed by its author on 2026-11-21 and 22, before building:

- **Option 2, grouped by job,** for the Map's left panel, and **all five**
  of the other wireframes: the Project window's list, the Workflow's
  palette, the inspectors, the search and filters, and the window and
  status bars.
- **One pull request** with [portals](0064-portals.md) and [select, then
  inspect](0065-select-then-inspect.md).
- **The lighter selected row everywhere:** the shared list changes once,
  so every view that uses it changes.
- **Inspectors: the Project window's and the Map's now.** The Workflow's,
  the Pixel editor's, Audio's, World state's and the Palette's wait.
- **No colour squares in the Workflow's palette.** A tint keeps one job:
  the colour its author gives a node.

Chosen while planning, and shown to its author in the plan:

- **Text stays the size it is.** The wireframes drew smaller text than the
  editor's 13 points; the calm comes from weight and outline.
- **Delete stays red, as a quiet button** no wider than its words.
- **The Map's tools are named for their group:** under ADD, "Anchor" and
  "NPC". The status bar and the help line keep their full names, and no
  key changes.
- **Launch is the window bar's main button.**
- **A set filter is a blue-wash button with a cross** that clears that
  filter alone; an unset one is quiet.
- **A place's views join into one control,** the one chosen filled with
  ink as before.

Chosen while building, for its author to check:

- **Swatches are one to a line in a panel as wide as it starts, and two
  once it is dragged wide enough for every name.** The wireframe drew two
  to a line. Two with their names whole need a panel about 240 points wide,
  and the panel starts at 190. It was first built 240 wide; the real-window
  suite then failed two walks, because a Workflow no longer fitted beside a
  Map with its inspector folded in a window 1200 wide, and a small window's
  Map lost a quarter of its stage. So the panel starts as wide as it was,
  and the divider still drags it. Doors, stairs and pickups, whose names
  are long, stay one to a line at any width tried. The NPCs to place stay a
  list.
- **A wall family's swatch is named "Family A" or "No family",** not by
  its full name, so two fit.
- **Floors are numbered** with a cap, 1 at the bottom, and the first nine
  swatches carry the number key that picks them.
- **The headings say what the list under them is for:** "PAINTING WITH",
  "DRAWING WITH", "PLACING", "CHANGE SELECTED WALLS TO" and "NPC TO PLACE".
- **Preview shows its key, `P`, as a cap,** and so does Show floor below,
  `F`.
- **The inspectors were restyled where they stand, not re-arranged.** The
  wireframe put each action on its own section's line, with Open map in
  the header. The plan also said no control moves, and the two could not
  both hold, so each button is where it was: Open, Open map and a portal's
  Open where it leads are the main buttons, everything else is quiet, and
  Delete is cut down to its words. Re-arranging is listed below as not
  done.
- **Launch does not turn orange while a game runs.** The plan said it
  would; the window bar does not know whether a game runs, and the Play
  tab and the clock's readout already say so in orange. Launch is blue
  always.
- **The clock keeps its order on the bar:** the two hour steps, joined,
  then Stop, then the readout that opens its panel. The wireframe put the
  readout between the hour steps, which would have moved what a press on
  the bar opens.
- **The status bar's History and Text are quiet buttons, and Inspector and
  On top are quiet buttons that are washed while on,** in place of tick
  boxes. They keep their places and sizes; they are boxed in a hairline
  each, where the wireframe divided them with single hairlines.
- **The view switch joins only while its buttons are on one line.**
  Wrapped onto lines of their own in a narrow window, they stay apart.
- **A search result in the Workflow's palette stays a box,** with a
  hairline, since it carries a second line saying where the node is from.
- **Scripts can find a set filter's cross** as `clear:<Filter>`.

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. The shared pieces | Quiet and main buttons, key caps, the two selected rows, eight icons, the joined control, the swatch grid | `ui` tests of each; the contrast test passes with the new pairs; every view's tests still pass | Built |
| 2. The Map's left panel | Grouped tools, the swatch grid, numbered floors | Map tests: each tool by click and key, each palette by tool | Built |
| 3. The Project window's list | The CONTENT heading, the place in hand filled, counts as caps | Browser tests of places, folders and counts | Built |
| 4. The Workflow's palette | Rows with a grip, headings with a count | Workflow tests of dragging, search, help and the keys | Built |
| 5. Inspectors | The Project window's and the Map's | Tests of every inspector button still finding and pressing it | Built |
| 6. Search and filters | Quiet filters, set ones with a cross, the joined view switch | Browser tests of ticking, clearing and saved views | Built |
| 7. The bars | Launch, the clock's controls, the status bar | Window and frame tests | Built |
| 8. Walks and record | A walk in the real window, and this record | The checks below | Built |

## Step results

Not in a pull request yet.

- **The shared pieces** are in `ui`: `Button.Quiet`, `Button.Key` and
  `NewQuietButton`; `DrawCap` for a key or a count; `List.InHand` for the
  filled row, `List.Caps` for counts and `List.Grid` for swatches;
  `DrawJoined` for buttons that are one choice; and icons for the Map's
  tools, a grip and a cross. A deeper blue was added for a cap on a filled
  row, and `TestTextReadsOnEveryGround` measures it with the other pairs.
- **The Map's left panel** lists its tools under SELECT, PAINT, ADD and
  FLOOR, each with an icon and its key. The other five places were changed
  by setting those strengths on the controls they already had.
- **Walks and record:** `TestTheCalmerEditor` in the real window selects a
  zone in the Project window, sets the Kind filter and clears it by its
  cross, takes a tool of each group in hand in the Map, opens a new
  workflow, and photographs each. No earlier walk was changed: they find
  controls by ID, and the ones that count colours count them in the Play
  tab.
- **Found by looking at the real window,** and fixed: the Home and Matches
  icons, which have no chip of their own, did not show on the filled row
  and are now reversed on it; the Unassigned swatch, whose row has no ID,
  always looked as if the pointer were over it; and the panel's width,
  above.

- **Checks:** `make check` passes: formatting, the strict lint with no
  findings, vet, and every test with the race detector. The real-window
  suite, run from a frozen copy with an empty screenshot folder, failed 2
  of its 87 tests on its first run, `TestQuickOpenFromAMap` and
  `TestViewsFitASmallWindow`, both from the wider panel. With the panel as
  wide as it was, it passed all 87 on its second run, on the last code of
  this increment. `TestTheCalmerEditor` passed each of the five times it
  ran.

## Changed after its author tried it

- None yet.

## Not in this increment

- **Re-arranging the inspectors,** with each action on its section's line
  and hairlines between sections, as the wireframe drew a zone's.
- The Workflow's, the Pixel editor's, Audio's, World state's and the
  Palette's inspectors.
- The Pixel editor, Audio, World state, Watch and Clock windows, which were
  not looked at.
- Tabs, tab tints, the save bar, fields, dialogs and menus.
- Smaller text.
- Any change to what a control does, or to its key.
