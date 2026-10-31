// Package share imports community gcsim configs from gcsim share links
// (gcsim.app/sh/<key> or gcsim.app/db/<id>).
package share

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const site = "https://gcsim.app"

// client bounds every request on its own, so a server that accepts a
// connection and never answers cannot hang a caller whose context has no
// deadline. noRedirect is the same, for reading where a database link
// redirects to.
var (
	client     = &http.Client{Timeout: 30 * time.Second}
	noRedirect = &http.Client{
		Timeout:       30 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
)

// maxBody is the most a shared result may weigh. A share holds the config
// and its statistics, well under this.
const maxBody = 10 << 20

// Result is the part of a shared gcsim result an import needs.
type Result struct {
	Key        string
	Config     string  // the config file as shared
	DPS        float64 // mean team DPS the share reports
	SimVersion string  // gcsim commit it ran on
}

var (
	shareRe = regexp.MustCompile(`/sh/([A-Za-z0-9_-]+)`)
	dbRe    = regexp.MustCompile(`/db/([A-Za-z0-9_-]+)`)
	keyRe   = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
)

// Fetch downloads the shared result behind a share link, a database link
// or a bare share key.
func Fetch(ctx context.Context, link string) (*Result, error) {
	key, err := resolve(ctx, link)
	if err != nil {
		return nil, err
	}
	body, err := get(ctx, site+"/api/share/"+key)
	if err != nil {
		return nil, err
	}
	var raw struct {
		ConfigFile string `json:"config_file"`
		SimVersion string `json:"sim_version"`
		Statistics struct {
			DPS struct {
				Mean float64 `json:"mean"`
			} `json:"dps"`
		} `json:"statistics"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decode share %s: %w", key, err)
	}
	if raw.ConfigFile == "" {
		return nil, fmt.Errorf("share %s has no config", key)
	}
	return &Result{Key: key, Config: raw.ConfigFile, DPS: raw.Statistics.DPS.Mean, SimVersion: raw.SimVersion}, nil
}

// resolve turns a link into a share key. Database links redirect to a
// share link.
func resolve(ctx context.Context, link string) (string, error) {
	if m := shareRe.FindStringSubmatch(link); m != nil {
		return m[1], nil
	}
	if m := dbRe.FindStringSubmatch(link); m != nil {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, site+"/db/"+m[1], nil)
		if err != nil {
			return "", err
		}
		resp, err := noRedirect.Do(req)
		if err != nil {
			return "", err
		}
		resp.Body.Close()
		if m := shareRe.FindStringSubmatch(resp.Header.Get("Location")); m != nil {
			return m[1], nil
		}
		return "", fmt.Errorf("database entry %s does not link to a share", m[1])
	}
	if keyRe.MatchString(link) {
		return link, nil
	}
	return "", errors.New("not a gcsim share link: " + link)
}

func get(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxBody {
		return nil, fmt.Errorf("GET %s: response larger than %d bytes", url, maxBody)
	}
	return body, nil
}

// Split is a config separated into the parts the library stores.
type Split struct {
	Members  []string          // character names in config order
	Build    string            // character setup lines, without params
	Params   map[string]string // character params by name, e.g. start_hp=1
	Rotation string            // everything after the setup: active, actions
	Dropped  []string          // options, target and energy lines, replaced by the common conditions
}

var (
	setupRe  = regexp.MustCompile(`^(\w+)\s+(char|add)\b`)
	headerRe = regexp.MustCompile(`^(options|target|energy|hurt)\b`)
	paramsRe = regexp.MustCompile(`\s*\+params=\[([^\]]*)\]`)
)

// SplitConfig separates a gcsim config into build, params and rotation.
// Setup statements start a line and end at the next semicolon; everything
// else is the rotation, kept as written. A setup statement may go on over
// lines that carry more of its key=value options; a line that starts any
// other way, or the end of the config, ends a statement whose semicolon
// is missing, so it cannot swallow the statement after it.
func SplitConfig(cfg string) Split {
	sp := Split{Params: map[string]string{}}
	var build, rot strings.Builder
	var stmt strings.Builder
	inSetup, inHeader := false, false
	seen := map[string]bool{}
	cfg = strings.ReplaceAll(cfg, "\r", "")

	finish := func() {
		text := strings.Join(strings.Fields(stmt.String()), " ") + ";"
		stmt.Reset()
		if inHeader {
			sp.Dropped = append(sp.Dropped, text)
		} else {
			m := setupRe.FindStringSubmatch(text)
			if m[2] == "char" {
				if p := paramsRe.FindStringSubmatch(text); p != nil {
					sp.Params[m[1]] = p[1]
					text = paramsRe.ReplaceAllString(text, "")
				}
				if !seen[m[1]] {
					seen[m[1]] = true
					sp.Members = append(sp.Members, m[1])
					if build.Len() > 0 {
						build.WriteString("\n")
					}
				}
			}
			build.WriteString(text + "\n")
		}
		inSetup, inHeader = false, false
	}

	for _, line := range strings.Split(cfg, "\n") {
		if i := strings.Index(line, "#"); i >= 0 {
			line = line[:i]
		}
		if i := strings.Index(line, "//"); i >= 0 {
			line = line[:i]
		}
		if (inSetup || inHeader) && !continues(line) {
			finish()
		}
		for line != "" {
			if !inSetup && !inHeader {
				trimmed := strings.TrimSpace(line)
				switch {
				case setupRe.MatchString(trimmed):
					inSetup = true
				case headerRe.MatchString(trimmed):
					inHeader = true
				default:
					rot.WriteString(line + "\n")
					line = ""
					continue
				}
				line = trimmed
			}
			before, after, found := strings.Cut(line, ";")
			stmt.WriteString(before + " ")
			if !found {
				line = ""
				continue
			}
			finish()
			line = after
		}
	}
	if inSetup || inHeader {
		finish()
	}
	sp.Build = build.String()
	sp.Rotation = strings.TrimSpace(collapseBlank(rot.String())) + "\n"
	return sp
}

// continues reports whether line, inside an unterminated setup statement,
// goes on with that statement: it is blank, or it starts with an option
// (key=value) or a +option (+params=[...]).
func continues(line string) bool {
	f := strings.Fields(line)
	return len(f) == 0 || strings.Contains(f[0], "=") || strings.HasPrefix(f[0], "+") || strings.HasPrefix(f[0], ";")
}

var blankRe = regexp.MustCompile(`\n\s*\n(\s*\n)+`)

func collapseBlank(s string) string {
	return blankRe.ReplaceAllString(s, "\n\n")
}
