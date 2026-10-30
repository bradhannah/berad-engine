# Increment 13: Single window

**Status:** Built

The thirteenth increment makes the editor one window in one process. It adopts
[increment 10](0010-one-window.md)'s shell as the only way the editor opens:
the **editor window**, whose **window bar** holds a **tab** for each thing
open, the project browser's first, with the **pinned clock** at its right end.
The orchestrator moves into the editor window's process, and each **pane**
reaches it over an in-memory connection carrying the same protocol. The
multi-window option goes with everything that only exists for it: separate
window processes, popping a tab out, the `--shell` flag and `make demo-shell`.
It is held to the same strict checks as [increment 1](0001-foundation.md).

## What it proves

1. **The editor only ever opens one window:** `berad-editor open` shows the
   editor window with the project browser's tab, and everything opened after
   it is a tab of that window. No other window, and no other process, starts.
2. **One process loses nothing:** the orchestrator is still the only writer of
   project data, panes still see a change only when its event arrives
   (strict mode, D2), and a Ctrl+C, a quit or a failure still saves every
   edit.
3. **A crashed tab is reported:** a pane that panics closes alone, the other
   tabs are told, and the orchestrator saves a crash report with its failure
   and recent protocol traffic, as it did for a window process (D10).
4. **Nothing of the multi-window option is left:** no flag, command, Makefile
   target, protocol field, code path, test or document describes separate
   windows or popping out.

## Decisions

Asked for by its author on 2026-10-30: "I think it's time to officially shift
to single window without the multi window option".

Agreed by its author on 2026-10-30, before building:

- **Tabs and panes.** The one operating-system window is the **editor
  window**. Its window bar holds **tabs**, and each view (the project browser,
  Map, Text, Workflow, Watch, Audio, Pixel editor, Palette Editor) runs in a
  **pane** of a tab; a split tab holds two. Documents say "the Map" or "the
  Map's tab", not "the Map window". The code and the protocol keep their
  window IDs, documented as a pane's ID. (Rejected: renaming them in the code
  too, and keeping "window" for each view.)
- **A crashed tab leaves a crash report.** The tab closes alone and the other
  tabs get a notice; the pane first tells the orchestrator of its failure, and
  the orchestrator saves a crash report with its stack and recent traffic.
  (Rejected: the stack in the log only, as in the spike.)
- **On top stays.** The status bar's toggle keeps the whole editor window
  above other applications' windows.
- **The orchestrator runs inside the editor window's process,** in this
  increment: `open` runs both, and the `window` command, the launcher, child
  processes and their crash reports go. (Rejected: keeping it a headless
  process of its own, and moving it in a later increment.)
