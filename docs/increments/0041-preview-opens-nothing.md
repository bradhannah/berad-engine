# Increment 41: Starting a preview opens nothing

**Status:** Built

A preview no longer moves the window, as agreed on 2026-11-11 in
[the editor features](../EDITOR_FEATURES.md#preview-testing-and-validation)
("starting a preview opens nothing"). The clock's panel stays closed until
its readout is pressed, and the clock's commonest controls sit in the window
bar. Watch follows the node being run only while its Follow switch is on,
and panning by hand turns Follow off. It builds on
[increment 40](0040-lines-do-what-was-done.md), and is held to the same
strict checks as [increment 1](0001-foundation.md).

## What it proves

1. **A preview starting opens nothing:** the clock's panel opens only when
   its readout is pressed.
2. **The clock's controls are on the bar:** −1h and +1h move the preview's
   clock, or the next start when none runs, and Stop or Start stops and
   starts the preview's clock. Stop is pale while no preview runs.
3. **Follow is the author's to turn on:** Watch's Follow switch is off at
   first. While on, on an orange wash, it brings the node being run into
   view each time it moves on. Panning the workflow by hand turns it off,
   and the status bar says so.
4. **Nothing else behaves differently:** the real-window walks pass.

## Decisions

Agreed by its author on 2026-11-11 (see the editor features, "starting a
preview opens nothing" and "explicit over clever"):

- **The clock's hour steps and pause sit in the window bar** beside its
  readout, and no panel opens over the inspector.
- **Watch has a Follow switch,** orange while on, that keeps the node being
  run in view; panning by hand turns it off, and the switch shows that.

Chosen while building, for its author to check:

- **The bar holds −1h, +1h and Stop.** The rate and the jump to a time stay
  in the panel the readout opens. Pause NPCs and Step stay in Watch.
- **Follow starts off** each time Watch opens, and brings a node into view
  when the NPC moves to it, not every frame. So the author can still look
  around while the NPC waits.
- **Zooming does not turn Follow off;** only panning, by the wheel or by
  dragging, does.

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. Nothing opens | The clock's panel opening only from its readout | A unit test | Built |
| 2. The clock on the bar | −1h, +1h and Stop beside the readout, carried out by the Clock view | A unit test of the bar | Built |
| 3. Follow | The switch, its orange wash, bringing the node into view, and panning turning it off | A unit test of Watch | Built |
| 4. Checks | `make check` and the real-window walks | All pass | Built |

## Step results

- **Nothing opens and the clock on the bar:**
  `TestStartingAPreviewOpensNothing` starts a preview and finds the panel
  closed until the readout is pressed. +1h works with no preview, Stop only
  while one runs, and −1h while it does.
- **Follow:** `TestFollowIsTheAuthorsToTurnOn` finds Follow off, turns it
  on, and pans with the wheel. That turns it off with "Follow is off: you
  moved the view."
- **The real-window walks:** `TestClockIsPinned` still moves the hidden
  Map's preview on an hour, with the panel closed.
- **Checks:** `make check` passes. All 66 real-window walks pass on the
  increment's code commit, run from a frozen copy. Not checked by eye: the
  bar's clock controls and Watch's Follow in a screenshot.

## Not in this increment

The rest of what was agreed for previews comes in increments of its own:
- Watch's **trail**: each node the watched NPC passed, what it did and why;
- a **restart caused by a change to the clock** written in the trail;
- **Values, Log and Path sharing one strip**, and a split with one status
  bar;
- **Play from here**.
