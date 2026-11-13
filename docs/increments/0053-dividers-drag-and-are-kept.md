# Increment 53: Dividers drag, and widths and folds are kept

**Status:** Built

A panel's divider now drags, and how panels are arranged is kept for this
person between runs, as agreed on 2026-11-11 in "every panel folds, when
told to" in [the editor features](../EDITOR_FEATURES.md#principles):
"Dividers drag, and a double-click puts one back. A panel's width and fold
belong to the kind of view … they are kept per person, never in the
project. … What will not fit is refused with the reason and the keys that
make room." [Increment 43](0043-panels-fold.md) made panels fold per kind.
It builds on [increment 52](0052-split-has-one-status-bar.md), and is held
to the same strict checks as [increment 1](0001-foundation.md).

## What it proves

1. **Dividers drag:** the line between the canvas and the inspector, in
   every view, and between a Map's or a Workflow's left panel and its
   canvas. The pointer shows it drags while over one.
2. **A double-click puts one back** at the panel's own width.
3. **A width belongs to the kind of view:** dragging a Map's inspector
   wider makes every Map's wider, and leaves Workflows alone.
4. **The canvas keeps its room:** a panel dragged into it stops, and the
   status bar refuses with the reason and the keys that make room: "The
   canvas keeps the rest of the window: Cmd+I folds the inspector and
   Cmd+Shift+I the left panel, to make room".
5. **Widths and folds are kept per person:** in the project's own state
   folder, `.berad/layout.json`, which its `.gitignore` keeps out of
   version control, for each size of screen. Opening the project again puts
   them back.

## Decisions

Chosen while building, for its author to check:

- **The narrowest panels:** an inspector 220 pixels, a left panel 140; the
  canvas keeps 240.
- **The grip is eight pixels wide,** centred on the line, taken from the
  panel and the canvas either side.
- **The arrangement is kept as it changes,** each time a drag ends or a
  panel folds, rather than when the editor quits, so a crash loses none.
- **A screen is known by its size,** as "1512x982", so a laptop's screen and
  a monitor keep their own arrangements.

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. Widths | The inspector's width set per kind for each view; the Map's and the Workflow's left panels' own widths | Unit tests pass as before | Built |
| 2. Dividers | Dragging, putting back, the canvas's room and its refusal; every view of a kind following | A unit test | Built |
| 3. Kept | The arrangement saved per person for each size of screen, and put back | Unit tests; a real-window walk | Built |
| 4. Checks | `make check` and the real-window walks | All pass | Built |

## Step results

- **Dividers:** `TestDividersDrag` drags a Map's inspector to 380, which
  the other Map takes, its left panel to 260 for both, then the inspector
  into the canvas's room, which stops at its edge and refuses saying
  Cmd+Shift+I; a double-click puts the inspector back for both.
- **Kept:** `TestTheArrangementIsKept` puts back a Map's widths and fold
  for this size of screen and not another's.
  `TestTheArrangementIsKeptPerPerson` finds the arrangement saved in
  `.berad/layout.json` and given back after the project opens again.
- In the real window, `TestDividersDragAndAreKept` drags a Map's left panel
  to 260 and its inspector to 500, quits, opens the project again, and
  finds both as they were left; its screenshot shows the wider panels.
- **Checks:** `make check` passes. All 74 real-window walks pass on the night's last commit, which carries
  this one, run from a frozen copy.

## Changed after the whole suite

The whole real-window suite found a fold undone when panels were folded
quickly: the orchestrator echoes the arrangement to the other panes, and an
older echo arriving after a newer fold put the older fold back. The window
now takes the kept arrangement only as it opens; what arrives after is its
own, echoed.
