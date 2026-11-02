# Increment 21: Flow modules

**Status:** Built; steps 10 to 14, from trying it, Planned

The twenty-first increment lets one workflow call another and carry on
when it returns. Today an NPC runs one workflow, its behaviour, and a flow
that repeats a few steps in several behaviours has to be drawn again in
each. Now any workflow can be called from another by a **Call workflow**
node: the called workflow, a module, takes inputs, a place among them,
goes back to its caller by named **Return** nodes, and hands back value
outputs. The caller's Call node has one output per way out, plus done.
Watch shows the path down through the calls. It builds on the local
values of [increment 15](0015-values-and-sensing.md) and the picker of
[increment 19](0019-value-picker.md), follows the direction in [workflow
design](../WORKFLOW_DESIGN.md#calls-between-workflows), and is held to the
same strict checks as [increment 1](0001-foundation.md).

## What it proves

1. **A workflow calls another and carries on:** a Call workflow node runs
   the module it names, and when the module returns the caller carries on
   from the Call's output named for the way out taken, or done.
2. **A module says how it is used:** a local value is a local, an input or
   an output; an input is filled by the caller, read once as the call
   starts, and starts as its own value where the caller leaves it
   unfilled or when the workflow runs as a behaviour of its own.
3. **A place can be passed:** a place local, new for locals as NPC is,
   filled by Pick on map at the caller, is where a Go to in the module
   goes.
4. **Ways out are named:** a Return node names one; two Returns with one
   name are one way out; renaming one keeps the lines its callers drew.
5. **Outputs come back:** each output is kept in a value of the caller's
   choosing, or left, on every way out.
6. **A call that no longer fits says so:** a way out the module no longer
   has, a module that calls itself, an input whose type no longer fits
   and a Call with no workflow are each a problem on the node and in the
   Problems window; the game carries on by done.
7. **Watch follows the calls:** an NPC inside a call shows the path from
   its behaviour to the node it is on, and the innermost call's locals;
   time up leaves every call inside the container at once.

## Decisions

Asked for by its author on 2026-11-02, while building increment 20: a
flow that can call a module and return, which [workflow
design](../WORKFLOW_DESIGN.md#calls-between-workflows) agrees on but
nothing builds.

Agreed by its author on 2026-11-02, before building, in answer to the
planning questions:

- **The whole interface in this increment:** named ways out, inputs, a
  place as an input and value outputs, not a smaller first call.
- **Ways out are made by a Return node,** at the module's top level.
- **Locals are fresh on each call:** a module's locals start at their
  start values, or its inputs' given values, each time it is called.
- **Any workflow can be called,** with no kind of workflow set apart as a
  module; a behaviour assigned to an NPC can also be called.

Agreed by its author on 2026-11-02, before building, from the wireframes
on the canvas "Workflow canvas wireframes", row "Flow modules: calling a
workflow and returning":

- **Call workflow,** in the Flow palette beside Return. The node shows
  CALL WORKFLOW, the module's name and its inputs as given, and one output
  per way out, then done. Its inspector chooses the workflow, with Open;
  fills each input with a fixed value, Pick on map for a place or Another
  value from the picker; and keeps each output in a value chosen with the
  picker, or Leave it. Open, or a double-click on the node, opens the
  module in its own tab.
- **The module's tab** gains a strip naming who calls it, as Called by 2:
  Shopkeeper day, Guard rounds, and its ways out. Its local values each
  show their role.
- **Return** shows RETURN and its way out's name. Any number of lines end
  at one; it has no output. Two Returns with one name are one way out.
  Inside a container a Return is flagged, until containers get named
  exits.
- **A local's role is Local, Input or Output,** chosen in its form above
  its type. An input's form says it is read once as the call starts and
  starts as below where the caller leaves it unfilled; an output's says it
  is handed back when the call returns, by any way out.
- **place is a new type, for locals only,** as NPC is: a spot on a map,
  its start value chosen by Pick on map, or No place yet.
- **Go to's destination is A place on a map,** today's Pick on map, **or A
  place value,** chosen with the picker locked to the workflow's place
  locals; the canvas then reads Go to local.place. With no place, a Go to
  carries on by no way there.
- **A call's problems,** on the node with a red mark and in the Problems
  window: a way out the module no longer has, its line kept and struck
  through until it is removed or the Return put back; a module that calls
  itself, directly or through others; an input whose type changed so its
  value no longer fits; and no workflow chosen. The game skips a Call with
  a problem, carrying on by done, as a Go to with no way there carries on.
- **Watch shows the path,** as Clerk: Shopkeeper day › Go to work › Go to,
  each part opening that tab with its live node marked, and the innermost
  call's locals. When a container's time is up, every call inside it is
  left at once.

Chosen while planning, for its author to check:

- **A way out keeps an identity of its own,** apart from its name, as a
  Random's options keep theirs: a Return holds it, and the Call's lines
  are drawn from it, so renaming a way out keeps every caller's line. A
  Return given a name another Return has takes that one's identity.
- **An input's or output's type is locked while any Call uses it,** as
  [increment 20](0020-moving-and-finding.md) locks a used World state
  value's, saying how many Calls use it.
- **Outputs are handed back on every way out,** a Return's or done's, so
  a caller never has to know which way out set them.
- **Place values only in Go to and in locals** for now; places in world,
  zone and NPC values, and in other nodes, come later.
- **The game needs every workflow,** not only each NPC's behaviour, so a
  call can find its module; a workflow missing from the game is a problem
  of the Call that names it.
- **A call stack per NPC:** each call keeps its own locals and the node it
  is on, and returning pops back to the caller's Call node; a clock jump
  or an edit restarts the behaviour with no calls.
- **Locating for tests:** the node kinds as "call-workflow" and "return";
  the Call's parts as "ui:call-workflow", the workflows it lists as
  "menu:workflow-named:<name>", "ui:call-open", "ui:input-<name>" and its
  "-pick", "-another", "-value" and "-clear", and "ui:keep-<name>" and
  "-leave"; a local's role as "ui:value-role-input" and so on; Watch's
  path as "state:path=<path>" and its parts as "ui:path-<n>"; a Call's
  outputs as "state:output:<kind>:<n>:<port>=<name>" and a lost way out as
  "state:missing:…"; the strip as "state:calledby=<line>".

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. Return | The Return node kind in the Flow palette, its way out's name with an identity of its own, two Returns of one name as one way out, flagged inside a container | Workflow tests make, rename and share a way out; a view test flags a Return in a container | Built |
| 2. Roles and places | A local's role, Local, Input or Output, in its form; the place type for locals with Pick on map | View tests set each role, declare a place local and pick its start, and save and load them | Built |
| 3. Call workflow | The Call workflow node: choosing the workflow, its outputs from the module's ways out and done, filling inputs, keeping outputs, Open and double-click | View tests choose a module, fill a number, a place and another value, keep an output, and open the module's tab | Built |
| 4. Go to a place value | Go to's destination as a place on a map or a place value, picked from the place locals | View tests pick a place local for Go to and read Go to local.place on the canvas | Built |
| 5. Called by and the lock | The Called by strip on a module's tab; an input's or output's type locked while a Call uses it | View tests read who calls a module and find a used input's type locked | Built |
| 6. Call problems | A missing way out struck through, calling itself, a type that no longer fits and no workflow chosen, on the node and in the Problems window | View tests raise and mend each problem | Built |
| 7. Calls in the game | Every workflow given to the game; a call stack per NPC; inputs read once, locals fresh, the Return's way out taken, outputs handed back, done by default, a Call with a problem carrying on by done | Runner tests call a module with a place input that goes there and returns by name, keep an output, and skip a broken call | Built |
| 8. Watching calls | Time up leaving every call inside the container; Watch's path and the innermost call's locals, each part opening its tab | Runner tests time up inside a call; a view test reads Watch's path and opens a part | Built |
| 9. Checks | `make check` and the real-window suite; a real-window test makes a module with a place input and two Returns, calls it from a behaviour and watches the NPC go there and come back | The checks pass with no lint findings | Built |
| 10. Picks that say so | Pick on map turning into Cancel pick on the control that started it alone, a pick started elsewhere taking it over; the tab that asked back in front after a pick, the new place lit and the status naming it | View tests start, take over and cancel a pick from each control, and find the asking tab in front after an answer | Planned |
| 11. One place control | The place box with the ring, the name and its zone and floor, Pick on map and Clear, for a local's default, a Call's input and Go to on a map; the box opening a searchable list of every place, by zone and floor, with Pick on map last | View tests read the box in each inspector, choose a place by name and find it saved | Planned |
| 12. Picking on the map | A banner naming what the place is for and what it is now, with Cancel; tools and all but anchors faded and taking no click; anchors drawn over NPCs; the anchor under the cursor named; the current place marked now; the floor's places listed in the inspector, chosen by name | Map tests read the banner and the current place, pick an anchor under an NPC, and choose one from the list | Planned |
| 13. The ring | The anchor's ring beside every place: the type's button, the locals list, the text on a node, the Anchors tool; the place type's line | View tests read the ring in each | Planned |
| 14. Checks again | `make check` and the real-window suite; a real-window test picks a Call's place on the map and finds the workflow back in front with the place | The checks pass with no lint findings | Planned |

## Step results

- **Return:** Return is in the Flow palette. Its node reads RETURN over
  its way out's name, and its inspector has one field, Way out. A Return
  holds its way out's name and a port, w1, w2 and so on, that a Call's
  lines are drawn from. A new Return takes a port no Return has and a name
  no way out has: finished, then finished 2 and on. Named as another
  top-level Return is, it joins that way out, taking its port; renamed
  away from a way out it shared, it takes a port of its own; a Return
  alone in its way out keeps its port, so callers keep their lines. done
  is refused as a name, as every Call has a done. A Return inside a
  container is flagged and is no way out. A workflow's ways out are its
  top-level Returns' ports, each once, named by the first Return to name
  it. Verified: workflow tests list two Returns of one name as one way out
  and none inside a container, and rename one keeping its port; a view
  test adds two Returns, joins one to the other by its name, renames it
  away, keeps a lone Return's port, refuses done and flags one in a
  container.
- **Roles and places:** a local's form has Role, Local, Input or Output,
  above Type, with a line under Input or Output saying what it means. The
  role is kept through a change of type. place joins the types for locals
  only, as NPC is; its Default is Pick on map with the anchor's name, or
  No place yet. A place is saved as its floor and anchor. Workflows are
  version 7. Verified: an engine test saves and reads an input place and
  an output and refuses a world value with a role; a view test sets each
  role, makes an input a place and picks its start on a map.
- **Call workflow:** Call workflow is in the Flow palette. Its node reads
  CALL WORKFLOW, the workflow's name in bold and what it gives its inputs,
  as place = Shelf, or inputs as they start, over a row for each way out
  and done. Its inspector's Workflow box lists the project's other
  workflows; Open, or a double-click on the node, opens the one chosen in
  its tab. Each input has a row by its type: a field for an integer or
  text, false and true or a list's choices, Pick on map for a place,
  Another value for any, which opens the picker locked to the input's
  type, and Clear while it is given something; unfilled, it says what it
  starts as. Each output has a box that opens the picker to choose a
  value to keep it in, and Leave it. The Call saves its module's ways out
  as its outputs; the window shows a Call's ways out as the module has
  them now, so a way out renamed in the module shows its new name at
  once and one added shows as a free output, and saves them with any
  change it makes to the Call, as drawing a line from one. A Call's
  references name the workflow it calls, so the project knows its
  callers, and the world values it gives or keeps. Verified: view tests
  choose Go to work, read its outputs, type 10 for one input, pick the
  desk for another, give the first another value instead, keep an output
  and leave it, open the module from Open and by a double-click, keep a
  line through a rename, strike through a way out removed, show one added,
  save it when a line is drawn from it, and show the ways out of a module
  chosen before it had loaded; a plug-in test lists a Call's references.
- **Go to a place value:** Go to's Destination is A place on a map, its
  anchor as before, now labelled On the map, or A place value, a box that
  opens the picker locked to the workflow's place locals. The canvas reads
  Go to · local.place. A Go to with no place value chosen, or one gone or
  no longer a place, is flagged; one whose place holds nothing carries on,
  as with no destination. Version 6 workflows give every Go to its anchor
  as where it goes. Set value and Check value are not offered place
  locals. Verified: a runner test goes to the desk a place local starts at
  and carries on with none; an engine test upgrades a version 6 Go to; a
  view test switches a Go to to a place value, finds it flagged, picks the
  place local, reads its title and switches back.
- **Called by and the lock:** a workflow other workflows call names them,
  and its ways out with done, in a strip along the top of its canvas:
  Called by 1: Clerk's day · Ways out: Fetched, Out of stock, done. The
  window asks who calls it when it or the project changes, and every two
  seconds besides. While any workflow calls it, an input's or output's
  type is fixed, with the line Fixed while Clerk's day calls this
  workflow, or while 2 workflows call this one, and Declare a new value…,
  which adds a new local; a local of its own changes type as before.
  Verified: a view test reads the strip, finds an input's type fixed and a
  local's not, and an input's free again once nothing calls it.
- **Call problems:** a Call is marked, with what to do, when it names no
  workflow, when the workflow it calls was deleted, when that workflow
  comes to call this one again, as Calls itself: Daily routine calls Go
  to work, which calls Daily routine, when a line is drawn from a way out
  the module no longer has, as Go to work has no way out called Closed
  any more, or when an input is given, or an output kept in, a value that
  no longer fits its type, as The input opens_at is now text; 10 no
  longer fits. Type text, or choose another value. The window loads the
  workflows called in turn to find a call back. The checks for a lost way
  out, a call back and an input that no longer fits are the engine's, so
  the game skips the same Calls. Verified: engine tests find a lost way
  out, an input that no longer fits and a call back, direct or through
  another; a view test raises and mends each problem.
- **Calls in the game:** an NPC keeps the calls it is inside. A Call goes
  to its module's start with fresh locals, its inputs read once, another
  value as it is then; a Return goes back by its way out, and a branch of
  the module's top level that ends goes back by done; going back hands the
  module's outputs to the values the Call keeps them in and carries on
  from the Call's output for that way out. A Call with no workflow, one
  the game lacks, one that would call its own workflow again or go 16
  deep, or one that does not fit, is skipped by done, saying so. A Return
  in a behaviour, which nothing called, starts it again. When a
  container's time is up, the outermost container whose time is up is
  left at once, with every call inside it. The game and the Play tab load
  every workflow a behaviour calls, in turn; the Map's preview does too,
  restarting an NPC inside a workflow that is edited. Verified: runner
  tests call a module with the desk, walk there, come back by Arrived
  with its output kept, and call again with fresh locals; skip a Call with
  no workflow, one that does not fit and one calling its own behaviour;
  go back by done from a branch that ends; start a behaviour again at a
  Return; and leave a call and its container when time is up.
- **Watching calls:** the preview sends the calls each NPC is inside and
  the workflow it is in now. Watch names the path after the NPC, as
  Clerk's day › Fetch stock › Go to, and draws the workflow it is in, with
  that workflow's locals; a place local shows its anchor's name. Clicking
  a workflow of the path draws it in Watch with the Call the NPC is
  inside lit, until the NPC comes back from that call; the last part
  follows it again. Verified: a view test reads the path, shows the day by
  its part, follows the clerk again, and shows the day once the call
  returns.
- **Checks:** `make check` passes with no lint findings, but for one
  audio test that timed out under the race detector with every package
  running, and passed alone and in the next full run. The sample's
  clerk's day now calls Fetch stock, a workflow taking a place, the shelf
  to start with, that goes there and comes back by Fetched three times in
  four and Out of stock otherwise, so the sample has a node of every kind.
  Two real-window tests: one makes a new workflow a module with two
  Returns and a place input, adds a Call of it to the clerk's day, reads
  its ways out on the Call, finds the module's strip naming the clerk's
  day and the input's type fixed, and reads the saved files; the other
  previews the sample, watches the clerk and finds Watch's path Clerk's
  day › Fetch stock › Go to with local.place the Shelf, then the path
  gone as the clerk comes back. The whole real-window suite, 47 tests,
  passed.
- **Steps 1 and 2 were built together,** in one commit, as the place type
  reached the shared value form while Return was being checked.

## Changed while building, short of the wireframes

- **No Problems window:** the editor has none. A Call's problems show as
  every node's do, a mark on the node and the problem in its inspector.
- **Watch's path draws a workflow in Watch, not in its tab:** Workflow
  tabs draw no live node, so a part of the path shows that workflow in
  Watch itself, its Call lit.
- **Return has no arrow icon:** its node reads RETURN over its name, as
  other nodes read their kind.
- **The input's row on the Call:** a place's name sits under Pick on map,
  as the inspector is too narrow for both on one line.

## Changed after its author tried it

Settled by its author on 2026-11-02, after trying it and from the
wireframes of places, Pick on map having confused its author: a second
click cancelled the pick without a word, the map stayed in front after a
pick, an anchor under an NPC was picked by mistake, and nothing said that a
place is an anchor.

- **Pick on map says when it is picking:** only the control that started
  a pick reads Cancel pick while it runs, and a Pick on map elsewhere takes
  the pick over, as Go to's did.
- **A pick comes back:** after an anchor is clicked, the tab that asked is
  in front again, the new place lit for a moment, and the status names it
  and says Undo puts the old one back.
- **One place control:** a local's default, a Call's input and Go to on a
  map show a box with the ring and the place's name in ink, its zone and
  floor under it, then Pick on map and Clear, with Another value on a
  Call's input. With no place, the box reads No place yet, grey and dashed.
  A Call's input row names the input over a small type tag, not
  place · place.
- **A place can be chosen by name:** the box opens a list of every place
  in the project, by zone and floor, with a search, and Pick on map last.
- **Picking is a mode of the map:** a banner says what the place is for
  and what it is now, with Cancel; the tools and all but the anchors fade
  and take no click; anchors are drawn over NPCs; the anchor under the
  cursor is named; the current place is filled and marked now; the
  inspector lists the floor's places, which can be chosen by name.
- **A place keeps its name, marked by the anchor's ring,** the ring the map
  already draws: on the place type's button, in the locals list, on a
  node's text and beside the Anchors tool, whose line says places NPCs go
  to. The type's line says a place is an anchor on a map. Calling it an
  anchor everywhere was the other choice, not taken.

## Not in this increment

- **Named exits from containers,** the tentative design a Return will one
  day share; until then a Return inside a container is flagged.
- **Where a module can be used, worked out** from its nodes, such as
  usable in NPC behaviour only.
- **Running a module on its own** with chosen inputs and a test NPC.
- **Place values beyond locals,** in world, zone and NPC values, and in
  nodes other than Go to.
- **A behaviour as a module with its inputs filled in per NPC:** an NPC's
  behaviour still runs with its inputs at their start values.
