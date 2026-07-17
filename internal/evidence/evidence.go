// Package evidence builds ranked, auditable evidence bundles per group.
// It sits BELOW the AI layer so every recommendation can be traced to source.
package evidence

import "github.com/EricGrill/linear-scout/internal/model"

func Build(act model.Activity, groups []model.Group) map[string]model.EvidenceBundle {
	issueByID := map[string]model.Issue{}
	for _, is := range act.Issues {
		issueByID[is.ID] = is
	}
	commentsByIssue := map[string][]model.Comment{}
	for _, c := range act.Comments {
		commentsByIssue[c.IssueID] = append(commentsByIssue[c.IssueID], c)
	}

	out := map[string]model.EvidenceBundle{}
	for _, g := range groups {
		b := model.EvidenceBundle{GroupKey: g.Key}
		for _, id := range g.IssueIDs {
			is, ok := issueByID[id]
			if !ok {
				continue
			}
			b.Links = append(b.Links, model.EvidenceLink{
				Kind: "issue", Ref: is.Identifier, URL: is.URL, Quote: is.Title,
			})
			for _, c := range commentsByIssue[id] {
				b.Links = append(b.Links, model.EvidenceLink{
					Kind: "comment", Ref: is.Identifier, URL: c.URL, Quote: excerpt(c.Body),
				})
			}
		}
		b.Score = float64(len(b.Links))
		out[g.Key] = b
	}
	return out
}

func excerpt(s string) string {
	const max = 140
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
