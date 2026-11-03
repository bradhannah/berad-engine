package zonepicker_test

import (
	"image"
	"testing"

	npcplugin "github.com/bradhannah/berad-engine/editor/plugins/npc"
	"github.com/bradhannah/berad-engine/editor/plugins/project"
	"github.com/bradhannah/berad-engine/editor/plugins/tilefloor"
	zoneplugin "github.com/bradhannah/berad-engine/editor/plugins/zone"
	"github.com/bradhannah/berad-engine/editor/ui"
	"github.com/bradhannah/berad-engine/editor/views/frame/frametest"
	"github.com/bradhannah/berad-engine/editor/views/zonepicker"
	"github.com/bradhannah/berad-engine/engine/ids"
	"github.com/bradhannah/berad-engine/engine/npc"
	floor "github.com/bradhannah/berad-engine/engine/tilefloor"
	"github.com/bradhannah/berad-engine/engine/zone"
)

// window is the window the picker opens over in these tests.
var window = image.Rect(0, 0, 1280, 800)

// town makes three zones: Harbour, in Coast, with two floors, a place and
// an NPC living in it; Smugglers' cave, in Coast/Caves; and Market, at the
// top. Harbour and Market are tagged town.
func town(t *testing.T) (*frametest.Docs, map[string]ids.ULID) {
	t.Helper()
	docs := frametest.NewDocs()
	made := map[string]ids.ULID{
		"Harbour": frametest.ID(1), "Smugglers' cave": frametest.ID(2), "Market": frametest.ID(3),
		"Quay": frametest.ID(4), "Loft": frametest.ID(5), "Fisher": frametest.ID(6),
	}
	entry := func(name, format, folder string, tags ...string) project.Entry {
		return project.Entry{ID: made[name], Name: name, Format: format, Folder: folder, Tags: tags}
	}
	entries := []project.Entry{
		entry("Harbour", zone.FormatName, "Coast", "town"), entry("Smugglers' cave", zone.FormatName, "Coast/Caves", "hard"),
		entry("Market", zone.FormatName, "", "town"), entry("Quay", floor.FormatName, ""),
		entry("Loft", floor.FormatName, ""), entry("Fisher", npc.FormatName, ""),
	}
	folders := []project.Folder{
		{Format: zone.FormatName, Path: "Coast"}, {Format: zone.FormatName, Path: "Coast/Caves"},
		{Format: zone.FormatName, Path: "Inland"},
	}
	docs.Put(project.IndexID(), "Project", project.NewIndexWithFolders(entries, folders))
	harbour := zoneplugin.NewDocument()
	harbour.Zone.Notes = "The east-coast fishing town."
	for index, name := range []string{"Quay", "Loft"} {
		if err := harbour.Zone.Insert(index, made[name]); err != nil {
			t.Fatal(err)
		}
		floorDoc, err := tilefloor.NewDocument(12, 8, "grass")
		if err != nil {
			t.Fatal(err)
		}
		if name == "Quay" {
			if err := floorDoc.Floor.InsertAnchor(0, floor.Anchor{ID: frametest.ID(7), Name: "Dock", Col: 1, Row: 1}); err != nil {
				t.Fatal(err)
			}
		}
		docs.Put(made[name], name, floorDoc)
	}
	docs.Put(made["Harbour"], "Harbour", harbour)
	docs.Put(made["Smugglers' cave"], "Smugglers' cave", zoneplugin.NewDocument())
	docs.Put(made["Market"], "Market", zoneplugin.NewDocument())
	fisher := npcplugin.NewDocument()
	fisher.NPC.Zone = made["Harbour"]
	docs.Put(made["Fisher"], "Fisher", fisher)

	return docs, made
}

// picking is an open picker over the town, and how to drive it.
type picking struct {
	t      *testing.T
	picker *zonepicker.Picker
	offer  zonepicker.Offer
	choice zonepicker.Choice
	chosen bool
	open   bool
}

func newPicking(t *testing.T, config zonepicker.Config, refuse map[ids.ULID]string) *picking {
	t.Helper()
	theme, err := ui.NewTheme()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(theme.Close)
	docs, _ := town(t)
	zones := zonepicker.Gather(docs, func(ids.ULID) {})
	for index := range zones {
		zones[index].Refused = refuse[zones[index].ID]
	}

	return &picking{
		t: t, picker: zonepicker.New(config, theme),
		offer:  zonepicker.Offer{Zones: zones, Folders: zonepicker.Folders(docs)},
		choice: zonepicker.Choice{Zone: ids.ULID{}, None: false}, chosen: false, open: true,
	}
}

func (p *picking) frame(input ui.Input) {
	p.choice, p.chosen, p.open = p.picker.Update(&input, nil, window, p.offer)
}

func (p *picking) click(name string) {
	p.t.Helper()
	at, found := p.picker.Locate(window, p.offer, name)
	if !found {
		p.t.Fatalf("no %s", name)
	}
	input := ui.NewInput(at)
	input.Down, input.Pressed = true, true
	p.frame(input)
	released := ui.NewInput(at)
	released.Released = true
	p.frame(released)
}

