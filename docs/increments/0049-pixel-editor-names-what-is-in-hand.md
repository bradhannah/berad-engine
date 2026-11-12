# Increment 49: The Pixel editor names what is in hand

**Status:** Built

The Pixel editor's status bar now says what is in hand, as a Map's has
since [increment 39](0039-readable-history.md) and a Workflow's since
[increment 44](0044-workflow-names-what-is-in-hand.md). This follows "a
history that can be read" in
[the editor features](../EDITOR_FEATURES.md#principles): "the status bar
always names the tool in hand and what the pointer is over". It builds on
[increment 48](0048-inspector-writes-links.md), and is held to the same
strict checks as [increment 1](0001-foundation.md).

## What it proves

1. **The status bar names what is in hand:** the sprite, the tool, and the
   colour the Pencil or Fill paints with, as "Lantern · Pencil · gloomy /
   grey 3". The Eraser and Eyedropper name no colour.
2. **It says what the pointer is over:** the pixel and its colour, as
   "pixel 1, 1: gloomy / grey 3".

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. What is in hand | The line, set once each frame has moved what it names | A unit test | Built |
| 2. Checks | `make check` and the real-window walks | All pass | Built |

## Step results

- `TestTheStatusBarNamesWhatIsInHand` paints a pixel grey 3 with the
  Pencil, points at it and reads both parts, then takes the Eraser and
  reads that it names no colour.
- A screenshot from `TestPixelEditorDrawsToATemplate` shows "Lantern ·
  Pencil · gloomy / iron 3 · pixel 4, 12: gloomy / iron 3" at the right of
  the status bar. The canvas's own readout under the tile says the same
  pixel; the status bar says it wherever the panels are.
- **Checks:** `make check` passes. All 70 real-window walks pass on the
  increment's code commit, run from a frozen copy.
