package enka

import "strconv"

// MainStat returns the main stat value of an artifact of the given
// rarity and level, by gcsim stat key (e.g. "atk%", "pyro%", "heal"),
// from the game's main stat table; ok is false for a stat no artifact
// has as its main stat, or a rarity or level it does not reach.
// Percentages are fractions, as gcsim wants them (0.466 for 46.6%).
func MainStat(rarity, level int, key string) (value float64, ok bool) {
	dataOnce.Do(func() { data, dataErr = loadGameData() })
	if dataErr != nil {
		return 0, false
	}
	for _, st := range stats {
		if st.gcsim != key {
			continue
		}
		table := data.mainStats[strconv.Itoa(rarity)][st.good]
		if level < 0 || level >= len(table) {
			return 0, false
		}
		return table[level], true
	}
	return 0, false
}
