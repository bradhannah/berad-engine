# Increment 33: Constants

**Status:** Planned

The thirty-third increment gives a game words and numbers that its designer
sets and the game never changes: **constants**. Today a sign's text, a
greeting or how long a clerk waits is typed into the node that uses it. To
change it later, the designer has to remember where it was typed and walk
the workflows to find it. A constant is declared once, in a list of its own
beside values, and nodes use it by name:
- its text runs from a sign to long dialogue, with colour, bold and italic;
- its whole numbers can stand wherever a node takes a typed-in number.

Constants are scoped as values are. An NPC's or a zone's constant can hold
a different value for each NPC or zone, edited on that NPC or zone, so one
shop workflow greets differently in every shop. Wherever constants and
values appear together, their paths say which they are, as `const.` and
`var.`.

It builds on:
- the values of [increment 15](0015-values-and-sensing.md);
- the speech of [increment 26](0026-speech-and-boxes.md);
- the World state window's parts from [increment 28](0028-signals.md);
- the value picker of [increment 29](0029-lines-state-and-picking.md);
- the Project window of [increment 32](0032-project-by-folders.md).

It takes the place of the text documents removed in [increment
31](0031-no-text-documents.md), and is held to the same strict checks as
[increment 1](0001-foundation.md).

## What it proves

1. **A designer changes a word or a number in one place:** a greeting used
   by three workflows, and a wait used by two, change everywhere when their
   constant changes, with nothing to hunt for.
2. **One workflow can say different things for different owners:** two
   clerks running the same shop workflow greet with their own text, each set
   on the clerk.
3. **Constants and values never get confused:** wherever both appear, a
   path reads `const.` or `var.`. A constant can't be set by the game. A
   used one can't be deleted.
4. **Long text reads well:** text of several paragraphs is written in a
   field made for it. It shows in the game with its colour, bold and
   italic.
5. **A typed-in value becomes a constant in one step:** Make a constant
   turns a node's number or text into a constant and uses it there.

## Decisions

Brad's rule for constants, given on 2026-11-06:

> As a rule, constants should be used where a game designer could imagine
> that a value within the workflow could be changed in the future - but
> doesn't want to walk through to hunt the hard coded value

Agreed by its author on 2026-11-06, while the Project window was redesigned
and in answer to the planning questions:

- **Constants are a list of their own,** separate from values, as values
  change while the game runs and constants never do.
- **They hold text and whole numbers,** with room for more types later.
  Text runs from a sign to long dialogue.
- **They are scoped as values are,** to the world, a zone or an NPC.
- **An NPC's or a zone's constant can differ for each one.** Each value is
  edited in two places:
  - on that NPC's or zone's inspector in the Project window;
  - in a table of every NPC or zone in the Constants window.
- **Paths say which they are, as `const.` and `var.`, wherever the two
  mix:** on nodes, in pickers, in message text and in logs. Inside their own
  lists, the prefix is left off.
- **Constants are a third part of World state,** beside Values and Signals,
  with a window of their own. On Home's places column, World state holds
  all three.
- **Messages, Check value and Set value can use them.**
- **They are deleted under the same rules as values.**
- **Their text takes markup** for colour, bold and italic, built in this
  increment. Pauses, and anything else that changes the flow, are left out.
- **Long text is not paged for now.**
- **Text documents are gone** ([increment 31](0031-no-text-documents.md)).

Chosen while planning, for its author to check:

- **Where they are kept:** in the world-state document, beside values and
  signals. That document moves to version 4, and a project's version 3
  document reads unchanged. Each NPC's or zone's own value is kept with the
  constant, so the inspector and the table edit the same thing. A constant
  is referred to by its ID, so renaming or moving it changes nothing that
  uses it.
- **Who has which value:** every NPC or zone has the constant's default
  until it is given its own.
  - `const.npc.…` reads the running NPC's own value.
  - `const.zone.…` reads the value of the zone the NPC is in.
- **Scopes:** world, zone and NPC only. A workflow's own locals have no
  constants, as a constant used in one workflow is still worth finding in
  the list.
- **A constant's type** is chosen when it is made, as a value's is.
- **Text:**
  - up to 4,000 characters, about a page;
  - its line breaks are kept;
  - it is written in a field of several lines, with buttons for bold,
    italic and colour.
- **Numbers:** whole numbers, as values' integers are.
- **Paths:**
  - All the places that write a path today, about ten, first share one way
    of writing it, and the prefix is added there.
  - Saved files refer by ID, so nothing saved changes.
  - The Log window reads paths with and without the prefix, so earlier
    logs still read.
- **Where constants stand in for typed-in values, by Brad's rule:**
  - **Text:** Say's, Show message's and Log's message, as a part of the
    message beside typed text and values.
  - **Values:** the operands of Set value, Check value and Find NPC nearby.
    Set value copies a constant's text, cut to a value's 200 characters.
    A constant longer than that is flagged on the node.
  - **Numbers:**
    - Wait's amount;
    - Move's tiles;
    - Find NPC nearby's distance;
    - Show message's and Shake screen's seconds;
    - each share of a Random choice.

    These take workflow format 10, where such a field holds its number or a
    constant. Workflows of format 9 read unchanged. A constant outside a
    field's limits is a problem on the node, and the game holds it to them.
  - **Left typed in:**
    - times of day, as constants hold no times;
    - names of ways out, containers and Together, and notes, as these label
      the workflow rather than the game.
