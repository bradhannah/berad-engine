# Increment 23: Kinds of workflow, and where they run

**Status:** Planned

The twenty-third increment gives every workflow a kind and a zone. Today
every workflow is an NPC's, and whether its places can be reached is
worked out from the NPCs that happen to run it: a workflow nobody runs yet
has no zone, and one run by NPCs in two zones can't suit both. Now a
workflow is made as an **NPC behaviour** or a **zone loop**, and each kind
offers only the nodes it runs. A workflow **runs in** one zone, or in any
zone: its places, the NPCs that follow it and the zones that loop it are
checked against that. What doesn't fit is greyed where it is chosen, asked
about before a change breaks it, and flagged after. A behaviour that runs
in any zone names no fixed places, so one behaviour can serve NPCs
scattered through every zone. The zone loop, the second kind, is a zone's
own workflow, set in its details, running while the zone is alive to set
its values and play its sounds. It builds on the calls of [increment
21](0021-flow-modules.md) and the zones and zone picker of [increment
22](0022-npc-zones.md), and is held to the same strict checks as
[increment 1](0001-foundation.md).

## What it proves

1. **Every workflow has a kind:** chosen when it is made and saved with
   it, and each kind offers only the nodes it can run.
2. **A zone runs a loop of its own:** a zone loop, set in the zone's
   details, runs while its zone is alive, setting values and playing
   sounds across the zone, and Watch watches it.
3. **A workflow says where it runs:** in one zone or in any zone, chosen
   with the zone picker; its places are checked against that, not against
   whoever follows it.
4. **Choosing offers what fits:** an NPC's behaviours, a zone's loops, a
   Call's workflows and a workflow's places are listed by kind and zone,
   those that fit first and the rest greyed with the reason.
5. **Changing where a workflow runs says what breaks:** the NPCs and zones
   it no longer fits and the places left behind are listed before the
   change and flagged after; Undo puts it back.
6. **One behaviour serves every zone:** a behaviour that runs in any zone,
   its places coming from inputs and values, can be followed by NPCs
   living anywhere.

## Decisions

Asked for by its author on 2026-11-03, after trying increment 22: built-in
values for an NPC's zone, behaviour and sprite led to whether a workflow
that goes to places in a zone should itself be tied to that zone, as one
workflow can be followed by many NPCs; and to the scaffold for kinds of
workflow, starting with NPC behaviour and adding the zone loop.

Agreed by its author on 2026-11-03, before building, in answer to the
planning questions:

- **A workflow's zone is set on the workflow:** it runs in one zone, or
  in any zone, and its places and the NPCs that follow it are checked
  against that, not against whoever runs it.
- **Choice is balanced against broken references:** what fits is offered
  first and the rest greyed with the reason; a change that would break
  something asks first, lists it and flags it after; the game carries on
  past whatever is flagged.
