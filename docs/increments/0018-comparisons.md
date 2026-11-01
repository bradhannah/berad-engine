# Increment 18: Comparisons by type

**Status:** Planned

The eighteenth increment makes Check value compare by its value's type.
Today it offers the same five comparisons whatever the type and flags the
ones that do not fit afterwards. Instead an integer compares with symbols,
`=` `≠` `<` `≤` `>` `≥`, or is between two numbers, or at its maximum or
minimum, against a typed number or another integer value; a bool has
nothing to choose but true or false; a choice is any of, or none of, a
ticked list of its choices; text is, or is not, typed text. The buttons a
type cannot use are not shown. It builds on [increment
15](0015-values-and-sensing.md)'s values and its Check value, and on the
agreed direction in [editor features](../EDITOR_FEATURES.md#workflows) and
[the workflow design](../WORKFLOW_DESIGN.md#values), and is held to the same
strict checks as [increment 1](0001-foundation.md).

## What it proves

1. **Only the comparisons a type allows are offered:** the inspector shows
   an integer's symbols, between and its two limits; a bool's true and false
   with no Compare row; a choice's any of and none of with its choices as
   toggles; text's is and is not. A saved node whose comparison no longer
   fits its value is still marked as a problem.
2. **An integer compares with another value:** With offers a number or
   another value; the chooser lists only integer values the node may read,
   and the comparison reads both live.
3. **A choice checks a list:** any number of its choices ticked, in one
   node; none ticked is a problem; the node reads "mood is any of happy,
   calm".
4. **Symbols read back in words:** a line under the row says what the
   chosen symbol means for this value, and the node's summary uses the
   symbol for an integer and words for every other type.
5. **Old workflows read unchanged:** workflow format 6 reads version 5 as
   it was, every old comparison and operand keeping its meaning.

## Decisions

Asked for by its author on 2026-11-01, paraphrased: the comparison should
change with the value's type; for an integer the left side is a value and
the right side a number or another value; a bool is a simple match, and so
is a choice, except that a choice could match a list, or the opposite, not
matching a list; symbols, `< <= == > >= !=`, so an integer's comparison is
more apparent; and true and false for bools.

Agreed by its author on 2026-11-01, before building, from the wireframes on
the canvas "Workflow canvas wireframes", row "Check value: comparisons by
type":

- **Symbols for integers, words for every other type.** Six symbols give
  the whole set in one row where words stopped at two; the words line under
  the row covers `<` against `≤`. Bool, choice and text keep words, and
  the two limits have none.
- **A bool has no Compare row,** only true or false; its row is called Is.
- **A choice is any of, or none of, a ticked list.** Check value stays a
  yes or no question; a Switch on a value, one outcome per choice, is a
  candidate for when a workflow wants a fan-out.
- **Another value on the right, for integers first.** Bool against bool
  and choice against choice wait for a workflow that needs them.
- **Between includes both ends:** 1 to 5 means 1, 2, 3, 4 or 5.
- **A bool is true or false in every word of the editor,** never yes or
  no; the node's outcomes stay yes and otherwise.

Chosen while planning, for its author to check:

- **The saved names:** the comparison stays one choice, `compare`, saved by
  name. `is`, `at-least`, `below`, `at-max` and `at-min` keep their names
  and meanings, `is` being equality for every type and drawn as `=` for an
  integer; `is-not`, `above`, `at-most`, `between`, `any-of` and `none-of`
  join them. The engine refuses a comparison its value's type cannot make,
  as it refuses `at-least` on a bool today, and the inspector never offers
  one.
- **The operands:** `against` stays the right side. Between adds a second
  operand, `to`. A choice's list is a new operand, `among`, holding the
  ticked choices; a saved `is` with one choice shows as any of with that
  one ticked, and saving writes `any-of` and `among`. Another value is an
  operand holding a value reference instead of a value, resolved when the
  check runs; a reference that is missing or of the wrong type takes
  otherwise and is marked in the editor.
- **Format:** workflow logic goes to version 6, which reads version 5
  unchanged, the new parameters having defaults.
- **The number or value switch** is two buttons over the With row, A
  number and Another value, the chosen one deciding whether a field or a
  chooser shows under it; between shows two fields, or two choosers.
- **Summaries:** "stock ≥ reorder_level", "stock between 1 and 5",
  "serving is true", "mood is any of happy, calm", "name is not
  \"Stranger\""; the status bar says the same when the node is hovered.

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. Comparisons in the engine | The new comparison names, `to` and `among`, operands that refer to a value, format 6 reading 5, and `compares` by type; what a Check value may not ask of each type | Unit tests of every comparison on every type, of references read live, and of a version 5 workflow upgraded; the replace edit passes the shared apply-then-undo test with the new parameters | Planned |
| 2. The inspector by type | Compare filtered by the value's type, the symbol row and its words line, no row for a bool, choice toggles, text, the number or value switch, two fields for between | View tests of each type's rows, of ticking choices and of choosing another value | Planned |
| 3. Summaries and problems | The node's summary and the status bar's words; problems for none ticked, a reference of the wrong type and a comparison the type cannot make | View tests of the summaries and of each problem | Planned |
| 4. Checks | `make check` and the real-window suite; a real-window test checks an integer against another value and a choice against a list, and the saved logic holds both | The checks pass with no lint findings | Planned |

## Step results

None yet.

## Changed after its author tried it

Not yet tried.

## Not in this increment

- **A Switch on a value,** one outcome per choice and one for anything
  else: a candidate in [the workflow design](../WORKFLOW_DESIGN.md#values).
- **Another value on the right for bool, choice and text,** until a
  workflow needs it.
- **Ranges or patterns on text:** text is, or is not, a whole value.
