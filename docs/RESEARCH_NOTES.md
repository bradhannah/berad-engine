# Research notes

Background research behind some editor decisions. These are notes and
proposals, not decisions; [the architecture](ARCHITECTURE.md) and
[editor features](EDITOR_FEATURES.md) hold what was decided.

## Editor windows and processes

Ebitengine supports one window per process. Options considered for real
operating-system windows:

| Option | Pros | Cons |
| --- | --- | --- |
| One Ebitengine process per window, with an orchestrator (chosen) | Native windows; all Go, so types are shared code; a crashed view does not take down the project | Every edit and selection crosses IPC; cross-window interactions need choreography |
| Browser or webview UI with a Go backend | Windows, tabs and mature widgets for free | Structures and widgets recreated in another language |
| A native Go toolkit for forms plus Ebitengine processes for maps | Native windows in one process for forms | Two toolkits and looks; weaker sharing of game widgets |
| One Ebitengine window with docked panels | Simplest shared state | Building a windowing system inside the editor |

Ebitengine 2.10's experimental VM mode (`exp/vmhost`) lets a host draw a guest
process's frames into its own window, inject input and receive audio. It does not
create windows; its uses are embedding a playtest of a real game in an editor
window and headless testing.

UI toolkits for the windows: ebitenui (retained-mode widgets) for editor UI, and
DebugUI for developer panels; the two can coexist.

## Time-based behaviour patterns

How games switch NPC behaviour over the day:

| Pattern | Pros | Cons |
| --- | --- | --- |
| Fixed schedule table (time window to activity and place) | Readable; a clock jump is a lookup | Rigid; variations need more tables |
| Routine swapping or keyed fallback | Story changes are easy | Whole schedules multiply |
| Prioritised condition stack (top valid entry wins, with override layers) | Scales from simple to complex; stateless evaluation | Harder to explain why an NPC acts; priority surprises |
| Behaviour tree with aborts | Composable; built-in interruption | Large trees; subtle abort rules; does not read as a day |
| Hub in a workflow graph (chosen direction) | Fits a visual workflow editor | Free jumps can sprawl; clock jumps need a defined entry point |
| Utility scoring | Emergent, quirky behaviour | Hard to guarantee timing; tuning effort |

A time-bound container with an exit node resembles a behaviour-tree decorator
that aborts its own branch, or a state-machine superstate with an exit
transition. **Proposal, not adopted:** after a clock jump or a restart, an NPC
re-enters its workflow at the hub, which evaluates current time and state rather
than history; this also fits entities reappearing when a zone is re-entered.

## Location references

Kinds of location a workflow parameter might refer to, simple to rich: a point
(anchor, with optional facing), an area, an object's use spot (for example where
to stand at a terminal), any free member of a group (handling contention), and a
conditional location whose place depends on flags. Named references survive map
edits, make shared workflows reusable and read better than coordinates; they
need where-used views and warnings when a used name is deleted. Comparable
patterns: waypoints and free points, furniture and idle markers, smart objects.

## Linked editor views

Proposal from the editor inventory discussion: one editor over one shared
project, with focused views that link to each other (clicking an NPC in a
workflow opens its definition; clicking a location focuses the map), and
workflows written against "self" plus parameters with a "preview as" choice.
