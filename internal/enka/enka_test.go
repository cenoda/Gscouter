package enka

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	raw, err := os.ReadFile("testdata/skirk.json")
	if err != nil {
		t.Fatal(err)
	}
	p, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if p.UID != "800000000" || p.Nickname != "Tester" || p.TTL != 60 {
		t.Errorf("profile = %+v", p)
	}
	if len(p.Skipped) != 1 || p.Skipped[0].AvatarID != 10000999 {
		t.Errorf("skipped = %+v, want the unknown character", p.Skipped)
	}
	if len(p.Characters) != 1 {
		t.Fatalf("got %d characters, want 1", len(p.Characters))
	}

	// The set hash appears both as a JSON string and a number in the
	// fixture; both must resolve.
	want := `skirk char lvl=90/90 cons=2 talent=9,10,8;
skirk add weapon="azurelight" refine=1 lvl=90/90;
skirk add set="finaleofthedeepgalleries" count=2;
skirk add stats hp=4780 atk=19 cr=0.144 cd=0.21 cryo%=0.466;
`
	if got := p.Characters[0].Config(); got != want {
		t.Errorf("config:\n%s\nwant:\n%s", got, want)
	}
}

func TestFetchStatusCodes(t *testing.T) {
	cases := map[int]error{
		http.StatusNotFound:           ErrNotFound,
		http.StatusFailedDependency:   ErrMaintenance,
		http.StatusTooManyRequests:    ErrRateLimited,
		http.StatusServiceUnavailable: ErrUnavailable,
	}
	for status, want := range cases {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !strings.HasPrefix(r.Header.Get("User-Agent"), "Gscouter/") {
				t.Errorf("User-Agent = %q", r.Header.Get("User-Agent"))
			}
			w.WriteHeader(status)
		}))
		c := &Client{HTTP: srv.Client(), BaseURL: srv.URL}
		if _, err := c.Fetch(context.Background(), "800000000"); !errors.Is(err, want) {
			t.Errorf("status %d: err = %v, want %v", status, err, want)
		}
		srv.Close()
	}
}

func TestFetchRejectsInvalidUID(t *testing.T) {
	c := &Client{BaseURL: "http://unused.invalid"}
	for _, uid := range []string{"", "12345", "abcdefghi", "012345678"} {
		if _, err := c.Fetch(context.Background(), uid); !errors.Is(err, ErrInvalidUID) {
			t.Errorf("uid %q: err = %v, want ErrInvalidUID", uid, err)
		}
	}
}

func TestNames(t *testing.T) {
	for _, c := range []struct{ got, want string }{
		{CharacterName("hutao", "ko"), "호두"},
		{CharacterName("hutao", "en"), "Hu Tao"},
		{CharacterName("hutao", "xx"), "Hu Tao"},
		{CharacterName("luminecryo", "fr"), "Voyageuse"},
		{CharacterName("nobody", "en"), ""},
		{WeaponName("staffofhoma", "ja"), "護摩の杖"},
		{ArtifactName("crimsonwitchofflames", "ko"), "불타오르는 화염의 마녀"},
		{StatName("cr", "en"), "CRIT Rate"},
		{StatName("atk%", "ko"), "공격력"},
	} {
		if c.got != c.want {
			t.Errorf("got %q, want %q", c.got, c.want)
		}
	}
}

func TestIcons(t *testing.T) {
	for _, c := range []struct{ got, want string }{
		{ArtifactIconURL("crimsonwitchofflames", Flower), "https://enka.network/ui/UI_RelicIcon_15006_4.png"},
		{ArtifactIconURL("crimsonwitchofflames", Circlet), "https://enka.network/ui/UI_RelicIcon_15006_3.png"},
		{ArtifactIconURL("", Sands), ""},
	} {
		if c.got != c.want {
			t.Errorf("got %q, want %q", c.got, c.want)
		}
	}
	for _, c := range []struct {
		w    Weapon
		want string
	}{
		{Weapon{Key: "staffofhoma", MaxLevel: 90}, "https://enka.network/ui/UI_EquipIcon_Pole_Homa_Awaken.png"},
		{Weapon{Key: "staffofhoma", MaxLevel: 40}, "https://enka.network/ui/UI_EquipIcon_Pole_Homa.png"},
		{Weapon{Key: "nothing", MaxLevel: 90}, ""},
	} {
		if url, _ := WeaponIcon(c.w); url != c.want {
			t.Errorf("WeaponIcon(%v) = %q, want %q", c.w, url, c.want)
		}
	}
	if _, rarity := WeaponIcon(Weapon{Key: "staffofhoma", MaxLevel: 90}); rarity != 5 {
		t.Errorf("staffofhoma rarity %d, want 5", rarity)
	}
}
