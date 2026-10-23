package orchestrator

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/engine/ids"
)

const logTailBytes = 4096

// ErrNoLauncher reports an Open request to an orchestrator that cannot start
// windows.
var ErrNoLauncher = errors.New("this orchestrator cannot open windows")

// Launcher builds the command that starts one window process.
type Launcher interface {
	Command(request LaunchRequest) (*exec.Cmd, error)
}

// LaunchRequest describes a window to start.
type LaunchRequest struct {
	View   string
	Target ids.ULID
	Window ids.ULID
	Socket string
	Build  string
}

// WindowArgs returns the command-line arguments a window process receives.
func WindowArgs(request LaunchRequest) []string {
	return []string{
		"window",
		"--socket", request.Socket,
		"--view", request.View,
		"--target", request.Target.String(),
		"--window", request.Window.String(),
		"--build", request.Build,
	}
}

// ExecutableLauncher starts windows by running an executable, normally the
// editor program itself, with WindowArgs.
type ExecutableLauncher struct {
	Path string
	Env  []string
}

// Command builds the window command.
func (l ExecutableLauncher) Command(request LaunchRequest) (*exec.Cmd, error) {
	if l.Path == "" {
		return nil, fmt.Errorf("%w: no executable", ErrConfig)
	}
	// The path is the configured editor executable, and windows are not bound to
	// a context: the orchestrator manages their lifetime.
	cmd := exec.Command(l.Path, WindowArgs(request)...) //nolint:gosec,noctx // See above.
	cmd.Env = append(os.Environ(), l.Env...)

	return cmd, nil
}

// CrashReport is written when a window process exits unexpectedly.
type CrashReport struct {
	Window  ids.ULID   `json:"window"`
	View    string     `json:"view"`
	Target  ids.ULID   `json:"target"`
	Exit    string     `json:"exit"`
	At      time.Time  `json:"at"`
	LogTail string     `json:"logTail"`
	Recent  []Recorded `json:"recent"`
}

type child struct {
	request LaunchRequest
	cmd     *exec.Cmd
	logPath string
}

type childExit struct {
	window ids.ULID
	err    error
}

// launch starts a window process and watches for its exit.
func (o *Orchestrator) launch(view string, target ids.ULID) (ids.ULID, error) {
	if o.cfg.Launcher == nil {
		return ids.ULID{}, ErrNoLauncher
	}
	window, err := o.ids.New()
	if err != nil {
		return ids.ULID{}, fmt.Errorf("window id: %w", err)
	}
	request := LaunchRequest{View: view, Target: target, Window: window, Socket: o.cfg.SocketPath, Build: o.cfg.Build}
	cmd, err := o.cfg.Launcher.Command(request)
	if err != nil {
		return ids.ULID{}, fmt.Errorf("prepare window: %w", err)
	}
	logPath := filepath.Join(logDir, window.String()+".log")
	logFile, err := o.project.root.OpenFile(logPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, filePerm)
	if err != nil {
		return ids.ULID{}, fmt.Errorf("open window log: %w", err)
	}
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		return ids.ULID{}, errors.Join(fmt.Errorf("start window: %w", err), logFile.Close())
	}
	o.children[window] = &child{request: request, cmd: cmd, logPath: logPath}
	go func() {
		waitErr := cmd.Wait()
		closeErr := logFile.Close()
		select {
		case o.exits <- childExit{window: window, err: errors.Join(waitErr, closeErr)}:
		case <-o.stopped:
		}
	}()

	return window, nil
}

// childExited handles a window process ending. An unexpected failure produces
// a crash report and a notice to the remaining windows.
func (o *Orchestrator) childExited(exit childExit) {
	started, ok := o.children[exit.window]
	delete(o.children, exit.window)
	defer delete(o.recent, exit.window)
	if !ok || exit.err == nil || o.stopping {
		return
	}
	reportPath, err := o.writeCrashReport(started, exit)
	if err != nil {
		o.cfg.Logger.Error("could not write window crash report", "window", exit.window, "error", err)
	}
	o.cfg.Logger.Warn("window closed unexpectedly", "window", exit.window, "error", exit.err, "report", reportPath)
	notice := protocol.Notice{
		Kind:   protocol.NoticeWindowCrashed,
		Text:   "A " + started.request.View + " window closed unexpectedly. A failure log was saved.",
		Window: exit.window,
		Report: reportPath,
	}
	o.publishNotice(notice)
}

func (o *Orchestrator) writeCrashReport(started *child, exit childExit) (string, error) {
	report := CrashReport{
		Window:  exit.window,
		View:    started.request.View,
		Target:  started.request.Target,
		Exit:    exit.err.Error(),
		At:      o.cfg.Clock().UTC(),
		LogTail: o.logTail(started.logPath),
		Recent:  nil,
	}
	if recent, ok := o.recent[exit.window]; ok {
		report.Recent = recent.list()
	}
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode crash report: %w", err)
	}
	path := filepath.Join(logDir, exit.window.String()+"-crash.json")
	if err := o.project.writeAtomic(path, append(encoded, '\n')); err != nil {
		return "", err
	}

	return path, nil
}

func (o *Orchestrator) logTail(path string) string {
	raw, err := o.project.root.ReadFile(path)
	if err != nil {
		return ""
	}
	if len(raw) > logTailBytes {
		raw = raw[len(raw)-logTailBytes:]
	}

	return string(raw)
}

// waitForChildren gives windows until the deadline to exit, then kills the rest.
func (o *Orchestrator) waitForChildren(deadline time.Time) {
	timer := time.NewTimer(time.Until(deadline))
	defer timer.Stop()
	for len(o.children) > 0 {
		select {
		case exit := <-o.exits:
			delete(o.children, exit.window)
		case <-timer.C:
			for window, started := range o.children {
				o.cfg.Logger.Warn("killing window that did not close", "window", window)
				if err := started.cmd.Process.Kill(); err != nil {
					o.cfg.Logger.Warn("kill window", "window", window, "error", err)
				}
			}

			return
		}
	}
}
