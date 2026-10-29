package web

import "net/http"

// The about page says what the site promises and what it keeps, in every
// site language. The updates page lists the changes a visitor can notice,
// newest first; its entries are written in English and Korean, the
// owner's language, and the other languages show the English.

// update is one day's entry on the updates page.
type update struct {
	Date string // YYYY-MM-DD
	// Scores is set when the day's changes move scores, shared results'
	// included.
	Scores bool
	En, Ko []string
}

// updates are the site's changes, newest first. Add an entry when a
// change is one a visitor can notice: new teams or characters, an engine
// update, a change to how scores are calculated, a new feature. The
// footer shows the newest date.
var updates = []update{
	{
		Date:   "2026-10-29",
		Scores: true,
		En: []string{
			"Teams try rotations written for higher constellations and keep the best, starting with Chasca C6 and Citlali C6.",
			"An engine fix for Vesna; her teams were checked again.",
			"New teams: three for Cyno, and Mavuika / Citlali / Prune / Nicole.",
			"An \"Our promises\" page, this updates page, and a Discord link at the bottom of every page.",
			"The rotation timeline tells a held or recast skill from a tap (\"Hold E\", \"E (recast)\"), and shows Mavuika's held charged attack as one action.",
		},
		Ko: []string{
			"높은 돌파 수치에 맞춰 쓰인 로테이션이 있으면 그것도 시도해 가장 높은 점수를 씁니다. 차스카 6돌과 시틀라리 6돌부터 적용했습니다.",
			"엔진의 베스나 구현을 고치고 베스나 팀을 다시 확인했습니다.",
			"새 팀: 사이노 팀 3개, 마비카 / 시틀라리 / 프루네 / 니콜.",
			"\"우리의 약속\" 페이지, 이 업데이트 내역, 모든 페이지 아래의 Discord 링크를 추가했습니다.",
			"로테이션 타임라인에서 홀드 E와 재시전 E를 짧게 누른 E와 구분하고(\"홀드 E\", \"E (재시전)\"), 마비카의 홀드 강공격을 동작 하나로 보여 줍니다.",
		},
	},
	{
		Date:   "2026-10-28",
		Scores: true,
		En: []string{
			"Scores take half as long: 50 simulations instead of 100. Shared results are calculated again, so their numbers move slightly.",
			"Result pages show an artifact score, each character's total stats, and talent levels with constellation bonuses.",
			"New teams for Sandrone, Lohen, Vesna and Vodyanitsa.",
		},
		Ko: []string{
			"점수 계산 시간이 절반으로 줄었습니다(시뮬레이션 100회에서 50회로). 공유된 결과도 다시 계산되므로 숫자가 조금 달라질 수 있습니다.",
			"결과 페이지에 성유물 점수, 캐릭터별 전체 스탯, 운명의 자리 보너스를 포함한 특성 레벨이 나옵니다.",
			"산드로네, 로엔, 베스나, 보댜니차의 새 팀을 추가했습니다.",
		},
	},
	{
		Date: "2026-10-27",
		En: []string{
			"Short links for shared results.",
			"A page for every library team and every character.",
			"New teams for Nefer with Nilou.",
		},
		Ko: []string{
			"공유한 결과에 짧은 링크가 생겼습니다.",
			"라이브러리의 모든 팀과 캐릭터에 각자의 페이지가 생겼습니다.",
			"닐루와 함께하는 네페르 새 팀을 추가했습니다.",
		},
	},
	{
		Date: "2026-10-26",
		En:   []string{"Gscouter opens, with every usable team from the community gcsim database."},
		Ko:   []string{"Gscouter를 열었습니다. 커뮤니티 gcsim 데이터베이스에서 쓸 수 있는 팀을 모두 담았습니다."},
	},
}

// lastUpdate is the date of the newest update, for the footer.
func lastUpdate() string { return updates[0].Date }

// updateView is an update's lines in the page's language.
type updateView struct {
	Date   string
	Scores bool
	Lines  []string
}

type updatesData struct {
	Meta    pageMeta
	Updates []updateView
}

func (s *Server) handleAbout(w http.ResponseWriter, r *http.Request) {
	l := localizerFor(r)
	m := pageMeta{Title: l.T("about.title") + " · Gscouter", Description: l.T("about.desc")}
	s.render(w, l, "about.html", s.indexed(r, l, m, "/about"))
}

func (s *Server) handleUpdates(w http.ResponseWriter, r *http.Request) {
	l := localizerFor(r)
	data := updatesData{Meta: pageMeta{Title: l.T("updates.title") + " · Gscouter", Description: l.T("updates.lead")}}
	for _, u := range updates {
		lines := u.En
		if l.Lang == langKorean {
			lines = u.Ko
		}
		data.Updates = append(data.Updates, updateView{Date: u.Date, Scores: u.Scores, Lines: lines})
	}
	s.render(w, l, "updates.html", data)
}
