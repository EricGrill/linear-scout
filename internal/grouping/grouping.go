// Package grouping infers app/product/project/team buckets with confidence.
package grouping

import "github.com/EricGrill/linear-scout/internal/model"

// Classify buckets issues, returning groups and the count of unclassified issues.
func Classify(act model.Activity, mappings map[string]string, groupBy string) ([]model.Group, int) {
	byKey := map[string]*model.Group{}
	order := []string{}
	unclassified := 0

	ensure := func(key, label, kind string, conf model.Confidence) *model.Group {
		g, ok := byKey[key]
		if !ok {
			g = &model.Group{Key: key, Label: label, Kind: kind, Confidence: conf}
			byKey[key] = g
			order = append(order, key)
		}
		return g
	}

	for _, is := range act.Issues {
		if app, key := matchMapping(is.Labels, mappings); key != "" {
			g := ensure("app:"+app, app, "app", 0.9)
			g.IssueIDs = append(g.IssueIDs, is.ID)
			continue
		}
		var metaKey string
		switch groupBy {
		case "team":
			metaKey = is.TeamID
		default:
			metaKey = is.ProjectID
		}
		if metaKey == "" {
			g := ensure("unclassified", "Unclassified", "unclassified", 0.3)
			g.IssueIDs = append(g.IssueIDs, is.ID)
			unclassified++
			continue
		}
		g := ensure(groupBy+":"+metaKey, metaKey, groupBy, 0.6)
		g.IssueIDs = append(g.IssueIDs, is.ID)
	}

	out := make([]model.Group, 0, len(order))
	for _, k := range order {
		out = append(out, *byKey[k])
	}
	return out, unclassified
}

func matchMapping(labels []string, mappings map[string]string) (app, key string) {
	for _, l := range labels {
		k := "label:" + l
		if app, ok := mappings[k]; ok {
			return app, k
		}
	}
	return "", ""
}
