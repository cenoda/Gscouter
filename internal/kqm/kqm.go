// Package kqm reads the community gcsim database at db.kqm.gg, the list
// gcsim.app links to as "shared by others". Every entry is a full gcsim
// config that the database reruns on recent gcsim versions.
package kqm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
)

const site = "https://db.kqm.gg"

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
	var all []Entry
	for skip := 0; ; skip += pageSize {
		page, err := fetch(ctx, skip)
		if err != nil {
			return nil, err
		}
		all = append(all, page...)
		if len(page) < pageSize {
			return all, nil
		}
	}
}

func fetch(ctx context.Context, skip int) ([]Entry, error) {
	q, err := json.Marshal(map[string]any{"query": map[string]any{}, "limit": pageSize, "skip": skip})
	if err != nil {
		return nil, err
	}
	u := site + "/api/db?q=" + url.QueryEscape(string(q))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", u, resp.Status)
	}
	var page struct {
		Data []Entry `json:"data"`
	}
	if err := json.Unmarshal(body, &page); err != nil {
		return nil, fmt.Errorf("decode database page at %d: %w", skip, err)
	}
	return page.Data, nil
}
