// Repro tests for AUDIT.md. They fail while the defects described there
// are present. They are not a fix.
package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cenoda/gscouter/internal/enka"
)

// TestRepro_SavedBuildsDoesNotCacheContextCancel shows that a caller
// cancelling SavedBuilds stores that error for savedRetry. The next
// caller, with a live context, never asks Enka.
func TestRepro_SavedBuildsDoesNotCacheContextCancel(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	var n atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n.Add(1)
		once.Do(func() { close(entered) })
		<-release
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	defer close(release)

	e := NewEnkaSource(&enka.Client{HTTP: srv.Client(), BaseURL: srv.URL})
	owner := enka.Owner{Username: "player", Hash: "h4sh"}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-entered
		cancel()
	}()
	if _, err := e.SavedBuilds(ctx, owner); err == nil {
		t.Fatal("cancelled fetch returned no error")
	}
	if _, err := e.SavedBuilds(context.Background(), owner); err != nil {
		t.Fatalf("fresh context still failed: %v (Enka calls: %d)", err, n.Load())
	}
	if got := n.Load(); got < 2 {
		t.Fatalf("Enka was called %d times; a cancelled fetch was reused", got)
	}
}

// TestRepro_EnkaProfileSingleflight shows that two overlapping lookups
// of one UID both reach Enka. The cache is filled only after a fetch
// returns, so a miss stampede is not collapsed.
func TestRepro_EnkaProfileSingleflight(t *testing.T) {
	var n atomic.Int32
	both := make(chan struct{})
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if n.Add(1) == 2 {
			close(both)
		}
		<-release
		w.Write([]byte(`{"uid":"800000001","playerInfo":{"nickname":"A"}}`))
	}))
	defer srv.Close()

	e := NewEnkaSource(&enka.Client{HTTP: srv.Client(), BaseURL: srv.URL})
	var wg sync.WaitGroup
	wg.Add(2)
	for range 2 {
		go func() {
			defer wg.Done()
			_, _ = e.Profile(context.Background(), "800000001")
		}()
	}
	select {
	case <-both:
		close(release)
		wg.Wait()
		t.Fatalf("one UID reached Enka %d times before either lookup returned", n.Load())
	case <-time.After(time.Second):
		close(release)
		wg.Wait()
	}
}

// TestRepro_EmptyResIsRejected shows that "?res=" is not a number but
// is applied as 0% resistance on every element.
func TestRepro_EmptyResIsRejected(t *testing.T) {
	for _, query := range []string{"res=", "res=%20"} {
		q, err := url.ParseQuery(query)
		if err != nil {
			t.Fatal(err)
		}
		e, err := enemyFromQuery(q)
		if err == nil {
			t.Fatalf("%q accepted as level %d resist %v", query, e.Level, e.Resist)
		}
	}
}

// TestRepro_FlushUsageStopsBackgroundFlush shows that SetUsage starts a
// time.Tick loop nothing ever stops. FlushUsage only writes the file.
func TestRepro_FlushUsageStopsBackgroundFlush(t *testing.T) {
	before := usageFlushGoroutines()
	s := &Server{}
	if err := s.SetUsage(t.TempDir() + "/usage.json"); err != nil {
		t.Fatal(err)
	}
	if err := s.FlushUsage(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(20 * time.Millisecond)
	if after := usageFlushGoroutines(); after > before {
		t.Fatalf("FlushUsage left %d background flush goroutine(s) running", after-before)
	}
}

func usageFlushGoroutines() int {
	buf := make([]byte, 1<<20)
	n := runtime.Stack(buf, true)
	return strings.Count(string(buf[:n]), "SetUsage.func1")
}
