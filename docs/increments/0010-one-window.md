# Increment 10: One window (spike)

**Status:** Planned

The tenth increment is a spike. It tries running the editor as one
operating-system window, the **shell**, whose first tab is the project browser
and whose other tabs hold the views that today each run as their own window
process. A **window bar** shows the tabs, each with its kind's icon; a
**pinned clock** on that bar shows the preview's time on every tab; and a
**split** shows two views side by side in one tab, such as a Map and its
Watch. Every view runs unchanged inside the shell, and the orchestrator still
owns all project data. It sits behind a flag, so the editor is unchanged
unless asked for, and it ends with its author judging whether to adopt it. It
is held to the same strict checks as [increment 1](0001-foundation.md).

## What it proves

1. **One window, unchanged views:** the project browser, Map, Text, Workflow
   and Audio open as tabs of one window, in one process, with no change to how
   a view is written (`frame.View`).
2. **Input goes where the author is working:** keys, the input method and the
   wheel reach only the focused pane; accented and composed text still types
   in a Text tab, and the arrow keys walk a preview only in the focused Map.
3. **Hidden tabs keep working:** a view's updates and its preview keep running
   while its tab is not shown; only what is shown is drawn.
4. **Time is always in sight:** the pinned clock shows the preview's time and
   rate on every tab, and opens the Clock view's controls under it.
5. **A Map and its Watch fit side by side** in one tab, both live.
6. **Sharing a process loses little:** a tab that crashes (Cmd+Shift+K) closes
   alone, and any tab can pop out into its own window process, keeping its
   undo.
7. **It is fast enough:** with a Map and its Watch showing and a preview
   running, a frame stays within the latency budget (D12).

## Decisions

Asked for by its author on 2026-10-25, paraphrased: launch only the project
browser and make it the editor's main window; give each kind of window its own
icon; and make the editor one application rather than a process per window,
each of which shows on macOS as an application of its own.

