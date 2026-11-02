# Increment 20: Moving and finding values

**Status:** Built

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
| 2. Move into | The right-click menu on a value or group with Move into and its submenu, Rename and Delete; a move as one change, refused where the name is taken; the line under the Name box | View tests move a value into a group, to the top level and into a new group, refuse a taken name, and undo | Built |
| 3. Moving what is used | The dialog listing each use as it will read, Move and Cancel; a value nothing uses moves at once | View tests move a used value through the dialog, cancel it, and move an unused one without it | Built |
| 4. Dragging | Dragging a value or group onto a group or its section's heading; other sections faded and refused; a closed group opening at rest | View tests drag into a group, out to the heading, into a group opened at rest, and refuse another section | Built |
| 5. The shared picker | The picker in a package of its own, the Workflow window using it as before | The picker tests of increment 19 pass unchanged | Built |
| 6. Find a value | The button and Cmd+F; the World state rail and every type; uses counted; Show opening the groups and selecting the row | View tests find a value inside a closed group and show it, narrow by scope, and count uses | Built |
| 7. Declare and show | The declare panel with the world, zones, NPCs and templates as where; the new value selected in the tree | View tests declare in the world, for an NPC and in a template, and find each selected | Built |
| 8. Checks | `make check` and the real-window suite; a real-window test moves a used value by the menu, drags another into a group, finds one and declares one | The checks pass with no lint findings | Built |

## Step results

- **Text on a node:** a text operand was quoted twice, once as the value
  writes itself and again by the node, so empty text read "\"\"". Now a
  node shows text as it was typed, in curly quotes, cut at 24 characters
  with an ellipsis; empty text reads empty, as Is npc.name empty?. Set
  value shows its text the same way. Verified: a view test reads a Check
  value's title for empty text, Ada, and long text with quote marks in it.
- **Move into:** a right-click on a value, group or template selects it,
  asking first about typed changes to the one being left, and opens a menu:
  Move into, Rename and Delete. Move into opens a submenu beside it, headed
  Inside World, Inside the guards template or Inside George's own: Top
  level, each group of that section indented under the group holding it,
  leaving out the row itself and what it holds, with where it already is
  shown but not offered; and New group…. A move keeps the name and is one
  change, the groups it needs declared with it, so Cmd+Z puts it back; the
  status says "Moved to world.quests.mood. Cmd+Z undoes it.", or why not,
  as "mood is already declared there". New group… makes a group named
  group, or group_2 and on when taken, at the section's top level, moves
  the row into it, and selects the group with its name ready to type. A
  template's Move into is greyed. Rename puts the Name box to type, and a
  line under the box says a dotted name moves it too. Menus gain
  submenus: an item opens its own beside it when the pointer rests on it
  or it is clicked. Verified: a view test moves world.mood into quests as
  one command and back to the top level, is refused where quests has a
  mood, moves it into a new group selected with its name to type, moves
  George's secret into a group of his own, finds no move for the guards
  template, and renames by the menu.
- **Moving what is used:** before a move, the window asks where the value,
  or every value in the group, is used, as a delete does. With no use it
  moves at once; with uses a dialog, Move mood into quests?, shows
  world.mood → world.quests.mood, says how many uses follow it and that a
  use names the value itself, not its path, so nothing needs rewriting,
  lists each use, and offers Move and Cancel, which leaves it. Input
  scripts name the buttons "dialog:move" and "dialog:cancel", as every
  dialog's are, and read its lines as "state:says=<line>". Verified: a
  view test cancels and then makes the move of a value a workflow uses,
  reading the use and the paths in the dialog, and finds a group holding
  it asks first too.
- **Dragging:** a value or group dragged onto a group of its own section
  moves into it, and dragged onto its section's own row, the World or
  Zone heading, its template's row or its NPC's or zone's row, moves to
  the top level; it asks first when used, as the menu does. While
  dragging, every row outside its section is washed out and refuses the
  drop, the group under the pointer is outlined, and the label beside the
  pointer carries the new path, as mood → world.quests.mood. Resting on a
  closed group for about half a second opens it. The list gains what this
  takes: rows to fade, the label to carry and the row a drag rests on;
  lists are now made by one constructor. Verified: a view test drags
  world.mood into quests and back onto World, is refused dropping it on
  the guards template, opens quests by resting on it and drops into it,
  and keeps a template's value in its template.
- **"Drop to move into group" on the row, short of the wireframe:** the
  group is outlined and the pointer's label gives the new path, which says
  the same; the list writes nothing on the row.
- **The shared picker:** the picker is now its own package, valuepicker,
  with its search, chips, rail, folding groups and declare panel. A
  window opens one with its words, as its title, what the button picks and
  what declaring does, its rail and the type it is locked to, and gives it
  a source: the values it lists, each with the heading, rail scope and root
  it sits under; where a name typed can be declared; and what a pick and a
  declare do. The Workflow window is one such source, its node's values,
  declare targets and choosing unchanged. The group last opened is kept by
  the window. Verified: the view tests of increments 15, 18 and 19 and the
  picker's and comparisons' real-window tests pass unchanged.
