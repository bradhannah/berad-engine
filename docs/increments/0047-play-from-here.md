# Increment 47: Play from here

**Status:** Built

A cell's menu on a Map now offers Play from here, beside Preview from here,
as agreed on 2026-11-11 in
[the editor features](../EDITOR_FEATURES.md#preview-testing-and-validation)
("Play from here starts the game at a cell for one run, beside Preview from
here"). It builds on [increment 46](0046-watch-keeps-a-trail.md), and is
held to the same strict checks as [increment 1](0001-foundation.md).

## What it proves

1. **The menu offers it:** Play from here sits between Preview from here
   and Set player start here, and is pale on a cell the player cannot stand
   on.
2. **It plays from the cell once:** the Play tab opens, or comes forward,
   and launches with the player on the cell, even when the project has no
   player start. The status bar says "Playing from 3,1 for this run.
   Launch plays from the player start again."
3. **The player start stays:** Launch, or Cmd+R, plays from the player
   start again.

## Decisions

Chosen while building, for its author to check:

- **Play from here can be chosen while a preview runs,** since the game
  plays in its own tab. Preview from here stays pale then, as before.
- **The zone played is the Map's,** with the floor shown, whichever zone
  holds the player start.

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. Play once from a cell | The Play tab's launch from a cell given, for one run | A unit test | Built |
| 2. The menu | Play from here in a cell's menu | A unit test; a real-window walk | Built |
| 3. Checks | `make check` and the real-window walks | All pass | Built |

## Step results

- **Play once from a cell:** `TestPlayFromHereLaunchesOnceFromACell`
  launches a project with no player start from 2,3, finds the status bar
  saying so, and Launch then refusing for want of a player start.
- **The menu:** `TestRightClickMenuActions` chooses Play from here on 9,8
  and finds the Play tab asked to launch from it. The menus listed by the
  other right-click tests now hold Play from here.
- In the real window, `TestPlayFromHere` plays the sample from 3,1, then
  Cmd+R plays from the player start at 0,2.
  Its screenshot shows the player inside the shop's door and the status
  bar saying the run is from 3,1.
- **Checks:** `make check` passes, and so does `TestLaunchPlaysTheSample`.
  All 70 real-window walks pass on
  [increment 49](0049-pixel-editor-names-what-is-in-hand.md)'s commit,
  which carries this one, run from a frozen copy.
