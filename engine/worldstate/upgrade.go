package worldstate

import (
	"encoding/json"
	"fmt"
)

// The templates version 1's NPC and zone values are upgraded into. Their
// time parts are zero, so no generated ID ever equals them.
const (
	sharedNPCs  = "0000000000000000000000TNPC"
	sharedZones = "0000000000000000000000TZNE"
	sharedName  = "shared"
)

// UpgradeType returns a type as version 2 names it: number became integer
// and yes-no bool. Workflows' local values were renamed the same way.
func UpgradeType(kind string) string {
	switch kind {
	case "number":
		return string(TypeInteger)
	case "yes-no":
		return string(TypeBool)
	}

	return kind
}

// upgradeFrom1 renames number to integer and yes-no to bool, and moves the
// values version 1 declared for every NPC or every zone into a template
// named shared for each, applied to none yet: each NPC and zone now owns
// its values, and gets a shared set from a template.
func upgradeFrom1(data json.RawMessage) (json.RawMessage, error) {
	var wire struct {
		Values []map[string]any `json:"values"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrData, err)
	}
	templates := map[string]string{string(ScopeNPC): sharedNPCs, string(ScopeZone): sharedZones}
	used := map[string]bool{}
	for _, value := range wire.Values {
		if kind, ok := value["type"].(string); ok {
			value["type"] = UpgradeType(kind)
		}
		scope, named := value["scope"].(string)
		if template, owned := templates[scope]; named && owned {
			value["parent"] = template
			used[scope] = true
		}
	}
	for _, scope := range []Scope{ScopeNPC, ScopeZone} {
		if used[string(scope)] {
			wire.Values = append(wire.Values, map[string]any{
				"id": templates[string(scope)], "name": sharedName, "scope": string(scope), "type": string(TypeTemplate),
			})
		}
	}
	upgraded, err := json.Marshal(wire)
	if err != nil {
		return nil, fmt.Errorf("upgrade world state: %w", err)
	}

	return upgraded, nil
}
