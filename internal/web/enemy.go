package web

import (
	"fmt"
	"html/template"
	"net/url"
	"strconv"
	"strings"

	"github.com/cenoda/gscouter/internal/config"
)

// A result page scores its teams against the standard enemy unless its
// URL sets up another: lvl is the enemy's level, res its resistance to
// everything in percent, and an element's parameter (pyro, ...,
// physical) its resistance to that element.

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
	all := e.Resist[0]
	if !read("res", &all) {
		return nil, bad
	}
	for i, el := range config.Elements {
		e.Resist[i] = all
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
// unless 100, res for the most common resistance unless 10%, and the
// elements that differ from it.
func addEnemy(v url.Values, e *config.Enemy) {
	if e == nil {
		return
	}
	if e.Level != config.StandardEnemy.Level {
		v.Set("lvl", strconv.Itoa(e.Level))
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
// Physical 40%, others 10%".
func (l localizer) enemyText(e *config.Enemy) string {
	if e == nil {
		return l.T("enemy.standard")
	}
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
	return fmt.Sprintf("Custom enemy: %s", english.enemyText(e))
}
