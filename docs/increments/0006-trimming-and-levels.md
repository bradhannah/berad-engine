# Increment 6: Trimming and levels

**Status:** Built

The sixth implementation step brings the terminal audio editor's waveform
work into the Audio window. An author sees a recording's **waveform**, trims
it to the **region** that plays, makes another recording from a region of
it, and evens out its **level** with the gain berad-audio measures. A **Check files**
button finds files that cannot be decoded. It is the second of the three
audio increments; ambient sounds and the soundscape follow. It is held to the
same strict checks as [increment 1](0001-foundation.md).

## What it proves

1. **A recording is trimmed by eye and ear:** its waveform shows where the
   sound is, the region's ends are dragged or typed, and the draft plays
   before it is applied.
2. **Levels are measured, not guessed:** berad-audio suggests a gain for the
   region, the author accepts it or sets one by hand, and a measured
   recording is measured again when its target changes.
3. **The windows never decode audio files:** the orchestrator decodes,
   measures and draws the peaks, and sends only what a window shows (D1).
4. **Trims and levels are saved and undone like every other change** (D3,
   D9); a drafted region is a draft, like typed text.

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. Analysis | Protocol version 12's analysis and check questions, answered off the orchestrator's loop | Headless orchestrator tests with generated sound | Built |
| 2. Trim and level edits | Setting a region, duplicating a region, accepting a suggested gain, and setting a gain or target by hand, each undoable | Headless apply-then-undo tests | Built |
| 3. Auditioning | A window plays part of a recording, looped or once, at the gain it would play at | Player tests | Built |
| 4. The waveform | `ui.Waveform`, and the Audio window's waveform, region and level sections, with Check files | View tests and a real-window test; listening judged by its author | Built; not yet judged |

## Decisions

Settled by its author on 2026-10-25 for all three audio increments (see [increment 5](0005-audio-window.md)): the Audio window does everything the terminal
audio editor does except its Ultima V routing, and audio is always saved.

Chosen while building, for its author to check:

- **The region is a draft until Apply.** Dragging an end, typing seconds,
  Suggest and Whole file change only the draft; Apply saves it, Cancel puts
  the saved one back. Like typed text, a drafted region counts as an unsaved
  change: choosing another recording or closing the window asks first.
- **A region shorter than 10 ms is refused**, and an end dragged past the
  other stops 10 ms short of it.
- **Use suggested level waits for Apply.** The suggestion is measured on the
  drafted region, but saving it on a region that is not saved would store a
  level for a part that does not play, so the button stays disabled, with a
  note saying why, until the region is applied.
- **Play region plays the draft**, at the gain it would play at: for a
  measured recording, the gain suggested for the draft. Pressed again, the
  button reads Stop and stops it.
- **Duplicate region** makes a new recording of the drafted region from the
  same file, with the next free ID (`door_knock_02`), and, when a cue is
  selected, adds it to the recordings that cue plays in turn. The file is
  not copied again.
- **A gain set by hand stays** when the region or file changes; a measured
  one is measured again when its target changes. A target can be set or
  returned to its family's. The dialogs accept what berad-audio accepts:
  gain from −48 to +12 dB, target from −60 to −6 dB.
- **The waveform's scale** runs from −72 dB, drawn as nothing, to 0 dB, as
  in the terminal audio editor. Each peak is the loudest moment of its share
  of the width, so a short click still shows when zoomed out.
- **Check files** marks a file that cannot be decoded as a problem until the
  file changes; the Problems filter shows it.
- **The analysis keeps the last four recordings decoded**, so zooming and
  moving the region do not decode them again, and runs at most two at once.

## Step results

- **Analysis:** protocol version 12 adds `AudioAnalysis`, which asks for a
  recording's peaks between two moments at a number of columns, and answers
  with its length, the peaks (0 to 255), the region berad-audio suggests
  from where its sound is, and a calibration suggestion for the drafted
  region; and `AudioCheck`, which decodes every recording with a file and
  lists those that cannot play. A file that cannot be decoded is an answer
  with a problem, not a refusal. Both run off the orchestrator's loop and
  stop when it does.
- **Trim and level edits:** `audio.set-region`, `audio.duplicate-region`,
  `audio.accept-calibration` and `audio.set-calibration` run on the audio
  worker like increment 5's edits, and undo takes each back. Accepting a
  suggestion measures the saved region again rather than trusting the
  window's number.
- **Auditioning:** the window's player plays part of a recording, or all of
  it, looped or once, at a given gain, and reports where it is, which the
  waveform draws as a playhead.
- **The waveform** (`editor/ui/waveform.go`) draws the peaks, the region
  washed in blue with blue ends to drag, and the playhead. In the Audio
  window it sits under the recordings a cue plays, for the one chosen, or
  under a recording's buttons, with −, + and Fit to zoom; Play region, Play
  whole file and Loop; the region's start and end in seconds with Suggest,
  Whole file, Apply and Cancel; Duplicate region; and the level: how it is set,
  the suggestion, Use suggested level, Set level… and Target…. (These read
  Use suggestion and Set gain… until [increment 7](0007-ambient-and-soundscape.md)
  renamed a recording's gain its level.)
- **Check files** sits in the toolbar and reports how many files cannot be
  decoded.

## Not in this increment

Ambient distance and fades, a new ambient cue from the descriptor's preset,
the soundscape and the monitoring volume, and editing classification
profiles and families ([the next increment](0007-ambient-and-soundscape.md)). The waveform shows one channel, the
loudest of the recording's channels at each moment.
