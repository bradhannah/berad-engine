# Increment 46: Watch keeps a trail

**Status:** Built

Watch now keeps a trail of the watched NPC, as agreed on 2026-11-11 in
[the editor features](../EDITOR_FEATURES.md#preview-testing-and-validation)
("Watch keeps a trail"). Each node the NPC passed is a line, with the game
time, the node it left and the one it reached, and what the node read or
did that sent it on. A restart caused by moving the clock is a line too,
with what it cleared. The Log, the Trail and the Values share one strip
under the workflow. It builds on
[increment 45](0045-history-lines-select.md), and is held to the same
strict checks as [increment 1](0001-foundation.md).

## What it proves

1. **Each change of node says why:**
   - a time switch and a time-bound container say the time they read and
     the time that held, as "Read 9:15: 9:00 to 12:00 holds", or that none
     did;
   - leaving a container when its time is up says so;
   - a check value says what it read, as "Read money: 12";
   - a random choice says what it rolled;
   - a Wait says how long it waited, and a Move how far it moved;
   - a blocked Move says where something was in its way;
   - a Go to says where it arrived;
   - a Find NPC nearby says whom it found and how far away, or that it
     found no one.
2. **A restart says what started it and what it cleared:** "Started again:
   the clock moved to 12:00, clearing what it was doing". An edit to the
   workflow, and a workflow that is gone, say so in the same way.
3. **The preview keeps the trail:** the watched NPC's latest changes,
   numbered, each at the game time it was made. Watching another NPC starts
   it again.
4. **One strip, three parts:** Log, Trail and Values are tabs at the left
   of the strip under the workflow. The workflow takes the window's width,
   where the values stood beside it.

## Decisions

Chosen while building, for its author to check:

- **The Log shows first,** as it did before; the Trail and the Values each
  wait for their tab.
- **A line of the trail is the node left and the node reached,** written
  as the footer's way is, then why in the muted ink. A change in a workflow
  other than the one shown says "a node in another workflow".
- **The values sit in as many columns as fit** across the strip, each as
  wide as the column beside the workflow was. When they do not all fit,
  the wheel scrolls them a row at a time, and the strip's header says
  which show, as "1–6 of 11 · scroll for more".
- **Watch keeps the latest 500 changes,** and a preview's state carries the
  latest 50, so none is missed between two states.
- **Nodes that do their work at once,** such as Set value and Log, give no
  reason: they always go on by their one output.

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. Why | Each change's reason in the NPC runner, and a restart's | Unit tests | Built |
| 2. The trail | The preview keeping the watched NPC's changes, numbered and timed | A unit test | Built |
| 3. One strip | Log, Trail and Values as tabs of one strip; the trail's lines | A unit test; a real-window walk | Built |
| 4. Checks | `make check` and the real-window walks | All pass | Built |

## Step results

- **Why:**
  - `TestEachChangeSaysWhy` reads a time switch outside its times, a jump
    to 10:00 that starts the NPC again and clears its Wait, the switch and
    the container holding, and the container's time running out.
  - `TestAMoveSaysWhatWasInItsWay` finds a Move blocked by the wall at
    5, 2, and a Wait of 2 seconds.
- **The trail:** `TestThePreviewKeepsTheWatchedNPCsTrail` watches the
  clerk, jumps to 12:00, and finds "Started again: the clock moved to
  12:00, clearing what it was doing" and "Moved 1 tile right", numbered
  and at the game time. Watching another NPC empties it.
- **One strip:** `TestTheStripShowsTheLogTheTrailOrTheValues` finds the
  Log first, each change kept once, the Trail and Values from their tabs,
  the Log's own buttons gone while the trail shows, and the trail started
  again for another NPC. `TestTheValuesScrollWhenTheyDoNotFit` scrolls 40
  values a row with the wheel.
- In the real window, `TestWatchKeepsATrail` watches the sample's clerk
  and moves the clock on an hour: the trail reads "Started again: the
  clock moved to 9:02, clearing what it was doing", then each node to the
  Say, with what the time switch and the container read. Its screenshots
  show the trail and the values, "1–6 of 9 · scroll for more".
- **Checks:** `make check` passes, and so do the walks that read Watch's
  log, `TestLoggingInWatch` and `TestWatchOpensBesideItsMap`. All 70
  real-window walks pass on
  [increment 49](0049-pixel-editor-names-what-is-in-hand.md)'s commit,
  which carries this one, run from a frozen copy.

## Not in this increment

The rest of what was agreed for previews comes in increments of its own:

- a zone or a behaviour shown by its name in Watch, and a split with one
  status bar;
- **Play from here**.