- **Panes connect over in-memory pipes** (`net.Pipe`) carrying the same
  protocol, framing and JSON codec. Every message is still encoded, so a pane
  only ever holds a copy and the orchestrator stays the only writer. The
  socket file goes. (Rejected: keeping the Unix socket inside the process,
  and Go channels of message structs, which would be a rewrite and let panes
  share memory with the orchestrator's documents.)

Already settled by its author, and kept:

- **Launch shows the project browser alone** (2026-10-25), now as the editor
  window's first tab; nothing opens a Map by itself.
- **Tabs grouped by kind, the pinned clock, and Watch beside its Map**
  ([increment 10](0010-one-window.md), 2026-10-25 and 2026-10-26).

Chosen while planning, for its author to check:

- **`make demo` opens the one window,** and `make demo-shell` goes, so trying
  an increment is `make demo DEMO_PROJECT=.cache/demo-<name>`.
- **Popping a tab out goes:** Cmd+Shift+O, each tab's pop-out box, and the
  protocol's pop out. The bar keeps its close boxes.
- **The build check goes.** A pane always comes from the same program as the
  orchestrator, so the editor no longer hashes its own executable; the
  protocol version stays, at 16.
- **A crash outside a tab ends the editor,** after the orchestrator saves
  everything; its failure goes to standard error.
- **A second launch on an open project** is still refused, now naming the
  editor's process instead of a socket.

Chosen while building, for its author to check:

- **On top is the whole window's.** Each pane's status bar keeps its toggle;
  turning it on or off in any pane does it for the editor window, and every
  pane's toggle shows it.
- **The quit dialog says "Every tab will close."** instead of counting
  windows, and a tab with unsaved changes and no name is "a text tab".
- **Words the author sees follow the vocabulary:** "Open in Audio", "Close
  tab" on the Map's keys sheet, "Start a preview in a Map", "set on the
  window bar's clock", "Changed in another tab", and every view's title ends
  in "(view …)", which the window bar leaves off.
- **The pinned clock closes only if a script or a crash closes it;** the bar
  then shows no readout until the editor opens again.
- **A hidden tab is not drawn,** as in the spike, so a Map behind another tab
  refreshes its sprites' art only once it is shown. Real-window tests that
  check drawing bring their pane's tab to the front with a script step
  (`front`).

Asked for by its author on 2026-10-30, after trying it: "can you make the
lantern use a palette for 'dark and gloomy'?", in the sample, not only in one
project.

- **The sample's palette is "gloomy",** in place of "Sample": a ramp of five
  dim iron greys, with a little violet, and the glass ramp in a smoky amber
  at 40%. The lantern is painted only from it, no longer with Default's
  greys, and the sample zone lists it. (Chosen for its author to check: the
  colours, #16151A to #544E60 for the iron and #6B4A2A and #9C7440 for the
  glass.)

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. Plan | This record; the decision in the editor features and the vocabulary | Its author has answered every question | Built |
| 2. One process | `open` runs the orchestrator and the editor window in one process, and the orchestrator shows every pane in the editor window. Gone with the processes: the launcher, child processes, the `window` command, the build check, `open --shell` and `make demo-shell`, the first Map window, popping out (protocol version 16), a host running alone and the standalone Clock window. A pane that panics tells the orchestrator before it closes, which saves its crash report and tells the other tabs; On top is the whole window's | Headless orchestrator tests with a fake editor window, among them a pane's crash report; the real-window tests of tabs pass | Built |
| 3. Pipes | The orchestrator hands out in-memory connections instead of listening on a socket; the client connects over one; each connection keeps its messages in order and does not hold a pane up while the orchestrator is busy; the lock names the process | Orchestrator, client and protocol tests pass over pipes; no socket file is made | Built |
| 4. Real-window tests | Every real-window test runs in the one window: a Map opened from the project browser, the editor quit from the browser once it is the only tab; the tests only for separate windows go; a tab crashed with Cmd+Shift+K leaves a report | `make gui-test` passes, its screenshots checked by eye | Built |
| 5. Documents | The architecture, editor features, vocabulary, research notes, READMEs and contributor rules describe one window in one process | No document describes a window process, `--shell` or popping out, except as history | Built |
| 6. Trying it | The sample project in the one window, for its author to try | Judged by its author | Built |

Where each step changes code:

- **One process:** `editor/cmd/berad-editor` (`main.go`), `editor/protocol`,
  `editor/client`, `editor/orchestrator` (`launcher.go` goes, `hosting.go`,
  `crash.go`, `handlers.go`, `previews.go`, `quit.go`), `editor/window`
  (`window.go`, `shell.go`, `surface.go` goes, `bar.go`, `quit.go`), the
  `Makefile`.
- **Pipes:** `editor/orchestrator` (`server.go`, `orchestrator.go`,
  `lock.go`, `config.go`), `editor/client`.
- **Real-window tests:** `editor/cmd/berad-editor/*_test.go`, and in
  `editor/window` a script locator for how many tabs are open.

## Step results

- **One process:** `berad-editor open` runs the orchestrator on a goroutine
  and the editor window on the main goroutine, as Ebitengine requires; the
  window closing stops the orchestrator, and the orchestrator stopping, for
  Ctrl+C or once a quit has closed every pane, closes the window. The
  orchestrator asks the editor window for every pane it opens and refuses to
  open one while the window is not connected (`ErrNoWindow`); the launcher,
  child processes, their logs and crash reports, the `window` command, the
  build check (`Hello.Build`), `open --shell`, `LaunchWindow`/`LaunchShell`,
  the first Map window, popping out (`Open.PopOut`, `client.PopOut`,
  Cmd+Shift+O, the bar's pop-out box), a host running alone (`ownWindow` and
  the `surface` interface) and `make demo-shell` are gone; protocol version
  16. A pane that panics sends `Crashed` with its stack before it closes; the
  orchestrator saves `.berad/logs/<window>-crash.json` with the failure and
  recent traffic and tells the other panes. On top is the whole window's: any
  pane's toggle sets it and every pane's toggle shows it. The quit dialog
  says "Every tab will close." Verified with `make check` (no lint findings,
  every package passing with the race detector); headless tests with a fake
  editor window: windows shown as panes, opening again drawing attention, the
  Clock asked for once, a pick opening a Map, Open refused without the
  window, a quit stopping the orchestrator once every pane has left, and a
  pane's crash report; unit tests of On top and the close button; and the
  six real-window tests of tabs, run in one process, their screenshots
  checked: tabs with close boxes only, and Watch beside its Map.
- **Pipes:** the orchestrator no longer listens on a socket. `Connect` hands
  a pane one end of a `net.Pipe` and serves the other, with the same
  handshake, framing and JSON codec; the client dials through a `Dialer`, and
  the editor window's panes through the orchestrator's `Connect`. Each
  connection reads into a queue of its own, 256 messages long, so a pane
  sending while the orchestrator is busy is not held up, and its messages,
  and then its leaving, still reach the orchestrator in order. The lock file
  names the process that holds the project, and a second launch is refused
  with it (`ErrProjectOpen by process N`); `SocketPath`, `RunningSocket` and
  the socket file are gone. The test of a killed editor now kills a child
  process that runs an orchestrator and a pane together. Verified with
  `make check`, the six real-window tests of tabs over pipes (no socket file
  made), and `make latency`: command to event over 2000 paints, median
  18.8 µs, p95 39.0 µs, worst 0.25 ms, against 27.0 µs, 49.7 µs and 0.23 ms
  over main's socket on the same machine.
- **Real-window tests:** every real-window test runs in the one window
  through one harness (`runEditor`). The project browser opens the sample
  zone when a test needs a Map (`openZone`), and quits the editor once its
  tab is the only one left (`quitWhenAlone`, on a new script locator,
  `state:tabs:N`); tests that only kept a Map to close it lost it. A script
  step can bring its pane's tab to the front (`front`, or `mustInFront`
  while waiting), since a hidden tab is not drawn: the test of a sprite on
  the Map uses it, and failed without it. A screenshot is of the whole
  window, so every test but those of tabs takes its pane's screenshots with
  its tab in front (`shoot`); before that, the quit test's screenshot showed
  the Map instead of the browser's dialog. The tests of tabs moved to
  `tabs_test.go` with names that do not say "shell"; the pop-out test, the
  separate-windows half of the frame-time test and the test of windows
  closing when the orchestrator is killed are gone; a tab crashed with
  Cmd+Shift+K leaves a crash report naming its view and failure
  (`TestCrashedTabLeavesAReport`). Verified with `make gui-test`: all 33
  tests pass, their screenshots checked by eye, among them the Map in front
  showing the recoloured stroke and the lantern, tabs with close boxes only,
  and Watch beside its Map.
