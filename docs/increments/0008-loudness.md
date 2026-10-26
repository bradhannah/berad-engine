# Increment 8: Loudness

**Status:** Judged by its author on 2026-10-25

The eighth implementation step makes every sound in a project as loud as it
is meant to be, without its author judging it by ear. Each recording is
**evened out** to one **reference loudness**, measured automatically; each
cue says how it is **meant to sound** next to that reference, in a few named
steps; and a Play sound node can ask for another step for one moment. The
Audio window shows a cue and a recording on screens of their own, and a
**Loudness** view checks the whole library. It is held to the same strict
checks as [increment 1](0001-foundation.md).

## What it proves

1. **Sounds are evened out without listening:** every recording is measured
   when it comes in and whenever its region or file changes, and its author
   sees where it sits on a meter rather than a number to type.
2. **Loud and quiet are choices on the cue:** Normal plays at the reference,
   and the other steps play a fixed amount quieter or louder.
3. **The editor says when a sound cannot be as loud as asked,** for a
   recording and for a cue, before a player hears it distort.
4. **A Play sound node can ask for another step,** once.
5. **Recordings and cues stay apart:** a recording two cues use is evened out
   once and plays at each cue's step.

## Decisions

Asked for by its author on 2026-10-25 (paraphrased): sounds evened out so
that none is much louder or quieter than another, to the way each is meant
to sound; shown on screen, since its author cannot judge it reliably by ear;
a baseline, above which a sound is loud and below which it is quiet;
fewer controls than a recording's level and target, a cue's gain and the
monitor levels; Play sound nodes allowed to change the volume in a special
case.

Agreed by its author on 2026-10-25, from the design board drawn for this
increment and the next:

- **Evening out belongs to the recording.** It is a fact about the sound,
  measured on its region, the same in every cue that plays it.
- **"Meant to sound" belongs to the cue.** It is a choice about how the sound
  is used: the same footstep can be Normal in one cue and Quieter in another.
- **A Play sound node may choose another step,** for a moment that must
  stand out or sit back. By default it plays the cue as it is.
- **Monitor levels and soundscape gains are for listening only,** and are
  labelled so: they are never saved and the game never hears them.
- **Setting a level by hand stays,** out of the way, for a recording measuring
  gets wrong.

Settled by its author on 2026-10-25, before building:

- **A cue plays louder than the reference through schema 5** of berad-audio's
  library, which lets a cue's gain, and a play's, reach +12 dB. Until now
  berad-audio kept both between 0 and 100%, so nothing could play louder than
  its recording's level. Recordings are evened out to Normal, and the editor
  checks each recording's loudest moment so that a louder step never
  distorts. The terminal audio editor keeps writing the schema it opened, so
  Ultima V Redux's files stay schema 3. (Rejected: evening recordings out to
  the loudest step with Normal at about 40%, under which a recording that
  cannot reach the loudest step plays quieter at every step, even Normal.)
- **Five steps:** Much quieter, Quieter, Normal, Louder and Much louder, at
  −12, −6, 0, +4 and +8 dB.
- **One reference loudness for every recording, and an adjustment for each
  category** (music, SFX and ambient) that its cues apply, starting at ±0. The
  reference is berad-audio's default of −20 dB until its author chooses
  another.

Proposed, for its author to check while building:

- **An acoustic family keeps choosing how a recording is measured,** as a
  short sound or a sustained one, and no longer sets the target, so two
  recordings in different families even out to the same reference. The
  audio project's descriptor gives each family both today.
- **Levels set by hand before this increment stay,** shown as set by hand; cue
  gains set before it show as the nearest step, or as their percentage when
  none is near.

Chosen while building, for its author to check:

- **Category adjustments live in the audio project's descriptor,** as
  version 2's `categoryGainsDb`, and reach the game through each cue's gain:
  setting one moves the gain of every cue in the category, so playback, which
  reads only the library, hears it without knowing. Each runs from −12 to
  +4 dB, so the loudest step stays within +12 dB. A descriptor stays version
  1 until an adjustment is set, so the terminal editor can still open it.
- **The reference is the descriptor's calibration target.** The editor aims
  every family at it, measuring each its own way, and leaves the families'
  own targets in the descriptor for the terminal editor. Changing the
  reference measures every measured recording again, in one undoable edit.
- **A step too loud for a recording is reported, not held back:** "Too loud
  for this recording: it would distort by 2.0 dB", when its loudest sample
  would pass −3 dBFS. berad-audio plays what the cue asks.
- **One Play under the waveform compares a recording by ear:** Play it,
  under the meter, chooses how loud: As recorded, unaltered; Evened out, at
  its level; and, on a cue's screen, In this cue, its step and its
  category's adjustment added. The meter rings the dot Play will play at, and a line
  says what it plays. Past the +12 dB a level reaches, an audition plays the
  rest as its cue's gain, as the cue would. Three buttons, one for each
  loudness, came first; see below.
- **Sounds are measured when they come in,** and so is each piece made from
  one; Measure all measures the rest. A level set by hand stays until Use the
  reference measures it again. The window's Measure, Use the reference and
  Set by hand… replace Use suggested level, Set level… and Target….
- **Meant to sound is a stepper in the inspector,** where the cue's gain
  was: −, the step with five rising bars, then +. A cue between steps, such as
  one the terminal editor set, shows as Custom with its dB, and − or + takes
  it to the nearest step.
