package web

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/cenoda/gscouter/internal/score"
)

// siteDescription is the preview text of pages without their own.
const siteDescription = "원신 UID로 쇼케이스 캐릭터 4명의 파티 DPS를 같은 적·같은 로테이션 기준으로 계산하고 다른 플레이어와 비교합니다."

// pageMeta is what a page tells link previews (KakaoTalk, Discord, ...)
// through Open Graph tags.
type pageMeta struct {
	Title       string
	Description string
	Image       string // absolute URL
}

// metaOf lets the head template take either a plain title or a pageMeta.
func metaOf(v any) pageMeta {
	switch v := v.(type) {
	case pageMeta:
		return v
	case string:
		return pageMeta{Title: v, Description: siteDescription}
	}
	return pageMeta{Title: "Gscouter", Description: siteDescription}
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

// carry is the face icon of the member dealing the most damage, or "".
func (tr teamResult) carry() string {
	if tr.Result == nil || len(tr.Result.Characters) == 0 {
		return ""
	}
	top := tr.Result.Characters[0]
	for _, c := range tr.Result.Characters[1:] {
		if c.DPS > top.DPS {
			top = c
		}
	}
	return face(top.Name)
}

func scoreMeta(tr teamResult) pageMeta {
	if tr.Result == nil {
		return pageMeta{Title: "Gscouter 결과 · " + tr.owner(), Description: tr.Problem}
	}
	m := pageMeta{
		Title:       fmt.Sprintf("%s의 파티 %s DPS · Gscouter", tr.owner(), thousands(tr.Result.DPS)),
		Description: tr.summary(),
		Image:       tr.carry(),
	}
	if tr.OneRotation != nil {
		m.Description += fmt.Sprintf(" | 로테이션 1회 %s DPS", thousands(tr.OneRotation.DPS))
	}
	return m
}

func versusMeta(d versusData) pageMeta {
	if !d.Scored {
		return pageMeta{Title: fmt.Sprintf("%s vs %s · Gscouter", d.A.owner(), d.B.owner())}
	}
	verdict, image := "비슷합니다", d.A.carry()
	switch {
	case d.Tie:
	case d.Diff > 0:
		verdict = fmt.Sprintf("%s 파티가 %.1f%% 높습니다", d.A.owner(), d.Diff*100)
	default:
		verdict = fmt.Sprintf("%s 파티가 %.1f%% 높습니다", d.B.owner(), -d.Diff*100)
		image = d.B.carry()
	}
	return pageMeta{
		Title: fmt.Sprintf("%s vs %s: %s · Gscouter", d.A.owner(), d.B.owner(), verdict),
		Description: fmt.Sprintf("%s %s DPS (%s) vs %s %s DPS (%s)",
			d.A.owner(), thousands(d.A.Result.DPS), d.A.summary(),
			d.B.owner(), thousands(d.B.Result.DPS), d.B.summary()),
		Image: image,
	}
}
