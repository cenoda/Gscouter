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
		Date: "2026-10-30",
		En: []string{
			"The source code of Gscouter is open under the GNU AGPL, linked at the bottom of every page.",
			"A loaded UID can import characters from a GOOD file (Genshin Optimizer, inventory scanners), so that characters outside the showcase can be picked too.",
			"A loaded showcase names the characters that would unlock the most teams if you put them in it, and counts down to when a changed showcase can be loaded again.",
			"Builds saved on Enka.Network can be picked: when a UID is public on an Enka account, its characters outside the showcase come with their saved builds, and a character with several builds can be picked with any of them.",
			"Comparing a result scored with your own rotation compares that rotation; it used to compare the library's.",
			"The link preview of a result scored with your own rotation shows that score, not the library's.",
			"Your own rotation is scored even when the library's rotation for the team fails on your build.",
			"The rotation editor's first chart is recorded with the library team's character settings, and keeps a wait that ends the rotation.",
			"Calculating in the browser no longer falls back to the server when one candidate rotation fails.",
			"A very short rotation of your own can be calculated in the browser; it used to fail there.",
			"The rotation editor keeps a custom enemy, from the result to the editor and back.",
			"The rotation editor records a rotation's delays as delays, not waits, so its timing stays the same.",
		},
		Ko: []string{
			"Gscouter의 소스 코드를 GNU AGPL로 공개하고, 모든 페이지 아래에 링크했습니다.",
			"UID를 불러온 뒤 GOOD 파일(Genshin Optimizer, 인벤토리 스캐너)에서 캐릭터를 가져와, 쇼케이스 밖의 캐릭터도 고를 수 있습니다.",
			"불러온 쇼케이스에 어떤 캐릭터를 넣으면 계산할 수 있는 파티가 가장 많이 늘어나는지 보여 주고, 쇼케이스를 바꾼 뒤 다시 불러올 수 있을 때까지 남은 시간을 셉니다.",
			"Enka.Network에 저장한 빌드도 고를 수 있습니다. Enka 계정에 UID가 공개되어 있으면 쇼케이스에 없는 캐릭터도 저장된 빌드로 나오고, 빌드가 여러 개인 캐릭터는 그중 하나를 골라 계산할 수 있습니다.",
			"직접 짠 로테이션으로 계산한 결과를 비교하면 그 로테이션으로 비교합니다. 전에는 라이브러리 로테이션으로 비교했습니다.",
			"직접 짠 로테이션으로 계산한 결과의 링크 미리보기가 라이브러리 로테이션 점수가 아니라 그 점수를 보여 줍니다.",
			"라이브러리 로테이션이 내 빌드에서 실패해도 직접 짠 로테이션은 계산합니다.",
			"로테이션 편집기의 첫 차트를 라이브러리 파티의 캐릭터 설정으로 녹화하고, 로테이션 끝의 대기도 남깁니다.",
			"브라우저 계산에서 후보 로테이션 하나가 실패해도 서버 계산으로 넘어가지 않습니다.",
			"아주 짧은 직접 짠 로테이션도 브라우저에서 계산됩니다. 전에는 브라우저에서 실패했습니다.",
			"로테이션 편집기가 결과에서 편집기로 갔다가 돌아올 때까지 직접 설정한 적을 유지합니다.",
			"로테이션 편집기가 로테이션의 delay를 wait로 바꾸지 않고 그대로 녹화해, 동작 타이밍이 바뀌지 않습니다.",
		},
	},
	{
		Date:   "2026-10-29",
		Scores: true,
		En: []string{
			"Teams try rotations written for higher constellations and keep the best, starting with Chasca C6 and Citlali C6.",
			"An engine fix for Vesna; her teams were checked again.",
			"New teams: three for Cyno, and Mavuika / Citlali / Prune / Nicole.",
			"An \"Our promises\" page, this updates page, and a Discord link at the bottom of every page.",
			"The rotation timeline tells a held or recast skill from a tap (\"Hold E\", \"E (recast)\"), and shows Mavuika's held charged attack as one action.",
			"A rotation editor: change a team's rotation as notes on its members' lanes, with the game's keys, and score it against the library's. A rotation that scores higher can be sent to the developer, teams outside the library can be scored with a rotation you write, and a switch picks up crystallize shards automatically.",
			"In the rotation editor, a gray band joins a selected note to the playhead when the two are apart.",
			"Weapons and artifact pieces show their icons, framed in their rarity's color.",
			"The rotation editor has settings: its keys can be changed (mouse buttons too), and the party can be dragged into your own order, which the number keys and the lanes follow.",
		},
		Ko: []string{
			"높은 돌파 수치에 맞춰 쓰인 로테이션이 있으면 그것도 시도해 가장 높은 점수를 씁니다. 차스카 6돌과 시틀라리 6돌부터 적용했습니다.",
			"엔진의 베스나 구현을 고치고 베스나 팀을 다시 확인했습니다.",
			"새 팀: 사이노 팀 3개, 마비카 / 시틀라리 / 프루네 / 니콜.",
			"\"우리의 약속\" 페이지, 이 업데이트 내역, 모든 페이지 아래의 Discord 링크를 추가했습니다.",
			"로테이션 타임라인에서 홀드 E와 재시전 E를 짧게 누른 E와 구분하고(\"홀드 E\", \"E (재시전)\"), 마비카의 홀드 강공격을 동작 하나로 보여 줍니다.",
			"로테이션 에디터: 파티의 로테이션을 캐릭터별 레인 위 노트로 게임 키 그대로 고치고, 라이브러리 로테이션과 비교해 점수를 계산합니다. 더 높게 나온 로테이션은 개발자에게 보낼 수 있고, 라이브러리에 없는 파티도 직접 짠 로테이션으로 계산할 수 있으며, 결정화 조각을 자동으로 줍는 스위치가 있습니다.",
			"로테이션 에디터에서 선택한 노트와 플레이헤드가 떨어져 있으면 회색 띠로 이어 보여 줍니다.",
			"무기와 성유물 부위에 아이콘을 표시하고, 등급은 테두리 색으로 보여 줍니다.",
			"로테이션 에디터에 설정이 생겼습니다. 키를 바꿀 수 있고(마우스 버튼 포함), 파티 순서를 끌어서 정하면 숫자 키와 레인이 그 순서를 따릅니다.",
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