- **References are kept by identity,** as
  [D4](../ARCHITECTURE.md#d4--identity-and-references) has kept them since
  increment 1, so renaming never breaks one: files hold IDs, and the
  editor shows names.
- **A character scattered through many zones,** such as an enemy type,
  follows a behaviour that runs in any zone and names no place markers,
  calling modules for what such characters share.
- **Kinds of workflow are scaffolded now:** NPC behaviour first, and the
  zone loop as the second kind.

Agreed by its author on 2026-11-03, before building, from the wireframes
on the canvas "Workflow canvas wireframes", rows "Workflows say where they
run" and "Kinds of workflow: NPC behaviour and the zone loop":

- **A workflow's tab** has a strip naming its kind, where it runs, who
  follows it or whose loop it is, and what it calls or what calls it, as
  NPC behaviour · Runs in Market · Followed by Clerk, Customer · Calls
  Fetch stock. With nothing selected, its inspector shows its name, its
  kind, a Runs in box that opens the zone picker, who follows it and who
  calls it.
- **The zone picker, asked for a workflow,** gives each zone's residents
  and how many of the workflow's places are in it, and offers Any zone
  where others are offered No zone. Choosing a zone that breaks something
  goes on to ask.
- **Changing where it runs** asks Run Shopkeeping in Harbour?: Market →
  Harbour; who follows it, each flagged until it follows a behaviour that
  fits; and the places it names in Market, each flagged until it is one in
  Harbour; with Cancel and Run in Harbour. Choosing Any zone asks the same
  way about its fixed places.
- **A place box in an any-zone workflow** offers New place input… and
  Another value… first, then the fixed places, under Or a fixed place:
  Haywire robot then runs in its zone.
- **An NPC's behaviour list** heads Run in Market, where Clerk lives, then
  Run in any zone, then, greyed, Run in other zones: Clerk can't follow
  them, each with its zone. No behaviour stays.
- **What doesn't fit is flagged on the node,** as increment 22's places
  are: a Call to a workflow that runs elsewhere, a place in the wrong zone,
  an NPC whose behaviour runs elsewhere, and a place no stairs lead to.
- **A zone loop's tab** looks as a behaviour's does, Zone loop · Runs in
  Market · Loop of Market; its inspector lists the values it has: its
  zone's, the world's, its locals, and a named NPC's, read only.
- **New ▾ makes an NPC behaviour or a zone loop,** in place of Workflow.
  Each kind has its own icon in the Workflows section, which both kinds
  share, with its folders.
- **Each kind's palette** offers only its nodes. A zone loop has no Go to,
  Move or Find NPC nearby, and its palette says so.
- **A zone's details gain a Zone loop box,** with Choose… and Open. Its
  list heads the zone's own loops, then any-zone loops with how many zones
  run them, then, greyed, other zones' loops. No zone loop stays.
- **Watch lists a zone loop first,** under its zone's name, with the path
  through its calls, what it is doing and its values.

Chosen while planning, for its author to check:

- **A workflow's kind is set when it is made and never changes.** Every
  workflow saved before kinds is an NPC behaviour.
- **A new workflow runs in any zone** until it is given a zone or a fixed
  place.
- **Choosing a fixed place in an any-zone workflow ties it to that
  place's zone** in the same step, as placing an NPC with no zone gives it
  one. When NPCs living elsewhere follow it, that asks first.
- **A behaviour that runs elsewhere can't be chosen for an NPC:** it is
  greyed with the reason. That is stricter than a place in another zone,
  which can still be chosen and is then flagged.
- **A Call offers only workflows of its own kind,** those running where
  it runs first, then those running in any zone; an any-zone workflow
  calls only any-zone ones. This narrows increment 21's any workflow can
  be called, so that adding a Go to to a shared module can never break a
  zone loop that calls it.
- **A zone has one loop,** chosen in its details as an NPC's behaviour
  is. A zone loop runs in one zone or in any zone, so one loop can serve
  many zones.
- **A zone loop runs while its zone is alive:** it starts when the Map's
  preview starts or the player arrives, runs over game time, pauses with
  the game, and stops when the player leaves, starting afresh, locals and
  all, on the way back. It does not run by turn.
- **A zone loop has no NPC of its own:** `zone.` is the zone it runs for;
  it sets zone and world values and its locals, and reads a named NPC's
  values without setting them.
- **A zone loop's sounds are heard across the zone,** at the cue's level,
  not from a spot.
- **Increment 22's move dialog changes with this:** moving an NPC no
  longer lists places, which now belong to its workflows. It says whether
  the NPC's behaviour runs in the new zone, and the NPC is flagged until it
  follows one that does.
- **Projects made before this increment give each workflow a zone** when
  they open: the zone its followers live in, when they all live in one;
  else the zone its fixed places are in, when they are all in one; else
  any zone, when it names no fixed place; else the zone most of its places
  are in, the rest flagged. It is saved at the next save, as increment 22's
  zones are.
- **Reaching a floor by the stairs is still checked for each NPC** that
  follows a workflow, from the floor it is placed on.
- **Workflows are format version 8,** holding their kind and zone, and
  zones version 6, holding their loop.
- **Locating for tests:** the Runs in box as "ui:runs-in", the change
  dialog's button as "ui:runs-in-change", New ▾'s items as
  "ui:new-behaviour" and "ui:new-zone-loop", a zone's Zone loop box as
  "ui:zone-loop", and a zone loop in Watch as "watching:<zone name>".

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. Kinds and zones in the file | A workflow's kind and the zone it runs in, workflow format 8, older workflows reading as NPC behaviours that run in any zone; the nodes each kind runs, a node a kind can't run refused; a zone's loop, zone format 6; edits that set a workflow's zone and a zone's loop | Engine tests save, read and upgrade both, and refuse a Go to in a zone loop; plug-in tests meet the undo contract | Planned |
| 2. Settling older projects | Opening a project gives each workflow its zone from its followers and places; the orchestrator's runners question answering a zone's loop too | An orchestrator test reopens a project saved before kinds and finds each workflow's zone | Planned |
| 3. Making each kind | New ▾ and the Workflows section's [+] making an NPC behaviour or a zone loop; each kind's icon; the palette and its search offering only a kind's nodes | View tests make each kind and read its palette | Planned |
| 4. Runs in | A workflow tab's strip; its inspector with nothing selected; the Runs in box opening the zone picker with Any zone, the picker giving residents and places for a workflow | View tests set a zone and any zone, and read the strip | Planned |
| 5. Changing where it runs | The question: who follows it or loops it, and the places left behind; the change in one undo step; a fixed place tying an any-zone workflow to its zone | View tests change where a workflow runs with and without breaking anything, choose a fixed place in an any-zone workflow, and undo | Planned |
| 6. Choosing what fits | An NPC's behaviour list, a zone's loop list and a Call's workflow list by kind and zone, the rest greyed; the Zone loop box in a zone's details; place lists keyed to the workflow's zone, an any-zone list offering New place input and Another value first | View tests read each list's order and refuse a greyed one with its reason | Planned |
| 7. What doesn't fit | Problems for a place outside the workflow's zone, a Call to a workflow of another kind or zone, an NPC whose behaviour runs elsewhere and a zone whose loop does; stairs still checked for each NPC; increment 22's move dialog saying whether the behaviour fits | View tests raise and mend each problem, and move an NPC whose behaviour runs only where it lived | Planned |
| 8. Running a zone loop | The runner running a loop with no body: values, waits, time switches, containers, calls, and sounds across the zone; the Map's preview and the Play tab running a zone's loop while the zone is alive | Runner tests run a loop that sets a zone value by the clock and plays a sound; a headless game test starts a loop with its zone and stops it on leaving | Planned |
| 9. Watching a zone loop | Watch listing a zone's loop first, under its name, with its path, what it is doing and its values | A view test watches a loop and reads its values | Planned |
| 10. Checks | `make check` and the real-window suite; a real-window test makes a zone loop for the sample zone, previews it and finds its value in Watch; another changes where the customer's behaviour runs and undoes it | The checks pass with no lint findings | Planned |

## Step results

None yet.

## Not in this increment

- **The other kinds of workflow** the [workflow
  design](../WORKFLOW_DESIGN.md#kinds-of-workflow) names: conversations,
  sprite workflows, object use and cut scenes.
- **Music and ambient-sound nodes for zone loops,** and handlers such as
  On enter and On exit.
- **A zone loop that runs by turn,** still open, and zone loops running
  while the player is elsewhere.
- **Changing a workflow's kind,** and modules usable by more than one
  kind, worked out from their nodes.
- **Filling a behaviour's inputs in an NPC's details,** for characters
  that need places of their own; meanwhile a small behaviour in each zone
  calls the shared one.
- **Moving without place markers:** an NPC's home, wandering near it,
  chasing or fleeing what it found, and a random spot nearby, which a
  behaviour running in any zone will want.
- **Characters that are not placed,** such as random enemies spawned in a
  zone; each is still an NPC placed once, and Duplicate NPC is still to
  come.
- **Built-in values and the Log node:** [increment
  24](0024-built-ins-and-log.md).
