// Package engine orchestrates the read-only pipeline and returns a Report.
// Every delivery surface calls Run; none duplicate this logic.
package engine

import (
	"context"
	"time"

	"github.com/EricGrill/linear-scout/internal/ai"
	"github.com/EricGrill/linear-scout/internal/config"
	"github.com/EricGrill/linear-scout/internal/evidence"
	"github.com/EricGrill/linear-scout/internal/grouping"
	"github.com/EricGrill/linear-scout/internal/ingest"
	"github.com/EricGrill/linear-scout/internal/model"
)

type Options struct {
	Window   string
	GroupBy  string
	Limit    int
	Now      time.Time
	Mappings map[string]string
	Rubric   config.RubricConfig
}

func Run(ctx context.Context, src ingest.Source, prov ai.Provider, opts Options) (model.Report, error) {
	act, err := ingest.Fetch(ctx, src, opts.Window, opts.Now)
	if err != nil {
		return model.Report{}, err
	}
	groups, unclassified := grouping.Classify(act, opts.Mappings, opts.GroupBy)
	bundles := evidence.Build(act, groups)

	recs, err := prov.Recommend(ctx, ai.Request{Groups: groups, Evidence: bundles})
	if err != nil {
		return model.Report{}, err
	}
	recs = ai.Validate(recs, opts.Rubric)
	if opts.Limit > 0 && len(recs) > opts.Limit {
		recs = recs[:opts.Limit]
	}
	return model.Report{
		GeneratedAt: opts.Now, Window: opts.Window, GroupBy: opts.GroupBy,
		Groups: groups, Recommendations: recs, Unclassified: unclassified,
	}, nil
}
