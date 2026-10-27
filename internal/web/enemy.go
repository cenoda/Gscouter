package web

import (
	"cmp"
	_ "embed"
	"encoding/json"
	"fmt"
	"html/template"
	"net/url"
	"strconv"
	"strings"

	"github.com/cenoda/gscouter/internal/config"
)

// A result page scores its teams against the standard enemy unless its
// URL sets up another: boss starts from a preset's resistances
// (config.Bosses), lvl is the enemy's level, res its resistance to
// everything in percent, and an element's parameter (pyro, ...,
// physical) its resistance to that element.

// bossNames[boss key][language] is a boss's in-game name; see genbosses.
//
//go:embed bosses.json
var bossNamesJSON []byte

var bossNames = func() map[string]map[string]string {
	var out map[string]map[string]string
	if err := json.Unmarshal(bossNamesJSON, &out); err != nil {
		panic("web: bosses.json: " + err.Error())
	}
	return out
}()

// bossName is a boss's in-game name in l's language, else in English.
func (l localizer) bossName(key string) string {
	byLang := bossNames[key]
	return cmp.Or(byLang[l.Lang], byLang[langEnglish], key)
}

// enemyFromQuery reads the enemy of a request, nil for the standard one.
func enemyFromQuery(q url.Values) (*config.Enemy, error) {
	e := config.StandardEnemy
	bad := inputError{"err.enemy", []any{config.MinLevel, config.MaxLevel, config.MinResist, config.MaxResist}}
	read := func(key string, into *int) bool {
		v := strings.TrimSpace(q.Get(key))
		if v == "" {
			return true
		}
		n, err := strconv.Atoi(v)
		*into = n
		return err == nil
	}
	if !read("lvl", &e.Level) {
		return nil, bad
	}
	if key := q.Get("boss"); key != "" {
		b, ok := config.FindBoss(key)
		if !ok {
			return nil, inputError{"err.boss", []any{key}}
		}
		e.Boss, e.Resist = b.Key, b.Resist
	}
	if q.Has("res") {
		var all int
		if !read("res", &all) {
			return nil, bad
		}
		for i := range e.Resist {
			e.Resist[i] = all
		}
	}
	for i, el := range config.Elements {
		if !read(el, &e.Resist[i]) {
			return nil, bad
		}
	}
	if e.Validate() != nil {
		return nil, bad
	}
	if e == config.StandardEnemy {
		return nil, nil
	}
	return &e, nil
}

// addEnemy adds e's parameters to v, none for the standard enemy: lvl
// unless 100, then either the boss and the elements that differ from its
// preset, or res for the most common resistance unless 10% and the
// elements that differ from it.
func addEnemy(v url.Values, e *config.Enemy) {
	if e == nil {
		return
	}
	if e.Level != config.StandardEnemy.Level {
		v.Set("lvl", strconv.Itoa(e.Level))
	}
	if b, ok := config.FindBoss(e.Boss); ok {
		v.Set("boss", b.Key)
		for i, r := range e.Resist {
			if r != b.Resist[i] {
				v.Set(config.Elements[i], strconv.Itoa(r))
			}
		}
		return
	}
	all := commonResist(e)
	if all != config.StandardEnemy.Resist[0] {
		v.Set("res", strconv.Itoa(all))
	}
	for i, r := range e.Resist {
		if r != all {
			v.Set(config.Elements[i], strconv.Itoa(r))
		}
	}
}

// commonResist is the resistance most of e's elements have, the first
// element's on a tie.
func commonResist(e *config.Enemy) int {
	best, count := e.Resist[0], 0
	for _, r := range e.Resist {
		n := 0
		for _, o := range e.Resist {
			if o == r {
				n++
			}
		}
		if n > count {
			best, count = r, n
		}
	}
	return best
}

// link is path with the query of the key-value pairs kv and enemy e.
func link(path string, e *config.Enemy, kv ...string) template.URL {
	v := url.Values{}
	for i := 0; i+1 < len(kv); i += 2 {
		v.Set(kv[i], kv[i+1])
	}
	addEnemy(v, e)
	if len(v) == 0 {
		return template.URL(path)
	}
	return template.URL(path + "?" + v.Encode())
}

