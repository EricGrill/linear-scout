package ai

import (
	"github.com/EricGrill/linear-scout/internal/config"
	"github.com/EricGrill/linear-scout/internal/model"
)

// Validate enforces the rubric: minimum confidence and (optionally) evidence.
func Validate(recs []model.Recommendation, rubric config.RubricConfig) []model.Recommendation {
	out := make([]model.Recommendation, 0, len(recs))
	for _, r := range recs {
		if float64(r.Confidence) < rubric.MinConfidence {
			continue
		}
		if rubric.RequireEvidence && len(r.Evidence) == 0 {
			continue
		}
		out = append(out, r)
	}
	return out
}
