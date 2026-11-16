# Increment 61: Views fit a small window

**Status:** Planned

Every view lays out without one part drawn over another in a small window:
a row of buttons wider than its room goes on onto another row, an
inspector's fields scroll above what stays at its foot, and a tab that
would need more room than the window has is refused with what makes room.
[Increment 56](0056-native-resolution-and-ui-scale.md) found the overlaps:
at 150% a window of 1200 × 832 of the screen's points is 800 × 555 of the
window's, and several views did not fit, as they would not at 100% in a
window that small. It follows "every panel folds, when told to" in
[the editor features](../EDITOR_FEATURES.md#principles): nothing folds or
moves by itself, and what will not fit is refused with the reason and the
keys that make room. It builds on
[increment 60](0060-braces-colour-and-crashes.md), and is held to the same
strict checks as [increment 1](0001-foundation.md).

## What it proves

1. **A row of buttons wraps:** a toolbar or a row in a panel too wide for
   its room carries on below, and what is under it moves down a row. Every
   button stays in sight; wide enough, the row is one row as before.
2. **An inspector's fields scroll:** in an inspector taller than its room,
   what lies between its heading and its foot scrolls with the wheel, with a
   thumb at the edge showing there is more. The save bar, Delete and the
   buttons at the foot stay in place and are never drawn over a field.
3. **What will not fit is refused, saying what helps:** Watch opens in a
   tab of its own when its Map's tab has no room for it beside, and a larger
   step of text and controls is refused while a tab would not fit at it.
   Each says why and names the keys that make room.
4. **Every view fits:** at 150% in a window of 1200 × 832, the Project
   window, World state, the Palette Editor, the Pixel editor, a Map, Audio
   and a Map with Watch beside it lay out with nothing drawn over anything
   else, seen in screenshots of the real window.

## Decisions

Agreed by its author on 2026-11-16, before building, from screenshots of
the overlaps and drawn options:

- **A row of buttons wraps to another row.** (Rejected: what does not fit
  under a More button at the row's end; the window stopping narrowing where
  the widest row fits.)
- **An inspector's fields scroll** between its heading and its foot, as the
  Project's inspector and a Workflow's node form already do. (Rejected:
  headings that fold their sections as well; the window stopping getting
  shorter where every field fits.)
- **A tab that needs more than the window is refused,** saying what makes
  room: Watch opens in its own tab instead of beside its Map, and a larger
  Text step is refused. (Rejected: the window widening to fit; the Map's
  panels folding to make room, which folds what the author did not fold.)

Chosen while building, for its author to check:

- **A row wraps by its two ends:** what starts a row stays at its left and
  what ends it at its right, as now; when they do not fit with room between
  them, the right end starts a line of its own below, against the right
  edge, and a control that does not fit on its line carries on at the start
  of the next. The Pixel editor's zoom, Template and Grid, the Palette
  Editor's Delete palette…, the Project window's sizes and views, Audio's
  toolbar buttons and World state's Find a value each go below together.
- **The Project window's facets carry on to further lines** rather than
  leave out those that did not fit on the second, as Changed and Problems
  were at 150% in a small window.
- **Audio's panel wraps its rows within its width,** about 200 points at
  150% in a small window: each row of buttons, the waveform's zoom beside
  its label, Play, Region, Whole file, Loop and Duplicate region, the
  region's fields and buttons, the meter's legend, the hint, what Play
  plays and a cue's problems. "As recorded", "Evened out" and "In this cue"
  touch while they fit together and carry on one by one when they do not.
- **The gradient wraps by its ends:** its right end and Steps go below its
  left end, each end's opacity stays under its own field, its description
  takes further lines, and its panel grows by what wrapped.
- **What stays at an inspector's foot:** World state's save bar and Delete;
  the Palette Editor's save bar, or for Default the note saying why it
  can't be changed; Audio's Delete cue or Delete recording; and a Map's
  preview lines, Clock, Watch and Keys. Everything between the heading and
  the foot scrolls, the zone's and floor's lines included.
- **A press beside the fields goes to the foot alone:** on the save bar,
  Delete or the heading, never to a field scrolled behind it. Tab brings a
  field scrolled out of view into the inspector's middle, as in a
  Workflow's node form, and only a field of the inspector's own column.
- **Lists in an inspector no longer stop at its room:** World state lists a
  template constant's holders up to its usual limit, and Audio a cue's uses
  up to eight, the rest counted, since they scroll.
- **A Map's left panel scrolls only when it must:** in a window too short
  for its tools, tiles and floors, Show floor below and Preview stay at its
  foot, above the status bar, and the lists scroll above them; in a window
  tall enough nothing moves. The Keys sheet stays inside the window and its
  rows scroll under its title.
- **What is refused, and where it is said:** Watch opening beside its Map,
  and a window asked for beside a tab from quick open, open in a tab of
  their own and say so in their own status bar; a larger Text step is
  refused in the status bar of the pane that asked, when a tab would be
  wider than the window or the window shorter than its shortest. Each names
  the sizes in the screen's points, as the window's frame is, and the keys
  that make room.
- **A split's divider keeps each pane's room:** it stops where either pane
  would lose its panels or the canvas's room, and says what makes room when
  the drag ends, as the panel dividers do.
- **A scripted run can open its window at a size,** given as
  `BERAD_WINDOW_SIZE=1200x832`, so the walks can be seen at 150% in a small
  window.

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. Seeing it | A scripted run opens its window at a size it is given, so the walks can be seen at 150% in 1200 × 832 | A unit test; screenshots of every walk | Planned |
| 2. Rows wrap | The Project window's view buttons, the Palette Editor's toolbar and gradient row, the Pixel editor's toolbar and Audio's toolbar and rows carry on below when too wide | Unit tests | Planned |
| 3. Inspectors scroll | World state's, the Palette Editor's, Audio's and a Map's inspectors scroll between heading and foot | Unit tests | Planned |
| 4. Refused when it will not fit | Watch beside its Map, and a larger Text step, refused with the reason and keys | Unit tests | Planned |
| 5. Checks | Screenshots at 150% in 1200 × 832, `make check` and the real-window walks | Nothing drawn over anything; all pass | Planned |

## Step results

## Not in this increment

- A Workflow drawn sharp at zooms other than 100%, left from increment 56.
