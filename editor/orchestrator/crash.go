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
	o.addNotice(protocol.Notice{
		Kind:    protocol.NoticeWindowCrashed,
		Text:    "The " + tabName(from.hello.View) + " tab closed unexpectedly. A failure report was saved.",
		Window:  from.id,
		Report:  reportPath,
		Key:     crashKeyPrefix + from.id.String(),
		Actions: []protocol.NoticeAction{{ID: reopenAction, Label: "Reopen"}, dismissButton()},
		More:    0,
	})
	o.crashedPanes[crashKeyPrefix+from.id.String()] = hostedPane{view: from.hello.View, target: from.hello.Target}
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

// A crashed tab reopens when told (increment 60): its notice offers Reopen,
// and nothing reopens by itself.
const (
	crashKeyPrefix = "crash:"
	reopenAction   = "reopen"
)

// reopenCrashed shows a crashed tab's view again, as its notice's Reopen
// asks, and ends the notice; one that cannot be shown says why.
func (o *Orchestrator) reopenCrashed(key string) {
	crashed, known := o.crashedPanes[key]
	if !known {
		o.endNotice(key)

		return
	}
	if _, err := o.launch(crashed.view, crashed.target); err != nil {
		o.addNotice(protocol.Notice{
			Kind: protocol.NoticeWindowCrashed, Window: ids.ULID{}, Report: "", Key: key, More: 0,
			Text:    "The " + tabName(crashed.view) + " tab closed unexpectedly, and could not be reopened: " + err.Error(),
			Actions: []protocol.NoticeAction{dismissButton()},
		})

		return
	}
	delete(o.crashedPanes, key)
	o.endNotice(key)
}

// tabName names a tab by its view as an author knows it: the project
// browser is the Project tab.
func tabName(view string) string {
	if view == "browser" {
		return "Project"
	}

	return view
}
