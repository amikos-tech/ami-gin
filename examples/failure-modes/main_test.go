package main

import (
	"bytes"
	"os/exec"
	"testing"
)

func TestFailureModesExampleOutput(t *testing.T) {
	cmd := exec.Command("go", "run", ".")
	cmd.Dir = "."

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("go run . error = %v\nstdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
	}
	if stderr.String() != "" {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}

	const expected = "hard: stopped after 1 indexed document: companion transformer \"domain\" on $.email failed to produce a value\n" +
		"soft: skipped 2 documents\n" +
		"soft: indexed 3 documents\n" +
		"soft: email-domain example.com row groups [0 2]\n"
	if stdout.String() != expected {
		t.Fatalf("stdout mismatch\nwant:\n%sgot:\n%s", expected, stdout.String())
	}
}
