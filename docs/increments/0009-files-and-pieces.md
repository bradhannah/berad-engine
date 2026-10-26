# Increment 9: Files and their pieces

**Status:** Judged by its author on 2026-10-25

The ninth implementation step lets an author cut one sound file into many
recordings and see them together, as its author did for Ultima V Redux: a
file of footsteps becomes four recordings, which two cues share, and the file
is never copied. A file gets a screen of its own, with every piece cut from
it marked on its waveform, **Find sounds** to propose the rest, and
the file's attribution, changed once for all of its recordings. It is held
to the same strict checks as [increment 1](0001-foundation.md).

## What it proves

1. **A file's pieces are seen together:** every recording cut from a file is
   marked on the file's waveform, and a sound no recording covers shows.
2. **Splitting is quick:** Find sounds proposes a piece for each sound
   between silences that no recording covers yet, and nothing changes until
   its author makes one.
3. **Attribution belongs to the file:** who made it, its licence and whether
   it is allowed in a commercial game change once, for every recording cut
   from it.
4. **Nothing is copied:** a piece is a recording with its own region of a
   shared file, as Duplicate region already makes one at a time ([increment 6](0006-trimming-and-levels.md)).

## Decisions

Asked for by its author on 2026-10-25, paraphrased: keep a sound file apart
from its uses, so one file can be cut into several sound effects and one
recording used in several places, without duplicates.

Agreed by its author on 2026-10-25, from the design board drawn for this
increment and the last:

- **Three layers:** a *file* on disk, never copied; a *recording*, a piece
  of one file; a *cue*, one use in the game, which takes turns between its
  recordings.
- **The Recordings list groups recordings by the file they were cut from,**
  and a file opens its own screen.

Settled by its author on 2026-10-25, before building:

- **Attribution stays on each recording,** as berad-audio keeps it, and the
  editor changes the attribution of every recording of a file together, in
  one undoable edit. Recordings of one file whose attribution disagrees show
  as a problem. (Rejected: a file-level record in a new schema.)

Chosen while building, for its author to check:

- **Find sounds splits at silences of at least 150 ms,** a silence being
  quieter than −40 dB, as berad-audio's region suggestion already counts it;
  shorter gaps stay inside one sound, such as a footstep's scuff.
- **It proposes only sounds no recording covers half of,** in orange on the
  file's waveform; Make recordings makes one recording of each, in order, in
  one undoable edit, each measured. They join the cue that plays the file's
  other pieces when exactly one cue does, and otherwise none.
- **The Recordings list is by file,** each file a row with how many
  recordings were cut from it, and recordings with no file under Empty slots.
- **Pieces of one file whose attribution differs are a problem** that the
  Problems filter shows, from files attributed before this increment.

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. Finding sounds | In berad-sdk: finding each sound between silences in a file, from the analysis berad-audio already keeps | Authoring tests with generated sound | Built (berad-sdk PR #17) |
| 2. File edits | Changing a file's attribution for all its recordings, and making several recordings from proposed pieces, each one undoable edit | Apply-then-undo tests | Built |
| 3. The windows | The Recordings list by file, the file's screen with its pieces, Find sounds and attribution, and the recording's screen showing the other pieces of its file | View tests and a real-window test | Judged |

## Step results

- **Finding sounds (berad-sdk):** `RecordingAudio.SuggestedWindows` gives a
  window for each sound that a given silence parts from the next, padded as
  `SuggestedWindow` pads and never overlapping.
- **File edits:** `audio.set-file-attribution` sets the attribution of every
  recording of a file, and `audio.make-pieces` makes a recording of each of
  several regions of one's file; an analysis now lists the sounds in its
  file. The attribution checkbox and dialog change the file's attribution.
- **The windows:** a file's screen shows its waveform with every piece
  outlined in blue, Find sounds and Make recordings, a button for each piece,
  and its attribution; a recording's screen says which piece of which file it
  is, outlines the file's other pieces on its waveform, opens its file, and
  lists the cues that play it with how loud each plays it.

## Not in this increment

Loudness, which this increment relies on for each new piece ([increment 8](0008-loudness.md)).
