package orchestrator

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// CrashReport is written when a pane fails (D10): what it showed, its failure
// with its stack, and its recent protocol traffic.
type CrashReport struct {
	Window  ids.ULID   `json:"window"`
	View    string     `json:"view"`
	Target  ids.ULID   `json:"target"`
	Failure string     `json:"failure"`
	At      time.Time  `json:"at"`
	Recent  []Recorded `json:"recent"`
}

// crashed saves the crash report of a pane that failed and is closing, and
// tells the other windows.
func (o *Orchestrator) crashed(from *window, message protocol.Message) {
	crashed, err := protocol.DecodePayload[protocol.Crashed](message)
	if err != nil {
		return
	}
	reportPath, err := o.writeCrashReport(from, crashed.Failure)
	if err != nil {
		o.cfg.Logger.Error("could not write a crash report", "window", from.id, "error", err)
	}
	o.cfg.Logger.Warn("a tab closed unexpectedly", "window", from.id, "view", from.hello.View, "report", reportPath)
	o.publishNotice(protocol.Notice{
		Kind:   protocol.NoticeWindowCrashed,
		Text:   "A " + from.hello.View + " tab closed unexpectedly. A failure report was saved.",
		Window: from.id,
		Report: reportPath,
	})
}

func (o *Orchestrator) writeCrashReport(from *window, failure string) (string, error) {
	report := CrashReport{
		Window:  from.id,
		View:    from.hello.View,
		Target:  from.hello.Target,
		Failure: failure,
		At:      o.cfg.Clock().UTC(),
		Recent:  nil,
	}
	if recent, ok := o.recent[from.id]; ok {
		report.Recent = recent.list()
	}
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode crash report: %w", err)
	}
	path := filepath.Join(logDir, from.id.String()+"-crash.json")
	if err := o.project.writeAtomic(path, append(encoded, '\n')); err != nil {
		return "", err
	}

	return path, nil
}
