package workflowview

import (
	"maps"
	"strings"

	flow "github.com/bradhannah/berad-engine/engine/workflow"
)

// helpRow is one output or setting on a node's help page: its name and
// what it means.
type helpRow struct {
	name string
	what string
}

// kindHelp is a node kind's help page: what it does, how long it takes,
// its outputs and when each is taken, its settings, and an example.
type kindHelp struct {
	about    string
	time     string
	outputs  []helpRow
	settings []helpRow
	example  string
}

// Words shared by several help pages.
const (
	helpInstant   = "Takes no time"
	helpNext      = "next"
	helpCarriesOn = "it has done its work, and carries on"
)

// helpFor returns a kind's help page; ok is false for a kind without one.
func helpFor(kind string) (kindHelp, bool) {
	help, ok := helpPages()[kind]

	return help, ok
}

// helpPages holds every kind's help page, beside its one line in hintFor.
func helpPages() map[string]kindHelp {
	pages := flowHelp()
	maps.Copy(pages, actionHelp())
	maps.Copy(pages, valueHelp())

	return pages
}

// flowHelp is the help of the Flow group's nodes.
func flowHelp() map[string]kindHelp {
	return map[string]kindHelp{
		flow.KindTimeSwitch: {
			about: "Looks at the time of day on the game clock and takes the output of the first time that " +
				"holds it. A time can run past midnight.",
			time: helpInstant, outputs: []helpRow{{"each time", "the game clock is within it"}, {"otherwise", "no time holds"}},
			settings: []helpRow{{"Times", "each a from–to time with an output of its own"}},
			example:  "08:00–18:00 → Shop hours; otherwise → Go to · Home",
		},
		flow.KindRandom: {
			about: "Takes one of its options at random, each as often as its share of the total. A run with the " +
				"same seed makes the same choices.",
			time: helpInstant, outputs: []helpRow{{"each option", "chosen by its share"}},
			settings: []helpRow{{"Options", "each a name and a share"}},
			example:  "tidy the shelf 3, stretch 1 → tidies three times in four",
		},
		flow.KindContainer: {
			about: "Holds steps that repeat while its time of day lasts. When the time is up the NPC leaves by " +
				"its exit at once, even partway through a step.",
			time: "Takes as long as its time lasts",
			outputs: []helpRow{
				{"inside", "its first step"}, {"exit", "its time is up, or it was reached outside its time"},
			},
			settings: []helpRow{{"Name", "what the steps inside are for"}, {"Time", "the time of day they run"}},
			example:  "Shop hours 08:00–18:00, holding Go to · Desk and Wait → exit → Go to · Home",
		},
		flow.KindCall: {
			about: "Runs another workflow, a module, giving it its inputs, and carries on by the output named " +
				"for the way out it ended at.",
			time: "Takes as long as the module runs",
			outputs: []helpRow{
				{"each way out", "the module ended at that Return"}, {"done", "it ended with nothing after it"},
			},
			settings: []helpRow{
				{"Workflow", "the module it runs"}, {"Inputs", "what it gives the module"},
				{"Keeps", "values its outputs are kept in"},
			},
			example: "Call · Fetch stock: fetched → Wait; none → Go to · Desk",
		},
		flow.KindReturn: {
			about: "Ends a module, going back to the Call that ran it by the output of its way out's name. Run " +
				"as a behaviour, it starts again.",
			time: helpInstant, outputs: nil, settings: []helpRow{{"Way out", "the name of the Call's output it leaves by"}},
			example: "Return · fetched",
		},
	}
}

// actionHelp is the help of the nodes that move, wait, play and note.
func actionHelp() map[string]kindHelp {
	return map[string]kindHelp{
		flow.KindGoTo: {
			about: "Walks the NPC to a place, finding its way around walls and up and down stairs, then " +
				"carries on.",
			time: "Takes as long as the walk", outputs: []helpRow{{helpNext, "it has arrived, or can't get there"}},
			settings: []helpRow{{"Goes to", "a place picked on a map or chosen by name, or a place value"}},
			example:  "Go to · Counter → Wait · 10 s",
		},
		flow.KindMove: {
			about: "Moves the NPC a number of tiles in one direction. If something is in the way it stops there.",
			time:  "Takes a step for each tile", outputs: []helpRow{{helpNext, "it has moved, or was blocked"}},
			settings: []helpRow{{"Direction", "up, down, left or right"}, {"Tiles", "how far"}},
			example:  "Move · down 2 tiles → Move · up 2 tiles",
		},
		flow.KindWait: {
			about: "Waits a while: on the game clock, which pauses and speeds up with the game, or in real time.",
			time:  "Takes its duration", outputs: []helpRow{{helpNext, "the time has passed"}},
			settings: []helpRow{{"Duration", "how long, in seconds, minutes or hours"}, {"Clock", "game time or real time"}},
			example:  "Wait · 30 min → Go to · Shelf",
		},
		flow.KindPlaySound: {
			about: "Plays a cue from the audio library, heard from the NPC, or across the zone in a zone loop.",
			time:  helpInstant, outputs: []helpRow{{helpNext, "the cue has started"}},
			settings: []helpRow{{"Cue", "what plays"}, {"Loudness", "as the cue is, or a step louder or quieter"}},
			example:  "Play sound · Chime → Find NPC nearby",
		},
		flow.KindNote: {
			about: "A note for authors on the canvas. It is not part of the flow, and nothing links to it.",
			time:  "Never runs", outputs: nil, settings: []helpRow{{"Text", "what the note says"}},
			example: "A note beside a container saying what its steps are for",
		},
		flow.KindLog: {
			about: "Writes a line of text and values to the Watch window's log while the preview runs, for you. " +
				"Players never see it.",
			time: helpInstant, outputs: []helpRow{{helpNext, helpCarriesOn}},
			settings: []helpRow{{"Message", "text, with values put in by Insert value…"}},
			example:  "Log · Serving {local.customer}",
		},
	}
}

