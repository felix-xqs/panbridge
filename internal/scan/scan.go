package scan

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/felix-xqs/panbridge/internal/limits"
	"github.com/felix-xqs/panbridge/internal/model"
	"github.com/felix-xqs/panbridge/internal/provider"
)

type Options struct {
	MaxDepth   int
	YieldEvery int
	YieldDelay time.Duration
	Budget     *limits.Budget
}

func DefaultOptions() Options {
	return Options{MaxDepth: 128, YieldEvery: 512, YieldDelay: 5 * time.Millisecond}
}

type Summary struct {
	Entries     int64 `json:"entries"`
	Files       int64 `json:"files"`
	Directories int64 `json:"directories"`
	Unsupported int64 `json:"unsupported"`
	Bytes       int64 `json:"file_bytes"`
}

// Tree visits metadata serially, with memory and open directories bounded by
// batch size and MaxDepth. It never reads file contents or follows link entries.
func Tree(ctx context.Context, source provider.DirectoryLister, opts Options, visit func(model.Entry) error) (Summary, error) {
	var summary Summary
	if opts.MaxDepth <= 0 || opts.MaxDepth > 128 || opts.YieldEvery <= 0 || opts.YieldDelay < 0 {
		return summary, fmt.Errorf("scan: invalid resource limits")
	}
	if source == nil || visit == nil {
		return summary, fmt.Errorf("scan: source and visitor are required")
	}
	var walk func(string, int) error
	walk = func(path string, depth int) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if depth > opts.MaxDepth {
			return fmt.Errorf("scan: maximum directory depth exceeded at %q", path)
		}
		return source.ListEntries(ctx, path, func(entry model.Entry) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			if !validChild(path, entry.Path) {
				return fmt.Errorf("scan: provider returned invalid child %q for %q", entry.Path, path)
			}
			name := entry.Path[strings.LastIndex(entry.Path, "/")+1:]
			if name == ".panbridge" || strings.HasPrefix(name, ".panbridge-upload-") || strings.HasPrefix(name, ".panbridge-index-") {
				return nil
			}
			if err := visit(entry); err != nil {
				return err
			}
			summary.Entries++
			switch entry.Type {
			case model.File:
				summary.Files++
				summary.Bytes += entry.Size
			case model.Directory:
				summary.Directories++
			default:
				summary.Unsupported++
			}
			if summary.Entries%int64(opts.YieldEvery) == 0 && opts.YieldDelay > 0 {
				timer := time.NewTimer(opts.YieldDelay)
				select {
				case <-ctx.Done():
					timer.Stop()
					return ctx.Err()
				case <-timer.C:
				}
			}
			if summary.Entries%int64(opts.YieldEvery) == 0 {
				if err := opts.Budget.Wait(ctx); err != nil {
					return err
				}
			}
			if entry.IsDirectory() {
				return walk(entry.Path, depth+1)
			}
			return nil
		})
	}
	err := walk("", 0)
	if err == nil {
		err = ctx.Err()
	}
	return summary, err
}

func validChild(parent, child string) bool {
	prefix := ""
	if parent != "" {
		prefix = parent + "/"
	}
	if !strings.HasPrefix(child, prefix) {
		return false
	}
	name := strings.TrimPrefix(child, prefix)
	return name != "" && name != "." && name != ".." && !strings.Contains(name, "/")
}
