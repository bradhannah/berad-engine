# Increment 20: Moving and finding values

**Status:** Planned

The twentieth increment makes the World state window easier to tidy and to
search, and makes text read well on a node. Today a value moves into a
group only by typing a dotted name in its Name box, which nothing in the
window says. Now a value moves by a right-click menu or by dragging it
onto a group, within its own section, with a dialog listing what uses it
first. [Increment 19](0019-value-picker.md)'s picker opens over World
state as **Find a value**, to find a value and show it in the tree or to
declare a new one there. And text on a node, quoted twice today, is quoted
once in curly quotes, with empty text called empty. It follows the
agreement in [editor features](../EDITOR_FEATURES.md#world-state-quests-and-rules),
and is held to the same strict checks as [increment
1](0001-foundation.md).

## What it proves

1. **Moving is found without being told:** a right-click on a value offers
   Move into, listing the groups of its own section; dragging a value onto
   a group, or onto its section's heading to take it out of a group, moves
   it there; nothing moves into another section.
2. **Moving keeps what uses a value:** workflows name a value by itself,
   not by its path, so nothing is rewritten; a value something uses asks
   first, listing each use as it will read after the move; a value nothing
   uses moves at once; Cmd+Z puts either back.
3. **The picker finds in World state:** Find a value, by its button or
   Cmd+F, opens the picker over the window with the World state's scopes;
   picking a value shows it in the tree, its groups opened and its row
   selected.
4. **Declaring from Find:** a name that matches nothing is declared from
   the picker, in the world, a zone, an NPC or a template, with any type
   and a starting value, and is then shown in the tree for the rest of
   its form.
5. **Text reads as written on a node:** Is npc.name “Ada”? and Is npc.name
   empty?, not "\"Ada\"" and "\"\"".

## Decisions

Asked for by its author on 2026-11-02, after trying increment 19,
paraphrased: values outside a group have no obvious way into one; with no
references, a right-click and drag and drop are expected; with references,
warn about them and offer to rewrite them. Moves stay within World, a zone
or an NPC, never between them. The text termination on a node is ugly. The
picker is liked; World state should have a search mode that uses it, to
find values and to define them.

Agreed by its author on 2026-11-02, before building, from the wireframes on
the canvas "Workflow canvas wireframes", rows "World state: moving values
into groups", "World state: find and declare with the picker" and "Text on
a node":

- **Move into, on a right-click,** a submenu of the value's own section,
  headed Inside World and so on: Top level, marked where the value already
  is, each group, nested groups indented, and New group…; then Rename and
  Delete value.
- **Dragging a value onto a group moves it into the group;** onto its
  section's heading takes it out of its group. While dragging, rows of
  other sections are faded and refuse the drop, the group under the
  pointer is lit with "Drop to move into group", the row dragged carries
  its new path, and a closed group opens after the pointer rests on it.
- **A value something uses asks first,** in a dialog that shows the path
  before and after and lists each use as it will read; Move or Cancel. No
  rewrite is offered because none is needed: a use names the value
  itself. A value nothing uses moves at once, saying so in the status
  line.
- **The Name box says a dotted name moves it,** under the box, as it does
  today without saying.
- **Find a value** sits at the right of Add value, Add group and Add
  template, and opens by Cmd+F too. Its rail is Everywhere, World, Zones,
  NPCs, Zone templates and NPC templates, with no Local, since a
  workflow's locals are found in its own window; every type is offered;
  each row shows how many places use the value. Enter, or Show, shows it
  in the tree.
- **Declare and show:** the declare panel offers the world, each zone,
  each NPC and each template as where, and every type; the new value is
  then selected in the tree with its inspector open.
- **Text on a node is in curly quotes once,** nothing escaped, cut with …
  when long; empty text reads as empty.

Chosen while planning, for its author to check:

- **The picker moves to a package of its own,** shared by the Workflow and
  World state windows, each giving it its rows, rail, rules for declaring
  and what a pick does. Its look and keys stay as increment 19 built them.
- **A move is one change,** as a dotted name's is today, and the groups a
  new group needs are declared with it; a group that already holds a value
  of that name refuses the move, saying so in the status line, as typing
  the name would.
- **Groups move as values do,** with everything in them, into another
  group of the same section but never into themselves.
- **Submenus are new to the editor's menus:** a menu item can open a
  second menu beside it, by hovering or by the right arrow.
- **The rest at rest:** a closed group opens after the pointer rests on it
  for about half a second while dragging.
- **Uses counted at once:** the Find rows' counts come from one request
  for every value's uses, made as the picker opens, rather than one
  request per value.
- **Empty text** reads "empty" in the node's usual face, as the editor
  draws no italics.
- **Locating for tests:** the menu's items as "menu:move-into" and
  "menu:into:<path>", a drop as a drag from "row:<path>" to "row:<path>",
  the dialog's buttons as "ui:move" and "ui:cancel", the button as
  "ui:find-value", and the picker's parts as increment 19 names them.

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. Text on a node | Text operands in curly quotes once, nothing escaped, cut with … when long; empty text as empty; the same for Set value | View tests of the labels of empty text, plain text, text with quote marks and long text | Built |
| 2. Move into | The right-click menu on a value or group with Move into and its submenu, Rename and Delete; a move as one change, refused where the name is taken; the line under the Name box | View tests move a value into a group, to the top level and into a new group, refuse a taken name, and undo | Planned |
| 3. Moving what is used | The dialog listing each use as it will read, Move and Cancel; a value nothing uses moves at once | View tests move a used value through the dialog, cancel it, and move an unused one without it | Planned |
| 4. Dragging | Dragging a value or group onto a group or its section's heading; other sections faded and refused; a closed group opening at rest | View tests drag into a group, out to the heading, into a group opened at rest, and refuse another section | Planned |
| 5. The shared picker | The picker in a package of its own, the Workflow window using it as before | The picker tests of increment 19 pass unchanged | Planned |
| 6. Find a value | The button and Cmd+F; the World state rail and every type; uses counted; Show opening the groups and selecting the row | View tests find a value inside a closed group and show it, narrow by scope, and count uses | Planned |
| 7. Declare and show | The declare panel with the world, zones, NPCs and templates as where; the new value selected in the tree | View tests declare in the world, for an NPC and in a template, and find each selected | Planned |
| 8. Checks | `make check` and the real-window suite; a real-window test moves a used value by the menu, drags another into a group, finds one and declares one | The checks pass with no lint findings | Planned |

## Step results

- **Text on a node:** a text operand was quoted twice, once as the value
  writes itself and again by the node, so empty text read "\"\"". Now a
  node shows text as it was typed, in curly quotes, cut at 24 characters
  with an ellipsis; empty text reads empty, as Is npc.name empty?. Set
  value shows its text the same way. Verified: a view test reads a Check
  value's title for empty text, Ada, and long text with quote marks in it.

## Changed after its author tried it

Not yet tried.

## Not in this increment

- **Moving between sections,** from World into a zone or an NPC: a
  different value to the game, not a tidier place for the same one.
- **Moving several values at once,** by a selection of rows.
- **Find in a workflow's locals,** which stay in the Workflow window.
- **Rewriting uses,** not needed while uses name a value by itself.
