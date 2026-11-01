# Increment 15: Values and sensing

**Status:** Built

The first world state, and the first way for NPCs to be aware of each other.
An author declares typed values in the World state window, or on a workflow
as its own local values. Workflows set and check those values, and find
another NPC within a number of tiles and read its values. It builds on NPC
workflows ([increment 3](0003-npc-workflows.md)) and is the scaffold the
[workflow design](../WORKFLOW_DESIGN.md) asks for: messages, more scopes and
the world-state categories build on it later. It is held to the same strict
checks as [increment 1](0001-foundation.md).

## What it proves

1. **Values are declared once, with a type, in the editor:** a node offers
   only declared values, and a value's type decides what can be written to
   it, without new engine code.
2. **NPCs become aware of each other:** a shopkeeper finds a customer
   waiting within two tiles and serves them, and the customer sees it is
   being served, built only from nodes.
3. **No NPC reaches into another:** a workflow sets only its own values, and
   reads another NPC's, found nearby or named.
4. **Runs repeat exactly:** the same preview gives the same values and the
   same choices every time.

## Decisions

Agreed by its author on 2026-10-31, before building:

- **Every value is declared before it is used,** with a name, a type, a
  scope and a default, so every reader knows its type and a typo cannot make
  a second value.
- **Four scopes:** `local.` belongs to one workflow, `npc.` to the NPC the
  workflow runs for, `zone.` to the zone it is in, and `world.` to the whole
  game. Pairs of NPCs, places and objects can become scopes later without
  changing these.
- **A declaration covers its whole scope:** declaring `mood` for NPCs gives
  every NPC its own `mood`. Nothing is declared for one NPC alone.
- **Names use lower-case letters, digits and underscores,** so a name such
  as `george's` is refused.
- **World, zone and NPC values are declared in one window,** saved as one
  project file. Local values are declared on their own workflow.
- **A workflow sets only its own values:** its NPC's, its zone's, the
  world's and its locals. It can read the values of an NPC it found, or of
  one it names, such as `npc.george.mood`, but never set them.
- **Local values last through a workflow's repeats.** They clear when it
  restarts after a clock jump or an edit, and when its NPC's zone changes.
- **The first types are number, yes or no, and a list of choices.**
- **The first nodes are Set value, Check value and Find NPC nearby.** Find
  NPC nearby looks within a number of tiles, and can look only for NPCs
  whose value matches.
- **Messages wait for a later increment.**

Agreed by its author on 2026-10-31, settling named reads:

- **An NPC's values belong to the NPC,** one set shared wherever it is
  placed, so George carries his mood from zone to zone. `npc.george.mood` is
  George's, with no numbering of placements and no `.all`.
- **An NPC is placed at most once in a zone,** and one zone runs at a time.
  A character that repeats, such as a guard, is several NPCs sharing one
  behaviour, and later one module.
- **Each NPC has a path name, such as `george`,** unique in the project and
  following the rules for value names.
- **NPCs act in a fixed order each tick, and each write is seen at once,**
  so a run repeats exactly.

Chosen while planning, for its author to check:

