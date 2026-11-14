# Increment 56: Native resolution and a scale for text and controls

**Status:** Built

The editor now draws at the screen's own resolution and offers a scale for
text and controls, which changes nothing else, as agreed on 2026-11-11 in
"every panel folds, when told to" in
[the editor features](../EDITOR_FEATURES.md#principles). Until now the
window's Layout answered in points, so on a screen of two pixels to a point
the editor drew at half the screen's resolution and was enlarged, and its
text was soft. It follows [increment 55](0055-reopens-as-left.md), and is
held to the same strict checks as [increment 1](0001-foundation.md).

## What it proves

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
  at the right of the status bar, between Inspector and On top, opens the
  list of steps, 75, 90, 100, 110, 125, 150, 175 and 200%; Cmd+Shift+=
  makes text and controls larger, Cmd+Shift+- smaller and Cmd+Shift+0 puts
  them back at 100%. Each change is said in the status bar. Cmd+= and Cmd+-
  stay the canvases' zoom.
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
  helpers, the theme's in `editor/ui` and a small package, `editor/ui/pen`,
  which turn points into the screen's pixels: about 100 direct uses of
  Ebitengine's vector drawing, 24 of clipping and 15 of drawing pictures,
  in `editor/ui` and the views, move onto them with the same arguments.
  The theme draws text with its faces opened at their size times the
  scale, and measures it in points. The window's Layout answers in the
  screen's pixels; each pane's own picture is the size of its part of the
  window in pixels.
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

Chosen while building, for its author to check:

- **The current step is marked "· now"** in the Text button's menu, and a
  change is said as "Text and controls at 125%: Cmd+Shift+0 puts them back
  at 100%". At 150% in a narrow window the status bar cuts that sentence
  short; History keeps it whole.
- **Text is measured at the scale it is drawn at,** in points, so a word
  fits the room it is drawn in. A line drawn without smoothing, such as a
  hairline, is a whole number of the screen's pixels, at least one, so lines
  of one width look alike at 125% or 150%; at one pixel to a point and 100%
  every line is as wide as before.
- **What floats over a canvas, or is a message on it, grows:** besides the
  zoom card, the Map's "out of view" tag and its "no floors yet" and
  "Loading the floor" messages. What belongs to the canvas does not: the
  pick tip beside an anchor, a preview's speech bubbles and box, Play's
  speech and box, and a template's guides and their names.
- **A sprite's picture in the Project window grows with its card,** fitted
  to whole pixels of the screen, as the Pixel editor's and the Map's sprites
  are.
- **Play fills its tab at the screen's own pixels;** before, it filled it in
  points and was enlarged. The game's picture is as large as before and
  sharper.
- **A Workflow zoomed other than 100% is drawn as before,** at 100% and then
  enlarged or reduced, now at the screen's pixels to a point; at 100% it is
  drawn directly and sharp. Drawing it sharp at every zoom is left out.
- **The scale is kept as `textScale` in each screen's arrangement,** 1 for
  100%, without a new format version: an older editor reads the file and
  leaves the scale out.
- **The window's narrowest is its points times the scale,** in the screen's
  points, so at 150% a window needs half as much again to fit the same
  panels.
- **An input script's run may start at another scale,** given as
  `BERAD_TEXT_SCALE=150`, and then opens its window that much larger so its
  walks lay out as they do at 100%. Without input scripts it is not read.

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. The scale | One scale from the screen and the author; Layout in screen pixels; the window in points; text, fills and lines drawn at the scale; pointer positions in points | Unit tests; the walks at 100% | Built |
| 2. Shared drawing | Icons, widgets, clipping and pictures in `editor/ui` and the window drawn at the scale | `make check`; the walks at 100% | Built |
| 3. Each view | Each view's own drawing at the scale, and each canvas's content at the screen's own pixels, a view at a time | Its walks at 100%; screenshots at 150% | Built |
| 4. The setting | The scale chosen in the status bar and with keys, kept for each size of screen | A real-window walk | Built |
| 5. Checks | `make check` and the real-window walks | All pass | Built |

## Step results

- **The scale:** `editor/ui/pen` holds the scale and draws in points. Its
  tests check that one pixel to a point at 100% changes nothing; that a
  point takes the device's scale times the author's; that rectangles which
  meet in points still meet in pixels at 125%; that lines are whole pixels;
  that a canvas draws without the author's scale and the window's comes
  back after it; and that a layer is the pixels under its rectangle.
  `TestAFieldAnswersInPointsAtAnyScale` clicks a wrapped field's second line
  at 100%, 200%, 150% and 250% and finds the caret on the same letter, drawn
  in the same place, and the field as high; `TestTextIsMeasuredInPoints`
  measures a sentence within a point of its width at 100%.
- **Shared drawing:** the theme's text, fills, outlines, rounded boxes and
  lines, the icons, the colour picker, sliders, dialogs, menus and every
  clipped list draw through the pen; the window's panes, quick open's sheet,
  a split's status bar and a folded inspector's wide view are layers of the
  screen's pixels.
- **Each view:** the Project window, the status bar and History, the Clock,
  World state, the Palette Editor, Audio and the zone picker needed only
  the pen. The Map draws its floor on a canvas: at 150%,
  `TestTheScaleLeavesTheMapsCellsAlone` finds the zoom still 100%, three
  cells two thirds as many of the window's points, and a drag painting the
  cells under it. A Workflow's and Watch's canvas likewise:
  `TestTheScaleLeavesTheWorkflowsZoomAlone` drags a node 40 by 20 of the
  window's points at 150% and finds it moved 60 by 30 on the canvas, with
  the zoom at 100%. In the Pixel editor,
  `TestTheScaleLeavesTheSpritesPixelsAlone` finds four pixels at 40× four
  fifths as many of the window's points at 125%, and the pencil painting
  the pixels dragged over. The Template editor's preview and Play's game
  are drawn at whole pixels of the screen.
- **The setting:** `TestTheTextButtonOffersTheSteps` opens the Text button
  and chooses 125%; `TestStepsStopAtTheEnds` steps from 75% to 200% and no
  further; `TestTheTextScaleIsChosenAndKept` and
  `TestTheTextScaleIsPutBack` keep the scale for this size of screen, say
  the change in the status bar, and put it back for this screen and not
  another's. In the real window, `TestTextAndControlsScale` presses
  Cmd+Shift+= three times to 150%, chooses 125% from the Text button,
  quits, opens the project again and finds 125%, then Cmd+Shift+0 puts it
  back at 100%; the Map's zoom stays 100% throughout.
- **Seen in the real window,** on a screen of two pixels to a point: every
  screenshot of the walks is at the screen's own pixels, 2400 × 1664 for a
  window of 1200 × 832 points. Side by side with the same Map's inspector
  from `main`, enlarged as the screen enlarges it, the text and outlines are
  sharp where `main`'s are soft, and nothing has moved. At 150% the text,
  controls and panels are half as large again while the Map's cells, marks
  and names, a Workflow's nodes, a sprite's pixels and the game in Play keep
  their size.
- **Seen at 150% in a small window:** a window of 1200 × 832 of the
  screen's points is 800 × 555 of the window's at 150%, and there several
  views' layouts do not fit, as they would not at 100% in a window that
  small: the Project window's S, M and L buttons sit over its folder tree;
  World state's and the Palette Editor's save bars cover the fields above
  them; the Palette Editor's buttons and its gradient panel overlap; the
  Pixel editor's tools run into its zoom buttons; the Map's Preview section
  covers its tool's help; Audio's buttons cover its title; and a Map in a
  split tab has its panels fill its pane, with no canvas left.
- **Not seen:** a screen of one pixel to a point. There, at 100%, every
  drawing helper takes the same arithmetic as before, and the unit tests,
  which draw at one pixel to a point, pass as before; the only change on
  screen is the status bar's Text button.
- **Checks:** `make check` passes. All 75 real-window walks pass at 100%
  on the night's last commit of code, run from a frozen copy in 15 minutes.
  Run with `BERAD_TEXT_SCALE=150` on the commit before Play's fix, each
  window opened half as large again, 73 pass, and Play's two walks pass at
  150% after it; the two that do not expect what only 100% gives:
  `TestTextAndControlsScale` starts by checking 100%, and
  `TestMapPaintsDrawsAndUndoes` expects Fit then one step in to give 100%,
  where the larger canvas fits the floor at a higher zoom.

## Changed after the whole suite

- **Play's game picture is drawn in its own pixels.** The screenshots
  showed the player's disc and the cells without art drawn at the window's
  scale into the game's picture, which is in the game's pixels; that
  picture is now drawn at one pixel to a point and then placed on the
  screen.
- **A screenshot step waits for its screenshot.** Saving a screenshot of
  the full resolution makes that frame slow, and Ebitengine catches up with
  several updates before it draws again: `TestSpriteShowsOnTheMap`'s script
  went on and closed the Map before its last screenshot was drawn. An input
  script now waits for the screenshot it asked for to be drawn.

## Not in this increment

- Views that fit a small window: the overlaps seen at 150% in a window of
  800 × 555 of the window's points, above.
- A Workflow drawn sharp at zooms other than 100%.
- Text and controls on a screen of one pixel to a point, seen in the real
  window.
