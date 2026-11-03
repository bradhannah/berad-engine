# Increment 24: Built-in values and the Log node

**Status:** Built

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
| 1. New types and built-ins | The zone, workflow and sprite types, compared by identity; each type's text form; an NPC's zone, behaviour and sprite as built-in values, as its name is; the catalog giving them | Engine tests read each built-in, compare each type, and read every type as text | Built |
| 2. Running them | The runner reading an NPC's built-ins and a named NPC's; Set value on its own sprite, another NPC's refused; a text value set from any value; the preview drawing a sprite once it is set | Runner tests set a sprite, refuse another NPC's and set text from a zone; a view test finds the preview drawing the new sprite | Built |
| 3. Choosing them | The value picker's This NPC · built in; Check value's Zone box with the zone picker, Workflow with a list, and Sprite; Set value's sprite box, and text from another value | View tests check a zone and a workflow, set a sprite, and set text from a zone | Built |
| 4. The Log node | Its kind in the Annotate palette of both kinds; its message of text and value references; its inspector with Insert value… and how the line would read; the runner writing lines | Workflow tests save and read a message; runner tests write a line holding each type; a view test inserts a value and reads the line | Built |
| 5. The log in Watch | The lines with the game time, the NPC or zone and the node; the watched NPC's or every NPC's; Clear; opening a line's node | View tests read lines, show every NPC's, clear, and open a node | Built |
| 6. Checks | `make check` and the real-window suite; a real-window test adds a Log to the customer's behaviour and a sprite change to the clerk's, previews the sample and reads the line in Watch | The checks pass with no lint findings | Built |

## Step results

- **New types and built-ins:** zone, workflow and sprite are value types
  that hold a document by its ID, read only for the built-ins and not
  offered when declaring a value. Every NPC has four built-in values,
  `npc.name`, `npc.zone`, `npc.behaviour` and `npc.sprite`, each starting
  as its details say, listed under each NPC in the World state window as
  its name is, and never declared, renamed or deleted. Zone, workflow and
  sprite compare by is and is not. Each type has a text form: a zone, a
  workflow or a sprite by its name; a place as Counter, Ground floor,
  Market; an NPC by its name value; integers in digits; true or false; a
  choice by its name; text as it is. None reads no zone, no behaviour, no
  sprite, no place or none. Verified: engine tests read each NPC's
  built-ins, compare each new type and read every type as text.
- **Running them:** the runner reads an NPC's built-ins, and a named
  NPC's. Set value sets its own NPC's sprite, which stays until set again;
  its zone and behaviour, and another NPC's values, are refused, the node
  carrying on and saying why. A text value can be set from a value of any
  type, as its text form, cut to the text's limit. The Map's preview draws
  an NPC's sprite once its workflow sets it, and the Play tab draws it
  too, its sprites loaded before the game starts. Verified: runner tests
  set a sprite, refuse the zone and set text from a zone, as its name; a
  view test finds the preview drawing Apron once the clerk's workflow sets
  it.
- **Choosing them:** the value picker lists an NPC's built-ins under THIS
  NPC, each marked built in, or built in · read only where its workflows
  can't set it; Set value never offers `npc.zone` or `npc.behaviour`.
  Check value on a zone offers is and is not and a Zone box that opens the
  zone picker; on a workflow or a sprite, a box that opens a list of the
  project's.
  Set value on a sprite shows its box; a text value set from Another
  value takes a value of any type, as text. A node names the zone,
  workflow or sprite it holds by its name: Set npc.sprite to Apron. A
  Set value on its NPC's zone or behaviour is flagged: An NPC's zone and
  behaviour are set in its details, not by a workflow. Verified: view
  tests check a zone against Town with the zone picker, set the sprite to
  Apron from the list, flag a Set value on the zone, and set text from
  `npc.zone`.