// enemyText describes an enemy in l's language, e.g. "Lv. 90 · Geo 70%,
// Physical 40%, others 10%", after the boss's name if it has one.
func (l localizer) enemyText(e *config.Enemy) string {
	if e == nil {
		return l.T("enemy.standard")
	}
	text := l.resistText(e)
	if b, ok := config.FindBoss(e.Boss); ok {
		if e.Resist != b.Resist {
			return l.T("enemy.boss_modified", l.bossName(b.Key), text)
		}
		return l.T("enemy.boss", l.bossName(b.Key), text)
	}
	return text
}

// resistText is enemyText without the boss.
func (l localizer) resistText(e *config.Enemy) string {
	if r, ok := e.Uniform(); ok {
		return l.T("enemy.text", e.Level, l.T("enemy.all", r))
	}
	all := commonResist(e)
	var parts []string
	for i, r := range e.Resist {
		if r != all {
			parts = append(parts, l.T("enemy.one", l.T("el."+config.Elements[i]), r))
		}
	}
	parts = append(parts, l.T("enemy.others", all))
	return l.T("enemy.text", e.Level, strings.Join(parts, l.T("enemy.sep")))
}

// enemyForm is the form that sets up a result page's enemy.
type enemyForm struct {
	Action string     // the page's path
	Keep   url.Values // the page's teams, kept as hidden fields
	Enemy  *config.Enemy
	Reset  template.URL // the page with the standard enemy
}

// enemyField is one number field of the form.
type enemyField struct {
	Name, Element string // Element is "" for the level
	Value         int
}

func newEnemyForm(path string, e *config.Enemy, kv ...string) enemyForm {
	keep := url.Values{}
	for i := 0; i+1 < len(kv); i += 2 {
		keep.Set(kv[i], kv[i+1])
	}
	return enemyForm{Action: path, Keep: keep, Enemy: e, Reset: link(path, nil, kv...)}
}

// Resists are the form's resistance fields.
func (f enemyForm) Resists() []enemyField {
	e := config.StandardEnemy
	if f.Enemy != nil {
		e = *f.Enemy
	}
	out := make([]enemyField, len(config.Elements))
	for i, el := range config.Elements {
		out[i] = enemyField{Name: el, Element: el, Value: e.Resist[i]}
	}
	return out
}

// Limits are the limits of the form's fields.
func (enemyForm) Limits() struct{ MinLevel, MaxLevel, MinResist, MaxResist int } {
	return struct{ MinLevel, MaxLevel, MinResist, MaxResist int }{config.MinLevel, config.MaxLevel, config.MinResist, config.MaxResist}
}

// bossOption is one preset in the form's list.
type bossOption struct {
	Key, Name string
	Resist    string // comma-separated, by config.Elements, for the script
	Selected  bool
}

// bossPresets are the presets in the form's list, for l.
type bossPresets struct{ Weekly, World []bossOption }

// presets lists the weekly bosses, then the world bosses, marking f's.
func (l localizer) presets(f enemyForm) (p bossPresets) {
	for _, b := range config.Bosses {
		var rs []string
		for _, r := range b.Resist {
			rs = append(rs, strconv.Itoa(r))
		}
		o := bossOption{Key: b.Key, Name: l.bossName(b.Key), Resist: strings.Join(rs, ","), Selected: f.Enemy != nil && f.Enemy.Boss == b.Key}
		if b.Weekly {
			p.Weekly = append(p.Weekly, o)
		} else {
			p.World = append(p.World, o)
		}
	}
	return p
}

// Level is the enemy's level.
func (f enemyForm) Level() int {
	if f.Enemy != nil {
		return f.Enemy.Level
	}
	return config.StandardEnemy.Level
}

// enemyQuery is e's query string, "" for the standard enemy.
func enemyQuery(e *config.Enemy) string {
	v := url.Values{}
	addEnemy(v, e)
	return v.Encode()
}

// cardEnemy describes a custom enemy on card images, "" for the
// standard one.
func cardEnemy(e *config.Enemy) string {
	if e == nil {
		return ""
	}
	if e.Boss != "" {
		return fmt.Sprintf("Enemy: %s", english.enemyText(e))
	}
	return fmt.Sprintf("Custom enemy: %s", english.enemyText(e))
}
