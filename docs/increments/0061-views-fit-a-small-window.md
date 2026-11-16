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
