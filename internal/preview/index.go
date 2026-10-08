package preview

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/felix-xqs/panbridge/internal/limits"
	"github.com/felix-xqs/panbridge/internal/model"
	"github.com/felix-xqs/panbridge/internal/provider"
	"github.com/felix-xqs/panbridge/internal/scan"
	_ "modernc.org/sqlite"
)

type Side int

const (
	Left Side = iota
	Right
)

// Index is an ephemeral current-tree index, not a historical sync baseline.
type Index struct {
	db        *sql.DB
	dir       string
	attempted [2]bool
	complete  [2]bool
	budget    *limits.Budget
}

func NewIndex(ctx context.Context) (*Index, error) {
	dir, err := os.MkdirTemp("", ".panbridge-index-*")
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", filepath.Join(dir, "index.db"))
	if err != nil {
		os.RemoveAll(dir)
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	index := &Index{db: db, dir: dir}
	_, err = db.ExecContext(ctx, `
		PRAGMA journal_mode=WAL;
		PRAGMA synchronous=NORMAL;
		PRAGMA cache_size=-8192;
		PRAGMA temp_store=FILE;
		PRAGMA mmap_size=0;
		CREATE TABLE entries (
			path TEXT PRIMARY KEY,
			sort_key TEXT NOT NULL,
			left_kind INTEGER, left_size INTEGER, left_mtime INTEGER,
			right_kind INTEGER, right_size INTEGER, right_mtime INTEGER
		);
		CREATE INDEX entries_order ON entries(sort_key);
	`)
	if err != nil {
		index.Close()
		return nil, fmt.Errorf("preview: initialize index: %w", err)
	}
	return index, nil
}

func (i *Index) Close() error {
	err := i.db.Close()
	return errors.Join(err, os.RemoveAll(i.dir))
}

// Scan permits one attempt per side; incomplete scans cannot be planned.
func (i *Index) Scan(ctx context.Context, side Side, source provider.DirectoryLister, opts scan.Options) (scan.Summary, error) {
	if side != Left && side != Right {
		return scan.Summary{}, fmt.Errorf("preview: invalid side")
	}
	if i.attempted[side] {
		return scan.Summary{}, fmt.Errorf("preview: side already scanned; create a new index")
	}
	i.attempted[side] = true
	i.budget = opts.Budget
	query := `INSERT INTO entries(path, sort_key, left_kind, left_size, left_mtime)
		VALUES(?, ?, ?, ?, ?) ON CONFLICT(path) DO UPDATE SET
		left_kind=excluded.left_kind, left_size=excluded.left_size, left_mtime=excluded.left_mtime
		WHERE entries.left_kind IS NULL`
	if side == Right {
		query = `INSERT INTO entries(path, sort_key, right_kind, right_size, right_mtime)
			VALUES(?, ?, ?, ?, ?) ON CONFLICT(path) DO UPDATE SET
			right_kind=excluded.right_kind, right_size=excluded.right_size, right_mtime=excluded.right_mtime
			WHERE entries.right_kind IS NULL`
	}
	var tx *sql.Tx
	var statement *sql.Stmt
	begin := func() error {
		var err error
		tx, err = i.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		statement, err = tx.PrepareContext(ctx, query)
		return err
	}
	defer func() {
		if statement != nil {
			statement.Close()
		}
		if tx != nil {
			tx.Rollback()
		}
	}()
	if err := begin(); err != nil {
		return scan.Summary{}, err
	}
	count := 0
	summary, err := scan.Tree(ctx, source, opts, func(entry model.Entry) error {
		result, err := statement.ExecContext(ctx, entry.Path, entry.Path+"/", entry.Type, entry.Size, entry.ModTime.UnixNano())
		if err != nil {
			return err
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if affected != 1 {
			return fmt.Errorf("preview: duplicate entry %q makes the scan unreliable", entry.Path)
		}
		count++
		if count%512 == 0 {
			if err := statement.Close(); err != nil {
				return err
			}
			statement = nil
			if err := tx.Commit(); err != nil {
				return err
			}
			tx = nil
			return begin()
		}
		return nil
	})
	if err != nil {
		return summary, fmt.Errorf("preview: scan side %d: %w", side, err)
	}
	if err := statement.Close(); err != nil {
		return summary, err
	}
	statement = nil
	if err := tx.Commit(); err != nil {
		return summary, err
	}
	tx = nil
	i.complete[side] = true
	return summary, nil
}