func (p *picking) typed(text string) {
	input := ui.NewInput(image.Pt(0, 0))
	input.Text = text
	p.frame(input)
}

func (p *picking) key(name string) {
	input := ui.NewInput(image.Pt(0, 0))
	input.Presses = []ui.Press{ui.ParsePress(name)}
	p.frame(input)
}

// clear empties the search.
func (p *picking) clear() {
	for range 10 {
		p.key("Backspace")
	}
}

func (p *picking) has(state string) bool {
	found, handled := p.picker.HasState(window, p.offer, state)

	return found && handled
}

// TestFindingAZone searches by name, folder and tag, narrows by a folder
// and a tag chip, and reads a zone's look.
func TestFindingAZone(t *testing.T) {
	t.Parallel()
	zones := newPicking(t, zonepicker.Config{For: "for Clerk to live in", Now: ids.ULID{}, Recent: nil, NoZone: false,
		GoesOn: false}, nil)
	if !zones.has("zones-shown=3") || !zones.has("zone-count:Coast=2") || !zones.has("zone-count:Coast/Caves=1") ||
		!zones.has("zone-count:Inland=0") || !zones.has("zone-detail:Harbour=Coast") {
		t.Fatal("every zone is listed, counted in its folder and the folders holding it")
	}
	zones.typed("har")
	if !zones.has("zones-shown=2") || !zones.has("zone-look=Harbour") {
		t.Fatal("a search matches a zone's name and a tag, hard, and the look shows the top match")
	}
	zones.typed("b")
	if !zones.has("zones-shown=1") || !zones.has("zone-look=Harbour") {
		t.Fatal("harb matches only Harbour")
	}
	zones.clear()
	zones.typed("caves")
	if !zones.has("zones-shown=1") || !zones.has("zone-look=Smugglers' cave") {
		t.Fatal("a search matches a zone's folder")
	}
	zones.clear()
	zones.click("zone-rail:Coast")
	if !zones.has("zones-shown=2") {
		t.Fatal("a folder on the rail narrows to it and the folders it holds")
	}
	zones.click("zone-rail:all")
	zones.click("zone-tag:town")
	if !zones.has("zones-shown=2") || !zones.has("zone-count:Coast=1") {
		t.Fatal("a tag chip narrows to the zones carrying it")
	}
}

// TestChoosingAZone chooses with a double-click and Enter, refuses a zone
// greyed with its reason, offers No zone, and lists the zones chosen lately.
func TestChoosingAZone(t *testing.T) {
	t.Parallel()
	_, made := town(t)
	refused := map[ids.ULID]string{made["Market"]: "the door is already here"}
	zones := newPicking(t, zonepicker.Config{
		For: "for this door to lead to", Now: made["Harbour"], Recent: []ids.ULID{made["Market"], made["Harbour"]},
		NoZone: true, GoesOn: true,
	}, refused)
	if !zones.has("zone-detail:Harbour=now") || !zones.has("zone-detail:Market=the door is already here") ||
		!zones.has("zone-count:recent=2") {
		t.Fatal("the zone it is now is marked, a refused one says why, and Recent counts the zones chosen lately")
	}
	zones.click("zone:Market")
	zones.key("Enter")
	if zones.chosen || !zones.open || !zones.has("zone-note=Market: the door is already here") {
		t.Fatal("a refused zone is not chosen, and the foot says why")
	}
	zones.click("zone:Smugglers' cave")
	zones.key("Enter")
	if !zones.chosen || zones.open || zones.choice.Zone != made["Smugglers' cave"] {
		t.Fatalf("Enter chooses the zone selected: %+v", zones.choice)
	}
	again := newPicking(t, zonepicker.Config{For: "", Now: ids.ULID{}, Recent: nil, NoZone: true, GoesOn: false}, nil)
	again.click(zonepicker.NoneID)
	if !again.chosen || !again.choice.None {
		t.Fatal("No zone chooses none")
	}
	closing := newPicking(t, zonepicker.Config{For: "", Now: ids.ULID{}, Recent: nil, NoZone: false, GoesOn: false}, nil)
	if _, found := closing.picker.Locate(window, closing.offer, zonepicker.NoneID); found {
		t.Fatal("No zone is offered only when asked for")
	}
	closing.key("Escape")
	if closing.chosen || closing.open {
		t.Fatal("Escape closes without choosing")
	}
}

// TestTheChooseButtonHoldsItsName widens the choose button for a long zone
// name, so its label is never cut off at the button's edges.
func TestTheChooseButtonHoldsItsName(t *testing.T) {
	t.Parallel()
	zones := newPicking(t, zonepicker.Config{For: "", Now: ids.ULID{}, Recent: nil, NoZone: false, GoesOn: false}, nil)
	apart := func(name string) int {
		zones.click("zone:" + name)
		choose, _ := zones.picker.Locate(window, zones.offer, zonepicker.ChooseID)
		cancel, _ := zones.picker.Locate(window, zones.offer, ui.DialogCancel)

		return choose.X - cancel.X
	}
	if short, long := apart("Market"), apart("Smugglers' cave"); long <= short {
		t.Fatalf("the choose button grows to hold Choose Smugglers' cave: %d, %d apart", short, long)
	}
}
