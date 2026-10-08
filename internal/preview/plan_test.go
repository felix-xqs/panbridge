package preview

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/felix-xqs/panbridge/internal/model"
	"github.com/felix-xqs/panbridge/internal/scan"
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

func file(path string, size int64) model.Entry {
	return model.Entry{Path: path, Type: model.File, Size: size, ModTime: time.Unix(1700000000, 0)}
}

func TestInitialMergeAndBlockedSubtrees(t *testing.T) {
	ctx := context.Background()
	index, err := NewIndex(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer index.Close()
	left := tree{
		"": {file("same", 10), file("different", 10), file("left-only", 3), file("a", 1), file("a-sibling", 1), {Path: "link", Type: model.Symlink}},
	}
	right := tree{
		"":       {file("same", 10), file("different", 11), file("right-only", 5), {Path: "a", Type: model.Directory}, {Path: "link", Type: model.Directory}},
		"a":      {{Path: "a/deep", Type: model.Directory}},
		"a/deep": {file("a/deep/child", 8)},
		"link":   {file("link/child", 4)},
	}
	for side, source := range map[Side]tree{Left: left, Right: right} {
		if _, err := index.Scan(ctx, side, source, scan.DefaultOptions()); err != nil {
			t.Fatal(err)
		}
	}
	operations := map[string]Operation{}
	summary, err := index.Plan(ctx, func(op Operation) error { operations[op.Path] = op; return nil })
	if err != nil {
		t.Fatal(err)
	}
	checks := map[string]Action{
		"left-only": CopyToRight, "right-only": CopyToLeft, "different": Review,
		"a": Review, "a-sibling": CopyToRight, "a/deep": Skip, "a/deep/child": Skip,
		"link": Skip, "link/child": Skip,
	}
	for path, expected := range checks {
		if operations[path].Action != expected {
			t.Errorf("%q: wanted %s, got %+v", path, expected, operations[path])
		}
	}
	if _, exists := operations["same"]; exists || summary.MetadataMatches != 1 || summary.NeedsReview != 2 {
		t.Fatalf("unexpected metadata matching or conflicts: %+v", summary)
	}
	if operations["a/deep/child"].Reason != "blocked_ancestor" {
		t.Fatal("children of a type conflict must be blocked")
	}
}

type incomplete struct{}

func (incomplete) ListEntries(ctx context.Context, path string, visit func(model.Entry) error) error {
	for n := 0; n < 600; n++ {
		if err := visit(file(fmt.Sprintf("file-%04d", n), 1)); err != nil {
			return err
		}
	}
	return errors.New("connection lost after a committed batch")
}

func TestPartialScanCannotGeneratePlan(t *testing.T) {
	ctx := context.Background()
	index, err := NewIndex(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer index.Close()
	if _, err := index.Scan(ctx, Left, incomplete{}, scan.DefaultOptions()); err == nil {
		t.Fatal("expected incomplete scan to fail")
	}
	if _, err := index.Scan(ctx, Right, tree{}, scan.DefaultOptions()); err != nil {
		t.Fatal(err)
	}
	if _, err := index.Plan(ctx, func(Operation) error { t.Fatal("partial scan produced an operation"); return nil }); err == nil {
		t.Fatal("expected planning to be refused")
	}
}

func TestMetadataMatchDoesNotClaimContentEquality(t *testing.T) {
	entry := file("same-metadata", 12)
	ctx := context.Background()
	index, err := NewIndex(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer index.Close()
	for _, side := range []Side{Left, Right} {
		if _, err := index.Scan(ctx, side, tree{"": {entry}}, scan.DefaultOptions()); err != nil {
			t.Fatal(err)
		}
	}
	summary, err := index.Plan(ctx, func(Operation) error { t.Fatal("metadata match should not schedule copying"); return nil })
	if err != nil || summary.MetadataMatches != 1 {
		t.Fatalf("unexpected plan: %+v, %v", summary, err)
	}
}

func TestDuplicateProviderEntriesInvalidateScan(t *testing.T) {
	ctx := context.Background()
	index, err := NewIndex(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer index.Close()
	if _, err := index.Scan(ctx, Left, tree{"": {file("duplicate", 1), file("duplicate", 2)}}, scan.DefaultOptions()); err == nil {
		t.Fatal("expected duplicate listing entries to fail")
	}
}
