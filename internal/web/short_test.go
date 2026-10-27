package web

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
)

// memLinks is a LinkStore in memory.
type memLinks struct {
	mu sync.Mutex
	m  map[string]string
}

func (s *memLinks) Add(_ context.Context, id, target string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if got, ok := s.m[id]; ok {
		return got, nil
	}
	s.m[id] = target
	return target, nil
}

func (s *memLinks) Get(_ context.Context, id string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.m[id], nil
}

func shorten(t *testing.T, client *http.Client, base, u string) (int, string) {
	t.Helper()
	resp, err := client.PostForm(base+"/s", url.Values{"u": {u}})
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, strings.TrimSpace(string(b))
}

func TestShortLink(t *testing.T) {
	srv := newTestSite(t)
	store := &memLinks{m: map[string]string{}}
	srv.SetLinks(store)
	ts := newTestServerFor(t, srv)
	noFollow := *ts.Client()
	noFollow.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

	long := "/score?" + url.Values{"a": {"800000001~" + packBuilds("Strong", hutaoTeam())}, "lvl": {"90"}}.Encode()
	code, short := shorten(t, ts.Client(), ts.URL, long+"&server=1")
	if code != http.StatusOK || !strings.HasPrefix(short, ts.URL+"/s/") || len(short) != len(ts.URL+"/s/")+shortID {
		t.Fatalf("shorten: %d %q", code, short)
	}
	// The same result gets the same link, with or without the server
	// fallback.
	if _, again := shorten(t, ts.Client(), ts.URL, long); again != short {
		t.Errorf("shorten again: %q, want %q", again, short)
	}

	resp, err := noFollow.Get(short)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusFound || resp.Header.Get("Location") != long {
		t.Errorf("open: %d to %q, want %q", resp.StatusCode, resp.Header.Get("Location"), long)
	}

	code, body := getKo(t, ts, "/s/AAAAAAAA")
	if code != http.StatusNotFound {
		t.Errorf("unknown link: status %d", code)
	}
	mustContain(t, body, "존재하지 않는 짧은 링크입니다.")
}

func TestShortLinkCollision(t *testing.T) {
	srv := newTestSite(t)
	store := &memLinks{m: map[string]string{}}
	srv.SetLinks(store)
	ts := newTestServerFor(t, srv)

	long := "/score?" + url.Values{"a": {"800000001~" + packBuilds("Strong", hutaoTeam())}}.Encode()
	_, short := shorten(t, ts.Client(), ts.URL, long)
	id := short[strings.LastIndex(short, "/")+1:]
	// Another result took the ID first.
	store.m = map[string]string{id: "/score?a=other"}
	_, short = shorten(t, ts.Client(), ts.URL, long)
	if got := short[strings.LastIndex(short, "/")+1:]; got != id+got[shortID:] || len(got) != shortID+1 {
		t.Errorf("after a collision: %q, want %q and one more character", got, id)
	}
}

func TestShortLinkRejects(t *testing.T) {
	srv := newTestSite(t)
	srv.SetLinks(&memLinks{m: map[string]string{}})
	ts := newTestServerFor(t, srv)
	packed := "800000001~" + packBuilds("Strong", hutaoTeam())
	for _, u := range []string{
		"",
		"https://evil.example/score?a=" + url.QueryEscape(packed),
		"//evil.example/score?a=" + url.QueryEscape(packed),
		"/teams?a=" + url.QueryEscape(packed),
		"/score?a=800000001:hutao,yelan,xingqiu,zhongli", // no builds
		"/score?a=800000001~garbage",
		"/versus?a=" + url.QueryEscape(packed), // no b
		"/score?a=" + url.QueryEscape(packed) + "&lvl=999",
		"/score?a=" + strings.Repeat("x", maxTarget),
	} {
		if code, body := shorten(t, ts.Client(), ts.URL, u); code != http.StatusBadRequest {
			t.Errorf("shorten %.60q: %d %q", u, code, body)
		}
	}
}

func TestShortLinkOff(t *testing.T) {
	ts := newTestServer(t)
	packed := "800000001~" + packBuilds("Strong", hutaoTeam())
	if code, _ := shorten(t, ts.Client(), ts.URL, "/score?a="+url.QueryEscape(packed)); code != http.StatusNotFound {
		t.Errorf("shorten without a store: %d", code)
	}
}

func TestShortLinkRateLimit(t *testing.T) {
	srv := newTestSite(t)
	srv.SetLinks(&memLinks{m: map[string]string{}})
	ts := newTestServerFor(t, srv)
	u := "/score?a=" + url.QueryEscape("800000001~"+packBuilds("Strong", hutaoTeam()))
	for i := range linksPerMinute {
		if code, _ := shorten(t, ts.Client(), ts.URL, u); code != http.StatusOK {
			t.Fatalf("link %d: %d", i, code)
		}
	}
	if code, _ := shorten(t, ts.Client(), ts.URL, u); code != http.StatusTooManyRequests {
		t.Errorf("link over the limit: %d", code)
	}
}
