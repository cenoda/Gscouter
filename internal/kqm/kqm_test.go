package kqm

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// serve points the package at a test database whose page for skip is
// page(skip), and counts the requests.
func serve(t *testing.T, page func(skip int) []string) *int {
	t.Helper()
	n := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		var q struct {
			Skip int `json:"skip"`
		}
		if err := json.Unmarshal([]byte(r.URL.Query().Get("q")), &q); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		var data []string
		for _, id := range page(q.Skip) {
			data = append(data, fmt.Sprintf(`{"_id":%q,"config":"x"}`, id))
		}
		fmt.Fprintf(w, `{"data":[%s]}`, strings.Join(data, ","))
	}))
	t.Cleanup(srv.Close)
	old := api
	api = srv.URL + "/api/db"
	t.Cleanup(func() { api = old })
	return &n
}

// ids are the entries id0 ... id(total-1) from skip, a page at most.
func ids(skip, total int) []string {
	var out []string
	for i := skip; i < min(skip+pageSize, total); i++ {
		out = append(out, fmt.Sprintf("id%d", i))
	}
	return out
}

func TestFetchAllPages(t *testing.T) {
	n := serve(t, func(skip int) []string { return ids(skip, 250) })
	entries, err := FetchAll(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 250 || entries[0].ID != "id0" || entries[249].ID != "id249" {
		t.Errorf("got %d entries", len(entries))
	}
	if *n != 3 {
		t.Errorf("%d requests, want 3", *n)
	}
}

func TestFetchAllExactPages(t *testing.T) {
	n := serve(t, func(skip int) []string { return ids(skip, 200) })
	entries, err := FetchAll(t.Context())
	if err != nil || len(entries) != 200 || *n != 3 {
		t.Errorf("got %d entries in %d requests, %v", len(entries), *n, err)
	}
}

// A server that ignores skip sends the first page forever. fetchAll
// stops instead of looping until its context ends.
func TestFetchAllServerIgnoresSkip(t *testing.T) {
	n := serve(t, func(int) []string { return ids(0, 1000) })
	if _, err := FetchAll(t.Context()); err == nil {
		t.Fatal("no error from a server that ignores skip")
	}
	if *n != 2 {
		t.Errorf("%d requests, want 2", *n)
	}
}

// An entry that moves to the next page while the database is read is
// kept once.
func TestFetchAllOverlap(t *testing.T) {
	serve(t, func(skip int) []string {
		if skip == 0 {
			return ids(0, 150)
		}
		return ids(skip-1, 150)
	})
	entries, err := FetchAll(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 150 {
		t.Errorf("got %d entries, want 150", len(entries))
	}
}
