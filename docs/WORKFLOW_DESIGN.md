# Workflow design

A draft of where workflows are heading: container exits, kinds of workflow,
the values a workflow can read and write, calls from one workflow to another,
and conversations. It comes from a design discussion on 2026-10-31. Nothing
here is built. Parts move into an increment, and into [editor
features](EDITOR_FEATURES.md), as they are agreed for building.

Status words are those of the feature list: **agreed** (settled direction),
**tentative** (current leaning, may change), **candidate** (an idea under
consideration), **open** (undecided).

## Containers

Today a time-bound container has one way out, its exit, taken when its time
is up or when it is reached outside its time. Links never cross a
container's wall. A free output inside a container repeats from its first
step, and a free output at the top level goes back to the start, so a line
drawn back to either only repeats what the rule already does.

- **Tentative: named exits.** An author places exit nodes inside a container
  and labels them, such as "Sent home" or "Errand". Lines inside end at an
  exit node, and any number can converge on one, so links still never cross
  the wall. Each exit node adds one output to the container's right edge, in
  the order the container's inspector lists them. An output has a stable
  identity apart from its label, as a random choice's options do: renaming
  an exit keeps the line outside, and deleting one removes it, undoably.
  Labels mean nothing to the engine. Auto-arrange places exit nodes in the
  last column, level with their outputs, so each path passes straight
  through the wall.
- **Tentative: time up stays its own output,** always last and never
  removable. It interrupts partway through a node; a named exit is a choice
  the flow makes. Keeping them apart lets a workflow treat them differently,
  and an author who wants them to lead to the same place connects both
  outputs to it.
- **Tentative: lines stay inside their container.** Nodes already do:
  dragging stops at the wall, and resizing cannot shrink a container past
  its contents. Lines do not: a line going back inside a container drops
  below every node between its ends, including nodes outside the container,
  and rises outside its left wall. Instead, a line with both ends in a
  container drops only below that container's nodes and rises inside its
  wall, and a line moved by hand stays inside too.
- **Tentative: a container grows to hold its lines.** Its size is the larger
  of the size its author gave it and what its nodes and lines need. The
  growth is worked out when drawing, not saved, so layout diffs stay quiet;
  a grown container that overlaps a neighbour is flagged, and auto-arrange
  makes room.
- **Candidate: collapsing a container** into a node showing its name, time
  and a summary such as "6 steps", with its exits still on its right edge.
  To keep what is inside from being hidden: a problem inside shows on the
  collapsed container, the Watch window marks it live and names the node an
  NPC is on, and double-clicking, or selecting a node inside from a search
  or a problem, expands it. Collapsing is layout, saved with node positions.

## Kinds of workflow

- **Tentative: deciding is kept apart from showing.** Logic sets state, and
  presentation reacts to it; nothing flows back. An NPC's behaviour
  workflow sets what the NPC is doing, such as washing dishes, and a sprite
  workflow chooses the animation from that. The trade-off is timing: logic
  that must wait for an animation to finish reads a signal the sprite side
  exposes, never one it sets in game state.
- **Agreed: workflows of different kinds allow different nodes.** A sprite
  workflow cannot walk an NPC anywhere, and a conversation cannot do
  anything confusing or unexpected to the player.
- **Candidate: the kinds the engine runs are first-class,** saved with each
  workflow, because the engine runs each differently:
  - **NPC behaviour** runs in sequence, over time. Its subject is the NPC.
  - **Sprite** reacts: given the current state, what shows. Its nodes check
    values, play animations, vary idle animations and play one-shot
    animations; it has no Go to and no waits in game time, and changes no
    game state. It belongs to a look, not to one NPC, so one sprite workflow
    serves every NPC sharing those sprites. The engine keeps drawing facing
    and walking from movement; a sprite workflow adds activities on top.
  - **Conversation** puts the player first (see Conversations).
  - Later, perhaps **object use** and **zone scripts or cut scenes**.
- **Candidate: give every workflow a saved kind now,** NPC behaviour being
  the only one, so files never have to be guessed at when a second kind
  arrives.
- **Candidate: the value an NPC's workflow sets to say what it is doing**,
  with the choices defined by the project, so a typo cannot break an
  animation. *Open:* its name. The Watch window's readout of what an NPC is
  doing, worked out from its current node ("Walking to Desk, 4 tiles to
  go"), is already called its activity in code.

## Values

- **Agreed: one type system** for node parameters, module inputs and
  outputs, a character's values and world state. Today's parameter types
  (place, duration, time range, sound cue) join integer, bool, text, NPC,
  item and quest as they are needed.
- **Agreed: start loose and signal intent.** Any workflow may read any value
  at first; what a workflow is meant for is shown by what it requires, not
  enforced by labelling it. Tighten only what proves confusing.
- **Agreed: every value is declared before it is used,** with a name, a type,
  a scope and a default. Names use lower-case letters, digits and
  underscores. World, zone and NPC values are declared in one window, and a
  workflow's local values on the workflow itself. [Increment
  15](increments/0015-values-and-sensing.md) builds the first of them.