- **Documents:** the architecture's established context, terms, D1 to D10,
  D12, D13 and D15 now speak of the editor window, tabs and panes, with D6's
  transport the in-memory pipes and D10's failure a pane's crash report; the
  editor features record the decision and the pinned clock; the vocabulary
  has the editor window, window bar, tab, pane, view, split and pinned clock
  instead of the window process and the Clock window; the research notes
  mark this as chosen; both READMEs, the contributor rules (panes never touch
  project files, though they share the orchestrator's process) and the
  comments in the code follow, and [increment 10](0010-one-window.md) notes
  that it was adopted here.

- **Trying it:** the sample project in the one window, in a fresh folder
  since the sample is made only in an empty project:
  `make demo DEMO_PROJECT=.cache/demo-single-window`. After trying it, the
  sample's lantern is painted from a palette of its own, gloomy, of dim iron
  and smoky glass; verified with `make check` and the real-window tests of the
  sample, the Pixel editor and the sprite on the Map, their screenshots
  checked by eye.

## Not in this increment

- **Restoring tabs** when the editor opens (D10's restoring windows).
- **Collapsing the inspector** beside Watch, still the first follow-up of the
  split.
- **Dragging tabs** to split or reorder them, and more than two panes in a
  tab.
- **One status bar for the window** instead of one per pane.
- **Linux and Windows:** judged on macOS, as increment 10 was.
