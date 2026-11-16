package orchestrator

import (
	"errors"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/bradhannah/berad-engine/editor/protocol"
	"github.com/bradhannah/berad-engine/engine/ids"
)

// A project that cannot open as it is (increment 57) says why before
// anything happens: each file that cannot be read, and a journal that no
// longer fits the files. Until its author answers in the Project tab,
// nothing is written and no change is taken. Open without them leaves each
// file out, never written over, and keeps the journal whole in the backups;
// Quit closes the editor having changed nothing.

// opening is what keeps the project from opening as it is.
type opening struct {
	files []unreadFile
	// journal says what is wrong with the journal, and misfit whether it
	// was not applied at all, so it is set aside rather than kept.
	journal string
	misfit  bool
}

// reasonNotOpen refuses a change while the project waits for its author's
// answer.
const reasonNotOpen = "The project is not open yet: answer the question in the Project tab first"

// backupJournal is the folder in the backups that keeps a journal set aside.
const backupJournal = "journal"

// askOpening holds the project, when it cannot open as it is, until its
// author answers; otherwise it tells what the windows are to show.
func (o *Orchestrator) askOpening(journal string, misfit bool) {
	if len(o.project.unread) == 0 && journal == "" {
		o.tellOpened()

		return
	}
	o.opening = &opening{files: o.project.unread, journal: journal, misfit: misfit}
	files := make([]protocol.UnreadFile, 0, len(o.project.unread))
	for _, file := range o.project.unread {
		files = append(files, protocol.UnreadFile{Path: file.path, Reason: file.reason})
	}
	o.session[protocol.TopicOpening] = mustJSON(protocol.Opening{
		Project: filepath.Base(o.cfg.ProjectDir), Files: files, Journal: journal,
	})
}

// openAnyway opens the project without what it could not read, as its
// author chose.
func (o *Orchestrator) openAnyway() {
	held := o.opening
	if held == nil {
		return
	}
	o.opening = nil
	delete(o.session, protocol.TopicOpening)
	if held.journal != "" {
		o.setJournalAside(held.misfit)
	}
	o.tellUnread(held.files)
	o.tellOpened()
	o.flushLogged()
}

// tellOpened leaves the notices of a project that opened: documents that
// stand in for files that cannot be read, and files holding what the
// editor does not know.
func (o *Orchestrator) tellOpened() {
	o.tellStandIns()
	o.tellUnknown()
}

// setJournalAside keeps a copy of the journal in the backups, and empties
// it when it was not applied, so it is never applied in part.
func (o *Orchestrator) setJournalAside(misfit bool) {
	data, err := o.project.root.ReadFile(journalPath)
	if errors.Is(err, fs.ErrNotExist) {
		return
	}
	var kept string
	if err == nil {
		kept, err = o.project.keepCopy(backupJournal, journalPath, data, o.cfg.Clock())
	}
	if err == nil && misfit {
		err = o.journal.reset()
	}
	if err != nil {
		o.cfg.Logger.Error("could not set the journal aside", "error", err)
		o.tellSaving(err)

		return
	}
	text := "The journal's changes were kept in the backups, at " + kept + ", and not applied."
	if !misfit {
		text = "The journal's changes to documents not in the project were kept in the backups, at " + kept + "."
	}
	o.addNotice(protocol.Notice{
		Kind: protocol.NoticeFilesLeftOut, Text: text, Window: ids.ULID{}, Report: "", Key: "journal-aside",
		Actions: []protocol.NoticeAction{dismissButton()}, More: 0,
	})
}

// journalSaid says what is wrong with a journal that does not fit the
// files, or that holds changes to documents not in the project.
func journalSaid(entries int, failure error, skipped int) string {
	if failure != nil {
		said := "The journal's changes do not fit the files"
		if entries > 0 {
			said = "The journal's " + changesSaid(entries) + " do not fit the files"
		}

		return said + ": " + plainReason(failure)
	}
	if skipped > 0 {
		return "The journal holds " + changesSaid(skipped) + " to documents that are not in the project"
	}

	return ""
}

// changesSaid is "1 change" or "14 changes".
func changesSaid(count int) string {
	if count == 1 {
		return "1 change"
	}

	return strconv.Itoa(count) + " changes"
}

// standInFor finds, among the files left out, the one a document every
// project holds was read from: its path ends in the document's short ID.
func (o *Orchestrator) standInFor(documentID ids.ULID) (string, bool) {
	suffix := "--" + documentID.Short(shortIDChars) + ".json"
	for _, file := range o.project.unread {
		if strings.HasSuffix(file.path, suffix) {
			return file.path, true
		}
	}

	return "", false
}

// tellStandIns leaves a notice for each document every project holds that
// stands in for a file that cannot be read.
func (o *Orchestrator) tellStandIns() {
	for _, documentID := range sortedIDs(o.docs) {
		entry := o.docs[documentID]
		if !entry.standIn {
			continue
		}
		o.addNotice(protocol.Notice{
			Kind: protocol.NoticeStandIn, Window: ids.ULID{}, Report: "", Key: "stand-in:" + documentID.String(),
			Text: entry.info.Name + " could not be read, so a fresh one stands in for it while the editor is open. " +
				"Nothing is written over its file, and changes to the stand-in are not kept.",
			Actions: []protocol.NoticeAction{dismissButton()}, More: 0,
		})
	}
}
