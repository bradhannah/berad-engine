# Increment 28: Signals

**Status:** Built

The twenty-eighth increment lets workflows tell each other that something
has happened, at once. Today one NPC can learn about another only by
reading its values, so a back-and-forth means setting a value and checking
it again and again. Now a **signal** is declared in the project and sent
with a **Send signal** node, to one NPC or to everyone in the zone; and a
workflow that answers it has a **receiver**, another start
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
  the signal being answered; or to everyone in this zone. It takes no
  time, and its inspector lists who would hear it.
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
- **Only the zone running hears a signal,** in the preview its zone, in
  the game the zone being played; NPCs in a zone that is not running don't
  hear it. Another zone is told through a value.
- **A broken signal call is worked out from where the signal goes:** to
  one named NPC, its behaviour must answer it; to everyone in this zone, a
  behaviour followed there or the zone's loop must; to an NPC a local
  holds, known only as it runs, some behaviour followed where the workflow
  runs must.
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
| 1. Signals in the project | Signals declared in the world state by identity, its next version; SIGNALS in the World state window, adding, renaming and deleting, asking first when used; their uses | Engine tests save, read and upgrade signals; view tests add, rename and delete one in use | Built |
| 2. The nodes | On signal, Resume and Send signal, saved and read; where Resume may go; one receiver for each signal; their one lines and help | Workflow tests save and read each node, flag a Resume outside a receiver and two receivers for one signal | Built |
| 3. Running signals | Signals landing on the next tick in order; the safe moments; keeping where the NPC was; Resume and taking over; holding signals during a receiver; inside a module; zone loops; the preview and the game passing signals between runners | Runner tests interrupt a Wait and a Go to, resume each, take over, hold a second signal, unwind a call, and repeat a run exactly | Built |
| 4. On the canvas | Receiver and Resume faces; Send signal's inspector with who would hear it; the signals answered on the tab and in an NPC's details; broken signal calls as problems and unheard receivers as warnings, amber, listed apart in the Problems window | View tests send to an NPC that answers, flag a broken call and an unheard receiver, and read the warnings apart | Built |
| 5. Watching signals | Handling, what it will resume and what is held in Watch; signals heard in the log | View tests read a signal being handled and one held | Built |
| 6. Checks | `make check` and the real-window suite; a real-window test gives the clerk's day a receiver that says a line and resumes, has the customer's behaviour signal the clerk, previews the sample, and reads the interruption and the line in Watch; deleting the receiver flags the Send | The checks pass with no lint findings | Built |

## Step results

- **Signals in the project:** the world state declares signals by
  identity, each a name fit for a path and a line about it; world states
  are now version 3, an older one declaring none. The World state window
  lists them under SIGNALS, each with how many workflows send and answer
  it; Add signal adds one ready to name, and a selected signal's name and
  line are changed in the inspector, which lists the workflows that send
  and answer it. Deleting one in use asks first, naming each workflow and
  node; every change is one undo step. Verified: engine tests save, read,
  find and upgrade signals and refuse two of one name; a plugin test adds,
  renames and refuses; a view test adds, renames and deletes one in use
  after asking, and an unused one at once.
- **The nodes:** On signal, Resume and Send signal are in a palette group
  of their own, Signals, with their one lines and help. On signal is a
  start: no line leads into it and it sits at the top level; it keeps who
  sent the signal in a local NPC value if asked. Send signal sends to one
  NPC by name, the NPC a local holds, or everyone in this zone. A
  workflow's receivers are found by signal, the first of two answering it.
  Verified: a workflow test saves and reads each node, finds the receivers
  and a receiver's path, and refuses a line into a receiver, a receiver in
  a container and a signal that is not an identity.
- **Running signals:** signals sent on a tick land on the next, in the
  order sent; a runner never hears its own. A signal lands only at a safe
  moment, standing on a cell between ticks, never inside a run of nodes
  that take no time; the NPC keeps where it was, its node, its work, its
  lanes and its calls, and runs the receiver's path in its behaviour.
  Resume goes back: a Wait with the time it had left, a Move with its
  tiles left, a Go to by a new way from where it stands, a sound or a
  shake as if it had kept playing, a Together's lanes where they were. A
  path into the flow from the start takes over, the interruption
  forgotten. While one is handled, signals heard wait, one of each, and
  land once it has resumed or been taken over; one nothing answers is
  gone. Lines the NPC had yet to say are dropped. Hearing a signal is
  written to the log, as Heard bell_rung from Customer. The preview and
  the game pass signals between their NPCs and the zone's loop. Verified:
  runner tests interrupt a Wait and resume it with its time kept, take
  over, hold a second signal and drop an unanswered one, interrupt a walk
  at a cell and find the way again, land inside a module and resume back
  into it, and repeat a run exactly.
- **On the canvas:** a receiver is drawn as a start, naming its signal;
  Send signal names its signal and whom it goes to; Resume says back where
  it was. Their inspectors choose the signal from those declared and the
  NPC from the project's, Send signal showing only the setting its Send to
  needs. Flagged: no signal or one not declared, two receivers for one
  signal, a Resume off any receiver's path, no NPC or local chosen, and a
  broken signal call, a Send nothing it goes to answers. A receiver
  nothing sends to is warned of in amber, beside problems' red.
  Auto-arrange puts each receiver's path in a row of its own under the
  flow. Verified: view tests flag a Send to Ada until her behaviour
  answers it, warn of a receiver nothing sends to, flag a lone Resume, and
  arrange a receiver's path under the flow.
