package preview

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/felix-xqs/panbridge/internal/model"
)

type Action string

const (
	CopyToRight   Action = "copy_to_right"
	CopyToLeft    Action = "copy_to_left"
	MkdirRight    Action = "mkdir_right"
	MkdirLeft     Action = "mkdir_left"
	Review        Action = "review"
	Skip          Action = "skip"
	MetadataMatch Action = "metadata_match"
)

type Operation struct {
	Action Action `json:"action"`
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

type PlanSummary struct {
	Paths            int64 `json:"paths"`
	CopiesToRight    int64 `json:"copies_to_right"`
	CopiesToLeft     int64 `json:"copies_to_left"`
	DirectoriesRight int64 `json:"directories_right"`
	DirectoriesLeft  int64 `json:"directories_left"`
	NeedsReview      int64 `json:"needs_review"`
	Skipped          int64 `json:"skipped"`
	MetadataMatches  int64 `json:"metadata_matches"`
}

func (s PlanSummary) ProposedOperations() int64 {
	return s.Paths - s.MetadataMatches
}

// Plan is a conservative initial-merge preview. Metadata matches are not
// content proofs, and missing entries are never interpreted as deletions.
func (i *Index) Plan(ctx context.Context, visit func(Operation) error) (PlanSummary, error) {
	var summary PlanSummary
	if !i.complete[Left] || !i.complete[Right] {
		return summary, fmt.Errorf("preview: both scans must complete before planning")
	}
	if visit == nil {
		return summary, fmt.Errorf("preview: operation visitor is nil")
	}
	// Appending '/' to each path makes a parent sort before all descendants,
	// with no interleaved sibling names such as 'a-else' breaking subtree ranges.
	rows, err := i.db.QueryContext(ctx, `SELECT path,
		left_kind, left_size, left_mtime, right_kind, right_size, right_mtime
		FROM entries ORDER BY sort_key`)
	if err != nil {
		return summary, err
	}
	defer rows.Close()
	blocked := ""
	for rows.Next() {
		if err := ctx.Err(); err != nil {
			return summary, err
		}
		var path string
		var lk, ls, lt, rk, rs, rt sql.NullInt64
		if err := rows.Scan(&path, &lk, &ls, &lt, &rk, &rs, &rt); err != nil {
			return summary, err
		}
		operation, blockChildren := decide(path, lk, ls, lt, rk, rs, rt)
		if blocked != "" && strings.HasPrefix(path, blocked+"/") {
			operation = Operation{Action: Skip, Path: path, Reason: "blocked_ancestor"}
		} else {
			blocked = ""
			if blockChildren {
				blocked = path
			}
		}
		summary.Paths++
		if summary.Paths%512 == 0 {
			if err := i.budget.Wait(ctx); err != nil {
				return summary, err
			}
		}
		switch operation.Action {
		case CopyToRight:
			summary.CopiesToRight++
		case CopyToLeft:
			summary.CopiesToLeft++
		case MkdirRight:
			summary.DirectoriesRight++
		case MkdirLeft:
			summary.DirectoriesLeft++
		case Review:
			summary.NeedsReview++
		case Skip:
			summary.Skipped++
		case MetadataMatch:
			summary.MetadataMatches++
		}
		if operation.Action != MetadataMatch {
			if err := visit(operation); err != nil {
				return summary, err
			}
		}
	}
	return summary, rows.Err()
}

func decide(path string, lk, ls, lt, rk, rs, rt sql.NullInt64) (Operation, bool) {
	operation := Operation{Path: path}
	if (lk.Valid && lk.Int64 != int64(model.File) && lk.Int64 != int64(model.Directory)) ||
		(rk.Valid && rk.Int64 != int64(model.File) && rk.Int64 != int64(model.Directory)) {
		operation.Action, operation.Reason = Skip, "unsupported_entry"
		return operation, true
	}
	if lk.Valid && rk.Valid && lk.Int64 != rk.Int64 {
		operation.Action, operation.Reason = Review, "file_directory_conflict"
		return operation, true
	}
	if !rk.Valid {
		operation.Action, operation.Reason = CopyToRight, "left_only"
		if lk.Int64 == int64(model.Directory) {
			operation.Action = MkdirRight
		}
	} else if !lk.Valid {
		operation.Action, operation.Reason = CopyToLeft, "right_only"
		if rk.Int64 == int64(model.Directory) {
			operation.Action = MkdirLeft
		}
	} else if lk.Int64 == int64(model.Directory) {
		operation.Action, operation.Reason = MetadataMatch, "both_directories"
	} else if ls.Int64 == rs.Int64 && lt.Int64 == rt.Int64 {
		operation.Action, operation.Reason = MetadataMatch, "same_size_and_mtime_unverified"
	} else {
		operation.Action, operation.Reason = Review, "baseline_or_content_check_required"
	}
	return operation, false
}
