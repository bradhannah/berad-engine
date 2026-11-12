# Increment 54: Arrangements by screen and by mode

**Status:** Built

How panels are arranged now depends on the screen and, for a Map, on
whether it is previewing, as agreed on 2026-11-11 in "every panel folds,
when told to" in [the editor features](../EDITOR_FEATURES.md#principles):
"The screen's size chooses starting widths once, the first time a kind of
view opens on it, and the status bar says so; each size of screen keeps its
own arrangement. A Map keeps two, one for editing and one for previewing,
each as it was last left in that mode. Nothing changes while the window is
resized: the canvas takes up the change, and the window stops narrowing
rather than fold a panel." It builds on
[increment 53](0053-dividers-drag-and-are-kept.md), and is held to the same
strict checks as [increment 1](0001-foundation.md).

## What it proves

1. **The screen chooses starting widths once:** the first time a kind of
   view opens on a size of screen, a small screen starts its panels
   narrower and a large one wider, and the status bar says "Panels start at
   widths for this 1440 × 900 screen: drag a divider to change them." After
   that the kind is as it was left.
2. **A Map keeps two arrangements:** what is folded and how wide, for
   editing and for previewing, each put back as it was last left in that
   mode when a preview starts or stops.
3. **The window stops narrowing** at the room the tab in front needs, its
   panels and the canvas's, so a narrower window never squeezes or folds a
   panel.

## Decisions

Chosen while building, for its author to check:

- **The starting widths:** a screen 1440 pixels wide or narrower starts an
  inspector at 250 and a left panel at 170; one 2200 or wider at 340 and
  230; between, the views' own widths, and the status bar then says
  nothing, as nothing was chosen.
- **A Map's first preview starts as the Map is,** so starting a preview
  changes nothing until the author changes it.
- **The window's narrowest is never more than it is now,** so a tab that
  needs more than the window has does not grow it; it only stops it
  narrowing further.

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. Starting widths | The screen's choice for a kind's first opening, said once | A unit test | Built |
| 2. Two for a Map | A previewing Map's arrangement kept apart and put back | A unit test; a real-window walk | Built |
| 3. The narrowest window | The window's limit from the tab in front | A unit test | Built |
| 4. Checks | `make check` and the real-window walks | All pass | Built |

## Step results

- **Starting widths:** `TestTheScreenChoosesStartingWidths` reads the
  widths for screens 1440, 1920 and 2560 wide.
- **Two for a Map:** `TestAMapKeepsAnArrangementForPreviewing` folds a
  previewing Map's inspector, stops the preview and finds it back as it
  was, and starts another and finds it folded. In the real window,
  `TestAMapKeepsAnArrangementForPreviewing` does the same with P and
  Cmd+I.
- **The narrowest window:** `TestTheWindowStopsNarrowing` needs room for a
  Map's left panel, inspector and canvas, and the strip for a folded
  inspector.
- **Checks:** `make check` passes, and so do `TestDividersDragAndAreKept`,
  `TestTheInspectorFolds` and `TestWatchOpensBesideItsMap`. The whole
  real-window suite runs again on the last increment.
