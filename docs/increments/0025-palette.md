# Increment 25: Finding and placing nodes

**Status:** Planned

The twenty-fifth increment makes the Workflow window's palette say what
each node does before it is placed. Today a click on a palette entry adds
the node after the selected one, joined from its first free output, or
unjoined when that node already leads somewhere; and what a node does is
shown only once it is on the canvas, at the foot of its inspector. Now the
palette has a **search** that matches what a node does as well as its
name, a **hover** gives each entry's one line, a **click** shows a help
page in the inspector, and **dragging** is the only way to place a node,
onto a line to put it in the flow or onto empty canvas to place it free.
It builds on the palette of [increment 3](0003-npc-workflows.md), dropping
onto a line from [increment 17](0017-reading-the-graph.md) and the kinds of
[increment 23](0023-workflow-kinds.md), and is held to the same strict
checks as [increment 1](0001-foundation.md).

## What it proves

1. **A node can be found by what it does:** searching "sound" or "value"
   finds the nodes that play or read them, by name or by description.
2. **A node can be understood before it is placed:** its help page says
   what it does, its outputs and when each is taken, its settings, whether
   it takes time, which kinds of workflow offer it, and an example.
3. **Nothing is placed by accident:** only a drag places a node, and a
   drop onto a line joins it into the flow.
4. **Every node explains itself:** a node kind cannot be offered without
   its one line and its help, so the palette never shows a node it can't
   explain.

## Decisions

Asked for by its author on 2026-11-03: a way to see what a node does
without placing it; dragging, not clicking, to place; a click showing help
in the inspector; and a hover giving a line of help.

Agreed by its author on 2026-11-03, from the wireframes on the canvas
"Workflow canvas wireframes", row "The palette: search, help and dragging":

- **The palette has a search at its top,** Find a node, matching each
  node's name, its group and what it does. While searching, the groups
  fold away into one list of results, each with its group and its one
  line, the words that matched marked; Esc clears it. A result is dragged
  to place it, as an entry is.
- **Hovering an entry shows its one line** in a tooltip beside it, ending
  Click for more · drag to place.
- **Clicking an entry shows its help page in the inspector,** headed About
  a node, with a link back to the selected node: what it does, whether it
  takes time, how many outputs it has, which kinds of workflow offer it,
  each output and when it is taken, each setting in a line, and an
  example. The selected node stays selected underneath.
- **Add after, on the help page,** puts the node after the selected one as
  a drop onto its line would: the line ends at the new node, and the new
  node's first output goes on to the old target.
- **Only dragging places a node.** Dropping onto a line puts it in the
  flow; dropping onto empty canvas places it free, flagged until a line
  leads to it. While dragging, the node shows its outputs, and the line
  under it lights with Drop to put it between Wait and Go to.

Chosen while planning, for its author to check:

- **Each node's one line and its help are kept with the editor,** beside
  its label, not in the engine: they are words for an author, which a game
  never shows. A test fails when a kind the palette offers has neither.
- **The search also matches each setting's name,** so "duration" finds
  Wait; settings are listed on the help page anyway.
- **A help page's example is a few nodes drawn small,** in words, as
  Is npc.shop.serving true? yes → Go to · Counter, not a picture of a
  workflow.
- **The help page closes** when a node, a line or the canvas is clicked,
  or by its back link; the palette entry shown is marked until it does.
- **Up and Down move through the results, and Enter shows the help of the
  one marked,** so the keyboard reaches help; Add after places it from
  there without a mouse.
- **Locating for tests:** the search as "ui:palette-search", a result as
  "result:<kind>", the help page's parts as "ui:help-back" and
  "ui:help-add", and the states "results=<count>", "help=<kind>" and
  "tooltip=<kind>".

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. Help for every node | Each kind's one line and help page text: what it does, its outputs, its settings, an example; a check that every kind offered has them | A view test finds help for every kind of both kinds of workflow | Planned |
| 2. The search | The search at the palette's top, its results with group, one line and marked words, Esc, Up, Down and Enter | View tests find nodes by name, by description and by a setting, and clear the search | Planned |
| 3. Help and hover | A click showing the help page in the inspector, back to the selected node, Add after splicing like a drop; the tooltip on hover | View tests show a help page, go back, add after a node already leading somewhere, and read a tooltip | Planned |
| 4. Placing by dragging | Clicks never place; dragging shows the node's outputs and lights the line under it; dropping on a line or on the canvas | View tests click without placing, drag onto a line and onto the canvas | Planned |
| 5. Checks | `make check` and the real-window suite; a real-window test searches for "value", reads Check value's help, adds it after a node with Add after and drags a Wait onto a line | The checks pass with no lint findings | Planned |

## Step results

None yet.

## Not in this increment

- **Help for a node already on the canvas** beyond its one line at the
  foot of its inspector, as today.
- **Recently used nodes** at the top of the palette.
