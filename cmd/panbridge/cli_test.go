package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestPreviewIsReadOnlyAndOutputIsBounded(t *testing.T) {
	left, right := t.TempDir(), t.TempDir()
	for _, name := range []string{"a", "b", "c"} {
		if err := os.WriteFile(filepath.Join(left, name), []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var out, errOut bytes.Buffer
	if err := run(context.Background(), []string{"dry-run", "--left", left, "--right", right, "--limit", "1", "--json"}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	lines := bytes.Split(bytes.TrimSpace(out.Bytes()), []byte("\n"))
	if len(lines) != 2 {
		t.Fatalf("expected one operation and one summary, got %d lines", len(lines))
	}
	var summary struct {
		Type    string `json:"type"`
		Shown   int64  `json:"shown"`
		Omitted int64  `json:"omitted"`
	}
	if err := json.Unmarshal(lines[1], &summary); err != nil {
		t.Fatal(err)
	}
	if summary.Type != "summary" || summary.Shown != 1 || summary.Omitted != 2 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	entries, err := os.ReadDir(right)
	if err != nil || len(entries) != 0 {
		t.Fatalf("preview mutated destination: %v, %v", entries, err)
	}
	for _, name := range []string{"a", "b", "c"} {
		data, err := os.ReadFile(filepath.Join(left, name))
		if err != nil || string(data) != name {
			t.Fatalf("preview mutated source %q: %q, %v", name, data, err)
		}
	}
}

func TestScanAndValidation(t *testing.T) {
	root := t.TempDir()
	var out, errOut bytes.Buffer
	if err := run(context.Background(), []string{"scan", "--root", root, "--json"}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if !json.Valid(bytes.TrimSpace(out.Bytes())) {
		t.Fatal("scan did not return valid JSON")
	}
	for _, args := range [][]string{
		{"scan"}, {"scan", "--root", root, "--scan-pause", "-1ms"},
		{"dry-run", "--left", root, "--right", root, "--limit", "-1"}, {"unknown"},
	} {
		if err := run(context.Background(), args, &out, &errOut); err == nil {
			t.Fatalf("invalid command accepted: %v", args)
		}
	}
}
