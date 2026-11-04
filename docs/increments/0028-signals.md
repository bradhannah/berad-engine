# Increment 28: Signals

**Status:** Planned

The twenty-eighth increment lets workflows tell each other that something
has happened, at once. Today one NPC can learn about another only by
reading its values, so a back-and-forth means setting a value and checking
it again and again. Now a **signal** is declared in the project and sent
with a **Send signal** node, to one NPC, to everyone in the zone or to
everyone; and a workflow that answers it has a **receiver**, another start
node, On signal, on its canvas. Hearing a signal interrupts the NPC at its
next safe moment, runs the receiver's path, and then a **Resume** node
sends it back to exactly where it was, or a line into the flow takes over
from there. A Send that nobody can answer is a broken signal call, flagged
before the game runs. It builds on the calls of [increment
21](0021-flow-modules.md), the zone loops of [increment
23](0023-workflow-kinds.md), the speech of [increment
26](0026-speech-and-boxes.md) and Together from [increment
27](0027-together.md), and is held to the same strict checks as [increment
1](0001-foundation.md).

## What it proves

1. **One NPC can make another act at once:** the Baker calls out, and the
   Guard, partway along his rounds, stops at the next cell and answers.
2. **An interruption is safe:** it never splits a run of instant nodes or
   leaves an NPC between cells, and Resume takes the NPC back to exactly
   where it was, its Wait keeping its time and its Go to finding a new way
   to the same place.
3. **A receiver can take over instead:** a line from its path into the
   flow carries on from there, where it was forgotten.
4. **A signal that can't be answered is caught early:** a Send whose
   receiver has no On signal for it is flagged as a broken signal call; a
   receiver nobody sends to is a warning.
5. **Signals repeat exactly:** they land on the next tick in the order they
   were sent, so a run with signals is the same every time.

## Decisions

Asked for by its author on 2026-11-03: a way for NPCs to signal each
other, for speech and for anything else, any number of signals; working
like an interrupt rather than a poll, for immediate action, made safe by
interrupting only at a change of state, never during a node's work.

Agreed by its author on 2026-11-03, in answer to the planning questions:

- **Workflows always listen; there is no listening container.** A
  receiver is another start node, registered to its signal, in the same
  workflow as the rest of its NPC's flow, so its path can loop back in.
  What a workflow answers is visible to the other workflows in its zone.
- **An interrupt lands only at a safe moment:** never inside a run of
  nodes that take no time, and inside a node that takes time only at that
  node's own safe moment, a walk at its next cell.
- **Resume is its own node,** returning to where the NPC was interrupted,
  as a return from interrupt does; a line from a receiver's path into the
  flow takes over instead.
- **A Send to a receiver with no On signal for it is a broken signal
  call,** a problem; **a receiver nobody sends to is a warning.**

Agreed by its author on 2026-11-03, from the wireframes on the canvas
"Workflow canvas wireframes", row "Signals: receivers that interrupt":

- **A receiver, On signal,** is drawn as a rounded start with a signal
  mark and the signal's name. Its inspector chooses the signal, keeps who
  sent it in a local NPC value, so Send signal can answer them, and lists
  the workflows that send it.
- **Send signal** names the signal and sends it to one NPC, chosen by name
  or held in a local, such as the one Find NPC nearby found or whoever sent
  the signal being answered; to everyone in this zone; or to everyone. It
  takes no time, and its inspector lists who would hear it.
- **A receiver's path ends** in Resume, back where it was; in a line into
  the flow, taking over; or free, going back to Start, as anywhere else.
- **What resuming continues:** a Wait its remaining time; a Go to by a new
  route, from where the NPC now stands; a Move its remaining tiles; a Say
  or a Play sound is done if it finished meanwhile; a Together each lane
  where it was. Inside a time-bound container whose time ran out
  meanwhile, the NPC leaves by time up at once. Locals are shared, so what
  the receiver set stays set.
- **One receiver at a time:** a signal heard while a receiver runs is
  held, one of each, and lands once the receiver has resumed or taken
  over; receivers never interrupt each other.
