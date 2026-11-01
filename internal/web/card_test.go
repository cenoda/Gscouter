package web

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cenoda/gscouter/internal/score"
)

// iconServer serves every face icon as a small PNG through handle, and
// sends every icon request to it, whatever the host.
func iconServer(t *testing.T, handle func(w http.ResponseWriter, r *http.Request) bool) {
	t.Helper()
	var buf bytes.Buffer
	icon := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for i := range icon.Pix {
		icon.Pix[i] = uint8(i * 7)
	}
	png.Encode(&buf, icon)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if handle != nil && !handle(w, r) {
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.Write(buf.Bytes())
	}))
	t.Cleanup(ts.Close)
	target, _ := url.Parse(ts.URL)
	old := iconClient
	iconClient = &http.Client{Transport: rewriteHost{target}}
	t.Cleanup(func() { iconClient = old })
	resetIcons()
	t.Cleanup(resetIcons)
}

type rewriteHost struct{ to *url.URL }

func (h rewriteHost) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.URL.Scheme, r.URL.Host = h.to.Scheme, h.to.Host
	return http.DefaultTransport.RoundTrip(r)
}

func resetIcons() {
	iconMu.Lock()
	clear(iconCache)
	iconMu.Unlock()
}

func cardTeam(names ...string) teamResult {
	res := &score.Result{DPS: 41234}
	for i, n := range names {
		res.Characters = append(res.Characters, score.Character{Name: n, Share: 0.4 - 0.1*float64(i)})
	}
	return teamResult{Result: res}
}

// TestCardsDoNotWaitOnIcons draws a card while another one is stuck
// fetching an icon: it must not wait for that fetch.
func TestCardsDoNotWaitOnIcons(t *testing.T) {
	stuck, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	iconServer(t, func(w http.ResponseWriter, r *http.Request) bool {
		if strings.Contains(r.URL.Path, "Hutao") {
			once.Do(func() { close(stuck) })
			<-release
		}
		return true
	})
	defer close(release)
	go scoreCard(context.Background(), cardTeam("hutao", "xingqiu", "yelan", "zhongli"))
	<-stuck
	done := make(chan struct{})
	go func() {
		scoreCard(context.Background(), cardTeam("raiden", "nahida", "xiangling", "bennett"))
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(20 * time.Second):
		t.Fatal("a card waited for another card's icon fetch")
	}
}

// TestCardIconsLoadInParallel checks that a card fetches its members'
// icons at the same time, not one after another.
func TestCardIconsLoadInParallel(t *testing.T) {
	var inFlight, peak atomic.Int32
	iconServer(t, func(w http.ResponseWriter, r *http.Request) bool {
		n := inFlight.Add(1)
		defer inFlight.Add(-1)
		for p := peak.Load(); n > p && !peak.CompareAndSwap(p, n); p = peak.Load() {
		}
		time.Sleep(200 * time.Millisecond)
		return true
	})
	versusCard(context.Background(), versusData{
		A: cardTeam("hutao", "xingqiu", "yelan", "zhongli"),
		B: cardTeam("raiden", "nahida", "xiangling", "bennett"),
	})
	if p := peak.Load(); p < 2 || p > iconFetches {
		t.Errorf("%d icon fetches at once, want 2 to %d", p, iconFetches)
	}
}

// TestFailedIconsAreRemembered checks that an icon that failed to load
// is not fetched again for every card.
func TestFailedIconsAreRemembered(t *testing.T) {
	var hits atomic.Int32
	iconServer(t, func(w http.ResponseWriter, r *http.Request) bool {
		hits.Add(1)
		http.Error(w, "down", http.StatusServiceUnavailable)
		return false
	})

	tr := cardTeam("hutao", "xingqiu", "yelan", "zhongli")
	scoreCard(context.Background(), tr)
	if n := hits.Load(); n != 4 {
		t.Fatalf("first card fetched %d icons, want 4", n)
	}
	scoreCard(context.Background(), tr)
	if n := hits.Load(); n != 4 {
		t.Errorf("second card fetched the failed icons again: %d fetches in all", n)
	}
	// Once the retry time has passed, they are fetched again.
	iconMu.Lock()
	for k, e := range iconCache {
		e.retry = time.Now().Add(-time.Second)
		iconCache[k] = e
	}
	iconMu.Unlock()
	scoreCard(context.Background(), tr)
	if n := hits.Load(); n != 8 {
		t.Errorf("after the retry time: %d fetches in all, want 8", n)
	}
}
