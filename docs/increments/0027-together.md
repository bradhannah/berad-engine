# Increment 27: Together

**Status:** Built

The twenty-seventh increment lets a workflow do several things at once
and go on only when all of them are done. Today an NPC follows one node at
a time: to play a sound, shake the screen and show a message together, a
workflow can start each and carry on, but cannot wait for all three. Now a
**Together** container holds **lanes**, numbered and named, that start at
once and each run their own steps; the container goes on by its one
output, all done, when every lane has finished. Play sound can wait until
its sound has played, so a lane holding it lasts as long as the sound. It
builds on the containers of [increment 3](0003-npc-workflows.md) and the
speech, boxes and shake of [increment 26](0026-speech-and-boxes.md), and is
held to the same strict checks as [increment 1](0001-foundation.md).

## What it proves

1. **Steps can run at once:** a sound, a shake and a line of speech start
   together, each in its own lane.
2. **The flow waits for all of them:** Together goes on only when every
   lane is done, never cutting one short, so nothing a lane would have done
   is left half-done.
3. **One body moves one way:** only one lane can move the NPC, and a
   second is flagged before the game runs.
4. **Running at once still repeats exactly:** lanes run in their order
   each tick, so a run of Together is the same every time.

## Decisions

Asked for by its author on 2026-11-03: a container that runs things at
once, such as playing a sound, shaking the screen and showing a message,
and does not continue until they are all done, with a clean way to split
its work into lanes 1 to n.

Agreed by its author on 2026-11-03, in answer to the planning questions:

- **Together always waits for every lane.** Going on when the first is
  done would stop the others partway, and complicate other workflows
  waiting on what a stopped lane would have done or sent. Cutting a step
  short is a signal's job, from [increment 28](0028-signals.md).

Agreed by its author on 2026-11-03, from the wireframes on the canvas
"Workflow canvas wireframes", row "Together: running steps at once":

- **Together is a container of lanes,** each a band with its number and
  name, starting at once from its own dot; Add lane adds one. A lane ends
  where its line runs out, marked lane done, rather than repeating as a
  time-bound container's steps do. Together has one output, all done.
- **Its inspector** names it and lists its lanes, each renamed in place or
  removed; removing a lane removes what is in it, in one undo step, and a
  Together keeps at least two lanes.
- **Lanes run in their order each tick,** only one lane can move the NPC,
  with Go to or Move, and inside a time-bound container time up stops every
  lane at once.
- **Watch lists every lane:** how many are running, what each is doing or
  that it is done, the canvas lighting each running lane's node. Step to
  next change stops at the next change in any lane, naming it.
- **Effects take time, so a lane can wait for them:** Play sound carries on
  at once or when it has played, a cue that loops offering only at once;
  Shake screen lasts its seconds; Say and Show message last until said or
  gone when they wait for it. Values and checks take no time.

Chosen while planning, for its author to check:

- **A lane holds steps in a line, not Calls, Returns or containers,** at
  first; each is flagged in a lane. Every node inside belongs to the lane
  it is reached from, and one reached from two lanes is flagged.
- **Together is offered in both kinds of workflow;** in a zone loop no
  lane moves anything, as a zone has no body.
- **When it has played waits for the cue's length,** from the audio
  library, which records each recording's length when it is measured: the
  longest of its recordings' played parts, so a run repeats whichever
  recording plays, and whether or not sound is on.
- **Lanes share the workflow's locals,** in lane order, so what one lane
  sets another reads on the next tick, the same every run.
- **A lane is saved as a random choice's option is,** an output with a
  stable identity and a name, so renaming a lane keeps its line.
- **Locating for tests:** a lane as "lane:<n>", Add lane as
  "ui:add-lane", and the states "lanes:<kind>:<n>=<count>",
  "lane:<n>=<what it is doing>" and "lane-done:<n>".

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. The container | Together with its lanes, each an output to its first step and a name, and all done; what a lane may hold and which lane a node is in; saved and read | Workflow tests save and read a Together, refuse one lane, and find each node's lane | Built |
| 2. Running lanes | The runner following each lane at once in lane order, going on by all done when every lane is done, time up stopping them all, the one lane that moves | Runner tests run three lanes to all done, stop them by time up, and move by one lane while the others wait | Built |
| 3. Waiting for a sound | Recordings' lengths in the audio library; Play sound's carry on; the runner waiting for the cue's length | Orchestrator tests measure a recording's length; runner tests wait for a sound, the same with sound off | Built |
| 4. On the canvas | Lanes drawn as numbered bands with their dots and lane done; Add lane; the inspector; flags for a second mover, a node in two lanes and what a lane can't hold | View tests add, rename and remove a lane and raise each flag | Built |
| 5. Watching lanes | Every lane in the preview's state and Watch, each running lane's node lit, Step naming the lane | View tests read each lane in Watch and step through a lane | Built |
| 6. Checks | `make check` and the real-window suite; a real-window test builds a Together of a sound that waits, a shake and a line of speech in the clerk's day, previews it and watches every lane finish before all done | The checks pass with no lint findings | Built |

