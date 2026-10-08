package provider

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/felix-xqs/panbridge/internal/model"
)

var ErrNotFound = errors.New("provider: entry not found")

// DirectoryLister delivers entries without retaining a whole directory in memory.
// A nil return means enumeration completed; any error makes the listing incomplete.
type DirectoryLister interface {
	ListEntries(ctx context.Context, path string, visit func(model.Entry) error) error
}

type UploadOptions struct {
	ModTime time.Time
	Hash    string
}

// Provider is the smallest contract required by the sync engine.
// Implementations must make mutating operations safe to retry.
type Provider interface {
	Name() string
	List(ctx context.Context, path string) ([]model.Entry, error)
	Stat(ctx context.Context, path string) (model.Entry, error)
	Open(ctx context.Context, path string) (io.ReadCloser, error)
	Put(ctx context.Context, path string, size int64, src io.Reader, opts UploadOptions) (model.Entry, error)
	Mkdir(ctx context.Context, path string) error
	Delete(ctx context.Context, path string) error
	Move(ctx context.Context, from, to string) error
}
