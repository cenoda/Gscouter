package web

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// IndexNow (https://www.indexnow.org) tells search engines that take
// part, Bing, Naver and Yandex among them, which pages were added,
// changed or removed, so that they recrawl them without waiting. The
// server keeps the fingerprint of each indexed page as last submitted in
// a state file and submits the pages whose fingerprint differs: every
// page the first time, then the ones a library update changed.

const (
	indexNowEndpoint = "https://api.indexnow.org/indexnow"
	indexNowBatch    = 10000 // URLs per request, the protocol's limit
)

type indexNow struct {
	key      string
	file     string
	endpoint string
	state    indexNowState
}

type indexNowState struct {
	Key   string            `json:"key"`
	Pages map[string]string `json:"pages"` // fingerprint by path, as last submitted
}

// SetIndexNow turns IndexNow on, with its state kept in file. A missing
// file is created with a new key, which the site then serves as
// /<key>.txt for search engines to check submissions against. Call it
// before Handler; SubmitChanges also needs SetSite.
func (s *Server) SetIndexNow(file string) error {
	var st indexNowState
	raw, err := os.ReadFile(file)
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return err
	default:
		if err := json.Unmarshal(raw, &st); err != nil {
			return fmt.Errorf("%s: %w", file, err)
		}
	}
	if st.Key == "" {
		st.Key = rand.Text()
		if err := saveIndexNow(file, st); err != nil {
			return err
		}
	}
	s.indexNow = &indexNow{key: st.Key, file: file, endpoint: indexNowEndpoint, state: st}
	return nil
}

func (s *Server) handleIndexNowKey(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	io.WriteString(w, s.indexNow.key)
}

// SubmitChanges submits the indexed pages, in every language, that were
// added, changed or removed since the last submission, and records them
// as submitted. It returns the number of URLs submitted.
func (s *Server) SubmitChanges(ctx context.Context) (int, error) {
	in := s.indexNow
	if in == nil || s.site == "" {
		return 0, errors.New("IndexNow needs SetIndexNow and SetSite")
	}
	site, err := url.Parse(s.site)
	if err != nil {
		return 0, err
	}
	now := s.fingerprints()
	var changed []string
	for _, path := range s.pages() {
		if in.state.Pages[path] != now[path] {
			changed = append(changed, path)
		}
	}
	for path := range in.state.Pages {
		if _, ok := now[path]; !ok {
			changed = append(changed, path)
		}
	}

	// Each request carries whole pages, so that a failure leaves the
	// pages already submitted recorded.
	perBatch := indexNowBatch / len(langs)
	sent := 0
	for len(changed) > 0 {
		n := min(perBatch, len(changed))
		batch := changed[:n]
		changed = changed[n:]
		var urls []string
		for _, path := range batch {
			for _, lang := range langs {
				urls = append(urls, s.site+langURL(path, lang.Code))
			}
		}
		if err := in.submit(ctx, site.Host, s.site+"/"+in.key+".txt", urls); err != nil {
			return sent, err
		}
		sent += len(urls)
		if in.state.Pages == nil {
			in.state.Pages = map[string]string{}
		}
		for _, path := range batch {
			if fp, ok := now[path]; ok {
				in.state.Pages[path] = fp
			} else {
				delete(in.state.Pages, path)
			}
		}
		if err := saveIndexNow(in.file, in.state); err != nil {
			return sent, err
		}
	}
	return sent, nil
}

func (in *indexNow) submit(ctx context.Context, host, keyLocation string, urls []string) error {
	body, err := json.Marshal(map[string]any{
		"host":        host,
		"key":         in.key,
		"keyLocation": keyLocation,
		"urlList":     urls,
	})
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "POST", in.endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("IndexNow: %s: %s", resp.Status, strings.TrimSpace(string(msg)))
	}
	return nil
}

// fingerprints identifies what each indexed page shows of the library,
// by path, so that a library update resubmits only the pages it changed.
func (s *Server) fingerprints() map[string]string {
	out := map[string]string{}
	all := sha256.New()
	for _, t := range s.lib.Teams {
		raw, _ := yaml.Marshal(t)
		raw = append(raw, t.Rotation...)
		// The page lists the alternatives the score tries.
		for _, alt := range t.Rotations()[1:] {
			meta, _ := yaml.Marshal(alt)
			raw = append(append(raw, meta...), alt.Rotation...)
		}
		h := sha256.Sum256(raw)
		out[teamPath(t)] = hex.EncodeToString(h[:8])
		fmt.Fprintln(all, t.ID)
	}
	list := hex.EncodeToString(all.Sum(nil)[:8])
	out["/"], out["/teams"] = list, list
	for c, teams := range s.byChar {
		h := sha256.New()
		for _, t := range teams {
			fmt.Fprintln(h, t.ID, t.Tier, referenceScore(t))
		}
		out[characterPath(c)] = hex.EncodeToString(h.Sum(nil)[:8])
	}
	// The pages' text (titles, descriptions, headings) shows on all of
	// them, so a change to it resubmits every page.
	text := pageText()
	for path, f := range out {
		out[path] = text + f
	}
	return out
}

// pageText identifies the site's messages and templates.
func pageText() string {
	h := sha256.New()
	for _, code := range slices.Sorted(maps.Keys(catalog)) {
		for _, key := range slices.Sorted(maps.Keys(catalog[code])) {
			fmt.Fprintf(h, "%s %s %q\n", code, key, catalog[code][key])
		}
	}
	fs.WalkDir(templateFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			raw, _ := fs.ReadFile(templateFS, path)
			fmt.Fprintf(h, "%s %d\n", path, len(raw))
			h.Write(raw)
		}
		return nil
	})
	return hex.EncodeToString(h.Sum(nil)[:4])
}

// saveIndexNow writes st to file, replacing it whole.
func saveIndexNow(file string, st indexNowState) error {
	raw, err := json.Marshal(st)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(file), ".indexnow-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(raw); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), file)
}
