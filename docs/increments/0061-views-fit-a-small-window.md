# Increment 61: Views fit a small window

**Status:** Built

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
  of the next; when the left end itself wrapped, the right end ends its
  last line if it fits there. The Pixel editor's zoom, Template and Grid,
  the Palette Editor's Delete palette…, the Project window's sizes and
  views, Audio's toolbar buttons and World state's Find a value each go
  below together.
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
  takes further lines, and its panel grows by what wrapped. An open panel,
  the gradient or Copy colours from…, stops under the toolbar in a short
  window, and the ramps above it give it their room; Copy colours from…'s
  instructions take further lines before its close button.
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
- **Watch has no inspector:** it says so, so a window needs room for its
  canvas only beside its Map, and it has no inspector divider, whose hidden
  grip lay over Step to next change once its pane was a little narrower. At
  100% in a window 1200 wide Watch still opens beside the Map, whose divider
  now stops where the Map keeps its panels and canvas, 18 points right of
  where it was.
- **Beside a Map from quick open, a Workflow needs room at 100% too:** a
  Map's panels and a Workflow's need about 1,440 points with their
  canvases, so in a window 1200 wide Cmd+Enter opens the Workflow in a tab
  of its own and says so, where before it opened beside with almost no
  canvas. Folding the Map's inspector, as the refusal says, makes room.
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
| 1. Seeing it | A scripted run opens its window at a size it is given, so the walks can be seen at 150% in 1200 × 832 | A unit test; screenshots of every walk | Built |
| 2. Rows wrap | The Project window's view buttons and facets, World state's header, the Palette Editor's toolbar, gradient and copy panel, the Pixel editor's toolbar and Audio's toolbar and rows carry on below when too wide | Unit tests | Built |
| 3. Inspectors scroll | World state's, the Palette Editor's, Audio's and a Map's inspectors scroll between heading and foot; a Map's left panel and the Keys sheet in a short window | Unit tests | Built |
| 4. Refused when it will not fit | A pane beside a tab, a larger Text step and a split's divider, refused with the reason and keys | Unit tests | Built |
| 5. Checks | Screenshots at 150% in 1200 × 832, `make check` and the real-window walks | Nothing drawn over anything; all pass | Built |

## Step results

- **Seeing it:** `TestAScriptedRunOpensAtTheSizeAskedFor` opens a scripted
  run at `BERAD_WINDOW_SIZE`, and an unscripted run never reads it. Every
  real-window walk was run at 150% in a window of 1200 × 832 before any
  change: 63 passed and 17 failed, most where a click landed on a part
  drawn over another. Their screenshots showed the overlaps the brief
  listed, and four more: World state's Find a constant over Add template,
  the Project's NPCs path under Cards, a Map's Preview button under the
  status bar, and the Keys sheet taller than the window.
- **Rows wrap:** `ui.WrapRow` places a row's two ends and carries them on
  below; `TestARowWrapsWhenTooWide` checks one line when wide, the right end
  on its own line when narrow, and nothing outside or over anything. At 800
  × 555 points, `TestTheToolbarWrapsInASmallWindow` in the Pixel editor and
  the Palette Editor, `TestTheHeadWrapsInASmallWindow` in the Project window,
  `TestTheHeaderWrapsInASmallWindow` in World state,
  `TestRowsWrapInASmallWindow` in Audio and
  `TestTheGradientWrapsInASmallWindow` find no part over another or outside
  its room. Run on the code before, the Pixel editor's, the Palette
  Editor's, the Project window's and the gradient's failed on the overlaps
  the screenshots showed, such as "Fill over zoom", "places over S" and
  "steps outside" the gradient's panel.
- **Inspectors scroll:** `ui.Scroll` holds a body's offset, scrolls with the
  wheel, hides what lies beside the body from presses, reveals a field for
  Tab or an input script, and clips what scrolls;
  `TestABodyScrollsBetweenHeadingAndFoot` and `TestTabRevealsAFieldOfTheBody`
  check them. At 800 × 555, `TestTheInspectorScrollsInASmallWindow` in World
  state, the Palette Editor, Audio and the Map finds the foot apart from the
  fields, the fields needing more height than they have, and the wheel
  scrolling them; World state's and Audio's press Delete with a field
  scrolled behind it and find Delete takes it, which World state's failed
  to without the foot taking presses first.
  `TestTheLeftPanelScrollsInASmallWindow` keeps a Map's Show floor below and
  Preview above the status bar, and `TestAKeySheetFitsAShortWindow` keeps
  the Keys sheet inside the window.
- **Refused when it will not fit:** `TestWatchOpensInItsOwnTabWithoutRoom`
  and `TestWatchFitsBesideItsMapAt100` open Watch beside its Map in a window
  1200 points wide and in its own tab, saying why, in one 800 wide;
  `TestALargerTextStepIsRefusedWithoutRoom` refuses 175% where a Map would
  not fit and takes 150%; `TestTheDividerKeepsEachPanesRoom` stops a split's
  divider at each pane's room.
- **The walks at 100%:** the first whole run on this increment passed 79 of
  81. `TestQuickOpenFromAMap` expected the Workflow beside the Map in a
  window 1200 wide; it now finds it in a tab of its own, saying why, then
  folds the Map's inspector and finds it beside. `TestNPCFollowsItsWorkflow`
  pressed Step to next change on Watch's hidden inspector divider, which
  started a drag instead; Watch now has none.
- **Every view fits:** `TestViewsFitASmallWindow` opens the Project window,
  World state, the Palette Editor, the Pixel editor, a previewing Map and
  Audio in six runs at 150% in a window of 1200 × 832, and saves twelve
  screenshots; Watch opens in a third tab, and Cmd+Shift+= leaves text and
  controls at 150%. Looked at in the screenshots, nothing in any of them is
  drawn over anything else: the Project window's sizes and views sit below
  the path and its facets take a second line; World state's Find a value
  sits below its Add buttons and its fields scroll above Save and Delete;
  the Palette Editor's toolbar takes two lines, its gradient wraps by its
  ends under the toolbar and Copy colours from…'s instructions wrap before
  its close button; the Pixel editor's zoom and Grid sit below its tools;
  a previewing Map keeps Show floor below, Stop preview, its preview lines,
  Clock, Watch and Keys in place with the rest scrolling; Watch says in its
  own tab why it is there; and Audio's toolbar, panel and inspector wrap
  and scroll within their room.
- **Checks:** `make check` passes. All 81 real-window walks pass at 100%,
  the new one among them, run from a frozen copy in 20 minutes. Run while
  the real-window walks ran, `make check` failed in tests that time the
  orchestrator and fade status messages by the clock; alone, it passes.

## Not in this increment

- **Watch's own rows wrapping:** its strip's tabs and buttons and its Pause,
  Step and Follow still run past a narrow pane. In a small window Watch now
  opens in its own tab, where they fit; beside a Map at 100% in a window
  1200 wide they run past its pane as before.
- **The walks written for 100% at 150% in a small window:** 17 of them
  failed there before this increment, most on a click landing on a part
  drawn over another; they were not run there again, as they assume a
  window that lays out as at 100%.
- **The tab bar at 150% in a small window** puts all but the Project tab
  under "2 more", the tab in front among them, as it does whenever tabs do
  not fit.
- A Workflow drawn sharp at zooms other than 100%, left from increment 56.
