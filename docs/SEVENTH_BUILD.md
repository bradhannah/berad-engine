# Seventh build: ambient sounds and the soundscape

The seventh implementation step finishes bringing the terminal audio editor's
work into the Audio window. An ambient cue gets a **distance**: how many
tiles away it is heard, on berad-audio's curve, which the window draws, and
its fades. The **soundscape** plays several cues together, and **monitor
levels** stand in for a player's volume settings while listening. Cues get
their classification **profile**, and recordings their **acoustic family**.
It is the last of the three audio builds, held to the same strict checks as
[the first build](FIRST_BUILD.md).

## What it proves

1. **An ambient cue is heard from a distance its author sets,** louder as the
   player comes closer, on the curve the game uses (berad-audio's
   `QuadraticDistanceGain`, from Ultima V Redux), drawn beside the setting.
2. **Cues are heard together before they go in a game:** two ambient beds,
   music and an SFX, each with its own gain, mute and solo, at monitor
   levels per category. None of it is saved.
3. **Cues and recordings are classified as the terminal editor does it,** so
   a search finds them by the kind of moment they play for.
4. **The Audio window does what the terminal audio editor does,** except its
   Ultima V routing, as its author decided.

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. Library layout | In berad-sdk: entries added inside the taxonomy, and a routing section a library lacks, are laid out like the rest | Authoring test comparing a grown library with its expected text | Built (berad-sdk PR #16) |
| 2. Ambient and classification edits | Protocol version 13's distance, profile and family edits, and what the library lists of them | Headless apply-then-undo tests | Built |
| 3. Soundscape and monitor | A window's player plays layers, each with its own runtime so its gain changes while it plays, at monitor levels per category | Player tests | Built |
| 4. The window | The inspector's distance, profile and family; the soundscape in its place | View tests and a real-window test; listening judged by its author | Built; not yet judged |

## Decisions

Settled by its author on 2026-10-25 for all three audio builds (see [the
fifth build](FIFTH_BUILD.md)): the Audio window does everything the terminal
audio editor does except its Ultima V routing, and audio is always saved.
Earlier, on 2026-10-24, its author asked that an ambient sound grow louder as
the player comes closer, on Ultima V Redux's curve, and that the distance be
each ambient cue's own, which schema 3 already stores (`radiusTiles`).

Chosen while building, for its author to check:

- **A new ambient cue is heard from 8 tiles away,** fading in and out over a
  second. When the audio project has an ambient preset, a new ambient cue
  also takes its ID's first part, profile, gain, how to hear it and its first
  recording's family from it, as the terminal editor does.
- **A distance runs from 0 to 1000 tiles,** 0 being everywhere, as a
  background; a fade from 0 to 60 seconds. The window says where the curve
  is at half volume.
- **An ambient cue heard from a distance plays one recording,** as berad-audio
  requires: Add recording… is off for it, and one that plays several in turn
  has no distance. An ambient cue with no distance, such as one whose
  category was changed, offers "Hear it from a distance"; changing the
  category alone does not add one.
- **The soundscape shows in place of the inspector,** from a toolbar button,
  as the terminal editor shows it in place of its details. Add to soundscape
  puts a cue in the layer for its category and plays it; a third ambient cue
  asks which ambient layer it replaces. A layer keeps its gain, mute and solo
  when its cue is replaced.
- **Ambient layers loop, as in a game; music plays once unless Loop is on;
  an SFX plays again every half second when Again is on,** one at a time.
- **Monitor levels run from 0 to 10, starting at 7,** through the audio
  project's own eleven volume levels when it has them, or in even steps. They
  apply to what starts next, and to the soundscape at once. **Reference**
  plays recordings at their calibrated level alone, without cue gains or
  monitor levels. Stop all stops the soundscape too, and keeps its layers;
  playing a region from the waveform leaves it playing.
- **Profiles are chosen, not edited,** from the library's, as in the terminal
  editor; each shows the family it is for, its phase, outcome and role in the
  mix. A new family's name uses lowercase letters, digits, `_` and `-`.
  Changing a recording's family measures a measured recording again, as
  berad-audio does.
- **A search also finds** a cue by its profile's name, phase, outcome, role,
  emitters, contexts and search terms, and a recording by its family.

## Step results

- **Library layout (berad-sdk):** `SaveChanged` gave entries added to an
  empty object inside the taxonomy top-level indentation, and rewrote a
  library's whole routing on one line, with `null` for each empty section,
  when a section was added. Both now keep the library's layout. berad-engine
  pins the last commit of PR #16.
- **Ambient and classification edits:** protocol version 13 lists the
  library's profiles and families, the audio project's ambient preset and
  volume levels, and each cue's profile and distance. `audio.set-ambient`,
  `audio.set-profile` and `audio.set-family` run on the audio worker, and
  undo takes each back. A new ambient cue gets its distance in the same
  edit.
- **Soundscape and monitor:** a window's player plays layers by slot, each on
  its own berad-audio runtime so its volume changes while it plays, waiting
  for their recordings like any cue. It plays each category at its monitor
  level, a recording alone at the level of the cues that play it.
- **The window:** the inspector shows an ambient cue's distance and fades
  with the curve, its profile, and a recording's family; menus offer the
  library's profiles and families and a new family. The soundscape and
  monitor show in the inspector's place.
- **Fixed along the way:** New cue… now suggests the ID as the name is typed,
  not only after Enter; the gain and target dialogs share one field dialog
  with the new family's.

## Changed after its author tried it

Settled by its author on 2026-10-25, after trying the sixth and seventh
builds:

- **The region's ends are easy to grab.** A press anywhere from the
  waveform's top to its bottom, within 10 pixels of an end, picks it up, even
  just outside the waveform when the end is at its edge. Each end has a flag
  with a grip pointing into the region, drawn inside the waveform so an end
  at either edge can be seen, and drawn heavier under the pointer. A dragged
  end moves as far as the pointer does, so a press that does not move changes
  nothing. A press elsewhere on the waveform still does nothing: only the two
  ends are dragged, as its author asked. Before, an end at the edge sat on
  the waveform's border, half of its tab took no presses, and a press had to
  land within 6 pixels of its line.
- **A recording's gain is its level.** "Gain" meant both a recording's dB and
  a cue's percentage, which its author found confusing. The level's buttons
  now read Use suggested level, Set level… and Target…, and "gain" means only
  a cue's, or a soundscape layer's.
- **Attribution shows in the panel,** for the recording in focus, under its
  level, as the terminal editor shows it in its details: creator, licence,
  source, source type, the file it was copied from and its processing steps,
  with the **Allowed in a commercial game** checkbox ticked right there, as an
  edit that undo takes back. Edit attribution… opens the dialog for the rest,
  which now also asks for the source type and lists processing steps, each
  what was done, the tool and who did it, with Add step and ×. A blank answer
  is `unknown`, and a step left blank is left out. The Attribution… buttons
  and the inspector's attribution are gone; the inspector shows a recording's
  file. Protocol version 13's attribution carries both new fields.
- **The panel scrolls** with the wheel when it is taller than the window, with
  a thumb at its right edge. Its buttons see no presses away from it.

Verified: `make check` and every real-window test pass, including a new one
that drags the region's start by hand, ticks the checkbox and records a
processing step. Only screenshots show the flags and the section.

## Not in this build

Placing ambient sounds in a zone, and the game's own ambient playback, are
game and engine work still to come. Profiles themselves are not edited, as in
the terminal editor. The terminal audio editor reads schema 4 once its
berad-audio pin moves, a berad-sdk follow-up.
