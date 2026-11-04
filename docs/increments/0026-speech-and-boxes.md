# Increment 26: Speech, message boxes and screen shake

**Status:** Planned

The twenty-sixth increment lets workflows show things to the player.
Today a workflow can only write a Log line for its author; nothing it does
reaches the screen except an NPC walking and the sprite it shows. Now a
**Say** node puts a line over its own NPC's head for a while, a **Show
message** node puts a message in a box at the foot of the screen that goes
away on its own, and a **Shake screen** node shakes the world. Lines and
boxes wait their turn in queues, never stop the game unless a box asks to
be closed, and can be waited for. It builds on the messages of the Log node
in [increment 24](0024-built-ins-and-log.md) and the help of [increment
25](0025-palette.md), and is held to the same strict checks as [increment
1](0001-foundation.md).

## What it proves

1. **An NPC can speak:** Say puts a line of text and values over its own
   NPC's head, in the preview and in the game, for as long as it takes to
   read.
2. **The game can tell the player something:** Show message puts a box at
   the foot of the screen that goes away on its own, or, when it must be
   seen, when the player closes it.
3. **Nothing talks over itself:** each NPC's lines, and the boxes, wait
   their turn in queues, and a workflow can wait until its line or box has
   gone.
4. **The screen can shake:** Shake screen shakes the world, not the boxes
   and speech over it, for a number of seconds.

## Decisions

Asked for by its author on 2026-11-03: messages that pop up in a box and
go away on their own, as in classic console role-playing games, and
messages by a character's head to show it is speaking out loud, submitted
to a queue; two characters holding a simple conversation, without
stopping the game, by temporary speech over their heads; and full-screen
effects early on. Ultima V Redux's kinds of message were reviewed: its
transient action cards that show, hold and fade, its timed overlays, its
message dialogs and its screen shake, a camera offset that leaves the
interface still.

Agreed by its author on 2026-11-03, in answer to the planning questions:

- **A workflow speaks only for its own NPC.** Two NPCs talking signal each
  other, as [increment 28](0028-signals.md) builds; neither makes the
  other speak.
- **Speech plays out when its speaker is off screen,** with its usual
  timing, but is not drawn.

Agreed by its author on 2026-11-03, from the wireframes on the canvas
"Workflow canvas wireframes", row "Messages: boxes and speech over heads":

- **Say, Show message and Shake screen are in a palette group of their
  own, Show.**
- **Say** has a line of text and values, put in by Insert value… as a
  Log's message is; it shows for as long as it takes to read, or for a
  number of seconds; and it carries on at once, the line waiting its turn,
  or when the line has been said, shown and gone. The NPC keeps walking
  while it talks.
- **Each speaker has a queue:** a line waits behind any line its NPC is
  already saying, then shows over its head in a bubble that fades as its
  time ends.
- **Show message** has a message of text and values; a name on the box,
  none or this NPC's; it goes away when it has been read, after a number
  of seconds, or when the player closes it, which alone holds the player
  until then; and it carries on at once, or when the box has gone.
- **The boxes have one queue,** from every NPC and zone loop, shown one at
  a time in the order they were asked for. The box says 1 of 2 while
  another waits, and a bar drains as its time runs.
- **Shake screen** has a strength, gentle, moderate or strong, and a
  number of seconds, and carries on when it stops. It moves the world: the
  map, its NPCs and the player, never the boxes, the speech or the
  editor's panels.
- **Every line said and every box shown is written to Watch's log,** by
  speaker, so a conversation can be followed in the preview.

Chosen while planning, for its author to check:

- **As long as it takes to read is 2 seconds, and a second for every 12
  characters, at most 10 seconds,** in real time: a player reads at their
  own pace whatever the game clock's rate. Seconds set by hand are real
  time too.
- **Say is offered only in NPC behaviours;** a zone loop has no head to
  speak over. Show message and Shake screen are offered in both kinds, and
  a zone loop's box has no name on it.
- **A line's text and values are read when the node runs,** not when it
  is shown, so a line waiting its turn says what was true when it was
  queued.
- **The queues and the shake are kept by the engine's runner, shared by
  every NPC and the zone loop,** as values are, so a run repeats exactly
  and Watch, the preview and the game agree. The preview and the Play tab
  only draw them.
- **The strengths shake by 2, 4 and 8 pixels of the game's scale,**
  settling to still as the time ends; two shakes at once take the stronger
  and the later end.
- **A box the player must close is closed with Enter, Space or a click,**
  and while it is up the player's arrows do nothing; NPCs and the clock run
  on, as only the player is held. In the editor's preview, the same.
- **Locating for tests:** a speech bubble as "speech:<NPC>", the box as
  "box", and the states "speaking:<NPC>=<line>", "box=<message>",
  "boxes=<count>" and "shaking".

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. The kinds | Say, Show message and Shake screen in the Show group, their settings and messages, saved and read; Say in behaviours only; their one lines and help | Workflow tests save and read each kind and refuse Say in a zone loop | Planned |
| 2. Running them | Each NPC's speech queue and the shared box queue, timed in real time; waiting until said or gone; holding the player for a box to close; the shake; lines and boxes in Watch's log | Runner tests queue two lines, wait until one is said, queue boxes from two NPCs, hold the player and shake | Planned |
| 3. Choosing them | Their inspectors, with the message field and Insert value…, how long they show and how they carry on; their faces on the canvas | View tests set a line with a value, choose how it shows and carries on, and read the node | Planned |
| 4. Drawing them | Speech over heads and the box in the Map's preview and the Play tab, the bubble fading, the bar draining, 1 of 2, closing a box, the world shaking under still boxes and speech, nothing drawn for a speaker off screen | View tests find a line over the customer, a box with its count, a closed box and a shake | Planned |
| 5. Checks | `make check` and the real-window suite; a real-window test gives the customer a Say and the clerk's day a Show message and a Shake screen, previews the sample, reads the speech, the box and the log in Watch | The checks pass with no lint findings | Planned |

## Step results

None yet.

## Not in this increment

- **Keeping a box in the player's message log,** drawn as a checkbox on
  the wireframes, until the game has a message log to keep it in.
- **Boxes placed by the speaker,** names on boxes for other NPCs, and
  styles of box beyond the one.
- **Other full-screen effects,** such as a flash or a fade, which would
  follow Shake screen's pattern.
- **Waiting for a sound to have played,** which [increment
  27](0027-together.md) adds to Play sound.
