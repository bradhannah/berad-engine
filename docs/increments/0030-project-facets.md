# Increment 30: Lines over nodes, and the Project by facets

**Status:** Built

The thirtieth increment comes from its author trying increment 29. A node
placed over a line sat on top of it, and the line seemed to go in at one
side and out at the other, so the node looked joined to it. Now the line
shows through the node, and a node nothing links to says so.

The Project window takes the first of the research-led options for large
projects: facets. Every property things have is a facet, and each value
says how many things have it. Ticking values turns the list into a table of
what is left, and a set of ticks can be saved with the project as a view.
For that, every document now records when it last changed, and every
workflow's problems are found without opening it. It builds on the Project
window and the canvas of [increment 29](0029-lines-state-and-picking.md).
It is held to the same strict checks as [increment 1](0001-foundation.md).

## What it proves

1. **A node over a line is not taken for one in it:** the line shows
   through the node, and a node nothing links to says so.
2. **Thousands of things can be sliced by any property:** each value says
   how many things ticking it would list, and no tick leads to nothing.
3. **Every thing knows when it last changed, and every workflow its
   problems,** without being opened.
4. **A narrowing can be kept:** a set of ticks is saved with the project as
   a view and applied again with a click.

## Decisions

Asked for by its author on 2026-11-05, after trying increment 29: whether
lines over nodes should be see-through or solid, since a node placed on a
line looked joined to it; and the Project window as option E, facets. The
atlas, option I, is kept as an idea, and a graph of how things relate may
follow once the strong associations between them are settled.

Agreed by its author on 2026-11-05, from the wireframes on the canvas
"Workflow canvas wireframes" and in answer to the planning questions:

- **A node over a line shows it is not joined,** by A with D of the row "A
  node over a line". A line that crosses a node is drawn over it at 40%
  strength, through its dots. A node nothing links to rings its loose dots
  red and says "Not linked: never runs" under itself. (Rejected: B, solid
  lines over nodes, which strike through them; C, the line showing through
  as dashes, which mean a loop on this canvas; E, a gap in the line; F, a
  shadow under the node.)
- **The Project window takes option E, facets,** of the row "The Project at
  scale".
- **The list stays a tree until it is narrowed.** With nothing typed or
  ticked it is the tree as before, with its folders and dragging. A search
  or a tick turns it into a table of what is left, under a bar of chips for
  the ticks and how many things are shown of all. (Rejected: always the
  table; the tree narrowed by the ticks.)
- **Changed and Problems are built now:** every document records when it
  last changed, and every workflow's problems are found without opening
  it.
- **Saved views are kept in the project,** so they are under version
  control and shared with anyone who opens it. (Rejected: kept in each
  person's editor.)

Chosen while building, for its author to check:

- **Seven facets:** Kind, Zone, Runs as, Folder, Tags, Changed and
  Problems. Ticks within a facet add up: any of them. Ticks in different
  facets each narrow: all of them. A value's count is how many things
  ticking it would list, so a value that would list nothing is not
  offered.
- **Zone** holds a zone itself, its floors, the NPCs living in it and the
  workflows running in it. A workflow that runs in any zone is in none.
- **Runs as** is NPC behaviour, Zone loop, or Called by workflows.
- **Folder:** a folder holds what its folders hold. Each is shown with its
  kind's icon, as "Market / Guards". A sound's category is its folder.
- **Changed:** Today, This week (the last seven days) and This month (the
  last thirty), each holding the ones before it, then Older. A document
  not changed since this increment is Older.
- **Problems** are named by kind of node: Not linked, Go to, Signals,
  Values, Sounds, Calls and Lanes. Unheard receiver is a receiver nothing
  sends to, and Unused a workflow no NPC or zone runs and nothing calls.
- **A facet lists six values,** then "N more…". A facet's heading folds
  it.
- **The table's columns** are Name, Kind, Zone, Folder, Used by and
  Changed: as many as fit beside a name 200 pixels wide, Kind, Zone and
  Changed first. Used by says which NPCs and zones run a workflow and which
  workflows call it.
- **A search still matches** names, folders and #tags, and finds Settings
  and World state's parts by their words, now in the table.
- **On a narrow window** the facets column hides once the list beside it
  would be narrower than 400 pixels. The search runs across the top, and
  typing still gives the table.
- **Saved views are a document every project holds,** "Project views", in
  `views/`. A game never reads them. Save as a view… offers a name made from
  the ticks; saving a name already used replaces that view. When the ticks
  are a saved view's, the bar offers Delete view instead, which undo brings
  back.
- **When a document changed is saved in its file.** The project index
  tells the windows once a day for each document, as finely as the facet
  looks, so editing does not send the index along with every keystroke.
- **Problems use the workflow window's own checks,** run without a window
  for a few workflows each frame, in turn.
- **Only lines that cross a node change:** the stretch inside the node is
  drawn again, faintly, over it.

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. Lines over nodes | A line crossing a node drawn over it at 40% strength; a node nothing links to rings its dots red and is tagged | A Wait dropped on a line is loose and crossed, and the node the line leads to is neither | Built |
| 2. When things changed | Each document's last change saved in its file and listed in the project index, which windows hear of once a day | A document's change is listed when it is made, again only the next day, and read back on reopening | Built |
| 3. Problems unopened | A check that finds a workflow's problems, what runs it and who calls it, without a window | A loose node is Not linked, and a workflow an NPC runs is used | Built |
| 4. Saved views and table rows | The Project views document; lists that tick rows and show columns | Views are saved, undone and refused when two share a name | Built |
| 5. Facets and the table | The facets column with counts, the table with its bar and columns, saving, applying and deleting views | Ticking narrows to a table, counts never offer nothing, and a saved view applies again | Built |

## Step results

- **Lines over nodes:** a unit test finds where a route runs inside a
  node, and another drops a Wait on a line and finds it loose and crossed.
  The palette's real-window test shows the faint line over the dropped
  node, its red dots and its tag.
- **When things changed:** the envelope of every saved document gains
  `changed`. A unit test changes a text over two days with a set clock and
  finds the index told once each day, the file keeping the last change, and
  the index reading it back on reopening.
- **Problems unopened:** a unit test finds a loose node's family and that
  an NPC's behaviour is used, once the orchestrator has said who runs it.
- **Saved views and table rows:** unit tests save views, undo them, refuse
  two of one name and tell views that narrow the same way.
- **Facets and the table:** unit tests tick a kind and a folder, check the
  counts and the cells, search, and save, apply and delete a view. The
  Project window's unit tests now leave the problem checks' questions out of
  the where-used questions they count. A real-window test ticks Workflows in
  the sample, saves the view, applies it again, and finds it saved with the
  project.
- **Checks:** no lint findings, and the format and vet checks pass. The
  real-window suite passes, 59 tests, among them the new one of the facets.
  Every unit test passes with the race detector, run uncached. Under `make
  check` the orchestrator's audio tests then timed out at ten seconds while
  the computer was busy. Increment 29's own commit, which passed before,
  failed the same way at the same time.

## Not in this increment

- **Sorting the table** by a column.
- **Problems of other things,** such as an NPC with no behaviour or a sound
  nothing plays.
- **Renaming a saved view;** saving it again under a new name and deleting
  the old one does it.
- **The atlas, and a graph of how things relate,** kept for when the strong
  associations between things are settled.
- **The other options for large projects:** a working set, stepping along
  relations, an overview, columns, and quick open.