- **The Log node:** Log is in the Annotate palette of both kinds of
  workflow, under Note. Its message is saved as text and values, each
  value by its reference, at most 32 parts and 200 characters of text.
  Its inspector's Message field shows each value as its path in braces,
  as Shopping as {npc.name}; Insert value… opens the value picker over
  every value the workflow can read, and puts the one picked where the
  caret was, saving the message; a path can be typed too, and one that
  names no value is refused, the status saying so. Under it, As it would
  read for Customer gives the line as it would read for the first NPC
  following the workflow, with each value as it starts, and Each type as
  text lists the text forms. The node shows LOG and its message, each
  value a chip holding its path. A value it uses that was deleted, or that
  an NPC following it lacks, flags it, and where-used finds the values it
  writes. The runner writes each Log's line as it passes, each value read
  then, a value it can't read written (no such value), the line cut to 200
  characters; and goes on at once. The sample clerk's day logs Serving
  and the customer it found. Verified: a workflow test saves and reads a
  message and refuses one of mixed parts, too many parts or too much text;
  runner tests write a line holding a value of each type, cut a long one,
  and write nothing while looping without waiting; a plug-in test lists a
  Log's values as uses; a view test types a message, inserts
  `local.count`, reads it as Count is 0, types `{npc.zone}`, reads it as
  Town, and refuses `{local.nope}`.
- **The log in Watch:** the preview numbers each line, with the game
  time, the NPC or the zone loop that wrote it, and the workflow and node;
  its state carries the latest 50. Watch keeps the last 500, starting
  afresh with each preview and kept after it stops, in LOG under the
  workflow: the newest at the foot, six at a time, the wheel scrolling
  back. Its first button, named for the watched NPC, shows that NPC's
  lines; Every NPC shows everyone's, each with whose it is; with no NPC
  watched it shows everyone's. Clear empties it. Each line ends with the
  workflow and node that wrote it, as Customer shops · Log, which opens the
  workflow with that node selected and in the middle of the canvas.
  Verified: a map test finds the clerk's line in the preview's state, with
  its number, placement, workflow and node; a Watch test keeps lines once
  each, shows the watched clerk's and every NPC's, opens a line's node,
  clears, keeps 500 and starts afresh; a workflow view test selects the
  node it is opened at.
- **Checks:** `make check` passes with no lint findings. A real-window
  test is new: it adds a Log to the customer's behaviour, typing Shopping
  as and inserting `npc.name`, and links it after the start; adds a Set
  value to the clerk's day that sets `npc.sprite` to Lantern; and previews
  the sample. The Map shows the clerk as the lantern; once the customer
  stands at the counter it is watched, Watch reads Shopping as Customer,
  and the line opens Customer shops with the Log selected. The tests that
  count the clerk's day's nodes count 20, with its Log. The whole
  real-window suite, 53 tests, passed.

## Changed while building, short of the wireframes

- **The value picker marks the built-ins row by row:** they are listed
  under THIS NPC, each marked built in or built in · read only, not under
  a This NPC · built in heading of their own.
- **A workflow or a sprite is chosen from a list in a box:** Check value
  on a workflow has no Open button, and Set value's sprite box names the
  sprite rather than drawing it small.
- **The Message field holds text, not chips:** each value shows as its
  path in braces, which can be typed as well as put in by Insert value…;
  the node on the canvas draws the chips. A value known only as the
  workflow runs, as the NPC a local holds, reads as its path in the line
  as it would read.
- **Lines from a loop that never waits are dropped,** as its sounds are,
  so it cannot flood the log; the NPC's status still says it is looping.

## Not in this increment

- **Built-in values for zones,** such as a zone's name or its loop.
- **Zone, workflow and sprite values in declarations,** for the world,
  zones, NPCs and locals.
- **Changing an NPC's behaviour from a workflow.**
- **Log lines in a game,** in a debug overlay; and journal or quest-log
  notes for players.
- **A changed sprite in a saved game,** which waits for saving games.
