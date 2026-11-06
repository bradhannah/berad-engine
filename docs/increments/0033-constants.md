# Increment 33: Constants

**Status:** Planned

The thirty-third increment gives a game words and numbers that its designer
sets and the game never changes: **constants**. Today a sign's text, a
greeting or how long a clerk waits is typed into the node that uses it. To
change it later, the designer has to remember where it was typed and walk
the workflows to find it. A constant is declared once, in a list of its own
beside values, and nodes use it by name:
- its text runs from a sign to long dialogue, with colour, bold and italic,
  written in a sheet made for writing;
- its whole numbers can stand wherever a node takes a typed-in number.

Constants are scoped as values are. A template holds constants beside its
values, and every NPC or zone the template is on holds them too, each with
a value of its own. So one shop workflow greets differently in every shop,
and each shopkeeper's words are kept on the shopkeeper. Wherever constants
and values appear together, their paths say which they are, as `const.`
and `var.`.

It builds on:
- the values of [increment 15](0015-values-and-sensing.md), with their
  templates and each NPC's and zone's own;
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
2. **One workflow can say different things for different NPCs:** two
   clerks with the shop template run the same shop workflow and greet with
   their own words, each set on the clerk.
3. **Constants and values never get confused:** wherever both appear, a
   path reads `const.` or `var.`. A constant can't be set by the game. A
   used one can't be deleted.
4. **Long text reads well:** text of several paragraphs is written in a
   sheet beside a preview of how the game draws it. It shows in the game
   with its colour, bold and italic.
5. **A typed-in value becomes a constant in one step,** and the other
   places where the same value was typed in are found for you: Make a
   constant uses it where it was made, then offers the rest.

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
  - in the Constants window, beside every NPC or zone that holds it.
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

Agreed by its author on 2026-11-06, after a second pass over the plan and
its wireframes:

- **Constants follow templates, as values do.**
  - A template holds constants beside its values. Every NPC or zone it is
    on holds them, with the default until given a value of its own.
  - That value is kept in the NPC's or zone's own file, so it is copied
    and deleted with it.
  - An NPC or a zone can also have constants of its own.
- **Long text is written in a sheet.** Write… opens a large sheet over the
  window: the text, the bold, italic and colour buttons, and a preview drawn
  as the game draws it, side by side.
- **Make a constant offers the other places typed the same.** After making
  a constant from a field, the editor lists the same field with the same
  value typed in other workflows. You tick those that mean the same thing
  and switch them to the constant, and one Undo takes back all of it.

Chosen while planning, for its author to check:

- **Where they are kept:**
  - In the files, constants are declared where values are: the world, a
    template, or an NPC's or zone's own. They sit in the same lists,
    marked as constants.
  - That is what lets a template hold both, and lets a folder such as
    `shop` hold both, each window showing its own. In the editor they stay
    a separate list, window and picker section, as agreed.
  - Nothing that reads values sees a constant. A name can't be both a
    value and a constant in the same place.
  - A constant is referred to by its ID, so renaming or moving it changes
    nothing that uses it.
  - The world-state document moves to version 4, NPCs to 6 and zones to 7.
    Earlier files read unchanged.
- **Each NPC's or zone's own values:**
  - Kept in its own file, for the constants of the templates on it.
  - A value is kept while its template is taken off, and counts again if
    the template goes back on.
  - A value for a constant that has been deleted is ignored, so Undo
    brings both back.
  - An NPC's or zone's own constant has one value, its default.
- **Who reads which value,** as for values:
  - `const.npc.…` reads the running NPC's value.
  - `const.zone.…` reads the value of the zone it is in.
  - Another NPC's or zone's value can be read by its name, or from the NPC
    found nearby.
  - A workflow whose NPC doesn't hold a constant it reads is treated as
    for a value it doesn't hold.
- **Scopes:** the world, NPC and zone templates, and each NPC's and zone's
  own. A workflow's locals have no constants.
- **A constant's type** can change while nothing uses it, as a value's
  can.
- **Text:**
  - up to 4,000 characters, about a page;
  - its line breaks are kept.
- **Numbers:**
  - Whole numbers, as values' integers are.
  - In a field with a unit, such as Wait's minutes, the constant is the
    amount, and the unit stays on the node. A constant's name is best to
    say its unit, as `restock_minutes` does.