- **Watching signals:** the preview's state carries the signal an NPC is
  handling, from whom, what it will resume and the signals waiting; Watch
  writes them on its NOW line, as Handling bell_rung from Customer, then
  Wait 7 min of 10 min, and the log has each signal heard. Verified: a
  Watch test reads a signal being handled, what it resumes and one held.
- **Checks:** `make check` passes with no lint findings. The sample
  declares bell_rung and out_of_stock: the customer rings the bell for the
  clerk once each time it reaches the counter, looking again for the clerk
  every second rather than starting over, and the clerk says Just a
  moment! and, once it is said, resumes;
  Fetch stock tells everyone in the zone when the shelf is empty, and the
  customer says Oh, no stock! and resumes. A real-window test is new: it
  previews the sample, watches the clerk handle bell_rung from the
  customer and reads both lines in Watch's log, then deletes the clerk's
  receiver, which flags the customer's Send as a broken signal call. The
  whole real-window suite, 57 tests, passed.

## Changed while building, short of the wireframes

- **No Problems window yet:** warnings show in amber on the node and in
  its inspector, as problems do in red; listing them apart waits for a
  Problems window, which the editor has none of, as increments 21 and 22
  found.
- **A broken signal call to everyone in this zone, or to the NPC a local
  holds,** is flagged only when no workflow answers the signal
  at all; narrowing it to the behaviours followed in the zone where the
  workflow runs is left for later. One to a named NPC checks that NPC's
  behaviour.
- **Who sends and who answers** is listed for each signal in the World
  state window, not yet on a workflow's tab, in an NPC's details, or in
  Send signal's and On signal's inspectors.
- **Watch shows a signal on its NOW line,** what it will resume and any
  signal held following it, rather than in rows of their own.
- **A runner never hears a signal it sent,** even to everyone in its zone.
- **A signal sent over and over keeps its receiver busy:** one held while
  the last is handled lands as soon as the NPC resumes. The sample's
  customer first rang the bell every second it waited, which kept the
  clerk saying Just a moment! and never fetching stock; it now rings once
  a visit. As the clerk now stops for the bell, the real-window test of
  the shop waits up to 45 seconds, not 15, for the first customer served.
- **Locating for tests:** a receiver is located as any node,
  "node:on-signal:<n>", and the warnings as "warning:<kind>:<n>" and
  "warnings:<count>"; a signal or group in the World state window is
  "signal:<path>", and the states "signals=<count>", "handling=<signal>",
  "resumes=<what>" and "held=<signal>"; a tab's tint is
  "tab-tint:<title>=<tint>".

## Changed after testing

Trying the build in the sample on 2026-11-04 asked for these. Each is
built.

- **No Everyone:** Send signal was built with a fourth choice, Everyone,
  as wireframed. As only the zone running hears a signal, it reached no
  one Everyone in this zone didn't, so it was taken out; another zone is
  told through a value. Verified: a workflow test refuses a Send to
  everyone.
- **Signals sit in groups, as values do:** a group holds signals and
  other groups, no deeper than values' groups; a name need differ only
  from those beside it, and a signal is named everywhere by its path, such
  as shop.bell_rung: in Send signal's and On signal's choices and faces,
  in problems, in Watch and in the log. A group is added from the SIGNALS
  heading's or a group's right-click menu, or with Add group while a
  signal is selected. A signal or group moves by Move into or by dragging
  it, and a group is deleted with what it holds, asking first when
  anything in it is used. The sample's two signals are in a shop group.
  World states stay version 3, which no release has read. Verified:
  engine tests name a signal by its path, find one by it, list what a
  group holds, and refuse a signal in no group, in a signal, in itself or
  beside one of its name; a view test adds a group and a signal in it from
  their menus, moves a signal in by its menu and out by dragging it,
  refuses a move beside one of its name, and deletes the group with what
  it holds; the real-window test of the bell reads shop.bell_rung in Watch
  and in the problem.
- **Description, not About:** a signal's or group's line is its
  description, as a value's is.
- **The SIGNALS heading's menu** adds a signal or a group. It offered Add
  template, which did nothing there.
- **The note of where the toolbar adds** follows Add signal. It was drawn
  after Add template, over Add signal.
- **Templates are marked:** the Templates rows, each template and each
  template applied to an NPC or zone carry the template icon.
- **Open tabs of one kind are told apart:** each tab on the bar's second
  row is washed in a tint, with a band of it along its top: the first no
  other open tab of its kind has, kept while the tab stays open. The tints
  are the workflow canvas's seven, now the theme's. Verified: a window
  test gives each tab of a kind its own tint, a new tab the tint a closed
  one left, and a tab one when all are taken; the real-window test of the
  bell finds Clerk's day green and Customer shops amber.

## Not in this increment

- **Signals that carry values,** as a Call gives a module its inputs.
- **Receivers in modules,** and receivers interrupting each other.
- **Listening only for a while,** or ignoring a signal for a time.
