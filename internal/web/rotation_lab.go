package web

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/cenoda/gscouter/internal/config"
	"github.com/cenoda/gscouter/internal/enka"
	"github.com/cenoda/gscouter/internal/library"
)

type rotationLab struct {
	dir    string
	engine string
}

// SetRotationLab enables the optional browser-only optimizer. Its assets are
// separate from the production scoring engine and never run on this server.
func (s *Server) SetRotationLab(dir string) error {
	if dir == "" {
		s.rotationLab = nil
		return nil
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	for _, name := range []string{"gso.wasm", "dictionary.json", "manifest.json"} {
		st, err := os.Stat(filepath.Join(abs, name))
		if err != nil {
			return fmt.Errorf("rotation lab %s: %w", name, err)
		}
		if !st.Mode().IsRegular() || st.Size() == 0 {
			return fmt.Errorf("rotation lab %s is not a nonempty regular file", name)
		}
	}
	b, err := os.ReadFile(filepath.Join(abs, "manifest.json"))
	if err != nil {
		return err
	}
	var manifest struct {
		Format int `json:"format"`
		Engine struct {
			Commit string `json:"commit"`
		} `json:"engine"`
		WASMHash       string `json:"wasm_sha256"`
		DictionaryHash string `json:"dictionary_sha256"`
	}
	if err := json.Unmarshal(b, &manifest); err != nil {
		return fmt.Errorf("rotation lab manifest: %w", err)
	}
	if manifest.Format != 1 || !regexp.MustCompile(`^[a-f0-9]{40}$`).MatchString(manifest.Engine.Commit) {
		return fmt.Errorf("rotation lab manifest requires format 1 and an exact engine commit")
	}
	for name, expected := range map[string]string{"gso.wasm": manifest.WASMHash, "dictionary.json": manifest.DictionaryHash} {
		if !regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(expected) {
			return fmt.Errorf("rotation lab manifest requires SHA-256 for %s", name)
		}
		f, err := os.Open(filepath.Join(abs, name))
		if err != nil {
			return err
		}
		h := sha256.New()
		_, err = io.Copy(h, f)
		f.Close()
		if err != nil {
			return err
		}
		if hex.EncodeToString(h.Sum(nil)) != expected {
			return fmt.Errorf("rotation lab %s SHA-256 does not match manifest", name)
		}
	}
	s.rotationLab = &rotationLab{dir: abs, engine: manifest.Engine.Commit}
	return nil
}

func (s *Server) handleRotationLab(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
	s.render(w, localizerFor(r), "rotation_lab.html", pageMeta{
		Title:       "로테이션 추론 실험실 · Gscouter",
		Description: "내 Enka 빌드로 브라우저에서 직접 로테이션을 탐색합니다.",
	})
}

func labJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(value)
}

func labError(w http.ResponseWriter, status int, message string) {
	labJSON(w, status, map[string]string{"error": message})
}

func (s *Server) handleRotationLabProfile(w http.ResponseWriter, r *http.Request) {
	uid := strings.TrimSpace(r.URL.Query().Get("uid"))
	if !uidPattern.MatchString(uid) {
		labError(w, 400, "올바른 UID를 입력해 주세요.")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	p, err := s.profiles.Profile(ctx, uid)
	if err != nil {
		labError(w, enkaStatus(err), localizerFor(r).message(err))
		return
	}
	type member struct {
		Key           string `json:"key"`
		Name          string `json:"name"`
		Face          string `json:"face"`
		Constellation int    `json:"constellation"`
		Weapon        string `json:"weapon"`
		Build         string `json:"build"`
	}
	chars := make([]member, 0, len(p.Characters))
	l := localizerFor(r)
	for _, c := range p.Characters {
		chars = append(chars, member{c.Key, l.charName(c.Key), face(c.Key), c.Cons, l.weaponName(c.Weapon.Key), buildToken(c)})
	}
	labJSON(w, 200, map[string]any{"uid": uid, "nickname": p.Nickname, "characters": chars, "skipped": len(p.Skipped)})
}

func (s *Server) handleRotationLabInput(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 24<<10)
	var input struct {
		Builds []string `json:"builds"`
	}
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(&input); err != nil {
		labError(w, 400, "잘못된 빌드 입력입니다.")
		return
	}
	if err := d.Decode(new(any)); err != io.EOF {
		labError(w, 400, "하나의 입력만 보낼 수 있습니다.")
		return
	}
	if len(input.Builds) != 4 {
		labError(w, 400, "캐릭터 네 명을 선택해 주세요.")
		return
	}
	chars := make([]enka.Character, 0, 4)
	for _, token := range input.Builds {
		c, err := parseBuildToken(token)
		if err != nil {
			labError(w, 400, "빌드를 읽을 수 없습니다. UID를 다시 불러와 주세요.")
			return
		}
		chars = append(chars, c)
	}
	if _, err := newSelection("", keys(chars)); err != nil {
		labError(w, 400, "서로 다른 캐릭터 네 명을 선택해 주세요.")
		return
	}
	t := s.lib.Find(keys(chars))
	if t == nil || t.Status != library.StatusSupported || t.Rotation == "" {
		labError(w, 422, "이 파티는 아직 실험실에서 지원하지 않습니다. 기존 라이브러리 로테이션이 있는 파티를 선택해 주세요.")
		return
	}
	build := buildLines(chars)
	for _, c := range chars {
		for _, m := range t.Members {
			if library.Canonical(c.Key) == library.Canonical(m) {
				build = config.AddParams(build, c.Key, t.Params[m])
			}
		}
	}
	warnings := []string{}
	for _, c := range chars {
		if c.UnknownSets > 0 {
			warnings = append(warnings, localizerFor(r).charName(c.Key)+": 알 수 없는 성유물 세트의 보너스는 제외됩니다.")
		}
	}
	labJSON(w, 200, map[string]any{
		"build": build, "rotation": config.RenameCharacters(t.Rotation, renames(t, chars)),
		"team_id": t.ID, "team_name": localizerFor(r).teamName(t), "engine": s.rotationLab.engine,
		"wasm_url": "/lab/assets/gso.wasm", "dictionary_url": "/lab/assets/dictionary.json",
		"manifest_url": "/lab/assets/manifest.json", "warnings": warnings,
	})
}

func (s *Server) handleRotationLabAsset(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("file")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	switch name {
	case "lab.js", "lab-worker.js":
		b, err := staticFS.ReadFile("static/" + name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		w.Write(b)
	case "gso.wasm", "dictionary.json", "manifest.json":
		if name == "gso.wasm" {
			w.Header().Set("Content-Type", "application/wasm")
		} else {
			w.Header().Set("Content-Type", "application/json")
		}
		http.ServeFile(w, r, filepath.Join(s.rotationLab.dir, name))
	default:
		http.NotFound(w, r)
	}
}
