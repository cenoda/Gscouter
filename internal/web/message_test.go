package web

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

type memMessages struct{ text, page []string }

func (m *memMessages) AddMessage(_ context.Context, text, page string) error {
	m.text = append(m.text, text)
	m.page = append(m.page, page)
	return nil
}

func TestMessage(t *testing.T) {
	srv := newTestSite(t)
	msgs := &memMessages{}
	srv.SetMessages(msgs)
	ts := newTestServerFor(t, srv)
	post := func(text, page string) int {
		resp, err := ts.Client().PostForm(ts.URL+"/message", url.Values{"text": {text}, "page": {page}})
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}
	if code := post("  the score looks low  ", "/score?a=1"); code != http.StatusNoContent {
		t.Errorf("message: status %d", code)
	}
	// Another site's address is not kept as the page.
	if code := post("hi", "https://example.com/x"); code != http.StatusNoContent {
		t.Errorf("message from elsewhere: status %d", code)
	}
	if code := post("   ", "/"); code != http.StatusBadRequest {
		t.Errorf("empty message: status %d", code)
	}
	if code := post(strings.Repeat("x", messageMaxRunes+1), "/"); code != http.StatusBadRequest {
		t.Errorf("long message: status %d", code)
	}
	if code := post("third", "/"); code != http.StatusNoContent {
		t.Errorf("third message: status %d", code)
	}
	if code := post("one too many", "/"); code != http.StatusTooManyRequests {
		t.Errorf("past the limit: status %d", code)
	}
	if len(msgs.text) != 3 || msgs.text[0] != "the score looks low" || msgs.page[0] != "/score?a=1" || msgs.page[1] != "" {
		t.Errorf("kept %q on %q", msgs.text, msgs.page)
	}
}

// Without a store there is no message form to post to.
func TestMessageOff(t *testing.T) {
	ts := newTestServerFor(t, newTestSite(t))
	resp, err := ts.Client().PostForm(ts.URL+"/message", url.Values{"text": {"hi"}})
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status %d", resp.StatusCode)
	}
}
