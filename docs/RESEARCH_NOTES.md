# Research notes

Background research behind some editor decisions. These are notes and
proposals, not decisions; [the architecture](ARCHITECTURE.md) and
[editor features](EDITOR_FEATURES.md) hold what was decided.

## Editor windows and processes

Ebitengine supports one window per process. Options considered for real
operating-system windows:

| Option | Pros | Cons |
| --- | --- | --- |
| One Ebitengine process per window, with an orchestrator (built until increment 12) | Native windows; all Go, so types are shared code; a crashed view does not take down the project | Every edit and selection crosses IPC; cross-window interactions need choreography |
| Browser or webview UI with a Go backend | Windows, tabs and mature widgets for free | Structures and widgets recreated in another language |
| A native Go toolkit for forms plus Ebitengine processes for maps | Native windows in one process for forms | Two toolkits and looks; weaker sharing of game widgets |
| One Ebitengine window with docked panels | Simplest shared state | Building a windowing system inside the editor |
| One Ebitengine window hosting the views as tabs, any of which can pop out into its own process (tried in [increment 10](increments/0010-one-window.md)) | One application; the views unchanged; a pinned clock and side-by-side panes | A view that crashes or hangs can take the others with it; the editor draws its own tabs |
| One Ebitengine window hosting the views as tabs, the orchestrator in the same process, nothing popping out (chosen: [increment 13](increments/0013-single-window.md)) | One application and one process; the views unchanged; panes reach the orchestrator over in-memory pipes carrying the same protocol | A view that hangs, or a failure outside every pane, takes the editor with it; the editor draws its own tabs |
| One host window running the window processes as Ebitengine VM guests | One application that keeps each view in its own process; the views unchanged | Experimental; several guests at once undocumented; every frame's drawing crosses processes |

Ebitengine 2.10's experimental VM mode (`exp/vmhost`) lets a host draw a guest
process's frames into its own window, inject input and receive audio. It does not
create windows; its uses are embedding a playtest of a real game in an editor
window and headless testing.

### One application (researched 2026-10-25)

On macOS each window process shows as an application of its own, with its own
Dock icon, menu bar and Cmd+Tab entry. Findings when looking for one
application:

- **Ebitengine will not get multiple windows:** its author closed
  [ebiten#1205](https://github.com/hajimehoshi/ebiten/issues/1205) in 2022,
  pointing to separate processes instead. A fork would have to undo a global
  user-interface singleton, one command queue and render thread, a Metal
  driver holding one view, and a public API with no window parameter; by
  estimate, weeks for a macOS-only version and months for a portable one,
  then a rebase on every release.
- **VM mode** ([ebiten#3438](https://github.com/hajimehoshi/ebiten/issues/3438),
  [package](https://pkg.go.dev/github.com/hajimehoshi/ebiten/v2/exp/vmhost)):
  the guest is an unchanged Ebitengine program built with the
  `ebitenginevmguest` tag and given an endpoint. The host starts it; its
  drawing is sent over a socket and replayed on the host's GPU. There is one
  session per connection. Keys, the mouse, the wheel, typed text and the input
  method (for guests using `exp/textinput`) are forwarded; the clipboard,
  dropped files, the window title and focus are not. Host and guest must use
  the same Ebitengine version.
- **Application identity on macOS:** Ebitengine makes every process a regular
  application at launch, overriding `LSUIElement`; changing the activation
  policy after start would hide a process from the Dock and Cmd+Tab, though
  which menu bar then shows is unchecked. `ebiten.SetWindowIcon` does nothing
  on macOS; `NSApplication`'s `applicationIconImage` changes a process's Dock
  icon while it runs. Menus can be added through `ebiten.RunOnMainThread`
  (2.10) with purego's `objc` package, without cgo. Since macOS 14 one process
  can bring another to the front only by yielding activation to it.
- **Other toolkits** each mean rewriting the widgets and every view: Gio has
  multiple windows but needs cgo on macOS and Linux; Fyne's input-method
  support is weak ([fyne#618](https://github.com/fyne-io/fyne/issues/618));
  SDL3 through purego has per-window text input with composition but needs a
  new renderer; raylib has one window; Dear ImGui (cimgui-go) docks and pops
  out viewports but needs cgo.
- **Other editors:** Godot is one window whose docks can float; Aseprite turned
  multiple windows off by default; Guigui, a GUI framework on Ebitengine by its
  author, is one window and still alpha.

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

## The Project window at scale (researched 2026-11-04)

Asked for by its author: options for a Project window that still works
when a project holds thousands of things, such as 64 zones, 410 NPCs, 520
workflows, 380 sprites and 240 sounds. Today's single tree works for the
sample's 25 things. At that size a section is a long scroll, folders hide
what is in them, and nothing says which things matter to the work at hand.
These are proposals, wireframed on the canvas "Workflow canvas
wireframes", row "The Project at scale"; facets was chosen, as the end of
this section says.

What the research says:

- **People reach things in small, known steps** more often than by a
  query, even when search is good; search wins when they can name the
  thing. Teevan et al., "The perfect search engine is not enough", CHI
  2004; Bergman et al., ACM TOIS, 2008.
- **Every step needs scent:** each choice should show how much lies behind
  it, and counts are the cheapest way. Pirolli and Card, "Information
  foraging", Psychological Review, 1999.
- **One hierarchy is not enough:** a workflow belongs to a zone, an NPC, a
  folder and a quest at once. Facets slice by any of them and never reach
  a dead end. Yee, Swearingen, Li and Hearst, "Faceted metadata for image
  search and browsing", CHI 2003.
- **Show what matters now:** rank by interest, from recent work and
  closeness to what is selected, and fold the rest. Furnas, "Generalized
  fisheye views", CHI 1986; Kersten and Murphy's task context in Mylyn,
  FSE 2006.
- **Overview first:** zoom and filter, then details on demand.
  Shneiderman, "The eyes have it", IEEE Visual Languages, 1996, and his
  tree-maps, ACM TOG, 1992.
- **Places are remembered:** people find things by where they put them.
  Robertson et al., "Data Mountain", UIST 1998.

The options:

| Option | What it is | Best for |
| --- | --- | --- |
| Facets | A column of properties with counts, such as kind, zone, how a workflow runs, tags, when it changed and its problems; ticking narrows a table of results, and a set of ticks can be saved as a view | Slicing thousands of things by any property |
| A working set | The things a named task touches, ranked by interest, the rest folded away; each task keeps its own set | Picking up where you left off |
| Relations | A selected thing's neighbourhood: where it runs, who follows it, what it calls and uses, each a step, with a trail back | Moving between things that belong together |
| An overview | A tree-map of the project, sized by count and coloured by kind, with recent work marked; clicking zooms in | Seeing where everything is, and where the work is |
| An atlas | Zones laid out as the author pictures the world, each holding its NPCs and workflows, with shared assets on shelves | Content that belongs to a place |
| Columns | Miller columns, one level each, ending in a preview | Deep folders, walked with the keyboard |
| Quick open | Cmd+P from any tab, ranked by how often and how lately something was opened | Reaching a thing you can name |

They combine: facets as the Project window, a working set in a left panel,
relations in the inspector, and quick open everywhere.

Chosen on 2026-11-05: facets, built in [increment
30](increments/0030-project-facets.md). The atlas is kept as an idea, with a
graph of how things relate, for when the strong associations between things
are settled.

