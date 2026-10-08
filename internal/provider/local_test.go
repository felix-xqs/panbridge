package provider

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/felix-xqs/panbridge/internal/model"
)

func TestLocalProviderOperations(t *testing.T) {
	root := t.TempDir()
	p, err := NewLocalProvider(root)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	if err := p.Mkdir(ctx, "notes"); err != nil {
		t.Fatal(err)
	}
	entry, err := p.Put(ctx, "notes/today.txt", int64(len("hello")), strings.NewReader("hello"), UploadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if entry.Path != "notes/today.txt" || entry.Size != 5 || !entry.IsFile() {
		t.Fatalf("unexpected entry: %+v", entry)
	}

	opened, err := p.Open(ctx, "notes/today.txt")
	if err != nil {
		t.Fatal(err)
	}
	content, err := io.ReadAll(opened)
	if closeErr := opened.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "hello" {
		t.Fatalf("unexpected content: %q", content)
	}
	if _, err := p.Put(ctx, "notes/today.txt", int64(len("updated")), strings.NewReader("updated"), UploadOptions{}); err != nil {
		t.Fatal(err)
	}
	updated, err := os.ReadFile(filepath.Join(root, "notes", "today.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(updated) != "updated" {
		t.Fatalf("unexpected replacement content: %q", updated)
	}

	if err := p.Move(ctx, "notes/today.txt", "archive/today.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Stat(ctx, "notes/today.txt"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	if err := p.Delete(ctx, "archive/today.txt"); err != nil {
		t.Fatal(err)
	}
}

func TestLocalProviderRejectsEscapingPaths(t *testing.T) {
	p, err := NewLocalProvider(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"../outside", "/absolute", "a/../../outside"} {
		if _, err := p.Stat(context.Background(), path); err == nil {
			t.Fatalf("expected path %q to be rejected", path)
		}
	}
}

func TestLocalProviderProtectsRoot(t *testing.T) {
	p, err := NewLocalProvider(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := p.Delete(ctx, "."); err == nil {
		t.Fatal("expected root delete to be rejected")
	}
	if err := p.Move(ctx, ".", "backup"); err == nil {
		t.Fatal("expected root move to be rejected")
	}
}

func TestReplaceFilePreservesDestinationOnFailure(t *testing.T) {
	root := t.TempDir()
	destination := filepath.Join(root, "existing.txt")
	if err := os.WriteFile(destination, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := replaceFile(filepath.Join(root, "missing.txt"), destination); err == nil {
		t.Fatal("expected replacement with a missing source to fail")
	}
	content, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "original" {
		t.Fatalf("failed replacement changed the destination: %q", content)
	}
}

func TestLocalProviderStreamingAcrossBatchesAndEarlyStop(t *testing.T) {
	root := t.TempDir()
	for n := 0; n < 600; n++ {
		if err := os.WriteFile(filepath.Join(root, fmt.Sprintf("file-%04d", n)), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	p, err := NewLocalProvider(root)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	if err := p.ListEntries(context.Background(), "", func(model.Entry) error { count++; return nil }); err != nil || count != 600 {
		t.Fatalf("incomplete streaming listing: %d, %v", count, err)
	}
	stop := errors.New("stop at entry 300")
	count = 0
	err = p.ListEntries(context.Background(), "", func(model.Entry) error {
		count++
		if count == 300 {
			return stop
		}
		return nil
	})
	if !errors.Is(err, stop) || count != 300 {
		t.Fatalf("listing did not stop at visitor error: %d, %v", count, err)
	}
}

func TestLocalProviderListingIdentifiesDirectoryLinks(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "linked")); err != nil {
		t.Skipf("creating links is unavailable: %v", err)
	}
	p, err := NewLocalProvider(root)
	if err != nil {
		t.Fatal(err)
	}
	var entry model.Entry
	if err := p.ListEntries(context.Background(), "", func(e model.Entry) error { entry = e; return nil }); err != nil {
		t.Fatal(err)
	}
	if entry.Type != model.Symlink || entry.IsDirectory() {
		t.Fatalf("directory link must not be traversable: %+v", entry)
	}
}
