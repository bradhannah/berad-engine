# Increment 62: A game runs on its own

**Status:** Planned

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

- None yet.

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. Loading | Package `gameload`: what a game plays, assembled from a source of documents that says of each whether it has loaded; the Play tab loads through it | Tests assemble a project from a source in memory; the Play tab's tests pass | Planned |
| 2. From a folder | A source that reads a project folder, upgrading old files in memory and writing nothing | The sample read from its folder is the sample the Play tab assembles | Planned |
| 3. Drawing | Package `player` draws a game: cells, characters and art, fitted to its room; the check on who uses Ebitengine | The Play tab's screenshots are what they were | Planned |
| 4. What a game shows | Bubbles, the message box, styled text and its fonts move to the player; the Map's preview and the Play tab draw through it | Their tests pass from the new place; a bubble and a box look as they did | Planned |
| 5. Sound | What plays when moves to the player; a game's sounds play from the project's audio folder | Tests of the rules; the folder's cues agree with what the editor lists: gains, distances and lengths | Planned |
| 6. The program | `player.Run` and `berad-play`; `make player`, `make play-demo`, `make play-game` | A walk in the real window plays the sample from its folder as the Launch walk plays it | Planned |

## Step results

- None yet.

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
