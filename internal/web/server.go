// Package web serves the Gscouter site: look up a UID, pick four showcase
// characters, and score or compare teams.
package web

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"github.com/cenoda/gscouter/internal/config"
	"github.com/cenoda/gscouter/internal/enka"
	"github.com/cenoda/gscouter/internal/library"
	"github.com/cenoda/gscouter/internal/score"
)

//go:embed templates/*.html
var templateFS embed.FS

// Settings control the simulation cost per score.
type Settings struct {
	Iterations    int
	Warmup        float64 // seconds
	Duration      float64 // seconds
	MaxConcurrent int     // simulations running at once
	MaxQueue      int     // simulations waiting before requests are rejected
}

// DefaultSettings measure 90 s from full energy, as players measure on a
// training dummy (design §3.1).
var DefaultSettings = Settings{
	Iterations:    100,
	Warmup:        0,
	Duration:      90,
	MaxConcurrent: 1,
	MaxQueue:      8,
}

// ProfileSource returns a player's converted showcase.
type ProfileSource interface {
	Profile(ctx context.Context, uid string) (*enka.Profile, error)
}

var errBusy = errors.New("server busy")

type Server struct {
	lib      *library.Library
	profiles ProfileSource
	settings Settings
	tmpl     *template.Template

	sem     chan struct{}
	waiting atomic.Int32
	results *cache[string, *score.Result]
}

func New(lib *library.Library, profiles ProfileSource, settings Settings) (*Server, error) {
	tmpl, err := template.New("").Funcs(funcs).ParseFS(templateFS, "templates/*.html")
	if err != nil {
		return nil, err
	}
	return &Server{
		lib:      lib,
		profiles: profiles,
		settings: settings,
		tmpl:     tmpl,
		sem:      make(chan struct{}, max(settings.MaxConcurrent, 1)),
		results:  newCache[string, *score.Result](1000),
	}, nil
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.handleIndex)
	mux.HandleFunc("GET /score", s.handleScore)
	mux.HandleFunc("GET /versus", s.handleVersus)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})
	return mux
}

// selection is four characters picked from one player's showcase, written
// as "UID:char1,char2,char3,char4" in URLs.
type selection struct {
	UID   string
	Chars []string
}

func (sel selection) String() string {
	return sel.UID + ":" + strings.Join(sel.Chars, ",")
}

func parseSelection(s string) (selection, error) {
	uid, chars, ok := strings.Cut(s, ":")
	if !ok {
		return selection{}, fmt.Errorf("잘못된 선택: %q", s)
	}
	return newSelection(uid, strings.Split(chars, ","))
}

func newSelection(uid string, chars []string) (selection, error) {
	if len(chars) != 4 {
		return selection{}, fmt.Errorf("캐릭터를 정확히 4명 골라 주세요 (%d명 선택됨)", len(chars))
	}
	seen := map[string]bool{}
	for _, c := range chars {
		if c == "" || seen[c] {
			return selection{}, fmt.Errorf("캐릭터 선택이 올바르지 않습니다")
		}
		seen[c] = true
	}
	return selection{UID: uid, Chars: chars}, nil
}

// selectionFromQuery reads a selection either from a packed parameter
// (e.g. "a") or from the picker form's uid and c parameters.
func selectionFromQuery(r *http.Request, packed string) (selection, error) {
	q := r.URL.Query()
	if v := q.Get(packed); v != "" {
		return parseSelection(v)
	}
	return newSelection(strings.TrimSpace(q.Get("uid")), q["c"])
}

// teamResult is one evaluated team, for the result templates.
type teamResult struct {
	Selection selection
	Nickname  string
	Chars     []enka.Character
	Team      *library.Team
	Result    *score.Result
	Warnings  []string
	// Problem explains why there is no score: unsupported team, blocked
	// team, Enka error, and so on.
	Problem string
}

