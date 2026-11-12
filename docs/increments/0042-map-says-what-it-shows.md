# Increment 42: A Map that says what it shows

**Status:** Built

The stage beyond a floor stops looking like content, as its author found
on 2026-11-11 when a Map showed only a dark grid and a line across it. The
grid beyond the floor shows only while the floor is being resized. The zone
origin no longer draws orange lines across the whole view, since orange
means running now. When no part of the floor is in view, the Map says which
way it is and that Fit brings it back. It builds on
[increment 41](0041-preview-opens-nothing.md), and is held to the same
strict checks as [increment 1](0001-foundation.md).

## What it proves

1. **The stage is plain beyond the floor:** its grid of cells shows only
   while Resize floor is the tool in hand.
2. **The origin is marked without orange:** cell 0,0 has an ink outline,
   with a page-tone line inside it so it shows on the stage as on the
   floor, and its label. No lines run across the view.
3. **A floor out of view is said:** a tag in the middle of the canvas
   reads "The floor is out of view, to the left. Fit brings it back."
4. **Nothing else behaves differently:** the real-window walks pass.

## Decisions

Proposed on 2026-11-11 after its author asked what a Map was showing, and
built when he asked for all the remaining work to be finished that
evening:

- **A colour has one job,** so the origin's orange went. See
  [the editor features](../EDITOR_FEATURES.md#principles).
- **Explicit over clever:** the tag only says where the floor is; it does
  not move the view.

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. A plain stage | The grid beyond the floor only with Resize floor | The Map's tests pass | Built |
| 2. The origin | An ink outline and label, no axes | The Map's tests pass | Built |
| 3. Out of view | The tag, and "state:floor-off-view" for input scripts | A unit test | Built |
| 4. Checks | `make check` and the real-window walks | All pass | Built |

## Step results

- **Out of view:** `TestTheMapSaysWhenTheFloorIsOutOfView` pans the floor
  wholly out of view, finds the Map saying so, and Fit bringing it back.
- **Checks:** `make check` passes. The real-window suite was not run on
  this increment: the computer's display slept while its author was out.
  A screenshot of a folded Map, taken with increment 43, shows the plain
  stage and the origin without its lines.
- **Noticed while building, and fixed:** Watch's log called a line's node
  "a deleted node" whenever the line came from a workflow Watch was not
  showing, since it had never loaded that workflow. Watch now loads the
  workflow of every logged line, and names the workflow alone until it has
  arrived. `TestALoggedLineNamesItsNode` checks it.
