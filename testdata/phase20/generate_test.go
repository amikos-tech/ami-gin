package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteFixtureToDirRejectsSymlink(t *testing.T) {
	directory := t.TempDir()
	target := filepath.Join(t.TempDir(), "outside.jsonl")
	if err := os.WriteFile(target, []byte("unchanged\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "fixture.jsonl")
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}

	if err := writeFixtureToDir(directory, "fixture.jsonl", []string{"{\"ok\":true}"}); err == nil {
		t.Fatal("writeFixtureToDir() error = nil, want symlink rejection")
	}
	contents, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "unchanged\n" {
		t.Fatalf("symlink target = %q, want unchanged", contents)
	}
}