- **Paths:**
  - All the places that write a path today, about ten, first share one way
    of writing it, and the prefix is added there.
  - `var.` marks the world's, zones' and NPCs' values, built-in ones too,
    as `var.npc.name`.
  - A workflow's locals keep `local.`, as no constant is local.
  - Saved files refer by ID, so nothing saved changes.
  - The Log window reads paths with and without the prefix, so earlier
    logs still read.
- **Where constants stand in for typed-in values, by Brad's rule:**
  - **Text:** Say's, Show message's and Log's message, as a part of the
    message beside typed text and values.
  - **What a node checks:** Check value and Find NPC nearby can check a
    constant itself, so one workflow can branch on an NPC's own constant.
    Set value only ever sets a value.
  - **Values given:**
    - the operands of Set value, Check value and Find NPC nearby;
    - a Call's inputs.

    Set value copies a constant's text, cut to a value's 200 characters.
    A constant longer than that is flagged on the node.
  - **Numbers:**
    - Wait's amount;
    - Move's tiles;
    - Find NPC nearby's distance;
    - Say's, Show message's and Shake screen's seconds;
    - each share of a Random choice.

    These take workflow format 10, where such a field holds its number or a
    constant, saved as an operand saves a value it refers to. Workflows of
    format 9 read unchanged. A constant outside a field's limits is a
    problem on the node, and the game holds it to them.
  - **Offered by type:** a picker offers only constants of a type the
    field takes.
  - **Left typed in:**
    - times of day, as constants hold no times;
    - names of ways out, containers and Together, and notes, as these label
      the workflow rather than the game.
- **Make a constant:** right-click any of these fields, or select typed
  text in a message, for Make a constant.
  - It asks for a name, and where it goes: the world, an NPC template or a
    zone template. It suggests the world, and a name from the node.
  - It declares the constant with what was typed, and uses it there.
  - It then lists the same field, in the same kind of node, with the same
    value typed in, in other workflows, such as other Waits of 30 minutes.
    None is ticked at first. Typed text is matched exactly.
  - One Undo takes back the constant and every switch.
- **Where it is used:**
  - The Constants window lists each constant's uses, as the Values window
    does.
  - A constant in use can't be deleted, and the window says where it is
    used. An unused one is deleted at once, and Undo brings it back.
  - A value used in an operand now counts as used too: in Set value's To,
    Check value's With, and and Among, and Find NPC nearby's Is. Today
    those uses aren't counted, so such a value can be deleted.
- **Finding a constant:** Find a constant matches a path, or any words in a
  constant's text, so a line can be found by what it says.
- **The writing sheet:**
  - It opens from Write… in the Constants window's inspector, and on an
    NPC's or a zone's inspector in the Project window.
  - For a template's constant, a list in the sheet switches between the
    default and each NPC's or zone's own value, to write them in turn.
  - The inspector shows the preview and how many characters are used.
- **Long text, until paging comes:** two warnings, where the text is used.
  - A Say or Show message that goes when its text has been read is flagged
    when the text needs more than the most reading time, ten seconds, as
    it would go before it was read. The warning suggests a Show message
    that goes when the player closes it.
  - A message whose text could be longer than one box holds is flagged, as
    paging is not built yet.
- **The markup:**
  - `[b]bold[/b]`, `[i]italic[/i]` and `[color=gold]gold[/color]`. They nest.
  - A colour is `#rrggbb` or one of a few names: red, orange, gold, green,
    blue, purple, grey and white. A named colour reaches the game as its
    name, so a game can draw it from its own palette.
  - The buttons write the tags, so they need not be remembered.
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
  - The Play tab, where the game runs today, draws them, and so do the
    sheet's and the inspector's previews.
  - Bold uses the semibold face the editor has. Italic needs IBM Plex Sans
    Italic and SemiBold Italic added, under the same open font licence as
    the faces it uses now.
