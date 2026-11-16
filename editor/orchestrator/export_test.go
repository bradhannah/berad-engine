package orchestrator

import "log/slog"

// EditHasWords reports whether a kind of edit is said in words of its own,
// for tests.
func EditHasWords(edit string) bool {
	if _, ok := editPhrases()[edit]; ok {
		return true
	}
	switch edit {
	case "project.create", "project.delete", "project.rename", "project.move", "project.folder":
		return true
	}

	return false
}

// OutsideAfterNote writes mine to a file of a project in dir, then theirs
// over it, as a change from outside can land just after the editor's write
// and before the editor notes what it wrote; it reports whether the file
// then counts as changed outside the editor, for tests.
func OutsideAfterNote(dir string, mine, theirs []byte) (bool, error) {
	opened, err := openProject(dir, slog.New(slog.DiscardHandler))
	if err != nil {
		return false, err
	}
	defer func() { _ = opened.close() }()
	const path = "changed.json"
	if err := opened.root.WriteFile(path, mine, filePerm); err != nil {
		return false, err
	}
	if err := opened.root.WriteFile(path, theirs, filePerm); err != nil {
		return false, err
	}
	opened.note(path, mine)
	changed, _ := opened.outside(path)

	return changed, nil
}
