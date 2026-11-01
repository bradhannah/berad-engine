# Increment 17: Reading the graph

**Status:** Planned

The seventeenth increment makes a workflow easier to read and to tidy on
the canvas, ahead of the containers work in [the workflow
design](../WORKFLOW_DESIGN.md#containers). Every leg of a line can be moved,
not only one; the status bar names the line under the pointer and a legend
says what the line looks mean; nodes, containers, outcomes and lines can be
coloured from a fixed set of tints, a line taking its outcome's colour
unless given its own; and a long line repeats its outcome's name at its
arrowhead. All of it is layout: nothing in it changes what the engine runs.
It builds on [increment 3](0003-npc-workflows.md)'s canvas and its lines
moved by hand, and on the agreed directions in [editor
features](../EDITOR_FEATURES.md#workflows), and is held to the same strict
checks as [increment 1](0001-foundation.md).

## What it proves

1. **Every leg of a line moves:** a back line has a handle on its drop leg,
   its level run and its end leg, and a forward line on its upright part.
   Each drag saves one offset, measured from the line's own node, so the
   line keeps its shape when its nodes move; Reset line clears them. There
   are no free waypoints: lines stay level and upright.
2. **The canvas says what a line is:** hovering a line names it in the
   status bar, with where it leaves, by which outcome, where it enters,
   whether it loops back and whether it was moved by hand. A Legend toggle
   in the toolbar explains the looks.
3. **Colour organises without meaning:** seven fixed tints, none blue and
   none red. A node, a container or an outcome can be given one. A line's
   colour is unset, from its outcome, or set by hand, and the inspector
   says which; selected and live always win over it. Colours are saved
   with the layout, and the engine ignores them.
4. **A long line says where it came from:** its outcome's name is drawn
   again near its arrowhead when the line is long; a short line, whose
   outcome row is in view, shows nothing more.

## Decisions

Asked for by its author on 2026-11-01, paraphrased: colours on nodes,
containers and lines, to keep a workflow organised; lines that move left
and right, not only up and down, since some could be moved and some could
not; a coloured outcome such as a check's "yes" setting the default colour
of the line that leaves it, which the line can override; and a legend, or
something, for what dashed and solid mean.

Agreed by its author on 2026-11-01, before building, from the wireframes on
the canvas "Workflow canvas wireframes":

- **Every interior leg moves, and nothing else:** no right-click to add a
  point and no free waypoints, so lines stay level and upright, each offset
  stays one saved number, and auto-arrange and Reset line still apply. An
  Add bend menu item that splits a leg in three stays a candidate until a
  real workflow needs it.
- **Seven fixed tints, declared nowhere in the project.** A project-wide
  set of named colours was judged overbuilt. No blue, which is selection,
  and no red, which is problems.
- **A line's colour has three states,** unset, from its outcome and set by
  hand, and the inspector shows which, with a dashed swatch for "from its
  outcome"; otherwise recolouring an outcome would seem to change some
  lines and not others for no reason.
- **The status bar names a hovered line,** and the legend is a toggle in
  the toolbar rather than always in view.
- **The outcome's name again at the arrowhead** of a long line, in the
  line's colour, small and muted like the free-output labels.

Chosen while planning, for its author to check:

- **The offsets:** a back line's drop leg uses the `Lane` a bend already
  has, which only forward lines used; its end leg gets a new field,
  measured from the entry it goes into, so the line keeps its shape when
  either node moves. Layout format version 3 adds it and the colours, and
  reads version 2 unchanged, as version 2 read version 1.
- **Where colours live:** in the layout, as a tint's name, never a colour
  value, so the editor's palette can change later: one per node or
  container by ID, one per outcome by the line key bends use, `node/output`,
  and one per line by the same key. A line with no entry takes its
  outcome's; a line may say "none" to override an outcome's tint with no
  colour.
- **The tints:** green, amber, teal, purple, pink, brown and slate, the
  editor's existing family hues (zones, art, workflows, sounds, texts and
  NPCs) plus amber, each a wash for a node's face and a darker ink for a
  line, its port and an outcome's row. A tinted outcome's port is filled
  with its ink, so the colour is visible at both ends of the line.
- **What wins:** a lit line (blue) and a live one (orange) over any tint; a
  tint over the grey of a time switch's otherwise, which is only the look
  of an uncoloured one; dashed for a loop whatever the colour.
- **The far-end label** appears on lines of 200 canvas pixels or more,
  before the arrowhead and clear of the line, in the line's ink.
- **The status bar's words:** "Wait · 30 min → Is serving? · loops back ·
  moved by hand", the nodes named as the inspector names them.

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. Handles on every leg | Grips on a back line's drop, level run and end legs; the end-leg offset in `Bend`; layout format version 3; dragging each, and Reset line clearing them | Route tests for each handle and for a line whose nodes then move; the bend edit passes the shared apply-then-undo test; version 2 layouts load | Built |
| 2. Naming and the legend | The status bar's line description on hover; the Legend toggle and its panel | View tests of the text for a forward line, a loop and a moved line, and of the toggle | Planned |
| 3. Colours | Tints in the layout; Colour rows in the inspector for a node, a container, each outcome of a node with several, and a line, with the three line states; the drawing of tinted faces, ports, rows and lines | The colour edit passes the shared apply-then-undo test; view tests of inherit, set and none, and of lit and live winning | Planned |
| 4. The outcome's name at the arrowhead | The label on long lines, in the line's ink | A view test of a long and a short line | Planned |
| 5. Checks | `make check` and the real-window suite; a real-window test drags a back line's end leg and colours an outcome, and both are saved | The checks pass with no lint findings | Planned |

## Step results

- **Handles on every leg:** `Bend` gains `End`, the end leg's offset from
  the entry it goes into, as layout format version 3, which reads version 2
  unchanged. A back line's drop leg takes the `Lane` a bend already had,
  its run `Depth` and its end leg `End`; each leg of a hovered or selected
  line draws a grip, and dragging one sets its offset alone, keeping the
  others. Reset line and auto-arrange clear all three. Verified: route
  tests move each leg by its own offset and keep the offsets when a node
  moves; the move edit with an end leg passes the shared apply-then-undo
  test; a version 2 layout upgrades and its bend reads as it was; view
  tests drag the drop and end legs, see the run span them, and reset them.
  `make check` passes with no lint findings.

## Changed after its author tried it

Not yet tried.

## Not in this increment

- **Working with many nodes:** multi-select, moving and deleting a group,
  duplicating and copying between workflows, nudging and arranging a
  selection, and dropping a node onto a line. The next canvas increment,
  agreed alongside this one.
- **Containers:** named exits, lines kept inside and collapsing, as [the
  workflow design](../WORKFLOW_DESIGN.md#containers) orders them.
- **Add bend,** a candidate until a real workflow needs it.
- **Reusable blocks of text** in the project view, a candidate whose
  purpose is open.