- **The window is the World state window** the [editor
  inventory](../EDITOR_FEATURES.md#editor-inventory) already names, rather
  than a new Values window.
- **An NPC's path name is suggested from its display name** when it is
  made, and can be changed. Files keep the NPC's ID, so a rename changes no
  workflow.
- **A project that already places an NPC twice in a zone opens,** with the
  zone marked as a problem; the Map window refuses a new second placement.
- **An NPC in a zone that is not running keeps its values.** In the
  preview, which runs one zone, that means its defaults until saves exist.
- **A fourth type, NPC, is for local values only.** Find NPC nearby puts
  what it finds in one, such as `local.customer`. Keeping an NPC in a saved
  value waits until saves are designed.
- **Files keep each value's ID,** so renaming a value changes no workflow.
  Deleting a value that is in use, or changing its type, marks each node it
  breaks as a problem.
- **Any picker can declare a new value,** with its scope and type filled in,
  so an author need not leave the workflow.
- **Set value sets a value, or adds to a number.** Check value compares a
  number with "is", "at least" or "below", and a yes or no or a choice with
  "is"; it takes "yes" or "otherwise".
- **"Within N tiles" is a square around the NPC on its own floor,** walls
  ignored. Find NPC nearby takes the nearest match, ties going to the first
  in placement order, and never finds the NPC itself.
- **In the preview, values start at their defaults,** and last until the
  preview stops. Game saves keep them once saves are designed.
- **The Watch window shows the watched NPC's values,** its local values, and
  the zone's and the world's.

Chosen while building, for its author to check:

- **A chooser declares a value from what is typed into it.** Typing a name
  no value has offers to declare it in each scope the node may use; it
  starts as a number, or as an NPC for where Find NPC nearby keeps what it
  finds, and its type is changed in the World state window or Local values.
- **Add value and Add local make a number named value,** then value_2 and
  so on, to be named in the inspector; a new list starts with two choices,
  first and second.
- **Where a value is used shows in its inspector,** in the World state
  window, rather than as a count beside every value in the list.
- **Find NPC nearby looks for an NPC's own value,** such as npc.waiting of
  the NPC it is looking at, and clears the local it keeps NPCs in when it
  finds none.
- **A value node that cannot do its work carries on,** as other nodes do:
  Set value goes on to the next node, and Check value and Find NPC nearby
  take "otherwise" and "none found". The Watch window says why.
- **NPCs act on the others as each tick began:** Find NPC nearby sees where
  the zone's NPCs stood at the start of the tick, and their values as they
  are, set by NPCs earlier in placement order.
- **Two NPCs given one path name are not refused,** as an edit changes one
  NPC at a time; the project browser warns about both. An NPC upgraded from
  version 1 has no path name until it is given one, with a suggestion shown.
- **A zone that already places an NPC twice** is marked on each such
  placement in the Map window's inspector, rather than in a list of the
  zone's problems, which the editor does not have yet.
- **Value nodes are titled as sentences on the canvas,** such as "Set
  npc.waiting to 0" and "Is npc.mood calm?", so what they set or ask fits
  the node; Check value and Find NPC nearby list their outputs as rows.
- **The Watch window lists the watched NPC's values** beside its workflow:
  its workflow's locals, a local holding an NPC by that NPC's name, then its
  own values, its zone's and the world's.
- **In the preview, values start at their defaults,** and a value's
  declaration changed during a preview applies at once, keeping what was
  set where it still fits.

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. Declarations | The world-state document: each value's ID, name, scope, type, choices, default and description, with a format version; name rules; edits to add, remove, rename and change a value, passing the shared undo test; where-used for values | A project's values can be declared, changed and undone, and save and load unchanged | Built |
| 2. NPC names and placements | NPC format 2, upgraded from 1, with a unique path name suggested from the display name; the Map window refusing a second placement of an NPC in one zone, and a problem for a zone that already has one | Every NPC has a unique path name, and no zone gains a second placement of one NPC | Built |
| 3. The World state window | Scopes, each scope's values with their type, default and how many workflows use them, and an inspector for the selected value; each change saved as it is made and undone with Cmd+Z | An author can declare and change values without leaving the editor | Built |
| 4. Local values | Workflow logic format 4, upgraded from 3, with the workflow's local values; Local values in the inspector when no node is selected; their edits, passing the shared undo test | A workflow's locals can be declared, changed and undone | Built |
| 5. The nodes in the editor | Set value, Check value and Find NPC nearby; a value parameter whose picker groups values by scope, offers only those a workflow may use, including other NPCs' values to read, and can declare a new one; a parameter that follows the chosen value's type; problem marks for missing or mismatched values | Each node can be built and checked in the Workflow window, and a broken value is marked | Built |
| 6. Running them | The engine's world state: values for the world, each zone and each NPC, kept with the NPC wherever it is placed, and each running workflow's locals; the three nodes in the runner; locals cleared on restart and on a change of zone; values in the preview stream and the Watch window | A shopkeeper serves a waiting customer in the preview, the same way every run, judged by its author | Built |
| 7. A shop to try, and deleting by right-click | The sample project's shop: its values, a clerk and a customer with their workflows, placed; a right-click menu on Workflow nodes and lines that deletes them | A new sample project previews the shop working; a right-click deletes a node or a line | Built |
| 8. Types, groups and templates | World-state format 2 and the upgrades to it: integer with optional limits, bool and text; groups; NPC and zone templates; each NPC's and zone's own values and applied templates in its own document, with an NPC's name; zone path names | Every kind of item can be declared, saved and read back unchanged, and older files upgrade | Built |
| 9. Editing them | Edits for groups, templates, applying them, and NPCs' and zones' own values, passing the shared undo test; dotted names that declare their groups; deleting refused while anything in it is used | Each change can be made and undone, and a used value cannot be deleted | Built |
| 10. The World state window as a tree | World, Zone and NPC, with templates and each zone and NPC as parents; an inspector for each kind of item; limits and Default; delete warnings that say where things are used | An author can organise values into groups and templates, and apply templates, without leaving the window | Built |
| 11. Values in workflows | Workflow format 5; choosers that show paths in the tree and declare groups from a typed path; problems for an NPC or zone that lacks a value a workflow uses; Check value's "at its maximum" and "at its minimum"; text values | A guard behaviour shared by two guards uses a template's values, and a missing one is marked | Built |
| 12. Running them again | One placement per NPC in the whole project; the runner holding integers at their limits, text, templates and named zones; the preview, the Watch window's tree of values, and the sample shop using a template and a group | The sample shop runs with its values in a template, the same way every run | Built |

## Step results

All on the branch `feat/values-and-sensing`. Verified with `make check` (no
lint findings, every package passing with the race detector) and every
real-window test passing, the new tests' screenshots checked.

- **Declarations:** `engine/worldstate` declares values: an ID, a name of
  lower-case letters, digits and underscores, a scope, a type, a list's
  choices with IDs of their own, a default and a description, format 1. The
  world-state plug-in's add, change and remove edits pass the shared undo
  test, and the add edit gives a new value its ID (D4). Every project holds
  one world-state document with a fixed ID. Workflows refer to world-state
  values and to the NPCs they name, so where-used finds them.
- **NPC names and placements:** NPC format 2, upgraded from 1, adds a path
  name. A new NPC's is suggested from its name, unique among the project's
  NPCs; the project browser edits it, refuses punctuation and warns when
  another NPC uses it. The Map window refuses a second placement of one NPC
  in a zone, saying why.
- **The World state window:** opened from the project browser's World state
  row, it lists the world, zone and NPC scopes, each scope's values with
  their type and what they start as, and edits the selected one: its name,
  type, choices, starting value and description, with where it is used.
  Each change is saved as it is made and undone with Cmd+Z; typed text waits
  for Enter or Save. Its form is shared with Local values. A real-window test
  declares npc.mood, names it and makes it a list.
- **Local values:** workflow format 4, upgraded from 3, holds a workflow's
  local values; a local can also be an NPC. Their edits pass the shared undo
  test. With nothing selected, the Workflow window's inspector lists them,
  adds one and edits it.
- **The nodes in the editor:** Set value, Check value and Find NPC nearby
  in a new Values group of the palette. A value parameter's chooser groups
  the values a node may use: a Set value only its own, a Check value also
  the values of the NPC a local holds and of each NPC with a path name. An
  operand follows its value's type: a number to type, no and yes, or a
  button for each choice. A missing value, a value of the wrong type for
  what is compared or set, and setting another NPC's value are marked as
  problems. A real-window test declares npc.served from a chooser, sets it,
  checks it and adds a local value.
- **Running them:** `worldstate.Store` holds the world's, each zone's and
  each NPC's values, the NPC's by its ID, wherever it is placed; a value
  never set is its default. The runner sets, checks and finds, reads another
  NPC's values but never sets them, keeps locals through repeats and clears
  them on a restart; a new runner, as in another zone, starts with none. The
  Map window's preview gives every NPC the store, its zone and the others in
  placement order, and the Watch window lists the watched NPC's values.
  Runner tests cover setting and adding, the zone's value, the found and the
  named NPC, a customer three tiles away never found, and the same values
  every run. A real-window test builds a customer and a clerk from nodes
  alone, places them side by side and previews them: the customer finds the
  clerk serving and stops waiting.

- **A shop to try, and deleting by right-click:** the sample project
  declares the shop's values and places a clerk and a customer, each with
  its own workflow; the Workflow window's right-click menu deletes a node or
  a line. A real-window test previews the sample and waits for the world to
  count a customer served.
- **Types, groups and templates:** world-state format 2, upgraded from 1,
  holds groups, templates and the integer, bool and text types; NPC and zone
  format 3 hold each one's own values and applied templates, and zones a
  path name. `worldstate.Catalog` finds a value for the world, an NPC or a
  zone: its own, an NPC's name, or a template's applied to it.
- **Editing them:** one change edit for any list of declarations, and edits
  that apply templates and set a zone's path name, all passing the shared
  undo test.
- **The World state window as a tree:** World, Zone and NPC, each scope's
  templates, and each NPC and zone with its own values, its name and its
  templates; forms for values, groups and templates with limits and
  Default; path names; applying templates; deleting refused while used,
  saying where.
- **Values in workflows:** workflow format 5, upgraded from 4; choosers
  offering what a workflow's NPCs have and declaring into groups and
  templates from a dotted name; a problem for an NPC following the workflow
  without a value it uses; Check value's "at its maximum" and "at its
  minimum".
- **Running them again:** the runner holds integers at their limits, reads
  named zones and sets text; the preview builds the catalog from every
  NPC's and zone's document; the Watch window lists grouped paths; an NPC
  is placed once in the whole project. A real-window test builds a shop in
  the editor's windows alone, a template made and applied in the World
  state window, and previews it; the sample shop uses a template and a
  group.

## Changed after its author tried it

Settled by its author on 2026-11-01, after trying it:

- **The sample project has a shop to try:** a clerk and a customer, each
  with its own workflow, and the values they use.
- **Right-clicking a Workflow node or line opens a menu that deletes it,**
  as Backspace does. The start node cannot be deleted.
- **Values are organised in groups,** so `world.quests.dragon_slain` sits
  in the group `quests`, and the World state window shows a tree. A group is
  declared, with a name and a description, and stays until it is deleted.
  Typing a dotted name, in the window or in a chooser, declares the groups
  it needs. World, zone and NPC values can be grouped; local values stay
  flat, belonging to one workflow.
- **Each NPC and each zone owns its values,** listed under it in the tree:
  `npc.george.secret` is George's alone. A **template** is a named set of
  values, such as `guards` holding `how_many_arrested`; applied to an NPC it
  gives that NPC the set, as `npc.guard1.guards.how_many_arrested`, and
  editing the template changes it for every NPC it is applied to. Zones
  have templates too. This replaces declaring an NPC or zone value for every
  NPC or zone at once.
- **A workflow names its own NPC's values without the NPC's name,** as
  `npc.guards.how_many_arrested`, so one behaviour serves every NPC with the
  template; an NPC following a workflow that lacks a value it uses is marked
  as a problem. Another NPC's values are read, never set, by name, and a
  zone's the same way, as `zone.market.crowded`, so zones have path names
  as NPCs do.
- **Every NPC has a `name` value,** text that starts as its display name
  and that workflows may set, such as a name the player learns.
- **The types are integer, bool, choice and text,** with NPC for local
  values only. An integer has an optional minimum and maximum, and a Set
  value or Add that would pass one holds it at the limit; Check value asks
  whether it is at its maximum or its minimum. A bool is true or false.
  Every type's starting value is called its **Default**.
- **Deleting is refused while anything is used:** a value, a group or a
  template that a workflow uses cannot be deleted, and the window says
  where it is used. Otherwise deleting a group or a template warns how many
  values go with it, and Undo brings them back. An NPC's or zone's place in
  the tree goes only when the NPC or zone is deleted.
- **An NPC is placed once in the whole project,** keeping things simple. An
  NPC that seems to travel is several NPCs, shown or hidden by values.
  This replaces placing one NPC once in each zone.

Chosen while building these changes, for its author to check:

- **One change edit adds, changes and removes declarations together,** all
  or none, so a dotted name's groups and its value, or a group with what it
  holds, are one undo step. New declarations carry temporary IDs that the
  orchestrator makes permanent (D4).
- **A name for one NPC's or zone's own values cannot be a template's
  applied to it,** and a template cannot be applied while the NPC or zone
  has its own value or group of that name, so no path means two values.
- **Deleting a value nothing uses happens at once,** as Undo brings it back;
  a group or template asks first, saying how many values go with it. A
  template applied to any NPC or zone counts as used, and the refusal names
  where.
- **In the World state window, NPCs and zones start closed,** as a project
  has many; groups and templates start open. An NPC's or zone's path name is
  edited there as well as in the project browser, and a new zone gets one
  suggested from its name; the sample zone is sample_zone.
- **Moving a value into or out of a group is renaming it,** with a dotted
  name from the top of its scope or its template: quests.dragon_slain moves
  it into quests, and dragon_slain moves it back out.
- **A chooser's "this NPC" values are those the workflow's NPCs have:** the
  name, their templates' values and their own. While no NPC follows the
  workflow, it offers every template's values. A dotted name starting with a
  template declares into the template; otherwise it can be one following
  NPC's own.
- **Only NPCs are checked for lacking a value a workflow uses.** A zone value
  is checked for being declared somewhere; checking each zone a workflow's
  NPCs are placed in is left for later.
- **An integer with no limits is held within a thousand million million
  either side of nought,** in practice no limit. Text is at most 200
  characters and is shown in quotes.
- **A world state from before the change is upgraded:** values declared for
  every NPC or every zone move into a template named shared for each,
  applied to nothing until its author applies it, as the world state cannot
  see the NPCs and zones.
- **Placing an NPC asks where it is placed already,** and is refused,
  naming the floor, if any floor in the project places it.

## Not in this increment

- Messages between NPCs, and requests with replies.
- Scopes for pairs of NPCs, for places and for objects.
- The world-state categories of the [workflow
  design](../WORKFLOW_DESIGN.md#world-state): stats, provisions and
  inventory, and the flags, skills and capabilities of the world-state tree.
- Values in conversations and sprite workflows, and what an NPC is doing.
- Placement conditions, such as an NPC appearing in a new zone after an
  event.
- Keeping values in game saves, whose format is still open.
- Checking that each zone a workflow's NPCs are placed in has the zone
  values it uses.
- Giving one NPC its own default for a template's value.
