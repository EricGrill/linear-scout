// Package ingest turns a time window into a normalized activity dataset.
package ingest

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/EricGrill/linear-scout/internal/model"
)

type Source interface {
	Issues(ctx context.Context, since time.Time) ([]model.Issue, error)
	Comments(ctx context.Context, since time.Time) ([]model.Comment, error)
}

// ParseWindow accepts forms like "7d", "24h", "2w".
func ParseWindow(s string, now time.Time) (time.Time, error) {
	if len(s) < 2 {
		return time.Time{}, fmt.Errorf("invalid window %q", s)
	}
	unit := s[len(s)-1]
	n, err := strconv.Atoi(s[:len(s)-1])
	if err != nil || n <= 0 {
		return time.Time{}, fmt.Errorf("invalid window %q", s)
	}
	switch unit {
	case 'h':
		return now.Add(-time.Duration(n) * time.Hour), nil
	case 'd':
		return now.AddDate(0, 0, -n), nil
	case 'w':
		return now.AddDate(0, 0, -7*n), nil
	default:
		return time.Time{}, fmt.Errorf("invalid window unit in %q", s)
	}
}

// Fetch pulls issues and comments for the window and normalizes them.
func Fetch(ctx context.Context, src Source, window string, now time.Time) (model.Activity, error) {
	since, err := ParseWindow(window, now)
	if err != nil {
		return model.Activity{}, err
	}
	issues, err := src.Issues(ctx, since)
	if err != nil {
		return model.Activity{}, fmt.Errorf("fetch issues: %w", err)
	}
	comments, err := src.Comments(ctx, since)
	if err != nil {
		return model.Activity{}, fmt.Errorf("fetch comments: %w", err)
	}
	return model.Activity{
		Since: since, Until: now, Issues: issues, Comments: comments,
		Projects: map[string]string{}, Teams: map[string]string{},
	}, nil
}
