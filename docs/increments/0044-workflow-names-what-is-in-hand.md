# Increment 44: A Workflow names what is in hand

**Status:** Built

A Workflow's status bar now says what is in hand, as a Map's has since
[increment 39](0039-readable-history.md): the workflow, what is selected,
and what the pointer is over. Over empty canvas it says that Tab adds a
node there. This follows "a history that can be read" in
[the editor features](../EDITOR_FEATURES.md#principles): "the status bar
always names the tool in hand and what the pointer is over". It builds on
[increment 43](0043-panels-fold.md), and is held to the same strict checks
as [increment 1](0001-foundation.md).

## What it proves

1. **The status bar names what is in hand:** "Clerk's day · Start selected
   · over Wait", "… · 3 nodes selected", or "… · a line selected".
2. **It says what the pointer is over:**
   - a node by its kind;
   - a line by where it comes from, as "over the line from Start · next";
   - on empty canvas, "Tab adds a node here".

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. What is in hand | The line, set once each frame has moved what it names | A unit test | Built |
| 2. Checks | `make check` | Passes | Built |

## Step results

- `TestTheStatusBarNamesWhatIsInHand` selects the start and points at a
  Wait, then at empty canvas, and reads each.
- `make check` passes. All 66 real-window walks pass on the increment's
  commit, run from a frozen copy.

## Not in this increment

The Pixel editor does not yet name its tool in hand.
