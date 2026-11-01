package links

import (
	"context"
	"path/filepath"
	"testing"
)

func TestStore(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "links.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := s.Get(ctx, "abc"); err != nil || got != "" {
		t.Fatalf("Get unknown = %q, %v", got, err)
	}
	if got, err := s.Add(ctx, "abc", "/score?a=1"); err != nil || got != "/score?a=1" {
		t.Fatalf("Add = %q, %v", got, err)
	}
	// A taken id keeps its first target.
	if got, err := s.Add(ctx, "abc", "/score?a=2"); err != nil || got != "/score?a=1" {
		t.Fatalf("Add taken = %q, %v", got, err)
	}
	s.Close()

	// Links survive a restart.
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if got, err := s.Get(ctx, "abc"); err != nil || got != "/score?a=1" {
		t.Fatalf("Get after reopen = %q, %v", got, err)
	}
}

func TestSends(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "links.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	for _, u := range []string{"/score?a=1&r=x", "/score?a=2&r=y", "/score?a=1&r=x"} {
		if err := s.AddSend(ctx, u); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.Sends(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Errorf("Sends() = %v, want the two links once each", got)
	}
}

func TestMessages(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "links.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	for _, m := range [][2]string{{"first", "/score?a=1"}, {"second", ""}, {"second", ""}} {
		if err := s.AddMessage(ctx, m[0], m[1]); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.Messages(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// Every message is kept, the same text twice too, newest first.
	if len(got) != 3 || got[0].Text != "second" || got[2].Text != "first" || got[2].Page != "/score?a=1" {
		t.Errorf("Messages() = %v", got)
	}
}
