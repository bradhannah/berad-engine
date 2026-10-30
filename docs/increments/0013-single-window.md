# Increment 13: Single window

**Status:** Planned

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

## Steps

| Step | Builds | Done when | Status |
| --- | --- | --- | --- |
| 1. Plan | This record; the decision in the editor features and the vocabulary | Its author has answered every question | Built |
| 2. Pipes | The orchestrator hands out in-memory connections instead of listening on a socket; the client connects over one; each connection keeps its messages in order and does not hold a pane up while the orchestrator is busy; the lock names the process | Orchestrator, client and protocol tests pass over pipes; no socket file is made | Planned |
| 3. One process | `open` runs the orchestrator and the editor window in one process; the orchestrator shows every pane in the editor window; quitting, Ctrl+C and a failure stop both; the launcher, child processes, the `window` command and the build check go | Headless orchestrator tests with a fake editor window; the real-window tests that open tabs pass | Planned |
| 4. No multi-window option | Pop out (protocol version 16), `open --shell`, a host running alone, the standalone Clock window, the first Map window, `make demo-shell` | Nothing in the code, Makefile or tests names them; `make check` is clean | Planned |
| 5. Crash reports for tabs | A pane that panics tells the orchestrator before it closes; the orchestrator saves its crash report and tells the other tabs | A headless test of a pane's report; a real-window test that crashes a tab with Cmd+Shift+K | Planned |
| 6. Real-window tests | Every real-window test runs in the one window: a Map opened from the project browser, the editor quit from the browser once it is the only tab; the tests only for separate windows go | `make gui-test` passes, its screenshots checked by eye | Planned |
| 7. Documents | The architecture, editor features, vocabulary, research notes, READMEs and contributor rules describe one window in one process | No document describes a window process, `--shell` or popping out, except as history | Planned |
| 8. Trying it | The sample project in the one window, for its author to try | Judged by its author | Planned |

Where each step changes code:

- **Pipes:** `editor/orchestrator` (`server.go`, `orchestrator.go`,
  `lock.go`, `config.go`), `editor/client`.
- **One process:** `editor/cmd/berad-editor` (`main.go`),
  `editor/orchestrator` (`launcher.go`, `hosting.go`, `previews.go`,
  `quit.go`), `editor/window` (`window.go`, `shell.go`).
- **No multi-window option:** `editor/protocol`, `editor/client`,
  `editor/orchestrator` (`hosting.go`, `handlers.go`), `editor/window`
  (`surface.go`, `bar.go`, `quit.go`), the `Makefile`.
- **Crash reports for tabs:** `editor/protocol`, `editor/client`,
  `editor/window` (`shell.go`), `editor/orchestrator`.
- **Real-window tests:** `editor/cmd/berad-editor/*_test.go`, and in
  `editor/window` a script locator for how many tabs are open.

## Step results

## Not in this increment

- **Restoring tabs** when the editor opens (D10's restoring windows).
- **Collapsing the inspector** beside Watch, still the first follow-up of the
  split.
- **Dragging tabs** to split or reorder them, and more than two panes in a
  tab.
- **One status bar for the window** instead of one per pane.
- **Linux and Windows:** judged on macOS, as increment 10 was.
