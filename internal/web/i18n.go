package web

import (
	"cmp"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Site languages. A visitor gets the language they picked (kept in a
// cookie), else the best one their browser asks for, else English.
const (
	langEnglish = "en"
	langKorean  = "ko"
	defaultLang = langEnglish
	langCookie  = "lang"
)

// langs are the languages offered in the picker, each named in itself.
var langs = []struct{ Code, Name string }{
	{langEnglish, "English"},
	{langKorean, "한국어"},
}

// localizer writes the site's text in one language. Auto is set when the
// language came from the browser rather than the visitor's pick.
type localizer struct {
	Lang string
	Auto bool
}

// english is for output that is always in English, like card images.
var english = localizer{Lang: langEnglish}

// T returns the message for key, formatted with args when there are any.
// A message missing in the language falls back to English, then to key.
func (l localizer) T(key string, args ...any) string {
	msg, ok := catalog[l.Lang][key]
	if !ok {
		msg = cmp.Or(catalog[defaultLang][key], key)
	}
	if len(args) == 0 {
		return msg
	}
	return fmt.Sprintf(msg, args...)
}

// name returns the message for prefix+key, or key when there is none:
// gcsim's names for things the catalog does not know yet.
func (l localizer) name(prefix, key string) string {
	if msg, ok := catalog[l.Lang][prefix+key]; ok {
		return msg
	}
	if msg, ok := catalog[defaultLang][prefix+key]; ok {
		return msg
	}
	return key
}

// localizerFor picks the language of a request.
func localizerFor(r *http.Request) localizer {
	if c, err := r.Cookie(langCookie); err == nil {
		if _, ok := catalog[c.Value]; ok {
			return localizer{Lang: c.Value}
		}
	}
	return localizer{Lang: negotiate(r.Header.Get("Accept-Language")), Auto: true}
}

// negotiate returns the site language an Accept-Language header prefers
// most, matching on the primary subtag ("ko-KR" is Korean), or
// defaultLang.
func negotiate(header string) string {
	best, bestQ := defaultLang, 0.0
	for _, part := range strings.Split(header, ",") {
		tag, params, _ := strings.Cut(part, ";")
		q := 1.0
		for _, p := range strings.Split(params, ";") {
			if v, ok := strings.CutPrefix(strings.TrimSpace(p), "q="); ok {
				f, err := strconv.ParseFloat(v, 64)
				if err != nil {
					f = 0
				}
				q = f
			}
		}
		primary, _, _ := strings.Cut(strings.ToLower(strings.TrimSpace(tag)), "-")
		if _, ok := catalog[primary]; ok && q > bestQ {
			best, bestQ = primary, q
		}
	}
	return best
}

// handleLang saves the language picked in the page header, or forgets it
// when the visitor picks "Auto", and goes back to the page they were on.
func (s *Server) handleLang(w http.ResponseWriter, r *http.Request) {
	c := &http.Cookie{Name: langCookie, Path: "/", SameSite: http.SameSiteLaxMode}
	if v := r.FormValue("lang"); catalog[v] != nil {
		c.Value, c.MaxAge = v, int((400 * 24 * time.Hour).Seconds())
	} else {
		c.MaxAge = -1
	}
	http.SetCookie(w, c)
	http.Redirect(w, r, backTo(r), http.StatusSeeOther)
}

// backTo is the page on this site that sent r, or the front page.
func backTo(r *http.Request) string {
	u, err := url.Parse(r.Referer())
	if err != nil || u.Host != r.Host {
		return "/"
	}
	uri := u.RequestURI()
	if !strings.HasPrefix(uri, "/") || strings.HasPrefix(uri, "//") {
		return "/"
	}
	return uri
}

// localeFuncs are the template functions that depend on the language.
func localeFuncs(l localizer) template.FuncMap {
	return template.FuncMap{
		"t": l.T,
		// th is a message with markup; the catalog is trusted.
		"th":       func(key string) template.HTML { return template.HTML(l.T(key)) },
		"lang":     func() string { return l.Lang },
		"langAuto": func() bool { return l.Auto },
		"langs":    func() any { return langs },
		"meta":     func(v any) pageMeta { return metaOf(l, v) },
	}
}

// catalog holds the site's text by language and key. Format verbs take
// the arguments in Korean order; %[n] reorders them where English needs.
var catalog = map[string]map[string]string{
	langEnglish: {
		"nav.lookup":  "UID lookup",
		"nav.teams":   "Teams",
		"lang.label":  "Language",
		"lang.auto":   "Auto",
		"lang.apply":  "Apply",
		"site.title":  "Gscouter · Genshin Impact team DPS calculator",
		"site.desc":   "Calculate the DPS of a team of four showcase characters from a Genshin Impact UID, against the same enemy with the same rotation, and compare it with other players.",
		"foot.method": "How scores work",
		"foot.method1": "Gscouter repeats the community gcsim rotation endlessly against a single Lv. 100 enemy with 10% resistance to everything, with particles at regular intervals, " +
			"and scores the average DPS over 150–450 s, after the starting energy has run out. Too little Energy Recharge lowers the score.",
		"foot.method2": "When energy or a cooldown would leave a character standing still, the score takes the better of waiting and filling the time with Elemental Skills, swaps or normal attacks. " +
			"\"One rotation\" is a single pass from full energy, the number community gcsim configs report.",
		"foot.credits": `Simulation <a href="https://github.com/genshinsim/gcsim">gcsim</a> · Rotations <a href="https://db.kqm.gg">KQM gcsim DB</a> · Character data <a href="https://enka.network">Enka.Network</a>`,
		"foot.legal": `Gscouter is an unofficial, non-commercial fan project with no ads or donations, and is not affiliated with HoYoverse. ` +
			`Genshin Impact and related names are trademarks of HoYoverse; character images are © HoYoverse, loaded from <a href="https://enka.network">Enka.Network</a>.`,
		"js.copied": "Link copied",
		"js.link":   "Link",

		"team.stderr":       "standard error",
		"team.one_rotation": "One rotation (full energy): %s DPS",
		"team.character":    "Character",
		"team.share":        "Share",
		"team.talents":      "Talents",

		"index.vs_heading":      "Pick the opponent",
		"index.vs_lead":         "Load the other player's UID and pick a team to calculate both teams' DPS side by side under the same conditions.",
		"index.heading":         "Team DPS from your showcase characters",
		"index.lead":            "Enter a UID and pick four characters. Every team is calculated against the same enemy with the same rotation. Share the result link or compare it with another player's team.",
		"index.step1":           "Enter a UID",
		"index.step2":           "Pick four characters",
		"index.step3":           "Share · compare",
		"index.uid_title":       "9–10 digits",
		"index.uid_placeholder": "UID (e.g. 800000000)",
		"index.load":            "Load",
		"index.showcase_hint":   "Only characters in the Character Showcase of your in-game profile can be loaded. Turn on \"Show Character Details\".",
		"index.suggested":       "Teams ready to calculate",
		"index.suggested_count": "%d",
		"index.no_suggested":    "No library team can be built from the showcase characters. Pick them yourself below.",
		"index.pick":            "Pick characters",
		"index.compare":         "Compare",
		"index.calculate":       "Calculate",
		"index.clear":           "Clear",
		"index.picked":          "%d / 4 selected",
		"index.skipped":         "Could not load a character (ID %d): %s",
		"index.supported":       "Supported teams",
		"index.supported_lead":  "%d teams from the community gcsim database can be calculated.",
		"index.find":            "Find by character",

		"problem.home": "Home",

		"score.copy":            "Copy result link",
		"score.repick":          "Pick other characters",
		"score.compare_heading": "Compare with another team",
		"score.compare_lead":    "Load an opponent's UID and pick a team to calculate both side by side under the same conditions. To compare with another of your own teams, enter your own UID.",
		"score.opponent_uid":    "Opponent UID",
		"score.load_opponent":   "Load opponent",

		"teams.title":       "Supported teams · Gscouter",
		"teams.placeholder": "Character names (e.g. hutao, furina)",
		"teams.search":      "Search",
		"teams.matching":    "%[2]d teams with \"%[1]s\" (of %[3]d)",
		"teams.total":       "%d teams",
		"teams.found":       "%d teams found (of %d)",
		"teams.hint":        "Search by gcsim key names. Separate several characters with spaces to show only the teams with all of them.",
		"teams.supported":   "supported",
		"teams.blocked":     "not supported by the engine",

		"vs.tie":          "About even",
		"vs.tie_detail":   "(difference %s, under 2%%)",
		"vs.left_higher":  "The left team is %s higher",
		"vs.right_higher": "The right team is %s higher",
		"vs.left":         "Left",
		"vs.right":        "Right",
		"vs.copy":         "Copy comparison link",
		"vs.left_team":    "Left team",
		"vs.right_team":   "Right team",

		"meta.result_title": "Gscouter result · %s",
		"meta.score_title":  "%s's team: %s DPS · Gscouter",
		"meta.one_rotation": " | one rotation %s DPS",
		"meta.team_higher":  "%s's team is %.1f%% higher",

		"err.selection": "Invalid selection: %q",
		"err.count":     "Pick exactly four characters (%d picked).",
		"err.chars":     "The character selection is invalid.",

		"problem.missing":     "%s is not in the showcase. Update the showcase and try again.",
		"problem.unsupported": "This team is not supported yet. Only teams in the library can be calculated.",
		"problem.blocked":     "The simulation engine does not support this team's characters yet. (%s)",
		"problem.busy":        "Too many calculations are running right now. Please try again shortly.",
		"problem.failed":      "The calculation failed.",

		"enka.invalid":      "The UID is not valid.",
		"enka.not_found":    "No player has this UID.",
		"enka.maintenance":  "The game is under maintenance, so the profile cannot be loaded.",
		"enka.rate_limited": "Too many requests to Enka.Network. Please try again shortly.",
		"enka.failed":       "Could not load the profile from Enka.Network.",

		"warn.draft":        "This team's rotation is not validated yet. The score may change.",
		"warn.energy":       "%s waits %.1f s per minute for energy. More Energy Recharge raises the score.",
		"warn.unknown_sets": "%s: %d artifact sets are not supported by the simulation engine yet, so their set bonuses are left out.",
		"issue.arlecchino":  "Known issue: Arlecchino's damage is calculated too low and the cause is under investigation. Adding about 20% to this result comes close to the real number.",

		"filler.wrap":  "In the looped rotation, %s.",
		"filler.sep":   "; ",
		"filler.skip":  "the burst of %s is skipped when short of energy",
		"filler.party": "while %s wait for burst energy, the time is filled with Elemental Skills (swapping to another member for theirs and straight back when on cooldown) or normal attacks",
		"filler.self":  "while %s wait for burst energy, the time is filled with their own Elemental Skill or normal attacks",
		"filler.skill": "while %s wait for their Elemental Skill cooldown, the time is filled with normal attacks",

		"tl.heading":       "Rotation timeline",
		"tl.label_cycle":   "One rotation",
		"tl.label_steady":  "Scoring window",
		"tl.note_cycle":    "One pass of the rotation from full energy.",
		"tl.note_cycle_to": "The first %.0f seconds of one pass of the rotation from full energy.",
		"tl.note_steady":   "%[2].0f seconds of the looped fight that is scored (from %[1].0f s). Time characters stand waiting for energy, cooldowns or stamina is hatched.",
		"tl.note_filled":   " The waits are filled with skills and normal attacks.",
		"tl.span":          "Window",
		"tl.seconds":       "%ss",
		"tl.damage":        "Total damage",
		"tl.dps":           "Window DPS",
		"tl.swaps":         "Swaps",
		"tl.count":         "%d",
		"tl.chart":         "DMG",
		"tl.aria":          "%s timeline: damage per second above, each character's field time and actions below",
		"tl.skill":         "Elemental Skill",
		"tl.burst":         "Elemental Burst",
		"tl.normal":        "Normal Attack",
		"tl.heavy":         "Charged · plunge · aimed",
		"tl.field":         "On field",
		"tl.wait":          "Waiting (energy · cooldown · stamina)",
		"tl.stints":        "Swaps and damage per stint",
		"tl.stints_lead":   "One row each time the character on field changes. The bar is the damage dealt during that stint, scaled to the largest one, colored by who dealt it. Damage from bursts and summons running in the background is included.",
		"tl.sources":       "Damage by ability",
		"tl.ability":       "Ability",
		"tl.hits":          "Hits",
		"tl.crit":          "Crit",
		"tl.dmg":           "Damage",
		"tl.no_damage":     "No damage in this window.",
		"tl.one_run":       "This is one simulation run (seed 1), so crits may differ a little from the average. The score is the average of many runs.",
		"tl.script":        "Rotation script (gcsim)",
		"tl.sec":           "%.1fs",
		"tl.tip_bucket":    "%.0f–%.0fs · damage %s",
		"tl.tip_field":     "%s on field %.1f–%.1fs (%s)",
		"tl.tip_wait":      "%s waits %s · %s",
		"tl.tip_action":    "%.1fs · %s %s",

		"action.attack":      "NA",
		"action.charge":      "CA",
		"action.skill":       "E",
		"action.burst":       "Q",
		"action.dash":        "Dash",
		"action.jump":        "Jump",
		"action.walk":        "Walk",
		"action.aim":         "Aim",
		"action.high_plunge": "Plunge",
		"action.low_plunge":  "Plunge",

		"action_long.attack":      "Normal Attack",
		"action_long.charge":      "Charged Attack",
		"action_long.skill":       "Elemental Skill",
		"action_long.burst":       "Elemental Burst",
		"action_long.dash":        "Dash",
		"action_long.jump":        "Jump",
		"action_long.walk":        "Walk",
		"action_long.aim":         "Aimed Shot",
		"action_long.high_plunge": "Plunging Attack",
		"action_long.low_plunge":  "Plunging Attack",

		"wait.insufficient_energy":  "not enough energy",
		"wait.skill_cd":             "Elemental Skill cooldown",
		"wait.burst_cd":             "Elemental Burst cooldown",
		"wait.insufficient_stamina": "not enough stamina",
	},
	langKorean: {
		"nav.lookup":  "UID 조회",
		"nav.teams":   "파티 목록",
		"lang.label":  "언어",
		"lang.auto":   "자동",
		"lang.apply":  "적용",
		"site.title":  "Gscouter · 원신 파티 DPS 계산",
		"site.desc":   "원신 UID로 쇼케이스 캐릭터 4명의 파티 DPS를 같은 적·같은 로테이션 기준으로 계산하고 다른 플레이어와 비교합니다.",
		"foot.method": "계산 방식",
		"foot.method1": "Lv.100 적 1명, 모든 내성 10%, 일정한 간격의 원소 입자 조건에서 커뮤니티 gcsim 로테이션을 끝없이 반복해, " +
			"시작 에너지가 바닥난 뒤인 150–450초 구간의 평균 DPS를 점수로 씁니다. 원소 충전 효율이 부족하면 점수가 내려갑니다.",
		"foot.method2": "에너지나 쿨타임 때문에 캐릭터가 서서 기다리게 되면, 그 시간을 원소전투 스킬·교체·평타로 채운 경우와 비교해 높은 쪽을 씁니다. " +
			"\"로테이션 1회\"는 에너지가 가득한 상태에서 한 번 돌린 값으로, 커뮤니티 gcsim 설정이 보고하는 숫자와 같은 기준입니다.",
		"foot.credits": `시뮬레이션 <a href="https://github.com/genshinsim/gcsim">gcsim</a> · 로테이션 <a href="https://db.kqm.gg">KQM gcsim DB</a> · 캐릭터 정보 <a href="https://enka.network">Enka.Network</a>`,
		"foot.legal": `Gscouter는 광고·후원 없이 운영하는 비영리 비공식 팬 프로젝트로 HoYoverse와 관련이 없습니다. ` +
			`원신 및 관련 명칭은 HoYoverse의 상표이며, 캐릭터 이미지는 © HoYoverse로 <a href="https://enka.network">Enka.Network</a>에서 불러옵니다.`,
		"js.copied": "링크를 복사했습니다",
		"js.link":   "링크",

		"team.stderr":       "표준오차",
		"team.one_rotation": "로테이션 1회(에너지 가득): %s DPS",
		"team.character":    "캐릭터",
		"team.share":        "비중",
		"team.talents":      "특성",

		"index.vs_heading":      "비교할 상대 고르기",
		"index.vs_lead":         "상대방의 UID를 불러와 파티를 고르면, 같은 조건에서 두 파티의 DPS를 나란히 계산합니다.",
		"index.heading":         "쇼케이스 캐릭터로 파티 DPS 계산",
		"index.lead":            "UID를 입력하고 캐릭터 4명을 고르면, 모든 파티를 같은 적·같은 로테이션 기준으로 계산합니다. 결과 링크를 공유하거나 다른 플레이어의 파티와 비교할 수 있습니다.",
		"index.step1":           "UID 입력",
		"index.step2":           "캐릭터 4명 선택",
		"index.step3":           "결과 공유 · 비교",
		"index.uid_title":       "9–10자리 숫자",
		"index.uid_placeholder": "UID (예: 800000000)",
		"index.load":            "불러오기",
		"index.showcase_hint":   "게임 프로필의 캐릭터 쇼케이스에 넣어 둔 캐릭터만 불러올 수 있습니다. \"캐릭터 상세 정보 표시\"를 켜 주세요.",
		"index.suggested":       "바로 계산할 수 있는 파티",
		"index.suggested_count": "%d개",
		"index.no_suggested":    "쇼케이스 캐릭터로 만들 수 있는 라이브러리 파티가 없습니다. 아래에서 직접 골라 보세요.",
		"index.pick":            "직접 고르기",
		"index.compare":         "비교하기",
		"index.calculate":       "계산하기",
		"index.clear":           "선택 해제",
		"index.picked":          "%d / 4명 선택",
		"index.skipped":         "불러오지 못한 캐릭터 (ID %d): %s",
		"index.supported":       "지원하는 파티",
		"index.supported_lead":  "커뮤니티 gcsim 데이터베이스에서 가져온 파티 %d개를 계산할 수 있습니다.",
		"index.find":            "캐릭터로 찾기",

		"problem.home": "처음으로",

		"score.copy":            "결과 링크 복사",
		"score.repick":          "다른 캐릭터 고르기",
		"score.compare_heading": "다른 파티와 비교하기",
		"score.compare_lead":    "상대 UID를 불러와 파티를 고르면 두 파티를 같은 조건에서 나란히 계산합니다. 내 다른 파티와 비교하려면 내 UID를 넣으세요.",
		"score.opponent_uid":    "상대 UID",
		"score.load_opponent":   "상대 불러오기",

		"teams.title":       "지원하는 파티 · Gscouter",
		"teams.placeholder": "캐릭터 이름 (예: hutao, furina)",
		"teams.search":      "찾기",
		"teams.matching":    "\"%s\"이(가) 들어간 파티 %d개 (전체 %d개)",
		"teams.total":       "전체 %d개",
		"teams.found":       "찾은 파티 %d개 (전체 %d개)",
		"teams.hint":        "gcsim 키 이름으로 찾습니다. 여러 캐릭터를 띄어 쓰면 모두 들어간 파티만 보여 줍니다.",
		"teams.supported":   "지원",
		"teams.blocked":     "엔진 미지원",

		"vs.tie":          "비슷합니다",
		"vs.tie_detail":   "(차이 %s, 2%% 미만)",
		"vs.left_higher":  "왼쪽 파티가 %s 높습니다",
		"vs.right_higher": "오른쪽 파티가 %s 높습니다",
		"vs.left":         "왼쪽",
		"vs.right":        "오른쪽",
		"vs.copy":         "비교 링크 복사",
		"vs.left_team":    "왼쪽 파티",
		"vs.right_team":   "오른쪽 파티",

		"meta.result_title": "Gscouter 결과 · %s",
		"meta.score_title":  "%s의 파티 %s DPS · Gscouter",
		"meta.one_rotation": " | 로테이션 1회 %s DPS",
		"meta.team_higher":  "%s 파티가 %.1f%% 높습니다",

		"err.selection": "잘못된 선택: %q",
		"err.count":     "캐릭터를 정확히 4명 골라 주세요 (%d명 선택됨)",
		"err.chars":     "캐릭터 선택이 올바르지 않습니다",

		"problem.missing":     "%s 캐릭터가 쇼케이스에 없습니다. 쇼케이스를 갱신한 뒤 다시 시도해 주세요.",
		"problem.unsupported": "아직 지원하지 않는 파티입니다. 라이브러리에 있는 파티만 계산할 수 있습니다.",
		"problem.blocked":     "계산 엔진이 아직 이 파티의 캐릭터를 지원하지 않습니다. (%s)",
		"problem.busy":        "지금 계산 요청이 많습니다. 잠시 후 다시 시도해 주세요.",
		"problem.failed":      "계산 중 오류가 발생했습니다.",

		"enka.invalid":      "UID 형식이 올바르지 않습니다.",
		"enka.not_found":    "해당 UID의 플레이어를 찾을 수 없습니다.",
		"enka.maintenance":  "게임 점검 중이라 정보를 가져올 수 없습니다.",
		"enka.rate_limited": "Enka.Network 요청이 너무 많습니다. 잠시 후 다시 시도해 주세요.",
		"enka.failed":       "Enka.Network에서 정보를 가져오지 못했습니다.",

		"warn.draft":        "이 파티의 로테이션은 아직 검증 전입니다. 점수가 바뀔 수 있습니다.",
		"warn.energy":       "%s: 원소 에너지가 부족해 분당 %.1f초를 기다립니다. 원소 충전 효율을 올리면 점수가 오릅니다.",
		"warn.unknown_sets": "%s: 계산 엔진이 아직 지원하지 않는 성유물 세트가 %d개 있어, 그 세트 효과는 빠졌습니다.",
		"issue.arlecchino":  "알려진 문제: 아를레키노의 피해가 실제보다 낮게 계산되는 버그가 있어 원인을 조사 중입니다. 현재 결과에 약 20%를 더하면 실제와 비슷합니다.",

		"filler.wrap":  "반복 로테이션에서 %s.",
		"filler.sep":   ". ",
		"filler.skip":  "%s의 원소폭발은 에너지가 모자라면 건너뛰었습니다",
		"filler.party": "%s의 원소폭발 에너지가 모자라 기다리는 동안 원소전투 스킬(쿨타임이면 다른 파티원으로 교체해 스킬을 쓰고 바로 복귀)이나 평타로 채웠습니다",
		"filler.self":  "%s의 원소폭발 에너지가 모자라 기다리는 동안 자기 원소전투 스킬이나 평타로 채웠습니다",
		"filler.skill": "%s의 원소전투 스킬 쿨타임을 기다리는 동안 평타로 채웠습니다",

		"tl.heading":       "로테이션 타임라인",
		"tl.label_cycle":   "로테이션 1회",
		"tl.label_steady":  "점수 구간",
		"tl.note_cycle":    "에너지가 가득한 상태에서 로테이션을 한 번 돌린 기록입니다.",
		"tl.note_cycle_to": "에너지가 가득한 상태에서 로테이션을 한 번 돌린 기록의 처음 %.0f초입니다.",
		"tl.note_steady":   "점수를 매기는 반복 구간(%.0f초부터) 중 %.0f초입니다. 캐릭터가 에너지·쿨타임·스태미나를 기다리며 서 있는 시간이 빗금으로 보입니다.",
		"tl.note_filled":   " 기다리는 시간을 스킬·평타로 채운 로테이션입니다.",
		"tl.span":          "구간",
		"tl.seconds":       "%s초",
		"tl.damage":        "총 피해",
		"tl.dps":           "구간 DPS",
		"tl.swaps":         "교체",
		"tl.count":         "%d회",
		"tl.chart":         "피해",
		"tl.aria":          "%s 타임라인: 위는 초당 피해, 아래는 캐릭터별 필드 시간과 행동",
		"tl.skill":         "원소전투 스킬",
		"tl.burst":         "원소폭발",
		"tl.normal":        "평타",
		"tl.heavy":         "강공격 · 낙하 · 조준",
		"tl.field":         "필드에 있는 시간",
		"tl.wait":          "대기 (에너지 · 쿨타임 · 스태미나)",
		"tl.stints":        "교체 순서와 구간별 피해",
		"tl.stints_lead":   "필드 캐릭터가 바뀔 때마다 한 줄입니다. 막대는 그 구간에 나간 피해로, 가장 큰 구간을 기준으로 한 길이이고 색은 피해를 준 캐릭터입니다. 뒤에서 도는 원소폭발·소환물 피해도 포함됩니다.",
		"tl.sources":       "능력별 피해",
		"tl.ability":       "능력",
		"tl.hits":          "횟수",
		"tl.crit":          "치명",
		"tl.dmg":           "피해",
		"tl.no_damage":     "이 구간에 준 피해가 없습니다.",
		"tl.one_run":       "시뮬레이션 1회(시드 1)의 기록이라 치명타 결과가 평균과 조금 다를 수 있습니다. 점수는 여러 번 돌린 평균입니다.",
		"tl.script":        "로테이션 스크립트 (gcsim)",
		"tl.sec":           "%.1f초",
		"tl.tip_bucket":    "%.0f–%.0f초 · 피해 %s",
		"tl.tip_field":     "%s 필드 %.1f–%.1f초 (%s)",
		"tl.tip_wait":      "%s 대기 %s · %s",
		"tl.tip_action":    "%.1f초 · %s %s",

		"action.attack":      "평타",
		"action.charge":      "강공격",
		"action.skill":       "E",
		"action.burst":       "Q",
		"action.dash":        "대시",
		"action.jump":        "점프",
		"action.walk":        "걷기",
		"action.aim":         "조준",
		"action.high_plunge": "낙하",
		"action.low_plunge":  "낙하",

		"action_long.attack":      "일반 공격",
		"action_long.charge":      "강공격",
		"action_long.skill":       "원소전투 스킬",
		"action_long.burst":       "원소폭발",
		"action_long.dash":        "대시",
		"action_long.jump":        "점프",
		"action_long.walk":        "걷기",
		"action_long.aim":         "조준 사격",
		"action_long.high_plunge": "낙하 공격",
		"action_long.low_plunge":  "낙하 공격",

		"wait.insufficient_energy":  "원소 에너지 부족",
		"wait.skill_cd":             "원소전투 스킬 쿨타임",
		"wait.burst_cd":             "원소폭발 쿨타임",
		"wait.insufficient_stamina": "스태미나 부족",
	},
}
