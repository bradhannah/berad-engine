# Increment 21: Flow modules

**Status:** Planned

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
- **Locating for tests:** the node kinds as "call-workflow" and "return",
  the Call's parts as "ui:call-workflow", "ui:call-open",
  "ui:input:<name>" and "ui:keep:<name>", a local's role as
  "ui:role-input" and so on, and Watch's path as "watch:path".

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. Return | The Return node kind in the Flow palette, its way out's name with an identity of its own, two Returns of one name as one way out, flagged inside a container | Workflow tests make, rename and share a way out; a view test flags a Return in a container | Planned |
| 2. Roles and places | A local's role, Local, Input or Output, in its form; the place type for locals with Pick on map | View tests set each role, declare a place local and pick its start, and save and load them | Planned |
| 3. Call workflow | The Call workflow node: choosing the workflow, its outputs from the module's ways out and done, filling inputs, keeping outputs, Open and double-click | View tests choose a module, fill a number, a place and another value, keep an output, and open the module's tab | Planned |
| 4. Go to a place value | Go to's destination as a place on a map or a place value, picked from the place locals | View tests pick a place local for Go to and read Go to local.place on the canvas | Planned |
| 5. Called by and the lock | The Called by strip on a module's tab; an input's or output's type locked while a Call uses it | View tests read who calls a module and find a used input's type locked | Planned |
| 6. Call problems | A missing way out struck through, calling itself, a type that no longer fits and no workflow chosen, on the node and in the Problems window | View tests raise and mend each problem | Planned |
| 7. Calls in the game | Every workflow given to the game; a call stack per NPC; inputs read once, locals fresh, the Return's way out taken, outputs handed back, done by default, a Call with a problem carrying on by done | Runner tests call a module with a place input that goes there and returns by name, keep an output, and skip a broken call | Planned |
| 8. Watching calls | Time up leaving every call inside the container; Watch's path and the innermost call's locals, each part opening its tab | Runner tests time up inside a call; a view test reads Watch's path and opens a part | Planned |
| 9. Checks | `make check` and the real-window suite; a real-window test makes a module with a place input and two Returns, calls it from a behaviour and watches the NPC go there and come back | The checks pass with no lint findings | Planned |

## Step results

None yet.

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