- **The cue's screen keeps the waveform** of the recording chosen in it, as
  increment 7 made it, and adds under its attribution how loud this cue
  plays each recording and how it plays: evened out, its step, a Play sound,
  the player's volume.
- **Play sound's Loudness this time** is As the cue is or a step, workflow
  format version 3; an older Play sound is upgraded to As the cue is. The map
  preview plays it at that step in its category.
- **The soundscape says it is for listening only:** its layers' gains read
  as their mix, and the monitor is the pretend player volume.
- **The sample project's library is schema 5,** its hum Quieter and the rest
  Normal, not measured, so Measure all can be seen working.

Open:

- **The words.** The design board uses *evened out*, *reference loudness*,
  *meant to sound* and *category adjustment*; the [vocabulary](../VOCABULARY.md)
  still says level, target and gain, and changes when its author settles
  them.

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. Schema 5 | In berad-sdk: a cue's gain and a play's gain up to +12 dB in schema 5, an upgrade from schema 4 when an editor asks, and each recording's headroom from the analysis it already keeps | Guard tests for schemas 3, 4 and 5; runtime gain tests | Built (berad-sdk PR #17) |
| 2. Measuring by default | New and changed recordings measured automatically, Measure all, and the reference loudness and category adjustments in the audio project's descriptor | Headless orchestrator tests | Built |
| 3. Meant to sound | A cue's step, the library check for recordings and for cues, and the Play sound node's step for one moment | Apply-then-undo tests and runner tests | Built |
| 4. The windows | The cue's screen, the recording's screen, the Loudness view, the soundscape relabelled, and the Play sound node's inspector | View tests and a real-window test; judged by its author with the meters, not by ear alone | Judged |

## Step results

- **Schema 5 (berad-sdk):** `audio.MaxGain` (+12 dB) bounds a cue's gain
  and a play's in the runtime; schema 5 lets a library's `volumeModifier`
  reach it, and `MaxCueGain` says what a schema allows. Upgrades go one
  version at a time, and `CreateLibrary` writes schema 5. Descriptor version
  2 adds `categoryGainsDb`, written by `SetDescriptorCategoryGain`;
  `SetDescriptorCalibrationTarget` sets the reference; and
  `CalibrationPeakCeilingDB` names the −3 dBFS ceiling. The format guard
  adds a schema-5 library and a version-2 descriptor. berad-engine pins the
  last commit of PR #17, and later of PR #18 (below).
- **Measuring by default and meant to sound:** protocol version 14 lists the
  reference, the adjustments and the largest cue gain, asks how loud every
  recording measures (`AudioLoudness`, kept by the orchestrator until a file,
  region or policy changes), and adds the edits `audio.set-loudness`,
  `audio.set-category-gain`, `audio.set-reference`, `audio.measure-all` and
  `audio.use-reference`, each undone like every other. A cue made new starts
  at Normal in its category, and changing a cue's category keeps its step.
  `engine/loudness` names the steps, and `npcrun` reports each Play sound's
  step with its cue.
- **The windows:** meters show a recording as recorded, hollow, and as it
  plays, filled, against the reference, a line; red where it cannot get
  there. The Loudness view, from the toolbar, sets the reference and the
  adjustments, measures everything, and checks every recording and every cue.

## Changed after its author tried it

Trying the windows, their author found too many Play buttons: a cue's screen
had seven, among them Play recording, Play region, Play whole file and the
three loudness buttons. A second design board, Play buttons, drew one Play
for each thing you hear, and its author approved it on 2026-10-25.

- **A cue has two Plays:** Play cue, up top, plays it as the game does, the
  next recording in turn; Play, under the waveform, plays the recording in
  focus. Play recording is gone.
- **Choices say what the waveform's Play plays; they add no buttons:**
  Region or Whole file, and how loud. Play region, Play whole file and the
  three loudness buttons are gone. Every Play reads Stop while it plays.
- **A recording's screen has no Play up top;** the one under its waveform
  plays it, and each cue under Played by has its own Play, which plays the
  recording as that cue does.

Chosen while building, for its author to check:

- **How loud Play plays starts as In this cue on a cue's screen and Evened
  out on a recording's own.** Cues keep their choice and recordings theirs,
  and both keep Region or Whole file, while the window is open; none is
  saved.
- **Choosing while Play plays plays again at once,** at the new choice, so a
  looping sound can be compared as recorded and evened out.
- **Space plays what the screen's main Play plays:** a cue as the game does,
  a recording as its waveform's Play is set, and a file whole, as recorded.
- **A cue's Play under Played by plays the region applied,** at the level
  saved and the cue's step, not a region being drafted.
- **The region's Whole file button reads Use whole file,** so it is not
  confused with the Whole file choice beside Play.

Found by its author while trying it:

- **With Loop on, the playhead stopped at the region's end** while the sound
  kept playing. berad-audio held a looping voice's position at its end once
  it had played through; berad-sdk PR #18 wraps it to where the loop has
  reached, and berad-engine pins that commit.

## Not in this increment

Files and their pieces ([increment 9](0009-files-and-pieces.md)), built alongside
it. Where an ambient sound is heard stays the cue's distance, from [increment 7](0007-ambient-and-soundscape.md). A game's own playback of the steps and
adjustments waits for the game to play sound.
