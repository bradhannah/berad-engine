# Increment 62: A game runs on its own

**Status:** Built

A project plays in a window of its own, outside the editor. Until now a game
was played only inside the editor: [increment 14](0014-launch.md)'s Launch
runs the engine's game runtime in a Play tab, but reading the project,
drawing it and playing its sounds were the Play tab's own code, in the editor
module, which no game may use. This increment moves that code into the engine
module as a **player** a game's program can use, puts the Play tab on it, and
adds **`berad-play`**, a program that is the player and nothing else:
`berad-play --project DIR` opens any project folder and plays it. It builds
on increment 14 and [increment 47](0047-play-from-here.md), and is held to
the same strict checks as [increment 1](0001-foundation.md).

## What it proves

1. **A project plays outside the editor:** `berad-play --project DIR` opens a
   window on the game's screen at the player start; the arrow keys walk,
   stairs change floor, NPCs follow their workflows and say their lines,
   sprites show and sounds are heard, as in the Play tab.
2. **Launch and the player are the same code:** the Play tab reads, draws and
   plays through the engine's packages, and looks and sounds as it did.
3. **A folder reads as the editor reads it:** the project assembled from the
   files on disk is the project the Play tab assembles from the editor's
   documents.
4. **The simulation stays headless:** only the player's packages and the
   program use Ebitengine; a test fails when any other engine package does,
   or when the engine uses the editor.
5. **Playing changes nothing:** the player only reads the project folder. A
   file in an older format is upgraded in memory, and never written.

## Decisions

Asked for by its author on 2026-11-16, paraphrased: the part that actually
plays a game, since nothing is played beyond the preview; a game that
compiles into its own program, without one fixed program that needs a great
deal built in, and without the fixed systems of an RPG maker.

Agreed by its author on 2026-11-16, before building:

