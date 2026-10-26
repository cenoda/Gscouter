package web

import (
	"cmp"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/cenoda/gscouter/internal/score"
)

// pageMeta is what a page tells link previews (KakaoTalk, Discord, ...)
// through Open Graph tags.
type pageMeta struct {
	Title       string
	Description string
	Image       string // absolute URL
}

// metaOf lets the head template take either a plain title or a pageMeta.
// Pages without their own preview text get the site's description.
func metaOf(l localizer, v any) pageMeta {
	switch v := v.(type) {
	case pageMeta:
		return v
	case string:
		return pageMeta{Title: v, Description: l.T("site.desc")}
	}
	return pageMeta{Title: "Gscouter", Description: l.T("site.desc")}
}

// owner names whose team a result is.
func (tr teamResult) owner() string {
	if tr.Nickname != "" {
		return tr.Nickname
	}
	return "UID " + tr.Selection.UID
}

// summary lists the members by damage share with their constellation,
// e.g. "hutao C1 52% · yelan C0 24% · ...".
func (tr teamResult) summary() string {
	type member struct {
		score.Character
		cons string
	}
	var ms []member
	for i, c := range tr.Result.Characters {
		m := member{Character: c}
		if i < len(tr.Chars) {
			m.cons = fmt.Sprintf(" C%d", tr.Chars[i].Cons)
		}
		ms = append(ms, m)
	}
	slices.SortStableFunc(ms, func(a, b member) int { return cmp.Compare(b.Share, a.Share) })
	var parts []string
	for _, m := range ms {
		parts = append(parts, fmt.Sprintf("%s%s %.0f%%", m.Name, m.cons, m.Share*100))
	}
	return strings.Join(parts, " · ")
}

// baseURL is the site's scheme and host as the client sees it, for the
// absolute URLs Open Graph needs. Caddy terminates TLS in production.
func baseURL(r *http.Request) string {
	scheme := "https"
	if r.TLS == nil && r.Header.Get("X-Forwarded-Proto") != "https" {
		scheme = "http"
	}
	return scheme + "://" + r.Host
}

// scoreMeta describes a result; base is the site's URL (see baseURL).
func scoreMeta(l localizer, tr teamResult, base string) pageMeta {
	if tr.Result == nil {
		return pageMeta{Title: l.T("meta.result_title", tr.owner()), Description: tr.Problem}
	}
	m := pageMeta{
		Title:       l.T("meta.score_title", tr.owner(), thousands(tr.Result.DPS)),
		Description: tr.summary(),
		Image:       base + "/card/score.png?" + url.Values{"a": {tr.Selection.String()}}.Encode(),
	}
	if tr.OneRotation != nil {
		m.Description += l.T("meta.one_rotation", thousands(tr.OneRotation.DPS))
	}
	return m
}

func versusMeta(l localizer, d versusData, base string) pageMeta {
	if !d.Scored {
		return pageMeta{Title: fmt.Sprintf("%s vs %s · Gscouter", d.A.owner(), d.B.owner())}
	}
	verdict := l.T("vs.tie")
	switch {
	case d.Tie:
	case d.Diff > 0:
		verdict = l.T("meta.team_higher", d.A.owner(), d.Diff*100)
	default:
		verdict = l.T("meta.team_higher", d.B.owner(), -d.Diff*100)
	}
	return pageMeta{
		Title: fmt.Sprintf("%s vs %s: %s · Gscouter", d.A.owner(), d.B.owner(), verdict),
		Description: fmt.Sprintf("%s %s DPS (%s) vs %s %s DPS (%s)",
			d.A.owner(), thousands(d.A.Result.DPS), d.A.summary(),
			d.B.owner(), thousands(d.B.Result.DPS), d.B.summary()),
		Image: base + "/card/versus.png?" + url.Values{"a": {d.A.Selection.String()}, "b": {d.B.Selection.String()}}.Encode(),
	}
}
