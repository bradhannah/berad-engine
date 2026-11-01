# Increment 15: Values and sensing

**Status:** Planned

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
   reads another NPC's.
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
  world's and its locals. It can read the values of an NPC it found, but
  never set them.
- **Local values last through a workflow's repeats.** They clear when it
  restarts after a clock jump or an edit, and when its NPC's zone changes.
- **The first types are number, yes or no, and a list of choices.**
- **The first nodes are Set value, Check value and Find NPC nearby.** Find
  NPC nearby looks within a number of tiles, and can look only for NPCs
  whose value matches.
- **Messages wait for a later increment.**
- **NPCs act in a fixed order each tick, and each write is seen at once,**
  so a run repeats exactly.

Chosen while planning, for its author to check:

- **The window is the World state window** the [editor
  inventory](../EDITOR_FEATURES.md#editor-inventory) already names, rather
  than a new Values window.
- **An NPC value belongs to each placed NPC.** One NPC placed twice is two
  NPCs, each with its own values, as each runs its own workflow today.
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

Open, to settle before building:

- **Reading a named NPC's values,** such as `npc.george.mood`. An NPC placed
  more than once is several NPCs, so a name may not say which. Each NPC
  would also need a name fit for a path, apart from its display name. The
  steps below leave named reads out until this is settled.

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. Declarations | The world-state document: each value's ID, name, scope, type, choices, default and description, with a format version; name rules; edits to add, remove, rename and change a value, passing the shared undo test; where-used for values | A project's values can be declared, changed and undone, and save and load unchanged | Planned |
| 2. The World state window | Scopes, each scope's values with their type, default and how many workflows use them, and an inspector for the selected value; each change saved as it is made and undone with Cmd+Z | An author can declare and change values without leaving the editor | Planned |
| 3. Local values | Workflow logic format 4, upgraded from 3, with the workflow's local values; Local values in the inspector when no node is selected; their edits, passing the shared undo test | A workflow's locals can be declared, changed and undone | Planned |
| 4. The nodes in the editor | Set value, Check value and Find NPC nearby; a value parameter whose picker groups values by scope, offers only those a workflow may use, and can declare a new one; a parameter that follows the chosen value's type; problem marks for missing or mismatched values | Each node can be built and checked in the Workflow window, and a broken value is marked | Planned |
| 5. Running them | The engine's world state: values for the world, each zone and each NPC, and each NPC's locals; the three nodes in the runner; locals cleared on restart and on a change of zone; values in the preview stream and the Watch window | A shopkeeper serves a waiting customer in the preview, the same way every run, judged by its author | Planned |

## Step results

None yet.

## Changed after its author tried it

None yet.

## Not in this increment

- Messages between NPCs, and requests with replies.
- Scopes for pairs of NPCs, for places and for objects.
- The world-state categories of the [workflow
  design](../WORKFLOW_DESIGN.md#world-state): stats, provisions and
  inventory, and the flags, skills and capabilities of the world-state tree.
- Values in conversations and sprite workflows, and what an NPC is doing.
- Keeping values in game saves, whose format is still open.