// valueHelp is the help of the Values group's nodes.
func valueHelp() map[string]kindHelp {
	return map[string]kindHelp{
		flow.KindSetValue: {
			about: "Sets one of the workflow's own values, its NPC's, its zone's, the world's or a local one, or " +
				"adds to a number.",
			time: helpInstant, outputs: []helpRow{{helpNext, helpCarriesOn}},
			settings: []helpRow{
				{"Value", "the value it sets"}, {"How", "set it to, or add to a number"},
				{"To", "a fixed value, or another value"},
			},
			example: "Set npc.shop.serving to yes",
		},
		flow.KindCheckValue: {
			about: "Looks at one value, the workflow's own or another NPC's, and compares it as its type allows: " +
				"a number with at least or below, a choice with any of, text with is.",
			time: helpInstant,
			outputs: []helpRow{
				{"yes", "the comparison holds"}, {"otherwise", "it doesn't, or the value can't be read"},
			},
			settings: []helpRow{
				{"Value", "the value to look at"}, {"Compare", "only the comparisons its type takes"},
				{"With", "a fixed value, or another value of its type"},
			},
			example: "Is npc.shop.serving true? yes → Go to · Counter; otherwise → Wait · 10 s",
		},
		flow.KindFindNPC: {
			about: "Looks for the nearest other NPC on its floor within a number of tiles, optionally one whose " +
				"value matches, and keeps it in a local value.",
			time: helpInstant,
			outputs: []helpRow{
				{"found", "an NPC was found and kept"}, {"none", "no NPC was near enough, and the local is cleared"},
			},
			settings: []helpRow{
				{"Within", "how many tiles around it"}, {"Where", "a value the NPC must have"},
				{"Into", "the local NPC value it keeps the NPC in"},
			},
			example: "Find NPC within 2 tiles where npc.shop.waiting is yes → found → Set npc.shop.serving to yes",
		},
	}
}

// offeredIn names the kinds of workflow that offer a node kind.
func offeredIn(kind string) string {
	var offered []string
	for _, purpose := range flow.Purposes() {
		if purpose.Allows(kind) {
			offered = append(offered, inSentence(purpose.Label())+"s")
		}
	}
	if len(offered) == 1 {
		return "In " + offered[0] + " only"
	}

	return "In " + strings.Join(offered, " and ")
}

// inSentence writes a label as it reads inside a sentence: its first
// letter small, unless it starts a name in capitals, as NPC does.
func inSentence(label string) string {
	if len(label) > 1 && strings.ToUpper(label[:2]) == label[:2] {
		return label
	}

	return strings.ToLower(label[:1]) + label[1:]
}

// helpSummary is the line under a help page's title: how long the node
// takes, how many outputs it has, and which kinds of workflow offer it.
func helpSummary(kind string, help kindHelp) string {
	outputs := "no outputs"
	switch count := len(help.outputs); {
	case count == 1 && help.outputs[0].name == helpNext:
		outputs = "1 output"
	case count > 0:
		outputs = strings.Join(rowNames(help.outputs), ", ")
	}

	return help.time + " · " + outputs + " · " + offeredIn(kind)
}

// rowNames lists the names of help rows.
func rowNames(rows []helpRow) []string {
	names := make([]string, 0, len(rows))
	for _, row := range rows {
		names = append(names, row.name)
	}

	return names
}

// searchText is everything a palette search looks through for a kind: its
// name, its group, its one line, its help and its settings.
func searchText(kind flow.Kind) string {
	parts := []string{kind.Label, kind.Group, hintFor(kind.Name)}
	if help, ok := helpFor(kind.Name); ok {
		parts = append(parts, help.about)
		parts = append(parts, rowNames(help.settings)...)
	}
	for _, param := range kind.Params {
		parts = append(parts, param.Label)
	}

	return strings.ToLower(strings.Join(parts, "\n"))
}

// matchesSearch reports whether a kind matches a search, and whether by
// its name, which puts it first.
func matchesSearch(kind flow.Kind, query string) (bool, bool) {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return true, false
	}
	byName := strings.Contains(strings.ToLower(kind.Label), query)

	return byName || strings.Contains(searchText(kind), query), byName
}
