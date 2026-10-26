# Increment 4: Sounds in workflows

**Status:** Built

The fourth implementation step: a workflow's **Play sound** node chooses a
cue from the project's audio library and plays it, so it can be heard while
it is chosen, and when an NPC reaches it in a preview. It is the first part of
moving audio editing into the editor; the Audio window comes later. It is held
to the same strict checks as [increment 1](0001-foundation.md).

## What it proves

1. **Workflows play sounds:** a Play sound node plays a cue once and carries
   on to the next node straight away.
2. **Cues are chosen by ear:** the cue parameter's chooser lists the audio
   library's cues by category, music, SFX and ambient, and plays any of them
   before it is chosen.
3. **Windows play sound without reading files:** the orchestrator sends the
   audio library and the recordings' bytes (D1, D8).
4. **The terminal audio editor and this editor share one set of files:**
   berad-audio's own formats, read only through its authoring package.

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. Shared formats | In berad-sdk: a reader for the audio project descriptor, the rule that gives a cue its category, and a guard on the schema-3 library and version-1 descriptor formats | Both editors can read the same files, and a format change fails a test | Built |
| 2. The audio library | The orchestrator reads `audio/project.json` and `audio/library.json` through berad-audio's authoring package; protocol version 10's audio library and recording messages | Headless tests list cues with categories and send recordings in parts | Built |
| 3. Play sound | A cue parameter type and the Play sound node kind in a new Audio group; the runner asks for a node's cue and the window running the preview plays it | Runner tests | Built |
| 4. Hearing it | A sound player in each window; the cue chooser in the Workflow window; the Map window playing NPCs' sounds in a preview | A real-window test chooses, plays and previews a cue; listening judged by its author | Built; listening not judged yet |

## Decisions

Settled by its author on 2026-10-24, before building:

- Audio editing moves into the editor, in Go. The berad-sdk terminal audio
  editor stays as it is for Ultima V Redux. Both keep berad-audio's data
  formats, or change them only through a new version, so Ultima V Redux stays
  compatible.
- The first step is choosing cues for workflows; the Audio window comes
  later.
- A **Play sound** node, in a new **Audio** group of the palette, has one cue
  parameter. It plays the cue once and carries on straight away. During a
  preview, the Map window running it plays the sound. Sound that comes from a
  place on the map is left for later.
- The project's audio lives in `audio/library.json` and `audio/project.json`
  in berad-audio's own formats, outside the document envelope, so the
  terminal audio editor opens the same files (an exception to D11).
- In this increment the editor only reads them; audio is edited in the terminal
  audio editor until the Audio window exists ([increment 5](0005-audio-window.md)). The orchestrator reads them again for every
  question, so changes made there show up at once.
- A cue parameter stores the cue's ID as the audio library spells it, such as
  `door/open`, not a ULID (an exception to D4). A cue whose ID changes in the
  terminal audio editor shows as missing.
- The orchestrator sends each recording's bytes to the windows that play it,
  in JSON first (the first part of D8).
- berad-audio's authoring package keeps reading and writing schema 3 and
  descriptor 1; any change becomes a new version with step-by-step upgrades.
- The chooser's search also finds cues by their search terms.

Chosen while building, for its author to check:

- An NPC looping without waiting plays nothing, since it would ask for its
  sound every tick; its status already says it is looping.
- In a preview, each cue plays once a tick however many NPCs ask for it.
  Stepping to the next change plays only the sounds of the tick the step ends
  on, not those of the ticks it hurries through.
- A cue whose recording is an intentional silent slot can be chosen and plays
  nothing. A cue whose recording is missing can be chosen, has no play button,
  and a window asked to play it says why once.
- Selecting another node stops what the Workflow window was playing;
  stopping a preview stops what its Map window was playing.
- Sounds play at full volume wherever the NPC is; distance comes with sound on
  the map.
- A recording's version comes from its file's size and modification time,
  not its content, so a window keeps the bytes it holds until the file
  changes.
- The sample project gains three generated tones, one of each category, so a
  new project has something to hear.

## Step results

- **Shared formats:** berad-audio's authoring package gains
  `ReadProjectDescriptor`, which reads version 1 of `project.json` with the
  rules the terminal audio editor applies, and `CueCategory`: the category the
  descriptor names for a cue, else ambient for a cue with an ambient routing
  profile, else SFX. A guard test holds one library and one descriptor using
  every field; renaming, retyping, adding or dropping an encoded field fails
  it. berad-engine pins that berad-sdk revision.
- **The audio library service:** the orchestrator's `projectAudio` reads the
  descriptor when there is one, else `audio/library.json` alone, and refuses
  a library or asset folder outside the project. It answers windows with
  every cue, ordered by ID: its name, category, gain, search terms and
  recordings, each with its status (ready, no file, missing or too large),
  size, version, calibration and the part that plays. A recording's bytes go
  out 8 MiB at a time. Loading the project skips the audio folder, which
  holds no documents.
- **Play sound:** `engine/workflow` gains the `cue` parameter type, holding a
  cue's ID or nothing, and the Play sound kind in the Audio group. The runner
  follows a Play sound in the same tick and lists its cue in the NPC's
  `Sounds` for that tick; with no cue it carries on and says why. The runner
  never plays anything itself, so a run repeats the same with or without
  sound.
- **The sound player:** `editor/sound` plays cues in one window. It asks the
  orchestrator for the library and for each recording a cue needs, keeps the
  bytes by version, and plays them through berad-audio's runtime on the
  window's own audio device, which opens only when something first plays.
  When it needs a new runtime for a newly fetched cue, sounds already playing
  finish on the old one.
- **The cue chooser:** the inspector shows the chosen cue's name and its ID,
  subdued (D4), with Play beside it. Clicking the cue opens the chooser: a
  search box over the cues grouped as Music, SFX and Ambient, each with its ID
  and a play button; clicking a cue chooses it, and Escape closes the
  chooser. On the canvas the node reads "Play sound · Door opens". A Play
  sound with no cue, or with a cue the library does not have, is marked as a
  problem, and so is one in a project with no audio library, which says how
  to add one.
- **The Map window** fetches the cues of every NPC's workflow when the
  workflow loads, and plays what the NPCs' Play sounds ask for as they reach
  them.

## Not in this increment

The Audio window and editing audio in the editor; sound on the map; ambient
sounds that keep playing, their distance and the loudest-of-each-cue rule;
nodes that set or stop music and ambient sounds; handlers and the zone loop.
[The editor features](../EDITOR_FEATURES.md#sound) record these as candidates.
