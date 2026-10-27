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
