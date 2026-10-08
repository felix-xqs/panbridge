package provider

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/felix-xqs/panbridge/internal/model"
)

// LocalProvider exposes a directory using paths relative to Root.
type LocalProvider struct {
	root string
}

func NewLocalProvider(root string) (*LocalProvider, error) {
	if strings.TrimSpace(root) == "" {
		return nil, fmt.Errorf("local provider: root is empty")
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("local provider: resolve root: %w", err)
	}
	absRoot, err = filepath.EvalSymlinks(absRoot)
	if err != nil {
		return nil, fmt.Errorf("local provider: resolve root links: %w", err)
	}
	info, err := os.Stat(absRoot)
	if err != nil {
		return nil, fmt.Errorf("local provider: stat root: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("local provider: root is not a directory")
	}
	return &LocalProvider{root: absRoot}, nil
}

func (p *LocalProvider) Name() string {
	return "local"
}

func (p *LocalProvider) List(ctx context.Context, path string) ([]model.Entry, error) {
	var result []model.Entry
	if err := p.ListEntries(ctx, path, func(entry model.Entry) error {
		result = append(result, entry)
		return nil
	}); err != nil {
		return nil, err
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Path < result[j].Path })
	return result, nil
}

// ListEntries reads at most 256 directory entries at a time in filesystem order.
func (p *LocalProvider) ListEntries(ctx context.Context, path string, visit func(model.Entry) error) error {
	if visit == nil {
		return fmt.Errorf("local provider: entry visitor is nil")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	dir, err := p.resolve(path)
	if err != nil {
		return err
	}
	file, err := os.Open(dir)
	if err != nil {
		return mapPathError(err)
	}
	defer file.Close()
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		entries, readErr := file.ReadDir(256)
		if readErr != nil && readErr != io.EOF {
			return mapPathError(readErr)
		}
		for _, entry := range entries {
			if err := ctx.Err(); err != nil {
				return err
			}
			info, err := entry.Info()
			if err != nil {
				return fmt.Errorf("local provider: inspect %q: %w", p.joinPath(path, entry.Name()), err)
			}
			if err := visit(toModelEntry(p.joinPath(path, entry.Name()), info)); err != nil {
				return err
			}
		}
		if readErr == io.EOF {
			return ctx.Err()
		}
	}
}

func (p *LocalProvider) Stat(ctx context.Context, path string) (model.Entry, error) {
	if err := ctx.Err(); err != nil {
		return model.Entry{}, err
	}
	fullPath, err := p.resolve(path)
	if err != nil {
		return model.Entry{}, err
	}
	info, err := os.Stat(fullPath)
	if err != nil {
		return model.Entry{}, mapPathError(err)
	}
	return toModelEntry(path, info), nil
}

func (p *LocalProvider) Open(ctx context.Context, path string) (io.ReadCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	fullPath, err := p.resolve(path)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(fullPath)
	if err != nil {
		return nil, mapPathError(err)
	}
	return file, nil
}

func (p *LocalProvider) Put(ctx context.Context, path string, size int64, src io.Reader, opts UploadOptions) (model.Entry, error) {
	if err := ctx.Err(); err != nil {
		return model.Entry{}, err
	}
	fullPath, err := p.resolveFile(path)
	if err != nil {
		return model.Entry{}, err
	}
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return model.Entry{}, fmt.Errorf("local provider: create parent: %w", err)
	}

	temp, err := os.CreateTemp(filepath.Dir(fullPath), ".panbridge-upload-*")
	if err != nil {
		return model.Entry{}, fmt.Errorf("local provider: create temporary file: %w", err)
	}
	tempName := temp.Name()
	defer os.Remove(tempName)

	if size >= 0 {
		src = io.LimitReader(src, size+1)
	}
	written, copyErr := io.Copy(temp, src)
	if copyErr != nil {
		temp.Close()
		return model.Entry{}, fmt.Errorf("local provider: write file: %w", copyErr)
	}
	if size >= 0 && written != size {
		temp.Close()
		return model.Entry{}, fmt.Errorf("local provider: size mismatch: expected %d, wrote %d", size, written)
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return model.Entry{}, fmt.Errorf("local provider: sync file: %w", err)
	}
	if err := temp.Close(); err != nil {
		return model.Entry{}, fmt.Errorf("local provider: close file: %w", err)
	}
	if !opts.ModTime.IsZero() {
		if err := os.Chtimes(tempName, opts.ModTime, opts.ModTime); err != nil {
			return model.Entry{}, fmt.Errorf("local provider: set modification time: %w", err)
		}
	}
	if err := replaceFile(tempName, fullPath); err != nil {
		return model.Entry{}, fmt.Errorf("local provider: replace file: %w", err)
	}
	return p.Stat(ctx, path)
}

func (p *LocalProvider) Mkdir(ctx context.Context, path string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	fullPath, err := p.resolveFile(path)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(fullPath, 0o755); err != nil {
		return fmt.Errorf("local provider: create directory: %w", err)
	}
	return nil
}

func (p *LocalProvider) Delete(ctx context.Context, path string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	fullPath, err := p.resolveFile(path)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(fullPath); err != nil {
		return mapPathError(err)
	}
	return nil
}

func (p *LocalProvider) Move(ctx context.Context, from, to string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	source, err := p.resolveFile(from)
	if err != nil {
		return err
	}
	destination, err := p.resolveFile(to)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return fmt.Errorf("local provider: create destination parent: %w", err)
	}
	if err := os.Rename(source, destination); err != nil {
		return mapPathError(err)
	}
	return nil
}

func (p *LocalProvider) resolve(path string) (string, error) {
	cleanPath := filepath.Clean(filepath.FromSlash(path))
	if path == "" || cleanPath == "." {
		return p.root, nil
	}
	if filepath.IsAbs(cleanPath) || cleanPath == ".." || strings.HasPrefix(cleanPath, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("local provider: path escapes root: %q", path)
	}
	return filepath.Join(p.root, cleanPath), nil
}

func (p *LocalProvider) resolveFile(path string) (string, error) {
	if path == "" || filepath.Clean(filepath.FromSlash(path)) == "." {
		return "", fmt.Errorf("local provider: path refers to provider root")
	}
	return p.resolve(path)
}

func (p *LocalProvider) joinPath(parent, name string) string {
	if parent == "" {
		return filepath.ToSlash(name)
	}
	return filepath.ToSlash(filepath.Join(parent, name))
}

func toModelEntry(path string, info fs.FileInfo) model.Entry {
	typeOfEntry := model.Other
	if info.Mode().IsRegular() {
		typeOfEntry = model.File
	} else if info.Mode()&os.ModeSymlink != 0 {
		typeOfEntry = model.Symlink
	} else if info.IsDir() {
		typeOfEntry = model.Directory
	}
	return model.Entry{
		Path:    filepath.ToSlash(path),
		Type:    typeOfEntry,
		Size:    info.Size(),
		ModTime: info.ModTime(),
	}
}

func mapPathError(err error) error {
	if os.IsNotExist(err) {
		return ErrNotFound
	}
	return err
}
