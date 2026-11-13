package orchestrator

import (
	"encoding/json"
	"strings"

	"github.com/bradhannah/berad-engine/engine/ids"
)

// Every change is a sentence (increment 39): what kind of change, to what,
// and where. The orchestrator says it when a command is applied, keeps it
// with the undo step the command joins, and answers Undo and Redo with it,
// so "Undid: Painted Ground floor" says what an undo took back. A step keeps
// the sentence of its first command.

// editPhrases says each kind of change in words, ahead of the name of what
// it changed.
func editPhrases() map[string]string {
	return map[string]string{
		"art.ramp.add": "Added a ramp to", "art.ramp.move": "Moved a ramp in",
		"art.ramp.remove": "Removed a ramp from", "art.ramp.rename": "Renamed a ramp in",
		"art.settings.tile-size": "Changed the tile size in", "art.shade.add": "Added a shade to",
		"art.shade.move": "Moved a shade in", "art.shade.remove": "Removed a shade from",
		"art.shade.set": "Changed a shade in", "art.sprite.template": "Changed the template of",
		"art.standin.set": "Changed a stand-in in", "art.template.set": "Changed",
		"art.tile.paint": "Painted", "game.settings.screen": "Changed the screen's size in",
		"game.settings.start": "Changed where the game starts in", "npc.behaviour.set": "Changed the behaviour of",
		"npc.constant.set": "Changed a constant of", "npc.notes.set": "Changed the notes on",
		"npc.path.set": "Changed the path of", "npc.sprite.set": "Changed the sprite of",
		"npc.template.apply": "Applied a template to", "npc.template.unapply": "Took a template off",
		"npc.values.change": "Changed the values of", "npc.zone.set": "Changed the zone of",
		"project.tags": "Changed the tags of", "project.touch": "Touched",
		"tilefloor.anchor.add": "Added an anchor to", "tilefloor.anchor.remove": "Removed an anchor from",
		"tilefloor.anchor.update": "Changed an anchor in", "tilefloor.paint": "Painted",
		"tilefloor.placement.add": "Placed an NPC on", "tilefloor.placement.remove": "Took an NPC off",
		"tilefloor.placement.update": "Changed an NPC's place on", "tilefloor.resize": "Resized",
		"tilefloor.sound.add": "Added a sound to", "tilefloor.sound.remove": "Removed a sound from",
		"tilefloor.sound.update": "Changed a sound in", "views.set": "Changed the saved views in",
		"workflow.layout.move": "Moved nodes in", "workflow.layout.tint": "Coloured nodes in",
		"workflow.locals.change": "Changed the local values of", "workflow.node.replace": "Changed a node in",
		"workflow.nodes.add": "Added nodes to", "workflow.nodes.remove": "Removed nodes from",
		"workflow.runs-in.set": "Changed where it runs:", "worldstate.signals.change": "Changed the signals in",
		"worldstate.values.change": "Changed the values in", "zone.constant.set": "Changed a constant of",
		"zone.floor.insert": "Added a floor to", "zone.floor.move": "Moved a floor in",
		"zone.floor.remove": "Removed a floor from", "zone.loop.set": "Changed the zone loop of",
		"zone.notes.set": "Changed the notes on", "zone.palette.insert": "Added a palette to",
		"zone.palette.remove": "Removed a palette from", "zone.path.set": "Changed the path of",
		"zone.sound.insert": "Added a sound to", "zone.sound.remove": "Removed a sound from",
		"zone.template.apply": "Applied a template to", "zone.template.unapply": "Took a template off",
		"zone.values.change": "Changed the values of",
	}
}

// sentenceArgs are the parts of a project edit's arguments a sentence names:
// the document it is about, its new name, and a folder's paths.
type sentenceArgs struct {
	ID   ids.ULID `json:"id"`
	Name string   `json:"name"`
	From string   `json:"from"`
	To   string   `json:"to"`
}

// sentence says a change to a document in words, before it is applied, so a
// rename can still name what it renames.
func (o *Orchestrator) sentence(documentID ids.ULID, edit string, args json.RawMessage) string {
	if said, ok := o.projectSentence(edit, args); ok {
		return said
	}
	name := o.documentName(documentID)
	if phrase, known := editPhrases()[edit]; known {
		return phrase + " " + name
	}

	return "Changed " + name + " (" + edit + ")"
}

// projectSentence says a change to the project's list of documents and
// folders, whose document is the project itself, by what it is about.
func (o *Orchestrator) projectSentence(edit string, args json.RawMessage) (string, bool) {
	var about sentenceArgs
	if !strings.HasPrefix(edit, "project.") || json.Unmarshal(args, &about) != nil {
		return "", false
	}
	switch edit {
	case "project.create":
		return "Made " + about.Name, true
	case "project.delete":
		return "Deleted " + o.documentName(about.ID), true
	case "project.rename":
		return "Renamed " + o.documentName(about.ID) + " to " + about.Name, true
	case "project.move":
		return "Moved " + o.documentName(about.ID) + " to another folder", true
	case "project.folder":
		return folderSentence(about), true
	}

	return "", false
}

// folderSentence says a folder made, deleted, renamed or moved.
func folderSentence(about sentenceArgs) string {
	switch {
	case about.From == "":
		return "Made the folder " + about.To
	case about.To == "":
		return "Deleted the folder " + about.From
	default:
		return "Renamed the folder " + about.From + " to " + about.To
	}
}

// documentName is a document's name, or says it is not known.
func (o *Orchestrator) documentName(documentID ids.ULID) string {
	if entry, ok := o.docs[documentID]; ok && entry.info.Name != "" {
		return entry.info.Name
	}

	return "a document"
}