- **The player is a library, and the common program is its plainest use.** A
  game's own program is the same library with the game's own Go code beside
  it. (Rejected: one fixed program for every game, which leaves a game no
  place for code of its own, so everything unique would have to go into the
  engine or a scripting layer; generating Go code from a project, which gives
  nothing that embedding the project's files does not.)
- **It lives in the engine module,** in packages of its own, with a check
  that only they use Ebitengine. (Rejected: a third module beside the engine
  and the editor.)
- **The Play tab moves onto it now,** so Launch cannot come to differ from a
  game. (Rejected: building the player beside the Play tab and joining them
  later, with two copies of the same rules meanwhile.)
- **This increment stops at the common program.** A game's own program, with
  its project inside it, follows once this can be pinned.
- **Preview stays in the Map and will run on the game runtime,** in place of
  the Map's own copy of the simulation. Its author had no preference on
  when; the assistant put it next, as increment 63.

Chosen while building, for its author to check:

- **The window opens at the largest whole number of times the game's
  screen** that fits in four fifths of the display, can be resized, and
  closes by its own close button. It has no fullscreen key and no menu. Its
  title is the name of the folder that holds the project.
- **Its keys are the Play tab's:** the arrows walk; Enter, Space or a click
  closes a box that waits.
- **Around the game the window is black.** The Play tab keeps the editor's
  stage colour there.
- **The clock starts at 08:00,** Preview's default, now the engine's
  (`game.DefaultStartSeconds`). A game's own start time is not project data
  yet.
- **Each run picks a new seed;** `--seed N` repeats a run.
- **A project that cannot start still opens,** and the window says why: no
  player start yet, the start's zone not in the project, or what the game
  runtime refused.
- **What cannot be read or played is said on the terminal,** once each, and
  the game starts without it: a file that is not the document it claims to
  be, one newer than the engine reads, a cue whose file is missing.
- **A folder is read as the editor opens it:** a document every project
  holds that the folder lacks, such as the world state, is supplied as a new
  project has it; Default, the engine's palette, is always the engine's
  whatever a file says; a document of one of the editor's own formats, such
  as saved views, is passed over without a word.
- **Sound opens one device at 48 kHz the first time something plays,** and
  plays each layer of the soundscape on a runtime of its own, as the editor
  does, so its volume changes while it plays.
- **The editor's own helpers read through the engine too:** the catalog of
  values, the declared signals and the names of places, which Workflow,
  World state and the Map use, now ask package `gameload`, by the rules they
  followed before.
- **IBM Plex Sans moved to the engine's player,** with its licence, since a
  game shows its text in it; the editor takes it from there and keeps IBM
  Plex Mono.
- **A script can play a game by itself,** for tests that open the real
  window: `BERAD_PLAY_SCRIPT` names a JSON file of steps that hold arrow
  keys, wait for a state, save a screenshot and quit. A state never reached
  ends the program with an error.
- **The engine module now uses Ebitengine and berad-audio,** in package
  `player` and the program only; `engine/boundary_test.go` fails when
  anything else in the engine does.

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. Loading | Package `gameload`: what a game plays, assembled from a source of documents that says of each whether it has loaded; the Play tab loads through it | Tests assemble a project from a source in memory; the Play tab's tests pass | Built |
| 2. From a folder | A source that reads a project folder, upgrading old files in memory and writing nothing | The sample read from its folder is the sample the Play tab assembles | Built |
| 3. Drawing | Package `player` draws a game: cells, characters and art, fitted to its room; the check on who uses Ebitengine | The Play tab's screenshots are what they were | Built |
| 4. What a game shows | Bubbles, the message box, styled text and its fonts move to the player; the Map's preview and the Play tab draw through it | Their tests pass from the new place; a bubble and a box look as they did | Built |
| 5. Sound | What plays when moves to the player; a game's sounds play from the project's audio folder | Tests of the rules; the folder's cues agree with what the editor lists: gains, distances and lengths | Built |
| 6. The program | `player.Run` and `berad-play`; `make player`, `make play-demo`, `make play-game` | A walk in the real window plays the sample from its folder as the Launch walk plays it | Built |

## Step results

Not in a pull request yet.

- **Loading:** `engine/gameload` gathers a `game.Project`, the tile size, the
  tile that shows each stand-in, the player and each NPC, and the cues to
  have ready, from a `Source` that says of each document whether it is held,
  waited for or missing. `editor/views/docsource` shows a window's documents
  as one; the Play tab subscribes to what a launch asks for. Verified by
  `gameload`'s tests and the editor's, and by the Launch walks in the real
  window.
- **From a folder:** `gameload.ReadFolder` reads an `fs.FS`. Verified by
  tests of what it skips, upgrades, leaves out and supplies, and by
  `TestTheSamplesFolderIsWhatTheEditorHolds`: the sample gathered from its
  files is deeply equal to the sample gathered from the documents an editor
  window holds.
- **Drawing:** `player.Screen` and `player.Art`. Verified by screenshots of
  the Launch walks compared with ones taken before the increment: the
  pictures that do not depend on an NPC's timing are the same byte for byte.
- **What a game shows:** `player.Writer` writes at a scale it is given, in
  place of the editor's pen; `editor/views/showdraw` hands it the editor's
  faces and scale. Verified the same way: the message box and the bubbles
  in the Map's preview are the same pixel for pixel, and a whole picture
  with both is the same byte for byte.
- **Sound:** `player.Sound` holds the rules, `player.AudioLibrary` reads the
  audio folder and `player.Audio` plays it. Verified by a test of the rules
  against a fake, and `TestAGameReadsTheAudioTheEditorLists`: each cue's
  gain, category, distance and length in ticks, and each category's
  adjustment, are the same read from the folder as the orchestrator lists
  them.
- **The program:** `player.Window`, `player.Run` and
  `engine/cmd/berad-play`. Verified by `TestPlayerPlaysTheSampleFromItsFolder`
  in the real window: the walk the Launch test takes, with the same counts
  of brick and figure, the tune, the hum and the chime heard, and no file of
  the project changed; and by hand on a project with no player start, which
  opens and says so.

## Changed after its author tried it

- None yet.

## Not in this increment

- **Preview on the game runtime** (increment 63): the Map keeps Preview, with
  its live edits, pause, step and watch, and drops its own player, clock, NPC
  runners, values and zone loop for the game runtime's. The runtime then
  needs to take edits while it runs, to pause and step, and to hold more than
  one zone.
- **A game's own program** with its project inside it.
- **An export** that leaves out what should not ship, such as an author's
  notes.
- **Launch starting a game's own program.**
- **Saves,** and playing beyond the start's zone.
- **What may belong in berad-sdk,** each to be discussed with its author
  before anything moves: the styled text the player writes with, bold,
  italic and coloured runs that wrap, which berad-text already lays out; and
  fitting a pixel screen to a window with every pixel the same size, which
  any pixel-art game could use.
- **A game's own look for what it shows.** The bubbles and the message box
  moved as they were, in the editor's colours and IBM Plex Sans.