- **Agreed: four scopes.** `local.` belongs to one workflow, `npc.` to the
  NPC it runs for, `zone.` to the zone it is in, and `world.` to the whole
  game. Pairs of NPCs, places and objects may become scopes later.
- **Agreed (2026-11-01): values are organised in declared groups,** shown
  as a tree, such as `world.quests.dragon_slain`; typing a dotted name
  declares the groups it needs. Local values stay flat.
- **Agreed (2026-11-01): each NPC and each zone owns its values,** and a
  **template**, a named set of values, gives the same set to every NPC or
  zone it is applied to, as `npc.guard1.guards.how_many_arrested`. A
  workflow names its own NPC's values without the NPC's name, so one
  behaviour serves every NPC with the template. Every NPC has a `name`
  value, text that starts as its display name.
- **Agreed (2026-11-01): the types are integer, with an optional minimum
  and maximum that a change is held at, bool, choice and text,** with NPC,
  and from [increment 21](increments/0021-flow-modules.md) place,
  for local values only.
- **Agreed: a workflow sets only its own values,** its NPC's, its zone's,
  the world's and its locals. It can read the values of an NPC it found, but
  never set them, whether it found the NPC nearby or names it, as in
  `npc.george.mood`; a zone it names, as `zone.market.crowded`, the same
  way.
- **Agreed: an NPC's values belong to the NPC.** Each NPC has a unique path
  name, such as `george`, and so does each zone. A character that repeats,
  such as a guard, is several NPCs sharing one behaviour.
- **Agreed (2026-11-01): an NPC is placed once in the whole project.** An
  NPC that seems to travel is several NPCs, shown or hidden by values.
- **Agreed (2026-11-02): an NPC lives in one zone,** set in its details,
  and goes only to places on that zone's floors, taking the stairs between
  them. A place in another zone, or on a floor no stairs lead to, is a
  problem for the NPC that runs the workflow. A module's places come from
  its callers, so a character in two zones is two NPCs or one module
  called by both.
- **Agreed (2026-11-01): what a workflow uses cannot be deleted;** the
  editor says where it is used.
- **Candidate: three sources of values.**

  | Source | Supplied by | Examples |
  | --- | --- | --- |
  | Context | The engine, always present for that kind of workflow | the NPC itself, the player, the clock, the zone |
  | Inputs | The author, where the workflow is used | a place, opening hours, a sound cue |
  | World state | The game, read while running | flags, stats, provisions, an NPC's own values |

  The NPC a behaviour workflow runs for is context, not an input: no caller
  passes it, and it flows through every call. A module that needs one says
  so, and cannot be used where there is none.
- **Candidate: a parameter holds a fixed value or a reference** to an input,
  to context or to world state, such as the module's input "place", the
  NPC's home, or the player's gold. The inspector offers only references of
  the parameter's type, picked from the world-state tree.
- **Candidate: three lifetimes for values a workflow produces,** and every
  write explicit, through a Set node or a call's output mapping:

  | Lifetime | Lasts | Example |
  | --- | --- | --- |
  | This run | Until the workflow restarts, or its NPC changes zone | a price just worked out |
  | The character | Saved with that NPC's identity | has greeted the player, what it is doing |
  | The world | Saved with the game | flags, provisions, quests |

  **Agreed:** local values, this run's, last through a workflow's repeats.
  They clear when it restarts after a clock jump or an edit, and when its
  NPC's zone changes.

  A value saved on a character or in the world is declared first, so save
  files keep their format version and upgrade like any other file.
- **Candidate: inputs are read-only.** A workflow changes state only through
  action nodes, such as Set flag, Change stat or Give quest.
- **Agreed (2026-11-01): Check value compares by its value's type** and
  stays a yes or otherwise question: symbols for an integer, `=` `≠` `<`
  `≤` `>` `≥`, with between and its limits, against a number or another
  integer value; true or false for a bool; any of or none of a ticked list
  for a choice; is or is not for text. [Increment
  18](increments/0018-comparisons.md) builds it. **Candidate: a Switch on a
  value,** one outcome per choice and one for anything else, when a
  workflow wants a fan-out; the time switch is one on the clock, and a
  conversation's stat or skill check would be another.
- **Candidate: an NPC is driven by one workflow at a time.** Actions move
  only the workflow's own NPC; other NPCs are reached through shared values,
  sensing or messages, so two workflows never fight over one body.
