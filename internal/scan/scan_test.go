package scan

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/felix-xqs/panbridge/internal/model"
)

type tree map[string][]model.Entry

func (t tree) ListEntries(ctx context.Context, path string, visit func(model.Entry) error) error {
	for _, entry := range t[path] {
		if err := visit(entry); err != nil {
			return err
		}
	}
	return nil
}

func TestTreeSkipsLinksAndInternalFiles(t *testing.T) {
	source := tree{
		"":           {{Path: "docs", Type: model.Directory}, {Path: "outside", Type: model.Symlink}, {Path: ".panbridge", Type: model.Directory}, {Path: ".panbridge-upload-inflight", Type: model.File}},
		"docs":       {{Path: "docs/readme.txt", Type: model.File, Size: 12}},
		"outside":    {{Path: "outside/secret", Type: model.File}},
		".panbridge": {{Path: ".panbridge/state.db", Type: model.File}},
	}
	var paths []string
	summary, err := Tree(context.Background(), source, DefaultOptions(), func(entry model.Entry) error {
		paths = append(paths, entry.Path)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if summary.Entries != 3 || summary.Files != 1 || summary.Directories != 1 || summary.Unsupported != 1 || summary.Bytes != 12 || len(paths) != 3 {
		t.Fatalf("unexpected scan: %+v, %v", summary, paths)
	}
}

func TestTreeCancellationInterruptsThrottle(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	opts := DefaultOptions()
	opts.YieldEvery, opts.YieldDelay = 1, time.Hour
	_, err := Tree(ctx, tree{"": {{Path: "file", Type: model.File}}}, opts, func(model.Entry) error {
		cancel()
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
}

func TestTreeRejectsMalformedPathsAndExcessiveDepth(t *testing.T) {
	for _, source := range []tree{
		{"": {{Path: "../escape", Type: model.File}}},
		{"": {{Path: "a", Type: model.Directory}}, "a": {{Path: "a/b", Type: model.Directory}}, "a/b": {{Path: "a/b/c", Type: model.Directory}}},
	} {
		opts := DefaultOptions()
		opts.MaxDepth = 2
		if _, err := Tree(context.Background(), source, opts, func(model.Entry) error { return nil }); err == nil {
			t.Fatal("expected invalid tree to fail")
		}
	}
}
