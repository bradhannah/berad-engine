package orchestrator

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
