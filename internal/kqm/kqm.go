// Package kqm reads the community gcsim database at db.kqm.gg, the list
// gcsim.app links to as "shared by others". Every entry is a full gcsim
// config that the database reruns on recent gcsim versions.
package kqm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"
)

const site = "https://db.kqm.gg"

// api is where the database's API is, a variable so tests can serve it.
var api = site + "/api/db"

// client bounds every request on its own, so a server that accepts a
// connection and never answers cannot hang a caller whose context has no
// deadline.
var client = &http.Client{Timeout: 30 * time.Second}

// maxPage is the most a page of the database may weigh: a page of
// pageSize full configs is well under a megabyte.
const maxPage = 10 << 20

// Database tags (gcsim's model.DBTag).
const (
	TagGcsim        = 1 // accepted by the gcsim team
	TagRandomDelays = 6 // rotation with random delays
	TagAPL          = 8 // action priority list instead of a rotation
	TagGuides       = 9 // from a KQM guide
)

// Simulation modes (gcsim's model.SimMode).
const ModeTTK = 2 // the fight ends when the action list ends

type Entry struct {
	ID           string   `json:"_id"`
	Config       string   `json:"config"`
	Description  string   `json:"description"`
	ShareKey     string   `json:"share_key"`
	AcceptedTags []int    `json:"accepted_tags"`
	LastUpdate   unixTime `json:"last_update"`
	Summary      struct {
		CharNames   []string `json:"char_names"`
		DPS         float64  `json:"mean_dps_per_target"`
		Mode        int      `json:"mode"`
		TargetCount int      `json:"target_count"`
		Team        []struct {
			Name   string `json:"name"`
			Cons   int    `json:"cons"`
			Weapon struct {
				Name   string `json:"name"`
				Refine int    `json:"refine"`
			} `json:"weapon"`
		} `json:"team"`
	} `json:"summary"`
}

// Link is the entry's page on the database.
func (e *Entry) Link() string { return site + "/db/" + e.ID }

func (e *Entry) HasTag(tag int) bool { return slices.Contains(e.AcceptedTags, tag) }

// Updated is the day the database last ran the entry, as YYYY-MM-DD.
func (e *Entry) Updated() string {
	if e.LastUpdate == 0 {
		return ""
	}
	return time.Unix(int64(e.LastUpdate), 0).UTC().Format(time.DateOnly)
}

// unixTime is Unix seconds, which the database stores as a number or a
// string.
type unixTime int64

func (t *unixTime) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		return nil
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return fmt.Errorf("unix time %s: %w", b, err)
	}
	*t = unixTime(n)
	return nil
}

// Investment is the team's total constellations and weapon refinements,
// a measure of how far it is from an ordinary account.
func (e *Entry) Investment() int {
	n := 0
	for _, c := range e.Summary.Team {
		n += c.Cons + max(c.Weapon.Refine, 1) - 1
	}
	return n
}

const pageSize = 100

// FetchAll downloads every entry of the database.
func FetchAll(ctx context.Context) ([]Entry, error) {
	raw, err := fetchAll(ctx)
	if err != nil {
		return nil, err
	}
	return decode(raw)
}

// Cached reads the entries from the file at path, as an earlier call
// saved them. If the file does not exist, it downloads every entry and
// saves the database's JSON there.
func Cached(ctx context.Context, path string) ([]Entry, error) {
	b, err := os.ReadFile(path)
	if err == nil {
		var raw []json.RawMessage
		if err := json.Unmarshal(b, &raw); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		return decode(raw)
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	raw, err := fetchAll(ctx)
	if err != nil {
		return nil, err
	}
	b, err = json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return nil, err
	}
	return decode(raw)
}

func decode(raw []json.RawMessage) ([]Entry, error) {
	entries := make([]Entry, len(raw))
	for i, r := range raw {
		if err := json.Unmarshal(r, &entries[i]); err != nil {
			return nil, fmt.Errorf("decode database entry %d: %w", i, err)
		}
	}
	return entries, nil
}

// fetchAll downloads every entry as the database's JSON. An entry that
// shows up again on a later page, as one can when the database changes
// while it is read, is kept once. A full page that brings no entry not
// seen before is an error: the server is not paging (it ignores skip),
// and asking for the next page would go on forever.
func fetchAll(ctx context.Context) ([]json.RawMessage, error) {
	var all []json.RawMessage
	seen := map[string]bool{}
	for skip := 0; ; skip += pageSize {
		page, err := fetch(ctx, skip)
		if err != nil {
			return nil, err
		}
		fresh := 0
		for _, r := range page {
			var e struct {
				ID string `json:"_id"`
			}
			if err := json.Unmarshal(r, &e); err != nil {
				return nil, fmt.Errorf("decode database entry at %d: %w", skip, err)
			}
			if e.ID != "" {
				if seen[e.ID] {
					continue
				}
				seen[e.ID] = true
				fresh++
			}
			all = append(all, r)
		}
		if len(page) < pageSize {
			return all, nil
		}
		if fresh == 0 {
			return nil, fmt.Errorf("database page at %d has no new entries: the server is not paging", skip)
		}
	}
}

func fetch(ctx context.Context, skip int) ([]json.RawMessage, error) {
	q, err := json.Marshal(map[string]any{"query": map[string]any{}, "limit": pageSize, "skip": skip})
	if err != nil {
		return nil, err
	}
	u := api + "?q=" + url.QueryEscape(string(q))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxPage+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxPage {
		return nil, fmt.Errorf("GET %s: page larger than %d bytes", u, maxPage)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", u, resp.Status)
	}
	var page struct {
		Data []json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &page); err != nil {
		return nil, fmt.Errorf("decode database page at %d: %w", skip, err)
	}
	return page.Data, nil
}
