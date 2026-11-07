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
		Date:   "2026-11-07",
		Scores: true,
		En: []string{
			"gscouter-engine now scores the same 0–120 s window, up to the last completed rotation, as gcsim, and calculates the scores of 4,569 library teams again, every team but one. Each team’s library build was compared with gcsim on the new window and is within 0.5% of it (on average 0.02% lower), so these teams’ scores move by about that much. Charts, timelines, character details and builds with content the engine lacks stay on gcsim.",
		},
		Ko: []string{
			"gscouter-engine이 이제 gcsim과 같은 0–120초(마지막으로 끝난 사이클까지) 구간으로 점수를 내고, 라이브러리 파티 4,569개(한 팀을 뺀 전부)의 점수를 다시 계산합니다. 파티마다 라이브러리 빌드를 새 구간에서 gcsim과 비교했고 0.5% 이내(평균 0.02% 낮음)로 같아서, 이 파티들의 점수는 그만큼 움직입니다. 차트, 타임라인, 캐릭터 상세와 엔진에 없는 내용이 들어간 빌드는 gcsim으로 계산합니다.",
		},
	},
	{
		Date:   "2026-11-05",
		Scores: true,
		En: []string{
			"Scores now measure the first 120 s of the fight from full energy, stopping at the end of the last completed rotation, instead of 150–450 s. Starting with full energy matches Spiral Abyss and Stygian Onslaught, where the first rotation’s bursts are already charged and Energy Recharge only has to sustain the rotations after it; stopping at the last completed rotation avoids counting a support setup at 120 s whose damage phase was cut off by the timer.",
			"While gscouter-engine is updated for the new 0–120 s last-completed-rotation window, all teams are calculated by gcsim.",
			"The footer credits gscouter-engine beside gcsim as the simulation.",
		},
		Ko: []string{
			"점수가 이제 150–450초 구간 대신, 원소 에너지가 가득 찬 상태에서 시작해 120초 동안(마지막으로 끝난 사이클까지만) 평균 DPS를 잽니다. 첫 사이클을 완충 상태로 시작하므로 나선 비경이나 지맥 제압전처럼 첫 사이클 원소폭발은 이미 차 있고 그다음 사이클부터만 원소 충전 효율이 필요한 실제 세팅 기준과 맞으며, 120초에 사이클이 덜 끝났을 때 딜 타임 없이 서포터 선작업 시간만 분모에 들어가 DPS가 낮아지던 현상이 사라집니다.",
			"새 0–120초(마지막 완료 사이클 기준) 구간에 맞춰 gscouter-engine을 업데이트하는 동안 모든 파티는 gcsim으로 계산합니다.",
			"푸터의 시뮬레이션 출처에 gcsim과 함께 gscouter-engine을 적었습니다.",
		},
	},
	{
		Date:   "2026-11-04",
		Scores: true,
		En: []string{
			"Scores of 4,430 library teams are now calculated by gscouter-engine, a port of gcsim, in your browser and on the server. Each team’s library build was compared with gcsim and its score is within 1% of gcsim’s, so these teams’ scores move a little, by less than 1%. Results with your own rotation chart, timelines and character details are still calculated by gcsim, and so is any build that uses something the engine does not have yet.",
			"For teams calculated by gscouter-engine, the engine now searches for fillers itself, and more thoroughly: it tries each candidate with the full 50 iterations instead of 10. Result pages that search for fillers are about twice as fast. In a few builds the result shows another filler than before, because the search found another one, usually a better one, and its score changes with it.",
			"17 more library teams, Nefer’s teams and Hu Tao / Yelan / Ineffa / Flins, are now calculated by gscouter-engine, 4,447 in all. Every weapon and artifact set is now in the engine, so builds that use them, which fell back to gcsim before, now run on the engine for these teams, within 1% of gcsim. Builds with anything the engine still lacks keep running on gcsim.",
		},
		Ko: []string{
			"라이브러리 파티 4,430개의 점수를 이제 gcsim을 옮긴 gscouter-engine이 브라우저와 서버에서 계산합니다. 파티마다 라이브러리 빌드를 gcsim과 비교했고 점수는 gcsim과 1% 이내로 같아서, 이 파티들의 점수가 1% 미만으로 조금 바뀝니다. 직접 만든 로테이션 차트, 타임라인, 캐릭터 상세, 그리고 엔진에 아직 없는 것을 쓰는 빌드는 여전히 gcsim이 계산합니다.",
			"gscouter-engine이 계산하는 파티는 이제 엔진이 필러를 직접, 더 꼼꼼하게 찾습니다. 후보마다 10회가 아니라 50회 전체로 돌려 봅니다. 필러를 찾는 결과 페이지가 약 두 배 빨라졌습니다. 일부 빌드에서는 대개 더 나은 필러를 찾아 전과 다른 필러가 나오고, 점수도 그에 따라 바뀝니다.",
			"라이브러리 파티 17개(네페르 파티들과 호두 / 야란 / 이네파 / 플린스)를 더해, 이제 4,447개 파티를 gscouter-engine이 계산합니다. 모든 무기와 성유물 세트가 엔진에 들어와서, 전에는 그 무기나 세트 때문에 gcsim으로 계산하던 빌드도 이 파티들에서는 엔진이 gcsim과 1% 이내로 계산합니다. 엔진에 아직 없는 것을 쓰는 빌드는 계속 gcsim이 계산합니다.",
		},
	},
	{
		Date: "2026-11-01",
		En: []string{
			"The page linked as “Privacy and data” in the footer (formerly “Our promises”) states its principles plainly and adds how to check them yourself: the site’s source code on GitHub, and your browser’s developer tools. The server’s error logs no longer hold the UID of a failed lookup.",
			"The share sheet puts the link first, with Copy link as its main button, and fits on one screen. Its artifact box now reads “Include artifact pieces” (on by default) and says what turning it off leaves out. A tap outside the sheet closes it.",
			"In the rotation editor, holding the attack key on a bow character (Chasca, Ganyu and others) adds an aimed shot; it used to add a charged attack, which gcsim rejects for bows.",
			"Artifact pieces are kept for builds whose flat substats (HP, ATK, DEF, Elemental Mastery) rolled at the top value every time. Their whole set of pieces was dropped before, because the game rounds those numbers up.",
			"A suggested party opened in a new tab or copied as a link keeps the builds of characters you added outside the showcase.",
			"Links in the header, the footer and the front page keep the page language.",
			"A GOOD file with talent levels above 10 (levels with the constellation bonus) skips those characters with a reason, instead of failing when the score is calculated.",
			"When a GOOD file's character is not imported, the reason is in the page's language and names the character once, by its in-game name: “Not imported: Hu Tao (no weapon equipped)”.",
			"The language picker in the header no longer switches the page as soon as an arrow key moves through the list. With the keyboard, press Enter or the Apply button that appears; a pick with the mouse or a finger switches right away as before.",
			"Questions, score reports and short link removals now go through “Message the developer” at the bottom of every page: only the developer reads them, and the page you are on is sent along.",
		},
		Ko: []string{
			"푸터의 “개인정보와 데이터” 페이지(이전 “우리의 약속”)가 원칙을 담백하게 적고, 직접 확인하는 방법을 더했습니다: GitHub의 소스 코드와 브라우저의 개발자 도구. 서버 오류 로그에 실패한 조회의 UID가 더 이상 남지 않습니다.",
			"공유하기 창에서 링크가 맨 위로 올라오고 링크 복사가 주 버튼이 되었으며, 창이 한 화면에 들어옵니다. 성유물 체크박스는 “성유물 부위 정보 포함”(기본 켜짐)으로 바뀌었고, 끄면 무엇이 빠지는지 설명이 붙었습니다. 창 바깥을 누르면 닫힙니다.",
			"로테이션 편집기에서 활 캐릭터(차스카, 감우 등)의 평타 키를 길게 누르면 조준 사격이 들어갑니다. 전에는 강공격이 들어가 gcsim이 거부했습니다.",
			"고정 수치 부옵션(HP, 공격력, 방어력, 원소 마스터리)이 매번 최고 수치로 오른 빌드도 성유물 부위 정보가 유지됩니다. 게임이 이 수치를 반올림해 보여주는 탓에 전에는 그 캐릭터의 부위 정보가 통째로 빠졌습니다.",
			"추천 파티를 새 탭으로 열거나 링크로 복사해도 쇼케이스 밖에서 추가한 캐릭터의 빌드가 유지됩니다.",
			"헤더, 푸터, 첫 화면의 링크가 페이지 언어를 유지합니다.",
			"특성 레벨이 10을 넘는(별자리 보너스가 더해진) GOOD 파일은 점수 계산 중에 실패하지 않고, 그 캐릭터를 이유와 함께 건너뜁니다.",
			"GOOD 파일의 캐릭터를 가져오지 못했을 때 그 이유가 페이지 언어로 나오고, 캐릭터는 게임 속 이름으로 한 번만 나옵니다. 예: “가져오지 못함: 호두 (장착한 무기 없음)”.",
			"헤더의 언어 선택 목록에서 방향키로 움직이기만 해도 페이지 언어가 바뀌던 문제를 고쳤습니다. 키보드로는 Enter를 누르거나 나타나는 적용 버튼을 누르면 바뀌고, 마우스나 손가락으로 고르면 전처럼 바로 바뀝니다.",
			"질문, 점수 제보, 짧은 링크 삭제 요청은 이제 모든 페이지 아래의 “개발자에게 메시지 보내기”로 보냅니다. 개발자만 읽고, 보고 있던 페이지 주소가 함께 전송됩니다.",
		},
	},
	{
		Date: "2026-10-31",
		En: []string{
			"The party list starts by picking a character or typing a name, then shows matching parties one per row. It no longer marks every party as supported.",
			"A note added in the rotation editor while it starts over from the library rotation is no longer replaced when the recording finishes or fails.",
			"Choosing \"Saved charts\" itself in the rotation editor's list no longer loads the first saved chart.",
			"Results and comparisons have a Share button: copy the link, post it to X, Facebook, Reddit or LINE, or send it through your phone's share menu. A result can also be saved, copied or sent as an image, with or without each character's build details.",
		},
		Ko: []string{
			"파티 목록은 캐릭터를 고르거나 이름을 입력하는 화면으로 시작하고, 맞는 파티를 한 줄에 하나씩 보여 줍니다. 모든 파티에 붙던 지원 표시는 빠지고, 실험적·엔진 미지원만 남습니다.",
			"로테이션 편집기가 라이브러리 로테이션으로 다시 시작하는 동안 추가한 노트가, 녹화가 끝나거나 실패해도 덮어써지지 않습니다.",
			"로테이션 편집기의 목록에서 \"저장한 차트\" 자체를 골라도 첫 번째 저장한 차트를 불러오지 않습니다.",
			"결과와 비교 페이지에 공유하기 버튼이 생겼습니다. 링크를 복사하거나 X·Facebook·Reddit·LINE에 올리거나 휴대폰 공유 메뉴로 보낼 수 있습니다. 결과는 이미지로 저장·복사·전송할 수도 있고, 캐릭터별 세부 스펙을 넣을지 뺄지 고를 수 있습니다.",
		},
	},
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
			"A rotation that writes a space before an action's settings, such as \"skill [counter=1]\", is filled out the same as one without the space; one team's score changed slightly.",
			"Deleting every note in the rotation editor stays deleted after reloading the page.",
			"The rotation editor's undo can take back a reset and the first note of a blank chart.",
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
			"동작 설정 앞에 공백을 쓴 로테이션(예: \"skill [counter=1]\")도 공백이 없는 것과 똑같이 채웁니다. 한 파티의 점수가 조금 바뀌었습니다.",
			"로테이션 편집기에서 노트를 모두 지우면 페이지를 새로 고쳐도 지워진 채로 남습니다.",
			"로테이션 편집기의 되돌리기로 초기화와 빈 차트의 첫 노트를 되돌릴 수 있습니다.",
		},
	},
	{
		Date:   "2026-10-29",
		Scores: true,
		En: []string{
			"Teams try rotations written for higher constellations and keep the best, starting with Chasca C6 and Citlali C6.",
			"An engine fix for Vesna; her teams were checked again.",
			"New teams: three for Cyno, and Mavuika / Citlali / Prune / Nicole.",
			"An \"Our promises\" page and this updates page.",
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
			"\"우리의 약속\" 페이지와 이 업데이트 내역을 추가했습니다.",
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
	// LinesLang is the language of the lines when it is not the page's
	// (English on a page in a third language), for screen readers.
	LinesLang string
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
		} else if l.Lang != langEnglish {
			data.LinesLang = langEnglish
		}
		data.Updates = append(data.Updates, updateView{Date: u.Date, Scores: u.Scores, Lines: lines})
	}
	s.render(w, l, "updates.html", data)
}