- **A signal is a moment, gone once heard;** anything that should last
  belongs in a value. Signals sent during a tick land on the next one, in
  the order they were sent.
- **Receivers live in NPC behaviours and zone loops,** not in modules, at
  first. A signal that lands while the NPC is in a module still interrupts
  it: resuming goes back into the module, and taking over unwinds the
  calls, as time up does.
- **A workflow's tab and an NPC's details name the signals it answers,**
  and the project lists each signal with who sends and who hears it.
- **Watch shows a signal being handled:** NOW, Handling bread_called from
  Baker, with what the receiver is doing; THEN, what it will resume; and
  WAITING, any signal held. Signals heard join the log.
- **Also flagged:** a Resume outside a receiver's path, and two receivers
  for one signal in one workflow. Problems are red and warnings amber, and
  the Problems window lists warnings apart, so a project with only
  warnings still has no problems.

Chosen while planning, for its author to check:

- **Signals are declared in the world state, beside its values,** each by
  its identity with a name and a line about it, listed under SIGNALS in
  the World state window, where they are added, renamed and deleted;
  deleting one in use asks first, listing its uses, as a value's does.
- **Zone loops send and receive too:** everyone in this zone includes the
  zone's loop, and a zone loop's Send signal can name an NPC.
- **Everyone means every NPC and zone loop running,** in the preview its
  zone, in the game the zone being played; NPCs in a zone that is not
  running don't hear it.
- **A broken signal call is worked out from where the signal goes:** to
  one named NPC, its behaviour must answer it; to everyone in this zone, a
  behaviour followed there or the zone's loop must; to everyone, some
  workflow must; to an NPC a local holds, known only as it runs, some
  behaviour followed where the workflow runs must.
- **Speech on an interrupt:** the line on screen finishes; the NPC's lines
  not yet shown are dropped, so it doesn't keep talking after turning
  away. Boxes already asked for stay in their queue.
- **A receiver's start is never reached by a line,** as Start isn't; the
  NPC enters it only by hearing its signal.
- **Locating for tests:** a receiver as "receiver:<signal>", Resume as
  "node:resume:<n>", a signal in the World state window as
  "signal:<name>", and the states "handling=<signal>", "resumes=<what>",
  "held=<signal>" and "warning:<kind>:<n>".

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. Signals in the project | Signals declared in the world state by identity, its next version; SIGNALS in the World state window, adding, renaming and deleting, asking first when used; their uses | Engine tests save, read and upgrade signals; view tests add, rename and delete one in use | Planned |
| 2. The nodes | On signal, Resume and Send signal, saved and read; where Resume may go; one receiver for each signal; their one lines and help | Workflow tests save and read each node, flag a Resume outside a receiver and two receivers for one signal | Planned |
| 3. Running signals | Signals landing on the next tick in order; the safe moments; keeping where the NPC was; Resume and taking over; holding signals during a receiver; inside a module; zone loops; the preview and the game passing signals between runners | Runner tests interrupt a Wait and a Go to, resume each, take over, hold a second signal, unwind a call, and repeat a run exactly | Planned |
| 4. On the canvas | Receiver and Resume faces; Send signal's inspector with who would hear it; the signals answered on the tab and in an NPC's details; broken signal calls as problems and unheard receivers as warnings, amber, listed apart in the Problems window | View tests send to an NPC that answers, flag a broken call and an unheard receiver, and read the warnings apart | Planned |
| 5. Watching signals | Handling, what it will resume and what is held in Watch; signals heard in the log | View tests read a signal being handled and one held | Planned |
| 6. Checks | `make check` and the real-window suite; a real-window test gives the clerk's day a receiver that says a line and resumes, has the customer's behaviour signal the clerk, previews the sample, and reads the interruption and the line in Watch; deleting the receiver flags the Send | The checks pass with no lint findings | Planned |

## Step results

None yet.

## Not in this increment

- **Signals that carry values,** as a Call gives a module its inputs.
- **Receivers in modules,** and receivers interrupting each other.
- **Listening only for a while,** or ignoring a signal for a time.
