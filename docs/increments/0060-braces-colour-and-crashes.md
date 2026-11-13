# Increment 60: Braces, a first colour, and a crashed tab

**Status:** Planned

A line can hold a brace, a sprite opens with the colour last used in it,
and a crashed tab reopens when told, as agreed on 2026-11-13 in
[the editor features](../EDITOR_FEATURES.md#principles). It settles the
review's findings 18, 27 and 30, and 55, found while tracing them. It
builds on [increment 59](0059-keys-in-a-field.md), and is held to the same
strict checks as [increment 1](0001-foundation.md).

## What it is to prove

1. **`{{` writes a brace (18):** "Use the {{red} door" reads "Use the {red}
   door" in the game, and reads back as typed in the field.
2. **A sprite opens with the colour last used in it (27):** kept per person.
   One never painted opens with none, and the Pencil and Fill refuse:
   "Choose a colour to paint with".
3. **A crashed tab reopens when told (30):** its notice offers Reopen.
   Opening documents, quick open and Launch keep working while the Project
   tab is closed.
4. **Notices wait their turn (55):** with several lasting at once, the
   newest shows with how many there are, and History holds each.
5. **Nothing else behaves differently:** the real-window walks pass.

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. Braces | `{{` in a line's text, both ways | Unit tests | Planned |
| 2. A first colour | The colour last used per sprite; none refused | Unit tests | Planned |
| 3. Reopen | Reopen in a crash's notice; opening and Launch without the Project tab | Unit tests; a real-window walk | Planned |
| 4. Notices | The newest with a count | Unit tests | Planned |
| 5. Checks | `make check` and the real-window walks | All pass | Planned |
