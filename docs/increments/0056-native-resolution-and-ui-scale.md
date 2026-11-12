# Increment 56: Native resolution and a scale for text and controls

**Status:** Planned

The editor is to draw at the screen's own resolution and offer a scale for
text and controls, which changes nothing else, as agreed on 2026-11-11 in
"every panel folds, when told to" in
[the editor features](../EDITOR_FEATURES.md#principles). It follows
[increment 55](0055-reopens-as-left.md), and will be held to the same strict
checks as [increment 1](0001-foundation.md).

## Why it is planned, not built

The window's Layout answers the window's size in points, so on a screen of
two pixels to a point the editor draws at half the screen's resolution and
is scaled up, and its text is soft. Drawing at the screen's resolution
means drawing every text and every line at the screen's pixel size, and so
laying out every view in screen pixels. Text cannot be drawn small and
scaled up sharp.

Every size the editor lays out with is a fixed number today: about 2,400
uses of the shared sizes in `editor/ui` (the line height, a button's
height, the insets and gaps, the text sizes) and about 700 more sizes of
the views' own, such as a node's width and the inspector's. Each of those
becomes a size times the scale. That reaches every view and every
real-window walk's points. It is planned as an increment of its own, to be
built and checked a view at a time, with screenshots at each scale, rather
than all at once.

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

## The approach proposed

- **One scale:** the screen's pixels to a point times the author's scale,
  read once a frame. The window's Layout answers the window's size in
  screen pixels.
- **Sizes in points:** the shared sizes in `editor/ui` become functions of
  the scale, as `ui.LineHeight()`, keeping their names. The theme opens its
  faces at each size times the scale, and opens them again when the scale
  changes.
- **A view at a time:** each view's own sizes move to points, in the order
  Project, Map, Workflow, Watch, the Pixel editor, Palette Editor, World
  state, Audio, Template editor, Play. Each step is checked with its
  real-window walks at a scale of 100% and with screenshots at 150%.
- **Input:** pointer positions arrive in screen pixels, as the layout does.
  Input scripts keep naming parts, so the walks need no points.
- **The canvas's own zoom stays as it is:** a Map's cells and a Workflow's
  nodes keep their zoom, which the scale does not change.

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. The scale | One scale from the screen and the author, Layout in screen pixels, the theme's faces at the scale | Unit tests; the walks at 100% | Planned |
| 2. Shared sizes | `editor/ui`'s sizes as functions of the scale | `make check`; the walks at 100% | Planned |
| 3. Each view | Each view's own sizes in points, a view at a time | Its walks at 100%; screenshots at 150% | Planned |
| 4. The setting | The scale chosen in the window, kept per person | A real-window walk | Planned |
| 5. Checks | `make check` and the real-window walks | All pass | Planned |