## Step results

- **The container:** Together holds its lanes, two to eight, each saved
  as an output with an identity apart from its name, and goes on by all
  done; a lane's first step must be inside it, and each node inside is in
  the lanes that reach it. Workflows are now version 9, which gives every
  Play sound the way it carried on before, at once. Verified: workflow
  tests save and read a Together, list its lanes then all done, find a
  node two lanes reach in both, refuse one lane, nine, two lanes by one
  output and a Together inside a container, and upgrade a version 8 Play
  sound.
- **Running lanes:** the runner starts every lane when the NPC comes to a
  Together and runs each in lane order every tick, a lane following its
  steps that take no time one after another as the flow does, and going
  on by all done once every lane has run out. A step that moves the NPC
  waits while it is between cells, so a second lane's Move waits for the
  first's to finish. Each change in a lane names the lane. Verified:
  runner tests run three lanes of different lengths to all done, follow a
  lane through two steps while an empty one is done at once, and walk in
  one lane while another counts down and a third steps down after.
- **Waiting for a sound:** the audio library now gives each recording the
  length of the part of it that plays, its region or its whole file,
  decoded once while the file is unchanged; a cue lasts as long as its
  longest recording, rounded up to a tick, and a looping cue, music or
  ambient, lasts none. Play sound's Carry on, at once or when it has
  played, waits that long in real time, from the library, never from what
  is heard. Verified: orchestrator tests give a region its length and a
  whole tone its decoded length, and none to a file that won't decode; a
  protocol test rounds a cue's ticks up and gives music none; runner tests
  wait for a sound's ticks, asking for it once, and carry on at once when
  its length is unknown.
- **On the canvas:** a Together is a container with a band for each lane,
  its number in a badge and its name at its left, the dot it starts from,
  lane done after a step with nothing after it, + lane along its foot,
  and all done when all its lanes are. Its inspector renames, adds and
  removes lanes; removing one removes the steps only it reaches, in one
  undo step, and two lanes stay. Add after puts a node in the first empty
  lane; dropping one in an empty lane's band makes it the lane's first
  step; a Together goes into a line by its all done. Flagged: a Call or a
  Return in a lane, a step two lanes reach, and a Go to or Move in a
  second lane. Play sound shows until played, and is flagged when it
  waits on a looping cue. Verified: view tests add, rename and remove a
  lane, put steps in empty lanes by Add after and by a drop, put a
  Together between two nodes, and raise each flag.
- **Watching lanes:** the preview's state lists each lane, what it is
  doing or that it is done; Watch's NOW line names each lane, and each
  running lane's node is outlined LANE 1, LANE 2 and so on; a step stops
  at the next change in any lane and names the lane. Verified: a map test
  reads each lane in the preview's state and steps to a lane's next step;
  a Watch test reads a waiting lane and a done one, lights the running
  lane's node and names the lane in the step.
- **Checks:** `make check` passes with no lint findings, but for
  TestRecordingIsSentInParts, which runs out of time under the whole
  suite's load and passes alone and with its package, as before. The
  sample's Fetch stock lifts its crate with a Together, a shake and a
  chime that waits until it has played. A real-window test is new: it
  puts a Together after the customer's start with a chime that waits, a
  ten-second shake and a line, previews the sample, and watches the
  chime's and the line's lanes finish while the shake's runs on, the
  customer coming round to its line again only once the shake is done.
  The whole real-window suite, 56 tests, passed.

## Changed while building, short of the wireframes

- **A Together sits at the top level,** never inside a time-bound
  container, as containers aren't nested; so time up stopping every lane
  waits until they are.
- **A lane's band is a guide.** Bands are equal in height and arranged
  with each lane's steps in its own, but which lane a step is in follows
  the lines from each lane's dot, wherever the step is moved.
- **A lane ending is not a change,** so Step to next change stops at a
  lane's next step, not at its end.
- **When it has played needs the audio library listed,** which the
  preview and the Play tab ask for as they start; a Play sound reached
  before it is carries on at once.

## Not in this increment

- **Going on when the first lane is done,** left out by its author's
  choice.
- **Calls, Returns and containers inside a lane.**
- **Waiting for a looping sound,** which never ends.