Agreed by its author on 2026-10-25, from the wireframes below and the options
in [the research notes](../RESEARCH_NOTES.md#editor-windows-and-processes):

- **Try one window with tabs, the views in one process,** any tab able to pop
  out into its own window process. Chosen over running today's window
  processes inside one window as Ebitengine VM guests, over staying with
  separate windows and switching between them, over forking Ebitengine for
  multiple windows, and over another toolkit.
- **The clock is pinned to the window bar,** not folded into the Map window: a
  preview's time belongs to the whole project, and the Watch and Workflow tabs
  need it too.
- **A Map and its Watch can share a tab side by side,** rather than Watch
  popping out.

Chosen for the spike, for its author to check:

- **Behind a flag:** `berad-editor open --shell` starts the shell; without the
  flag nothing changes, and every existing real-window test still passes.
- **The shell is the project browser's window process.** The orchestrator
  launches it as it launches the browser today, and its hello says it hosts
  panes. Any other window the orchestrator is asked to open, it asks the shell
  to host instead of starting a process. The shell gives each pane its own
  connection, so to the orchestrator a pane is a window as before: its own undo
  group, subscriptions, picks and attention. The orchestrator stays its own
  headless process.
- **A pane is today's window host,** with its input, size, title, attention and
  On top supplied by the shell instead of read from Ebitengine.
- **Every pane runs every frame,** so updates and a preview never stall; only
  shown panes are drawn, each into an image of its own, so views still draw
  from (0, 0).
- **Opening something already open selects its tab** (D13's attention), and
  asks the operating system for attention only when the window is not in
  front.
- **Closing the project browser's tab quits the editor,** through the same ask
  as Cmd+Q (D10: "later: when the project browser closes"). Cmd+W closes any
  other tab.
- **Watch opens beside a Map of its zone** when one is open, as the right pane
  of that Map's tab, and otherwise as a tab of its own. Closing either pane
  leaves the other.
- **The Clock pane always runs,** so the bar can show its time. A preview
  starting, or the Map's Clock button, opens the clock's panel instead of a
  tab.
- **Each pane keeps its own status bar** for now, and its On top keeps the
  whole window above the others.
- **A crash in a pane** is caught: that pane's connection closes, the tab says
  it closed, and the stack goes to the shell's log. The orchestrator's crash
  report and its notice to other windows still cover processes only.
- **Popping out** asks the orchestrator for the same view in a window process
  of its own; the tab closes once that window has joined, so the undo list
  they share carries over (an undo list ends when its last view leaves).

## Wireframes

Letters in brackets stand for each kind's icon.

The shell, with the project browser's tab first:

```text
┌─────────────────────────────────────────────────────────────────────┐
│ [P] Project │ [M] Sample zone × │ [A] Audio × │ +     ◷ 08:00 ▾ │ ⧉ │
├─────────────────────────────────────────────────────────────────────┤
│ the focused tab's view, drawn as today from (0, 0) of its pane      │
│                                                                     │
├─────────────────────────────────────────────────────────────────────┤
│ the pane's status bar                                               │
└─────────────────────────────────────────────────────────────────────┘
```

The pinned clock during a preview, its panel open over a Map tab:

```text
┌─────────────────────────────────────────────────────────────────────┐
│ [P] Project │ [M] Sample zone │ [A] Audio │      ◷ 08:30 LIVE ▾ │ ⧉ │
├─────────────────────────────────────────────┬───────────────────────┤
│ TOOLS │ floor canvas                        │ the Clock view,       │
│       │                                     │ unchanged (320 × 300) │
│       │                                     │                       │
│       │                                     └───────────────────────┤
│       │                                                 │ INSPECTOR │
└─────────────────────────────────────────────────────────────────────┘
```

A Map and its Watch sharing a tab; the focused pane is outlined in blue:

```text
┌─────────────────────────────────────────────────────────────────────┐
│ [P] Project │ [M] Sample zone ▏[W] Watch │ +         ◷ 08:30 LIVE ▾ │
├──────────────────────────────────────┬┬─────────────────────────────┤
│ [M] Sample zone · Ground floor       ││ [W] Watch · Shopkeeper  ⧉ × │
│ TOOLS │ canvas        │ INSPECTOR    ││ live workflow canvas        │
│       │               │              ││ NOW · LAST · AT             │
│       │               │              ││ [Pause NPCs] [Step to next] │
└──────────────────────────────────────┴┴─────────────────────────────┘
```

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. Panes | The window host split so a shell can run several: input, size, title, attention and On top come from outside; a window process behaves exactly as before | Existing view, window and real-window tests pass unchanged | Built |
| 2. Hosting | Protocol version 15: a hello that hosts panes, a Host message naming the view, target and window to host, and Open's pop out. The orchestrator hosts in a connected shell what it would otherwise start, counts hosted windows when deciding whether one is open or opening, and quits when the shell's process ends | Codec round-trip tests; headless orchestrator tests with a fake shell | Built |
| 3. The shell | `open --shell`; the window bar, with new icons for the project, the clock and Watch; tabs, focus, and routing of input and the input method; panes run hidden and are drawn only when shown; a crash closes one tab; Cmd+W and Cmd+Q; the window's title from the focused pane | A real-window test opening the browser, a Map and Audio as tabs | Built |
| 4. Pinned clock | The Clock pane always running; the bar's readout from it, orange while a preview runs; its panel under the readout; a preview starting, or the Map's Clock button, opens the panel | A real-window test that starts a preview, switches tabs and reads the clock | Planned |
| 5. Split | Watch beside a Map of its zone; a divider to drag; closing either pane | A real-window test with a preview running and Watch beside its Map | Planned |
| 6. Pop out | A tab moved to its own window process, keeping its undo | A real-window test: undo in the popped-out Map undoes a stroke made in its tab | Planned |
| 7. Measuring and judging | Frame time and command-to-event time with a Map and its Watch showing and a preview running, against separate windows | Numbers recorded here; judged by its author | Planned |

Where each step changes code:

- **Panes:** `editor/window` (`host` takes its input, size and window calls
  from whoever runs it).
- **Hosting:** `editor/protocol`, `editor/client`, and in `editor/orchestrator`
  launching (`launch`), opening (`openWindow`) and the Clock window
  (`openClock`).
- **The shell, pinned clock, split and pop out:** `editor/cmd/berad-editor`
  for the flags; new files in `editor/window` for the shell and its bar; a short summary of the time from
  `editor/views/clockview` for the bar; window-kind icons in `editor/ui`.

## Step results

- **Panes:** a host now reaches the window it is shown in only through a
  small `surface` (`editor/window/surface.go`): the frame's input, what the
  close button asked for, and the title, attention and On top. A window
  process gives its host its own Ebitengine window, so nothing it does
  changed; a shell can give each pane a part of its window instead. Verified
  with `make check` (no lint findings, race tests passing), a new test that
  the close button asks the view to close, and all 23 real-window tests
  (`make gui-test`) passing unchanged.
- **Hosting:** protocol version 15. A hello can say its window hosts others
  (`Hosts`); the orchestrator then sends it a `Host` message, naming the view,
  target and window, for every window it opens instead of starting a
  process, and the pane connects as that window. `Open` can name a pane to
  pop out: its window starts as a process of its own, and the pane is told to
  close, saving first, once that window has joined. `LaunchShell` starts the
  shell's process with `--shell`, and when that process ends the editor quits,
  telling any window still open on its own to close. The hosting state is in
  `editor/orchestrator/hosting.go`. Verified with headless tests against a
  fake shell (hosting, attention to a pane, the quit count, pop out, and
  quitting when a shell process ends, which fails without that rule), the
  codec round-trip tests, `make check` and the real-window tests. The
  command-line flags come with the shell, in step 3.
- **The shell:** `berad-editor open --shell` starts the project browser's
  window as a shell (`editor/window/shell.go`, `bar.go`, `pane.go`). Each
  window the orchestrator asks it to host connects off the frame loop and
  opens as a tab in front, with its kind's icon (new ones for the project,
  the clock and Watch) and its title. One input reader serves the window: the
  tab in front gets the frame's input in its own coordinates, including the
  input method's caret, and every other pane gets none but still runs. A
  pane that ends closes its tab; one that panics is caught, closes alone, and
  the others show a notice, with the failure in the shell's log. Closing the
  project browser's tab, or the window, asks to quit the editor; a tab's
  close box asks its view first, as Cmd+W does. The window's title follows
  the tab in front, and a pane's screenshot is of the whole window. Verified
  with a real-window test that opens a Map, a text document and the Audio
  window from the browser as tabs, paints the Map while another tab is in
  front, quits from the browser's tab, and finds one window process and no
  failures; unit tests for catching a pane's panic and removing tabs;
  `make check` and the other real-window tests.

## Changed after its author tried it

Not tried yet.

## Not in this increment

- The project browser's row of buttons that open the Audio window, the Clock
  and a Map without choosing a document first, Dock and taskbar icons for each
  window process, and launching only the browser without the flag. They do not
  depend on the spike.
- Collapsing the inspector (D13). In a split at 1200 pixels wide the Map keeps
  its inspector and its canvas is narrow; collapsing it is the first follow-up
  if the split is kept.
- Dragging a tab to split, more than two panes, and reordering or restoring
  tabs.
- Moving the orchestrator into the shell's process.
- Linux and Windows: the spike is judged on macOS.

## If it is adopted

Adopting the shell changes, in the change that adopts it: the established
context ("one Ebitengine process per editor window"), D10 (when the editor
quits, and crash reports for panes), D13 (opening something already open
selects its tab), and the vocabulary (window process; new terms for the
shell, tab, pane, window bar and pinned clock).
