package enka

import (
	"cmp"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// SavedBuild is a build a player saved on Enka.Network: a snapshot of a
// showcase character, under a name the player gave it.
type SavedBuild struct {
	ID        int
	Name      string
	Order     float64 // the player's order on Enka, lowest first
	Character Character
}

// savedBuild is one build in the /api/profile/<username>/hoyos/<hash>/builds/
// response, an object of these by avatar ID. Its avatar_data is a showcase
// avatarInfoList entry as it was when saved.
type savedBuild struct {
	ID    int         `json:"id"`
	Name  string      `json:"name"`
	Order json.Number `json:"order"` // a decimal string, "-1.0000000000"
	// Live marks the showcase as Enka last refreshed it, not a saved
	// snapshot.
	Live       bool   `json:"live"`
	Public     *bool  `json:"public"`
	AvatarData avatar `json:"avatar_data"`
}

// maxNameLen bounds a saved build's name as shown, in runes.
const maxNameLen = 40

// ParseBuilds converts a saved builds response. It leaves out live
// builds, which are the showcase rather than snapshots, and builds that
// gcsim cannot run. Each character's builds come in the order the player
// put them on Enka, the most recently saved first among those the player
// left unordered; characters are sorted by key.
func ParseBuilds(raw []byte) ([]SavedBuild, error) {
	if !loadData() {
		return nil, dataErr
	}
	var resp map[string][]savedBuild
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("decode enka builds: %w", err)
	}
	var out []SavedBuild
	for _, builds := range resp {
		for _, b := range builds {
			if b.Live || b.Public != nil && !*b.Public {
				continue
			}
			c, err := data.convert(b.AvatarData)
			if err != nil {
				continue
			}
			order, _ := strconv.ParseFloat(b.Order.String(), 64)
			name := strings.TrimSpace(b.Name)
			if r := []rune(name); len(r) > maxNameLen {
				name = string(r[:maxNameLen-1]) + "…"
			}
			out = append(out, SavedBuild{ID: b.ID, Name: name, Order: order, Character: c})
		}
	}
	slices.SortFunc(out, func(a, b SavedBuild) int {
		return cmp.Or(
			strings.Compare(a.Character.Key, b.Character.Key),
			cmp.Compare(a.Order, b.Order),
			cmp.Compare(b.ID, a.ID),
		)
	})
	return out, nil
}
