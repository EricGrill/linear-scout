// Package drafts turns recommendations into reviewable Linear issue metadata.
// It NEVER writes to Linear; it only produces draft structs.
package drafts

import "github.com/EricGrill/linear-scout/internal/model"

type Draft struct {
	Title    string
	Body     string
	Labels   []string
	Priority int
	Evidence []model.EvidenceLink
}

func FromReport(r model.Report) []Draft {
	out := make([]Draft, 0, len(r.Recommendations))
	for _, rec := range r.Recommendations {
		title := rec.DraftTitle
		if title == "" {
			title = rec.Summary
		}
		out = append(out, Draft{
			Title: title, Body: rec.DraftBody, Labels: rec.SuggestedLabels,
			Priority: rec.SuggestedPriority, Evidence: rec.Evidence,
		})
	}
	return out
}
