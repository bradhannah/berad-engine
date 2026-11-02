# Increment 19: The value picker

**Status:** Planned

The nineteenth increment moves choosing a value out of the inspector into
a **picker**, a dialog over the window. Today a chooser is a search field
and a list squeezed into the inspector's width; as a project's values grow,
across world groups, templates, each NPC and each zone, the list gets too
long and too narrow to search. The picker has room: a search box matching
any part of a path, type chips, a scope rail with counts, results with
their type and description, groups that fold when nothing is typed, and
declaring a value that matches nothing. The inspector keeps only the box,
and every chooser opens the same picker. It builds on [increment
15](0015-values-and-sensing.md)'s choosers and [increment
18](0018-comparisons.md)'s Another value, follows the agreement in [editor
features](../EDITOR_FEATURES.md#workflows), and is held to the same strict
checks as [increment 1](0001-foundation.md).

## What it proves

1. **One picker for every chooser:** Set value's and Check value's Value,
   Find NPC nearby's Only one whose and Keep it in, and an integer
   operand's Another value all open it, each with its own rules about which
   values are offered, as the choosers have today.
2. **Searching has room:** the search box is focused on open and matches
   any part of a path; type chips narrow by type and are locked where the
   type must match; the scope rail counts matches by scope and narrows to
   one; results show path, type and description.
3. **A long list is a tree:** with nothing typed, groups fold as the World
   state tree does, arrows open them, and the last group opened is
   remembered within the session.
4. **Declaring is still one step:** a name that matches nothing is declared
   from the picker, with where, type and starting value, and chosen at
   once, as the chooser does today.
5. **The keyboard is enough:** Enter on the focused box opens the picker,
   typing searches, arrows move and open groups, Enter picks, Esc closes.

## Decisions

Asked for by its author on 2026-11-01, paraphrased: choosing a value gives
no dropdown; the list could get so big that something must pop out and
layer on top, to make searching for variables easier; it is too cramped in
the dropdown.

Agreed by its author on 2026-11-01, before building, from the wireframes on
the canvas "Workflow canvas wireframes", row "Value picker":

- **A dialog over the window, not a popover on the box.** A popover is a
  shorter trip for a short list and cramped again for a long one.
- **The inspector keeps only the box,** showing the path and one line of
  type, limits and description; the inline search and list go.
- **Type chips and a scope rail,** the chips locked where an operand must
  match its value's type, values of another type counted and hidden,
  shown on request.
- **Groups fold when nothing is typed,** as the World state tree does;
  typing flattens the list to matches.
- **Declaring from the picker stays,** laid out with room: where, type and
  starting value.

Chosen while planning, for its author to check:

- **The dialog** is the editor's existing dialog layer, as Pick on map's
  dialog is, about 760 by 540 pixels and smaller on a small window, with
  the rest of the window dimmed; a click outside closes it as Esc does.
- **Which values are offered** stays as today's roles decide: a Set value
  its own, a Check value also other NPCs' and zones' values, Find NPC
  nearby's Only one whose an NPC's own values and Keep it in a local NPC
  value, Another value the integers a Check value may read.
- **The rail's counts** are of the current search and type, so narrowing
  one narrows the others.
- **The last group opened** is remembered per window for the session, not
  saved.
- **Locating for tests:** the picker's parts are named for input scripts
  as the chooser's rows are, "valuerow:<path>" and "declare:<path>", with
  "picker:<scope>" for the rail and "picker:type:<type>" for a chip.

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. The picker dialog | A dialog with the search, the results under headings with type and description, picking and closing; the inspector's Value box opens it and the inline chooser goes | View tests open it from the box and by Enter, search, pick, and close with Esc; the chooser tests of increment 15 pass through it | Built |
| 2. Every chooser | Find NPC nearby's two choosers and Another value open the picker with their rules; the type lock and the count of hidden values | View tests of each chooser's offered values and of the lock | Built |
| 3. The rail and the tree | The scope rail with counts, groups folding when nothing is typed, arrows opening them, the last group remembered | View tests of the counts, of narrowing by scope, of folding and of the memory | Built |
| 4. Declaring from the picker | Where, type and starting value when nothing matches, then chosen | View tests of declaring in each scope, and of the locked type | Planned |
| 5. Checks | `make check` and the real-window suite; a real-window test opens the picker, searches, picks and declares | The checks pass with no lint findings | Planned |

## Step results

- **The picker dialog:** `valuepicker.go` draws a dialog over the shaded
  window, 760 by 540 pixels or smaller in a small window, with the title
  and what it chooses for, a search focused on open, the values under
  their headings with their type, the rows that would declare the name
  typed, the keyboard's ways, and Cancel and Choose. Typing narrows,
  arrows move, a click on a row or Enter picks, Enter with nothing
  selected taking the top value that matches or else the top row that
  would declare one; Escape, Cancel and a click outside close it. The
  inspector's boxes, for a value and for an operand's Another value, open
  it and keep only the box, with the value's type, limits and description
  on a line under it; the inline search and list are gone. Verified: a
  view test opens the picker from the Value box, declares from it, narrows
  by typing and picks with Enter, and closes it with Escape and with a
  click outside; the chooser tests of increments 15 and 18 pass through
  it unchanged.
- **Every chooser:** Find NPC nearby's two boxes and an integer's Another
  value open the picker, as the Value box does, each offering what its
  chooser did. Type chips sit under the search, All and each type, and
  narrow the list; one chip is locked on for an operand, which must match
  its value's type, and for Keep it in, which keeps an NPC. A line under
  the list counts and names the values the search matches that the type
  hides. Show them chooses All, or, where the type is locked, lists them
  last under their headings, marked as another type, where a click says
  why it cannot be picked instead of picking it. Verified: view tests
  narrow a Value picker by a chip and show every type again; lock an
  operand's picker to integers, count and list the bool it hides without
  picking it, then pick an integer; and lock Keep it in to NPC values
  while Only one whose offers the found NPC's values and no world value.
- **The rail and the tree:** a rail down the left of the list names the
  scopes the chooser's values come from, Everywhere first, then Local,
  This NPC, This zone and World, and for a Check value Other NPCs and
  Zones, each with how many values of the types offered match the
  search; a click narrows the list to one. With nothing typed the values
  are a tree under their headings: a value at the top of its scope by its
  path, a group's values under the group's row, as world.quests, by their
  names from there, folded until a click on the row, Enter, or the right
  arrow opens it; the left arrow closes it, or goes from a value to its
  group. Typing flattens the list to the paths that match. The last group
  opened is open again in the next picker, while the window is open. Each
  row shows its type and limits at the right and its description after
  its name. Verified: a view test counts values by scope with nothing
  typed and with a search, narrows to local values, opens and closes
  world.quests by a click and by the arrows, finds it open again in the
  next picker, and picks a value inside it with Enter; the Check value
  test of increment 15 searches for the template values that now fold.

## Changed after its author tried it

Not yet tried.

## Not in this increment

- **A popover on the box,** rejected.
- **Recent or favourite values** at the top of the picker; a candidate once
  real projects show what gets chosen again and again.
- **Choosing several values at once.**