- **Agreed: NPCs become aware of each other first by sensing.** A Find NPC
  nearby node looks within a number of tiles, optionally only for NPCs whose
  value matches, and the workflow then reads the found NPC's values.
  Messages come later. Pairs of NPCs, slots on places and objects, requests
  with replies and scenes that cast several NPCs are candidates beyond that.

## World state

- **Agreed: three engine categories for what characters have,** each game
  defining its own lists, as an earlier game's design proved:
  - **Stats:** numbers on one character, such as strength, dexterity,
    intelligence and level.
  - **Provisions:** a fixed set of counted resources the party shares, such
    as gold and food.
  - **Inventory:** items, each an object definition with characteristics
    such as equippable, consumable or quest item.
- **Candidate:** what separates provisions from stats is scope: a stat
  belongs to a character, provisions to the party. NPCs may hold provisions
  and inventory too, so a shopkeeper's stock and takings need nothing new.
  The world-state tree [editor features](EDITOR_FEATURES.md) agrees on
  (flags, stats, skills and capabilities) gains provisions and inventory.

## Calls between workflows

- **Agreed:** a workflow can use another, reusable one, as a module holding
  a flow with inputs and outputs known ahead of time.
- **Agreed (2026-11-02): a module declares an interface:** its inputs and
  value outputs, each a local value given the role Input or Output; and its
  named ways out, each made by a **Return** node at its top level, two
  Returns of one name being one way out. Any workflow can be called. A
  **Call workflow** node names the module, fills its inputs with a fixed
  value, a place picked on the map or another value, has one output per
  way out and done, and keeps each value output in a value of the caller's
  or leaves it. A module's locals are fresh on each call. A place is a new
  type for locals, and Go to can go to a place value. [Increment
  21](increments/0021-flow-modules.md) builds it.
- **Candidate: where a module can be used is worked out** from the nodes in
  it and shown, such as "usable in NPC behaviour and conversations"; a
  module with a Go to cannot be used in a sprite workflow.
- **Candidate:** the agreed reusable workflows assigned to many NPCs become
  calls: an NPC's behaviour is a module with its inputs filled in for that
  NPC.
- **Agreed (2026-11-02):** a module that calls itself, directly or through
  others, is flagged, and so is a way out a Call uses that the module no
  longer has, an input whose value no longer fits its type, and a Call
  with no workflow chosen; the game carries on past such a Call by done.
  An input's or output's type is locked while a Call uses it.
- **Candidate:** a module can be run on its own with chosen inputs, a test
  NPC and a test profile, as a conversation can.

## Conversations

Deferred, but designed for.

- **Agreed: conversation first.** A conversation is about talking with the
  player: what the NPC says, what the player can answer, and branching on
  both. Changes to the character or the world come second, as capabilities
  within the conversation.
- **Agreed: a conversation allows only certain nodes,** so it can do nothing
  confusing or unexpected.
- **Candidate:** its nodes are NPC lines, player choices with conditions on
  them, and stat or skill checks with several outcomes, and its effects are
  setting flags, giving or taking items and provisions, giving quests and
  changing stats. Containers may group topics, and modules using only
  conversation nodes, such as a barter exchange, work there too. Its
  context is the NPC speaking and the player.

## Settled questions

Agreed by its author on 2026-10-31:

- **A named exit can be taken while a container's time still holds.** The
  NPC leaves the container at once by that exit.
- **Reaching a container outside its time takes time up,** as if its time
  had just run out.
- **A path that ends with nothing connected in a module returns by a
  default "done" outcome,** where a behaviour workflow repeats instead.
- **A module's inputs are read once, when the call starts,** like a
  function's arguments; context and world state are read live.

Following from the agreed rule that an NPC leaves a container at once when
its time is up, even partway through a node:

- **Time up unwinds calls.** When a container's time is up while its NPC is
  inside a module it called, the NPC leaves the module and the container at
  once.

## Open questions

- Can an exit leave more than one level of nested containers? This waits on
  nesting, itself still open.

## Order

**Candidate:** each step usable on its own, in this order: containers
(exits, lines kept inside, collapsing); values (what an NPC is doing, the
world-state tree with provisions, references in parameters, Set and Check
nodes); sprite workflows reacting to what an NPC is doing; calls between
workflows; conversations. Values and sensing are planned first, in
[increment 15](increments/0015-values-and-sensing.md), ahead of containers.

**Agreed (2026-11-01):** two canvas increments come before containers, so
that building workflows stops snagging on the canvas: [increment
17](increments/0017-reading-the-graph.md), reading the graph (handles on
every leg of a line, the status bar naming a line, colours), and a planned
one for working with many nodes (multi-select, duplicate, nudge, dropping a
node onto a line). Both are layout: nothing in them changes what the engine
runs.
