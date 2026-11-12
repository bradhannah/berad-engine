# Increment 55: The window reopens as it was left

**Status:** Built

Opening a project again now puts the editor window back as it was left, as
agreed on 2026-11-11 in "every panel folds, when told to" in
[the editor features](../EDITOR_FEATURES.md#principles): "The window
reopens as it was left, its frame, tabs and what sits beside what, and says
what could not be found." It builds on
[increment 54](0054-arrangements-by-screen-and-mode.md), and is held to the
same strict checks as [increment 1](0001-foundation.md).

## What it proves

1. **The frame:** where the window was on the screen and its size, kept
   for each size of screen.
2. **The tabs:** each tab reopens in order, what sat beside what sits
   beside it again with the divider where it was, and the tab that was in
   front is in front.
3. **What could not be found is said:** a tab whose document is no longer
   in the project is left out, and the status bar says "Reopened as it was
   left. Could not find a Workflow: each is no longer in the project".
4. **Quitting keeps the tabs as they were:** the panes closing as the
   editor quits do not change what reopens; closing a tab by hand does.

## Decisions

Chosen while building, for its author to check:

- **Play is not reopened,** as opening it launches the game. The Project
  window is always open, so it is not reopened either.
- **The frame is kept once it has stayed put half a second,** so a window
  being dragged is not kept at every step.
- **An input script's run does not move or size the window, and starts at
  the views' own widths,** so the real-window walks do not depend on the
  screen they run on or on where the window was left. Increment 54's
  starting widths now leave scripted runs alone.

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. Kept | The frame, the tabs, the splits and the tab in front kept for each size of screen, but not while quitting | A unit test | Built |
| 2. Reopened | The frame put back, the tabs reopened in order beside what they were beside, and what could not be found said | A unit test; a real-window walk | Built |
| 3. Checks | `make check` and the real-window walks | All pass | Built |

## Step results

- `TestTheWindowReopensAsItWasLeft` plans a Map with Watch beside it, a
  Workflow and Play, leaving out the Project window and Play; follows the
  Map and Watch as they open and the Workflow as it is not found; and puts
  back the split's share and the tab in front, saying the Workflow could not
  be found.
- In the real window, `TestTheWindowReopensAsItWasLeft` opens the sample
  zone's Map and the Fetch stock workflow, quits, opens the project again,
  and finds both tabs back with "Reopened as it was left"; its screenshot
  shows them.
- **Checks:** `make check` passes. All 74 real-window walks pass on the night's last commit, which carries
  this one, run from a frozen copy.
