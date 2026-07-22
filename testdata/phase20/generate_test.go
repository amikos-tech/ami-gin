package main

import (
	"bytes"
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

func TestGeneratedFixturesMatchCommitted(t *testing.T) {
	wantOrder := []string{
		"nested_high_cardinality.jsonl",
		"mixed_type_arrays.jsonl",
		"number_heavy.jsonl",
		"combined.jsonl",
	}
	fixtures := generatedFixtures()
	if len(fixtures) != len(wantOrder) {
		t.Fatalf("len(generatedFixtures()) = %d, want %d", len(fixtures), len(wantOrder))
	}
	for index, fixture := range fixtures {
		if fixture.name != wantOrder[index] {
			t.Fatalf("generatedFixtures()[%d].name = %q, want %q", index, fixture.name, wantOrder[index])
		}
		got := renderFixture(fixture.records)
		want, err := os.ReadFile(fixture.name)
		if err != nil {
			t.Fatalf("read committed fixture %q: %v", fixture.name, err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("generated fixture %q differs from its committed bytes; refresh with go run ./testdata/phase20/generate.go", fixture.name)
		}
		if !bytes.HasSuffix(got, []byte("\n")) || bytes.HasSuffix(got, []byte("\n\n")) {
			t.Fatalf("generated fixture %q must end with exactly one newline", fixture.name)
		}
	}
}