- **The sample shows constants without changing what it does:**
  - the clerk's "Shop's open!" comes from a constant in the shop template;
  - the clerk serves for a world constant's minutes, the same as now.

  The real-window tests that read them pass as before.

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. Constants kept | In the engine: text and number constants declared in the world, in templates, and as an NPC's or zone's own; each NPC's and zone's own values in its file; world state at version 4, NPCs at 6 and zones at 7, each upgraded from the last; checks on names, types, lengths and owners; a constant's value for an NPC or zone | Unit tests upgrade each file, refuse a bad name, type or length, refuse a name taken by a value, and find an NPC's own value, the default, and a value kept while its template is off | Planned |
| 2. Constants edited | The edits to add, rename, move, describe, retype and delete a constant and set its default; and to set or clear an NPC's or zone's own value, in its file; each undone | Unit tests make each edit and undo it, refuse a retype or delete while the constant is used, and bring a deleted constant's values back with Undo | Planned |
| 3. One way to write a path | Every path written by one function, with `const.` and `var.` wherever both can appear and `local.` kept; the Log window reading both forms | Unit tests find prefixes on a node, in a picker and in a log, none in the Values list, and an older log still reading | Planned |
| 4. The Constants window | World state's third part, laid out as Values is: the world, templates, zones and NPCs; adding constants and groups; Find a constant by path or words; the inspector with the preview, the count, each NPC's or zone's value for a template's constant, the uses and the type; deleting as for values; Constants under World state in the Project window | Unit tests make, rename, move, retype and delete a constant, find one by its words, set an NPC's value from the inspector, and list a constant's uses | Planned |
| 5. The writing sheet | The sheet over the window: the text, the bold, italic and colour buttons, the preview drawn as the game draws it, the count, and a list switching between the default and each holder's value | A unit test writes a template constant's default and two NPCs' own values in one sheet, and its preview shows the bold, italic and colour | Planned |
| 6. On an NPC and a zone | A Constants section on an NPC's and a zone's inspector in the Project window: the constants of its templates and its own, each with its value or the default, opening the sheet, with Use the default | A unit test sets an NPC's value on its inspector, finds it in the Constants window, and clears it back to the default | Planned |
| 7. In messages, checks and values given | The value picker offering constants beside values, by type; constants as parts of messages; a constant checked by Check value and Find NPC nearby; constants in the operands of Set value, Check value and Find NPC nearby, and in a Call's inputs; operand uses counted for values and constants | Unit tests put a constant in a Say, a check, an operand and an input, find each use, and refuse to delete a value used only in an operand | Planned |
| 8. In number fields | Workflow format 10; a constant in Wait's amount, Move's tiles, Find NPC nearby's distance, Say's, Show message's and Shake screen's seconds, and a Random choice's shares; a constant out of a field's limits flagged | Unit tests read a format 9 workflow unchanged, use a constant in each field, and flag one out of limits | Planned |
| 9. Make a constant, and the other places | Make a constant on number fields and typed text; then the same field with the same value typed in other workflows, listed to tick and switch; one Undo for all of it | Unit tests make a constant from a Wait, find two other Waits typed the same, switch one, and undo all of it at once | Planned |
| 10. In the game | Constants read as the game runs: an NPC's or zone's own value or the default, another's by name or found nearby, in messages, checks, operands, inputs and number fields, with numbers held to their fields' limits | Unit tests run two NPCs with the same template and their own greetings, a Wait from a constant, a check on an NPC's constant, and a Set value from a long text cut to 200 characters | Planned |
| 11. Markup and long text | The engine reading markup into runs, with warnings for tags open or unknown; reading time from the text shown; the two long-text warnings; the Play tab and the previews drawing bold, italic and colour, with Plex Sans Italic and SemiBold Italic added; the Log window without markup; a value's text never read as markup | Unit tests read nested tags, flag a bad tag, time a line by its shown text, flag a long text that would go before it was read, and strip markup in a log; a screenshot shows a line in bold, italic and colour | Planned |
| 12. Tests and checks | The sample's greeting and serving time as constants; a real-window test that adds a constant to a template, writes an NPC's own value in the sheet from its inspector, uses it in a Say with markup, makes a constant from a Wait and switches another Wait to it, plays and sees the line, and is refused deleting it while used; the real-window tests moved to the prefixed paths | `make check` passes, and so does the real-window suite | Planned |

## Step results

None yet.

## Not in this increment

- **Paging long text,** and pauses or anything else that changes the flow.
- **Constants of other types,** such as times, true or false, choices or
  lists. Times of day stay typed in.
- **A value in a number field,** such as a Wait as long as a value says.
  The saved shape would allow it later.
- **Translations.** Constants would be where a game's words are translated,
  later.
- **Finding a constant's text from the Project's search.** Find a constant
  in the Constants window finds it by its words.
- **Constants in Watch,** beside an NPC's values.
- **A game of its own drawing markup.** The engine gives such a game the
  runs, and it draws them its own way. The Play tab is where the game runs
  today.
