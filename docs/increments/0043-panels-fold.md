# Increment 43: Panels fold, when told to

**Status:** Built

A panel folds only when its author says so, and folding it for one Map
folds it for every Map and leaves Workflows alone, as agreed on 2026-11-11
in [the editor features](../EDITOR_FEATURES.md#principles) ("every panel
folds, when told to"). The inspector folds with Cmd+I as well as its
toggle. A Map's or a Workflow's left panel folds with Cmd+Shift+I or its
own button. Folded, a Map's panel is a rail of its tools, and a Workflow's
palette is a strip with its name. It builds on
[increment 42](0042-map-says-what-it-shows.md), and is held to the same
strict checks as [increment 1](0001-foundation.md).

## What it proves

1. **A fold belongs to a kind of view:** folding one Map's inspector or
   left panel folds every Map's, and a Workflow's stays as it was.
2. **Keys and buttons fold:** Cmd+I folds and opens the inspector, and
   Cmd+Shift+I the left panel. A fold button (‹) sits at the left panel's
   top right, and (›) on the rail or strip opens it again.
3. **A Map's rail is its tools:** each tool by its key, the tool in hand
   filled, and a click picks it; the canvas starts at the rail.
4. **A Workflow's strip is its palette folded:** "NODES" down the strip,
   and the canvas starts at it. Tab or a double-click on empty canvas
   still offers every node.
5. **Nothing else behaves differently:** the real-window walks pass.

## Decisions

Agreed by its author on 2026-11-11 (see the editor features, "every panel
folds, when told to" and "explicit over clever"):

- **The inspector and a view's left panel each have a fold button and a
  key,** Cmd+I and Cmd+Shift+I; folded, a panel is a strip with its name
  on it, and the left panel of a Map or a Workflow is a rail of its tools.
- **A panel's fold belongs to the kind of view.**

Chosen while building, for its author to check:

- **A Workflow's palette folds to a strip with its name,** not a rail. Its
  nodes have no icons to make a rail of, and Tab already offers them all.
- **A Map's rail shows each tool by its key letter,** since its tools have
  no icons; the status bar names the tool in hand.
- **Folds last while the editor runs.** They are not yet kept from one run
  to the next.

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. Folds per kind | The editor window keeping each kind of view's inspector and left panel the same; Cmd+I and Cmd+Shift+I | A unit test of the window | Built |
| 2. The Map's rail | The fold button, the rail of tools, the canvas from the rail | A unit test of the Map | Built |
| 3. The palette's strip | The fold button, the named strip, the canvas from it | A unit test of the Workflow | Built |
| 4. Checks | `make check` and the real-window walks | All pass | Built |

## Step results

- **Folds per kind:** `TestFoldsBelongToAKindOfView` folds one Map's
  inspector and left panel, finds the other Map following and a workflow
  not, then opens one Map's inspector and finds every Map's open.
- **The Map's rail:** `TestTheMapsPanelFoldsToARail` covers it.
- **The palette's strip:** `TestThePaletteFoldsWhenToldTo` covers it.
- **In the real window:** `TestTheInspectorFolds` now covers these:
  - a workflow's inspector folds from its toggle and opens from its strip;
  - Cmd+I and Cmd+Shift+I fold and open it;
  - the Project's inspector, of another kind, stays open.

  It used to find the Project's folding with it, which the agreed design
  replaces. Screenshots at 1512 × 916 show a Map with its rail and a
  Workflow with its strip.

## Not in this increment

The rest of "every panel folds, when told to" changes how panels are sized
and remembered, and comes on its own:
- dividers that drag, with widths kept per kind of view;
- folds and widths kept from one run to the next, per person;
- an arrangement for each size of screen, and a Map's second arrangement
  for previewing;
- refusing with a reason what will not fit;
- the window reopening as it was left;
- drawing at the screen's own resolution, and a scale for text and
  controls.