- **Make a constant:** right-click any of these fields, or a typed part of
  a message, for Make a constant.
  - It asks for a name and scope, suggesting the world and a name from the
    node.
  - It declares the constant with what was typed, and uses it there.
  - Undo takes back both.
- **Where it is used:**
  - The Constants window lists each constant's uses, as the Values window
    does.
  - A constant in use can't be deleted, and the window says where it is
    used. An unused one is deleted at once, and Undo brings it back.
  - A value used in an operand now counts as used too. Today it doesn't,
    so such a value can be deleted.
- **The markup:**
  - `[b]bold[/b]`, `[i]italic[/i]` and `[color=gold]gold[/color]`. They nest.
  - A colour is `#rrggbb` or one of a few names: red, orange, gold, green,
    blue, purple, grey and white.
  - The field's buttons write the tags, so they need not be remembered.
  - Typed text in Say and Show message takes markup too, not only
    constants.
  - A value's text in a message is shown as written, never read as markup,
    as the game sets it.
  - A tag left open, or one not known, is shown as typed and flagged as a
    warning where it was written.
  - The Log window shows the text without its markup.
  - Reading time counts only the text shown.
- **Who draws the markup:**
  - The engine reads the markup into runs of text, each with its bold,
    italic and colour, so any game drawing text can use them.
  - The editor's Play tab draws them.
  - Bold uses the semibold face the editor has. Italic needs IBM Plex Sans
    Italic and SemiBold Italic added, under the same open font licence as
    the faces it uses now.
- **The sample shows constants without changing what it does:**
  - the shop's greeting becomes a constant with the same words;
  - the clerk's wait becomes a constant with the same seconds.

  The real-window tests that read them pass as before.

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. Constants kept | In the engine: a constant's ID, name, folder, scope, type, default and description; each NPC's or zone's own value; the world-state document at version 4, with an upgrade from version 3; checks on names, types, lengths and owners; looking up a constant's value for an owner | Unit tests upgrade a version 3 document, refuse a bad name, type, length or owner, and find an owner's own value or the default | Planned |
| 2. Constants edited | The edits to add, rename, move, describe and delete a constant, set its default, and set or clear an owner's value, each undone | Unit tests make each edit and undo it, and refuse a delete while the constant is used | Planned |
| 3. One way to write a path | Every path written by one function, with `const.` and `var.` wherever both can appear; the Log window reading both forms | Unit tests find prefixes on a node, in a picker and in a log, none in the Values list, and an older log still reading | Planned |
| 4. The Constants window | World state's third part: constants by scope and folder; a field of several lines with bold, italic and colour; a number field; a table of every NPC or zone with its own value; uses; deleting as for values; Constants under World state in the Project window | Unit tests make, rename, move and delete a constant, set an NPC's value in the table, and list a constant's uses | Planned |
| 5. On an NPC and a zone | A Constants section on an NPC's and a zone's inspector in the Project window, with each constant's value for it, editable, and the default shown when it has none of its own | A unit test sets an NPC's value on its inspector and finds it in the table, and clears it back to the default | Planned |
| 6. In messages and operands | The value picker offering constants beside values; constants as parts of messages; constants in the operands of Set value, Check value and Find NPC nearby; operand uses counted for values and constants | Unit tests put a constant in a Say and an operand, find both uses, and refuse to delete a value used only in an operand | Planned |
| 7. In number fields, and Make a constant | Workflow format 10; a constant in Wait's amount, Move's tiles, Find NPC nearby's distance, Show message's and Shake screen's seconds, and a Random choice's shares; a constant out of a field's limits flagged; Make a constant on those fields and on typed text | Unit tests read a format 9 workflow unchanged, use a constant in each field, flag one out of limits, and make a constant from a field and undo it | Planned |
| 8. In the game | Constants read as the game runs: an NPC's or zone's own value or the default, in messages, operands and number fields, with numbers held to their fields' limits | Unit tests run two NPCs with their own greetings, a Wait from a constant, and a Set value from a long text cut to 200 characters | Planned |
| 9. Markup | The engine reading markup into runs, with warnings for tags open or unknown; reading time from the text shown; the Play tab drawing bold, italic and colour, with Plex Sans Italic and SemiBold Italic added; the text field's buttons; the Log window without markup; a value's text never read as markup | Unit tests read nested tags, flag a bad tag, time a line by its shown text and strip markup in a log; a screenshot shows a line in bold, italic and colour | Planned |
| 10. Tests and checks | The sample's greeting and wait as constants; a real-window test that makes a constant, sets an NPC's own value on its inspector, uses it in a Say with markup and in a Wait, makes a constant from a number, plays and sees the line, and is refused deleting it while used; the real-window tests moved to the prefixed paths | `make check` passes, and so does the real-window suite | Planned |

## Step results

None yet.

## Not in this increment

- **Paging long text,** and pauses or anything else that changes the flow.
- **Constants of other types,** such as times, true or false, choices or
  lists. Times of day stay typed in.
- **Translations.** Constants would be where a game's words are translated,
  later.
- **Finding a constant's text from the Project's search.** The Constants
  window has its own search.
- **Drawing markup in a game outside the editor.** The engine gives such a
  game the runs, and it draws them its own way.
