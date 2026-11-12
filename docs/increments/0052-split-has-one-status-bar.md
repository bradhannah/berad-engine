# Increment 52: A split has one status bar

**Status:** Built

A tab split in two, a Map with Watch beside it, now has one status bar,
across the whole tab, as agreed on 2026-11-11 in
[the editor features](../EDITOR_FEATURES.md#preview-testing-and-validation):
"a split has one status bar". The same entry asks that Watch show a zone or
a behaviour by its name, which it already does: its Values read
"npc.zone Sample zone" and "npc.behaviour Clerk's day", and its header
names the workflows of the path. It builds on
[increment 51](0051-views-say-their-changes.md), and is held to the same
strict checks as [increment 1](0001-foundation.md).

## What it proves

1. **One status bar:** the bar spans the tab under both panes, and shows
   the focused pane's message, what is in hand, History and its toggles.
2. **Each pane keeps its own:** its messages, History and refusals stay
   its own, and show when it has the focus.
3. **The divider and the focus outline stop above the bar.**

## Decisions

Chosen while building, for its author to check:

- **A press in the bar under the other pane focuses that pane first,** as
  any press in a pane does, and then acts on its bar, which is now the
  one shown. So the bar never acts for a pane it does not show.
- **The panes keep the room they had for their own bars,** so nothing in
  them moves when a tab is split or joined.

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. One bar | The focused pane's bar across the tab, its input in the tab's coordinates, and input scripts finding its parts | A unit test; real-window walks | Built |
| 2. Checks | `make check` and the real-window walks | All pass | Built |

## Step results

- `TestASplitHasOneStatusBar` lays out a split tab and a tab alone: both
  panes of the split have the tab's area for the bar, the right pane's
  starting at the tab's left edge, and the pane alone has its own.
- In the real window, `TestWatchKeepsATrail`, `TestLoggingInWatch` and
  `TestWatchOpensBesideItsMap` pass with Watch beside the Map; a screenshot
  shows one bar under both, saying Watch's message.
- **Checks:** `make check` passes. The whole real-window suite runs again
  on the last increment.
