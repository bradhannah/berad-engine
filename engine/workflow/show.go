package workflow

// GroupShow is the palette group of the nodes that show the player
// something: speech, a message box and a shake of the screen.
const GroupShow = "Show"

// The Show group's kinds.
const (
	KindSay         = "say"
	KindShowMessage = "show-message"
	KindShake       = "shake-screen"
)

// TypeSeconds is a whole number of real seconds, from 1 to MaxSeconds.
const TypeSeconds ParamType = "seconds"

// MaxSeconds is the most a Seconds setting can be.
const MaxSeconds = 60

// secondsLabel names a Seconds setting.
const secondsLabel = "Seconds"

// The Show group's settings: how long a line or a box shows, and for how
// many seconds when set by hand; whether the flow carries on at once or
// once it is said or gone; the name on a box; how a box goes away; how
// strongly the screen shakes.
const (
	ParamShows    = "shows"
	ShowsReading  = "reading"
	ShowsSeconds  = "seconds"
	ParamSeconds  = "seconds"
	ParamCarry    = "carry"
	CarryAtOnce   = "at-once"
	CarryWhenDone = "when-done"
	ParamBoxName  = "name"
	BoxNameNone   = "none"
	BoxNameNPC    = "npc"
	ParamAway     = "away"
	AwayRead      = "read"
	AwaySeconds   = "seconds"
	AwayClosed    = "closed"
	ParamStrength = "strength"
	ShakeGentle   = "gentle"
	ShakeModerate = "moderate"
	ShakeStrong   = "strong"
)

// Defaults of the Show group's settings.
const (
	defaultShowSeconds  = 4
	defaultShakeSeconds = 2
)

// showKinds declares Say, Show message and Shake screen.
func showKinds(next []Output) []Kind {
	message := Param{
		Name: ParamMessage, Label: "Message", Type: TypeMessage, Default: mustJSON([]Segment{}), Choices: nil,
	}
	seconds := Param{
		Name: ParamSeconds, Label: secondsLabel, Type: TypeSeconds, Default: mustJSON(defaultShowSeconds), Choices: nil,
	}
	carry := func(done string) Param {
		return Param{
			Name: ParamCarry, Label: "Carry on", Type: TypeChoice, Default: mustJSON(CarryAtOnce),
			Choices: []Choice{{Value: CarryAtOnce, Label: "At once"}, {Value: CarryWhenDone, Label: done}},
		}
	}
	sayLine := message
	sayLine.Label = "Line"

	return []Kind{
		{
			Name: KindSay, Label: "Say", Group: GroupShow,
			Params: []Param{
				sayLine,
				{Name: ParamShows, Label: "Shows for", Type: TypeChoice, Default: mustJSON(ShowsReading), Choices: []Choice{
					{Value: ShowsReading, Label: "As long as it takes to read"}, {Value: ShowsSeconds, Label: secondsLabel},
				}},
				seconds, carry("When it has been said"),
			},
			Outputs: next,
		},
		{
			Name: KindShowMessage, Label: "Show message", Group: GroupShow,
			Params: []Param{
				message,
				{Name: ParamBoxName, Label: "Name on the box", Type: TypeChoice, Default: mustJSON(BoxNameNone),
					Choices: []Choice{{Value: BoxNameNone, Label: "None"}, {Value: BoxNameNPC, Label: "This NPC"}}},
				{Name: ParamAway, Label: "Goes away", Type: TypeChoice, Default: mustJSON(AwayRead), Choices: []Choice{
					{Value: AwayRead, Label: "When it has been read"}, {Value: AwaySeconds, Label: "After seconds"},
					{Value: AwayClosed, Label: "When the player closes it"},
				}},
				seconds, carry("When it has gone"),
			},
			Outputs: next,
		},
		{
			Name: KindShake, Label: "Shake screen", Group: GroupShow,
			Params: []Param{
				{Name: ParamStrength, Label: "Strength", Type: TypeChoice, Default: mustJSON(ShakeModerate),
					Choices: []Choice{
						{Value: ShakeGentle, Label: "Gentle"}, {Value: ShakeModerate, Label: "Moderate"},
						{Value: ShakeStrong, Label: "Strong"},
					}},
				{Name: ParamSeconds, Label: secondsLabel, Type: TypeSeconds, Default: mustJSON(defaultShakeSeconds),
					Choices: nil},
			},
			Outputs: next,
		},
	}
}

// UsesSeconds reports whether a Say or a Show message shows for the seconds
// set by hand, so its Seconds setting matters.
func (n Node) UsesSeconds() bool {
	switch n.Kind {
	case KindSay:
		shows, err := n.Choice(ParamShows)

		return err == nil && shows == ShowsSeconds
	case KindShowMessage:
		away, err := n.Choice(ParamAway)

		return err == nil && away == AwaySeconds
	case KindShake:
		return true
	}

	return false
}

// Seconds returns a node's whole seconds.
func (n Node) Seconds() (int, error) { return n.Number(ParamSeconds) }