func (s *Server) evaluate(ctx context.Context, sel selection) teamResult {
	tr := teamResult{Selection: sel}
	p, err := s.profiles.Profile(ctx, sel.UID)
	if err != nil {
		tr.Problem = enkaMessage(err)
		return tr
	}
	tr.Nickname = p.Nickname
	for _, want := range sel.Chars {
		i := slices.IndexFunc(p.Characters, func(c enka.Character) bool { return c.Key == want })
		if i < 0 {
			tr.Problem = fmt.Sprintf("%s 캐릭터가 쇼케이스에 없습니다. 쇼케이스를 갱신한 뒤 다시 시도해 주세요.", want)
			return tr
		}
		tr.Chars = append(tr.Chars, p.Characters[i])
	}

	tr.Team = s.lib.Find(sel.Chars)
	switch {
	case tr.Team == nil:
		slog.Info("unsupported team", "members", strings.Join(sorted(sel.Chars), ","))
		tr.Problem = "아직 지원하지 않는 파티입니다. 라이브러리에 있는 파티만 계산할 수 있습니다."
		return tr
	case tr.Team.Status == library.StatusBlocked:
		tr.Problem = "계산 엔진이 아직 이 파티의 캐릭터를 지원하지 않습니다. (" + tr.Team.Notes + ")"
		return tr
	}

	var build strings.Builder
	renames := map[string]string{}
	for _, c := range tr.Chars {
		build.WriteString(c.Config())
		build.WriteString("\n")
		for _, m := range tr.Team.Members {
			if library.Canonical(m) == library.Canonical(c.Key) {
				renames[m] = c.Key
			}
		}
	}
	rotation := config.RenameCharacters(tr.Team.Rotation, renames)
	cfg := config.Assemble(config.Conditions{Duration: s.settings.Duration, Iterations: s.settings.Iterations}, build.String(), rotation)

	res, err := s.score(ctx, cfg)
	if err != nil {
		if errors.Is(err, errBusy) {
			tr.Problem = "지금 계산 요청이 많습니다. 잠시 후 다시 시도해 주세요."
		} else {
			slog.Error("score failed", "team", tr.Team.ID, "uid", sel.UID, "err", err)
			tr.Problem = "계산 중 오류가 발생했습니다."
		}
		return tr
	}
	tr.Result = res
	tr.Warnings = warnings(tr.Team, res)
	for _, c := range tr.Chars {
		if c.UnknownSets > 0 {
			tr.Warnings = append(tr.Warnings, fmt.Sprintf("%s: 계산 엔진이 아직 지원하지 않는 성유물 세트가 %d개 있어, 그 세트 효과는 빠졌습니다.", c.Key, c.UnknownSets))
		}
	}
	return tr
}

// score runs a simulation, reusing the cached result for the same config.
func (s *Server) score(ctx context.Context, cfg string) (*score.Result, error) {
	sum := sha256.Sum256([]byte(cfg))
	key := hex.EncodeToString(sum[:])
	if res, ok := s.results.get(key); ok {
		return res, nil
	}

	if int(s.waiting.Add(1)) > s.settings.MaxQueue+cap(s.sem) {
		s.waiting.Add(-1)
		return nil, errBusy
	}
	defer s.waiting.Add(-1)
	select {
	case s.sem <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	defer func() { <-s.sem }()

	start := time.Now()
	res, err := score.Run(ctx, cfg, score.Options{
		Iterations: s.settings.Iterations,
		Warmup:     s.settings.Warmup,
		Seed:       1,
	})
	if err != nil {
		return nil, err
	}
	slog.Info("scored", "dps", int(res.DPS), "took", time.Since(start).Round(time.Millisecond))
	s.results.put(key, res, 24*time.Hour)
	return res, nil
}

func warnings(team *library.Team, res *score.Result) []string {
	var out []string
	if team.Status == library.StatusDraft {
		out = append(out, "이 파티의 로테이션은 아직 검증 전입니다. 점수가 바뀔 수 있습니다.")
	}
	for _, c := range res.Characters {
		if c.EnergyWait >= 1 {
			out = append(out, fmt.Sprintf("%s: 원소 에너지가 부족해 분당 %.1f초를 기다립니다. 원소 충전 효율을 올리면 점수가 오릅니다.", c.Name, c.EnergyWait))
		}
	}
	return out
}

func enkaMessage(err error) string {
	switch {
	case errors.Is(err, enka.ErrInvalidUID):
		return "UID 형식이 올바르지 않습니다."
	case errors.Is(err, enka.ErrNotFound):
		return "해당 UID의 플레이어를 찾을 수 없습니다."
	case errors.Is(err, enka.ErrMaintenance):
		return "게임 점검 중이라 정보를 가져올 수 없습니다."
	case errors.Is(err, enka.ErrRateLimited):
		return "Enka.Network 요청이 너무 많습니다. 잠시 후 다시 시도해 주세요."
	default:
		slog.Error("enka fetch failed", "err", err)
		return "Enka.Network에서 정보를 가져오지 못했습니다."
	}
}

func sorted(s []string) []string {
	out := slices.Clone(s)
	slices.Sort(out)
	return out
}
