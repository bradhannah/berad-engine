# Increment 56: Native resolution and a scale for text and controls

**Status:** Planned

The editor is to draw at the screen's own resolution and offer a scale for
text and controls, which changes nothing else, as agreed on 2026-11-11 in
"every panel folds, when told to" in
[the editor features](../EDITOR_FEATURES.md#principles). It follows
[increment 55](0055-reopens-as-left.md), and will be held to the same strict
checks as [increment 1](0001-foundation.md).

## What it is to prove

1. **The screen's own resolution:** text and lines are drawn at the
   screen's pixel size, as sharp as other applications' on the same
   screen.
2. **A scale for text and controls:** set per person, kept with the
   window's arrangement, from 75% to 200%. It changes the size of text,
   controls and panels, and nothing else: no view's content, zoom or
   arrangement.
3. **Nothing else changes:** at a scale of 100% on a screen of one pixel to
   a point, every view draws as it does now.

## Decisions

Agreed by its author on 2026-11-13, before building:

- **The setting is in every status bar and on keys:** a "Text 100%" button
  at the right of the status bar, beside On top, opens the list of steps,
  75, 90, 100, 110, 125, 150, 175 and 200%; Cmd+Shift+= makes text and controls larger, Cmd+Shift+- smaller and
  Cmd+Shift+0 puts them back at 100%. Each change is said in the status
  bar. Cmd+= and Cmd+- stay the canvases' zoom.
- **The scale is kept for each size of screen,** with the panels' widths
  and the window's frame in `.berad/layout.json`, so a laptop and a monitor
  keep their own. A screen seen for the first time starts at 100%.
- **On a canvas, only what floats over it grows:** the zoom card, palettes,
  legends and menus. Everything drawn as part of a map or a workflow, its
  cells, marks, the names beside anchors and NPCs, nodes and their text,
  stays at the canvas's own zoom, so it never covers more of the map.

## The approach, as checked against the code

Planned first as laying out every view in screen pixels, with each of
about 2,400 uses of the shared sizes and 700 of the views' own becoming a
size times the scale. Checked against the code on 2026-11-13 before
building, the editor allows a far smaller change, so it is built this way
instead:

- **Layout stays in points.** Every view keeps laying out in points, as
  now; a point is the screen's pixels to a point times the author's scale.
  The shared sizes in `editor/ui` and the views' own stay as they are.
- **The scale is applied where drawing happens.** Text and fills, outlines,
  rounded boxes, lines, icons, clipping and pictures all go through a few
  helpers in `editor/ui`, which turn points into the screen's pixels: about
  60 direct uses of Ebitengine's vector drawing and 15 of drawing pictures
  in the views move onto them. The theme draws text with its faces opened
  at their size times the scale, and measures it in points. The window's
  Layout answers in the screen's pixels; each pane's own picture is the
  size of its part of the window in pixels.
- **Pointer positions arrive in points,** turned from pixels in the one
  place the window reads them, so views, input scripts and their
  `point:x,y` locators need no change.
- **A canvas draws its content at the screen's own pixels,** without the
  author's scale: a Map's cells, a sprite's pixels in the Pixel editor, a
  Workflow's nodes, Watch's nodes, the game in Play and a template's
  preview. A canvas's content is laid out in its own points, the window's
  points times the author's scale, so its zoom, its cell sizes and where it
  is panned to do not change.
- **What is kept stays in points:** panel widths in `layout.json` and the
  dividers' sizes are in points, so a panel grows with the scale; the
  window's frame, the screen's size that keys an arrangement, and
  increment 54's starting widths are in the screen's own points, which the
  scale does not change.

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. The scale | One scale from the screen and the author; Layout in screen pixels; the window in points; text, fills and lines drawn at the scale; pointer positions in points | Unit tests; the walks at 100% | Planned |
| 2. Shared drawing | Icons, widgets, clipping and pictures in `editor/ui` and the window drawn at the scale | `make check`; the walks at 100% | Planned |
| 3. Each view | Each view's own drawing at the scale, and each canvas's content at the screen's own pixels, a view at a time | Its walks at 100%; screenshots at 150% | Planned |
| 4. The setting | The scale chosen in the status bar and with keys, kept for each size of screen | A real-window walk | Planned |
| 5. Checks | `make check` and the real-window walks | All pass | Planned |
