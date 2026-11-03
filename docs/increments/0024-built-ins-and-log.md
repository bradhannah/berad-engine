# Increment 24: Built-in values and the Log node

**Status:** Planned

The twenty-fourth increment lets workflows read more of an NPC and say
what they are doing. Today an NPC's one built-in value is its name, and a
workflow cannot tell its author anything: Watch shows values and the node
an NPC is on, but nothing a workflow writes. Now every NPC has built-in
values for the zone it lives in, its behaviour and its sprite, beside its
name; **zone**, **workflow** and **sprite** join the value types, compared
by identity; a workflow can switch its own NPC's sprite; every value has a
text form; and a **Log** node writes a line of text and values into a log
in the Watch window. It builds on the values of [increment
15](0015-values-and-sensing.md), the comparisons of [increment
18](0018-comparisons.md), the value picker of [increment
19](0019-value-picker.md) and the kinds of [increment
23](0023-workflow-kinds.md), and is held to the same strict checks as
[increment 1](0001-foundation.md).

## What it proves

1. **Every NPC has its zone, behaviour and sprite as values:** built in,
   never declared, read by any workflow; its zone and behaviour are read
   only, set in its details.
2. **A zone, a workflow and a sprite are compared by identity:** Check
   value asks whether one is or is not another, chosen with the zone
   picker or a list, and renaming keeps the check.
3. **A workflow can change how its NPC looks:** Set value sets its own
   NPC's sprite, and the preview shows the new one at once.
4. **Every value can be read as text:** each type has a text form, and a
   text value can be set from a value of any type.
5. **A workflow can say what it is doing:** a Log node writes a line of
   text and values, and Watch lists the lines with the game time and the
   node that wrote each.

## Decisions

Asked for by its author on 2026-11-03, after trying increment 22:
`npc.george.zone`, `npc.george.behaviour` and `npc.george.sprite` as
built-in values, each with a text form, so a workflow can print debug
information.

Agreed by its author on 2026-11-03, before building, in answer to the
planning questions:

- **An NPC's zone is a value of a new zone type,** read only, as an NPC
  lives in one zone set in its details.
- **An NPC's behaviour is a read-only value** that refers to the workflow
  by identity, not to its name as text, so renaming the workflow keeps a
  check of it; its text form is the workflow's name.
- **A workflow sets its own NPC's sprite with Set value;** other NPCs'
  sprites are read only, as all their values are.
- **Debug information is printed by a Log node into Watch.**

Agreed by its author on 2026-11-03, before building, from the wireframes
on the canvas "Workflow canvas wireframes", row "Built-in values and the
Log node":

- **The Log node,** in the Annotate palette, shows LOG and its message,
  each value in it a chip. Its inspector's Message mixes text with values
  put in by Insert value…, shows how the line would read, and lists each
  type as text.
- **Watch gains a log:** the newest line at the foot, each with the game
  time, the line, and the workflow and node that wrote it, which opens the
  workflow with that node selected; the watched NPC's lines or every NPC's,
  and Clear.
- **The value picker lists This NPC · built in:** `npc.name`, `npc.zone`,
  `npc.behaviour` and `npc.sprite`, each with its type and whether its
  workflows can set it; another NPC's are read only.
- **Check value on a zone** offers is and is not and a Zone box that opens
  the zone picker; **on a workflow,** a Workflow with Choose… and Open.
- **Set value on a sprite** shows the sprite small with Choose…; a text
  value set from another value says the other's type, as text.

Chosen while planning, for its author to check:

- **The built-ins can't be declared, renamed or deleted,** and are listed
  under each NPC in the World state window, as its name is.
- **Zone, workflow and sprite are types for the built-ins only,** for
  now, as NPC and place were for locals at first. Check value compares one
  with a fixed zone, workflow or sprite, or with another value of its
  type, such as `npc.clerk.zone`.
- **A sprite set by a workflow stays until it is set again;** each NPC
  starts with the sprite in its details when the preview or the game
  starts.
- **A zone loop has none of these:** it has no NPC of its own, and reads a
  named NPC's built-ins as any workflow can.
- **Each type's text form:** a zone, workflow or sprite by its name; a
  place as Counter, Ground floor, Market; an NPC by its name value;
  integers in digits; true or false; a choice by its name; text as it is,
  without quotes. None is no zone, no behaviour, no sprite or none.
- **A Log's message is saved as text and value references,** each by its
  value's ID, so renaming a value keeps the message.
- **Log goes on at once,** as Set value does, and both kinds of workflow
  have it.
- **Watch keeps the last 500 lines,** each cut to 200 characters, and
  starts afresh with each preview. The Play tab's lines wait for a game's
  debug overlay.
- **Locating for tests:** a Log node as "node:log:<n>", Insert value… as
  "ui:insert-value", the log's lines as "log-line:<n>", and the states
  "log-lines=<count>" and "log-line=<text>".

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. New types and built-ins | The zone, workflow and sprite types, compared by identity; each type's text form; an NPC's zone, behaviour and sprite as built-in values, as its name is; the catalog giving them | Engine tests read each built-in, compare each type, and read every type as text | Planned |
| 2. Running them | The runner reading an NPC's built-ins and a named NPC's; Set value on its own sprite, another NPC's refused; a text value set from any value; the preview drawing a sprite once it is set | Runner tests set a sprite, refuse another NPC's and set text from a zone; a view test finds the preview drawing the new sprite | Planned |
| 3. Choosing them | The value picker's This NPC · built in; Check value's Zone box with the zone picker, Workflow with a list, and Sprite; Set value's sprite box, and text from another value | View tests check a zone and a workflow, set a sprite, and set text from a zone | Planned |
| 4. The Log node | Its kind in the Annotate palette of both kinds; its message of text and value references; its inspector with Insert value… and how the line would read; the runner writing lines | Workflow tests save and read a message; runner tests write a line holding each type; a view test inserts a value and reads the line | Planned |
| 5. The log in Watch | The lines with the game time, the NPC or zone and the node; the watched NPC's or every NPC's; Clear; opening a line's node | View tests read lines, show every NPC's, clear, and open a node | Planned |
| 6. Checks | `make check` and the real-window suite; a real-window test adds a Log to the customer's behaviour and a sprite change to the clerk's, previews the sample and reads the line in Watch | The checks pass with no lint findings | Planned |

## Step results

None yet.

## Not in this increment

- **Built-in values for zones,** such as a zone's name or its loop.
- **Zone, workflow and sprite values in declarations,** for the world,
  zones, NPCs and locals.
- **Changing an NPC's behaviour from a workflow.**
- **Log lines in a game,** in a debug overlay; and journal or quest-log
  notes for players.
- **A changed sprite in a saved game,** which waits for saving games.
