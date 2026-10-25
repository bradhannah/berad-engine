# Fifth build: the Audio window

The fifth implementation step: the **Audio window**, where an author adds
sounds to the project and edits its cues, and an **Audio section** in the
project browser that lists them. Adding a sound copies its file into the
project and asks who made it and whether it may be used in a commercial game.
It is the first of three builds that bring the terminal audio editor's work
into the editor; trimming and levels, then ambient sounds and the soundscape,
follow. It is held to the same strict checks as [the first
build](FIRST_BUILD.md).

## What it proves

1. **Sounds are added and edited in the editor:** a sound file becomes a cue
   with a recording; a cue's name, category, gain, description, search terms
   and the recordings it plays in turn all change in the Audio window.
2. **Every sound's terms are tracked:** each recording records who made it,
   where it came from, its licence and whether it is cleared for a commercial
   game; the window shows and filters what is not.
3. **Audio changes are saved and undone like every other change** (D3, D9),
   while the terminal audio editor can still change the same files.
4. **The formats stay shared:** every write goes through berad-audio's
   authoring package; schema 4 is a new version, so schema-3 files elsewhere
   stay schema 3.

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. Editing libraries | In berad-sdk: creating libraries and cues, deleting cues and unused recordings, saving after every edit, noticing another writer's change, writing a cue's category into the descriptor | Authoring tests, including untouched entries kept byte for byte | Built (berad-sdk PR #14) |
| 2. Schema 4 | In berad-sdk: each recording's `commercialUse`, an upgrade from schema 3 when an editor asks, and the format guard for both versions | Guard tests for schemas 3 and 4 | Built (berad-sdk PR #15) |
| 3. Audio edits | Protocol version 11's audio edits and their undo, applied one at a time on a worker; windows told when the library changes | Headless orchestrator tests | Built |
| 4. The Audio window | Its list, its panel and inspector, adding a sound with the Open dialog, and the Audio section of the project browser | View tests and a real-window test; listening judged by its author | Built; not yet judged |

## Decisions

Settled by its author on 2026-10-25, before building:

- The Audio window does everything the terminal audio editor does, except
  its Ultima V routing: sounds per tile type, footstep surfaces, named
  emitters and the music, dungeon and naval groups. Where sounds play comes
  from object definitions, placements, NPCs and handlers instead. The
  routing in a library is kept as it is, and never shown.
- Audio is always saved, like everything else: each change is written at
  once, and Cmd+Z undoes it. There is no Save button.
- Adding a sound opens the system's Open dialog. The editor copies the file
  into the project and asks straight away who made it, where it came from and
  its licence, with an **Allowed in a commercial game** checkbox and a
  reminder that some free licences forbid commercial use.
- That checkbox is stored in berad-audio's format as schema 4, in each
  recording's provenance. Both editors read schemas 3 and 4. A schema-3
  library becomes schema 4 when this editor first writes it; the terminal
  editor keeps writing the schema it opened, so Ultima V Redux's files stay
  schema 3.
- The work comes in three builds: this one adds and edits sounds and cues,
  the next trims them and evens out their levels, and the third adds ambient
  distance and fades and the soundscape.

Chosen while building, for its author to check:

- A recording counts as **not cleared** until its author ticks the box: an
  upgraded library starts with none cleared. One whose creator or licence is
  unknown is marked **attribution incomplete**. Both are problems the list
  shows and filters; a recording with no file has neither.
- A new cue's ID is suggested from the file's name and the category, such as
  `sfx/door_knock`, can be changed before the cue is made, and never after.
  It uses lowercase letters, digits, `_` and `-`, in parts joined by `/`.
- A new cue goes in an automatic `unclassified` classification profile, and
  says how to hear it as "Not described yet." until its author does.
- A new cue starts with an empty slot; the first sound added to it fills the
  slot, and later ones are more recordings it plays in turn.
- Deleting a cue keeps its recordings, shown as not used, and its category
  goes with it. Only a recording no cue plays can be deleted. Nothing deletes
  a copied file, as with undo.
- The editor does not hold berad-audio's lock while it is open: it notices
  when another program changed the files, and an edit that meets such a
  change is refused and can be made again.
- The gain buttons step by 10%, between 0 and 100%.
- A blank attribution answer is recorded as `unknown`. The path the file was
  chosen from is not recorded, since it is the author's own disk; its name is.

## Step results

- **Editing libraries (berad-sdk):** authoring gains `CreateLibrary`,
  `CreateCue`, `DeleteCue`, `RoutingUses`, `DeleteRecording`,
  `SetRecordingProvenance`, `SaveChanged` (which decodes only recordings
  whose file or excerpt changed), `CheckDisk` with
  `ErrLibraryChangedOnDisk`, and the descriptor's `ParseProjectDescriptor`,
  `EditorOptions`, `CreateProjectDescriptor` and `SetDescriptorCueCategory`.
  Saving now keeps untouched entries byte for byte when entries are deleted
  and inside the taxonomy, so a library saved after every edit keeps a
  readable history in git.
- **Schema 4 (berad-sdk):** `provenance.commercialUse` is required in schema 4
  and rejected in schema 3; `EditorOptions.SchemaVersion` upgrades on open.
  The guard keeps the schema-3 fixture as it was and adds a schema-4 one.
  berad-engine pins the last commit of PR #15.
- **Audio edits:** protocol version 11 adds `AudioEdit`, answered with a
  result naming what it made, and `AudioChanged`, sent to every window. The
  edits make a cue, add a sound (into a new cue, an empty slot or as another
  recording), replace a recording's file, change a cue, its category or its
  recordings, change a recording's note or attribution, and delete a cue or
  an unused recording. Each runs on a worker that opens the library afresh,
  applies the edit and saves, so the orchestrator never waits for a copy.
  Undo steps hold the values each edit changed; an undo waits while the
  window's last change is still being saved.
- **The Audio window** (`editor/views/audioview`), as the design board drew
  it: the toolbar with Add sound…, New cue… and Stop all; the list of cues
  under Music, SFX and Ambient, or of recordings, those no cue plays apart,
  with a search and filters for problems and for what is not cleared; the
  panel with the selected cue, Play, and the recordings it plays in turn,
  each played, moved, taken out, replaced or attributed; and the inspector
  with the cue's name, ID, category, gain, how to hear it, search terms,
  where it is used and Delete, or a recording's file, where it came from, its
  note and Delete. The window asks before typed changes are lost.
- **The project browser** lists the cues in an Audio section under their
  categories. A cue opens in the Audio window, which shows it; the section's
  [+], New ▾ › Sound… and the right-click menu add a sound there; Delete asks
  where the cue is used first.
- **The sample project's** library is schema 4, its generated tones cleared.

## Not in this build

Trimming a recording and its waveform, duplicating a clip, and calibrating
levels (the next build); ambient distance and fades, a new ambient cue from
the descriptor's preset, the soundscape and the monitoring volume (the build
after). Classification profiles and families are shown nowhere yet. The
terminal audio editor reads schema 4 once its berad-audio pin moves, a
berad-sdk follow-up.