- **Find a value:** a Find a value button at the right of the add buttons,
  or Cmd+F, opens the shared picker over the World state window, titled
  Find a value, in World state. It lists every value the window holds:
  the world's under WORLD, each zone's and NPC's own under its name, and
  each template's under ZONE TEMPLATE or NPC TEMPLATE and its name, every
  type offered, groups folding as in the Workflow window. Its rail is
  Everywhere, World, Zones, Zone templates, NPCs and NPC templates, with
  no Local. Each row says at its right how many places use it, as used 2
  times or unused, from one where-used question for the whole world state
  asked as Find opens; a use now carries the item it refers to, so one
  answer counts every value. Show, Enter or a click closes the picker and
  shows the value in the tree: its section, template, NPC or zone and
  groups are opened, its row selected and scrolled to, and its inspector
  shown, asking first about typed changes to the row being left. Verified:
  a view test counts a value's two uses and an unused one, shows
  world.quests.dragon_slain from a closed quests, narrows to NPCs and
  counts their values and their templates', and shows George's secret by
  Enter, opening George.
- **Declare and show:** a name in Find that no value has gives way to
  the picker's declare panel. Where offers the world, each NPC and zone
  template and each NPC and zone by name; a name starting with a scope, as
  npc., stays in that scope, and one whose first name names a template or
  an NPC or zone there, as npc.guards.on_duty or npc.george.mood_today,
  goes only there. Every type is offered, with its start. Declare and
  show, or Enter, declares it with the groups its name needs and then
  shows it in the tree, selected with its inspector open for its limits,
  choices and description, which the panel says are set there. Verified:
  a view test offers shift in the world, the guards template, George and
  the market; declares world.shop.open_hour starting at 9, George's own
  mood_today as true, and on_duty in the guards template by Enter, and
  finds each selected.
- **Checks:** `make check` passes with no lint findings, and so does the
  whole real-window suite, 45 tests. A new real-window test declares
  world.done from a workflow's Set value, then in the World state window
  moves it by its right-click menu into a new group, asked first as the
  workflow uses it, and names the group progress; adds a value in progress
  and drags it out onto World; finds done by Enter; and declares
  world.shop.open_hour from Find. The Set value's box follows done to
  world.progress.done, and the saved world state and logic hold all of it.
  Its screenshots showed the menu offering "Delete integer" and the dialog
  asking to move into "group" when the group was new; the menu now says
  Delete value, group or template, and the dialog "into a new group,
  group". Add value with a value in a group selected adds it in that
  group, as before.
- **Each use as it will read, short of the plan:** where a value is used is
  answered as the workflow and the kind of node, as Workflow 1 · a Check
  value node, not the node's sentence, which only the Workflow window
  writes; the dialog lists that.
- **Submenus by the keyboard, short of the plan:** the editor's menus take
  no arrow keys yet, so a submenu opens by the pointer only.

## Changed after its author tried it

Found by its author on 2026-11-02, trying it:

- **Typing reached neither picker.** The editor sends what is typed only
  to the text field a window says has focus, and neither the World state
  window nor the Workflow window said the picker's search, or its declare
  panel's start, had it; input scripts type straight into a window, so the
  tests never saw it. Find and increment 19's picker in the Workflow
  window now hand the editor the picker's focused field. Verified: view
  tests find the search focused as each picker opens.

Asked for by its author on 2026-11-02, after trying it, paraphrased: a
right-click on a group in World state should add a value to it; and
changing a value's type on the fly is a problem when what uses it expects
its type. Agreed the same day from the wireframes on the canvas "Workflow
canvas wireframes", row "World state: add inside a group from its menu",
choosing to lock the type over asking first:

- **Add from a row's right-click menu.** A group's menu, headed In
  quests, offers Add value ›, by type, and Add group, then Move into,
  Rename and Delete group. The World heading adds at the top of the world;
  a template's row adds in the template; an NPC's or zone's row adds to its
  own values and offers Apply template ›. What is added is selected with
  its name ready to type.
- **The add buttons say where they add,** as adds in quests, beside them.
- **A value in use keeps its type.** While anything uses it, its other
  type buttons are greyed and a line says why; Declare a new value… opens
  Find to declare one beside it. A value nothing uses changes type freely.
  (Rejected: asking first and letting the change through, leaving each use
  a problem to mend.)

## Not in this increment

- **Moving between sections,** from World into a zone or an NPC: a
  different value to the game, not a tidier place for the same one.
- **Moving several values at once,** by a selection of rows.
- **Find in a workflow's locals,** which stay in the Workflow window.
- **Rewriting uses,** not needed while uses name a value by itself.
