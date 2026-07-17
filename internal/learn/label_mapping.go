package learn

import (
	"fmt"
	"sort"
)

// LabelMappingMission infers `label:<name> → app` mappings by observing which
// group a label's issues predominantly land in. A label that consistently
// appears within one group is a strong signal that it identifies that app.
type LabelMappingMission struct{}

func (LabelMappingMission) Name() string { return "label-mapping" }

func (LabelMappingMission) Run(in Input) Artifact {
	// issueID → group label, ignoring unclassified groups (no signal there).
	issueGroup := map[string]string{}
	for _, g := range in.Groups {
		if g.Kind == "unclassified" {
			continue
		}
		for _, id := range g.IssueIDs {
			issueGroup[id] = g.Label
		}
	}

	// Per label, tally the group labels of the issues that bear it.
	type tally struct {
		total  int
		byGrp  map[string]int
	}
	labelTallies := map[string]*tally{}
	for _, is := range in.Activity.Issues {
		grp, ok := issueGroup[is.ID]
		if !ok {
			continue
		}
		for _, lbl := range is.Labels {
			t := labelTallies[lbl]
			if t == nil {
				t = &tally{byGrp: map[string]int{}}
				labelTallies[lbl] = t
			}
			t.total++
			t.byGrp[grp]++
		}
	}

	// Deterministic ordering for stable output.
	labels := make([]string, 0, len(labelTallies))
	for lbl := range labelTallies {
		labels = append(labels, lbl)
	}
	sort.Strings(labels)

	art := Artifact{Mission: "label-mapping"}
	for _, lbl := range labels {
		key := "label:" + lbl
		if _, known := in.Existing.AppMappings[key]; known {
			continue // don't re-propose an already-learned mapping
		}
		t := labelTallies[lbl]
		topGrp, topCount := "", 0
		for grp, c := range t.byGrp {
			if c > topCount || (c == topCount && grp < topGrp) {
				topGrp, topCount = grp, c
			}
		}
		if topGrp == "" {
			continue
		}
		purity := float64(topCount) / float64(t.total)
		art.Candidates = append(art.Candidates, Candidate{
			Key: key, App: topGrp, Support: topCount, Purity: purity,
		})
		art.Rationale = append(art.Rationale,
			fmt.Sprintf("%q appears in %q %d/%d times (purity %.2f)", lbl, topGrp, topCount, t.total, purity))
	}
	return art
}

// PurityEvaluator accepts candidates whose support and purity clear both bars.
type PurityEvaluator struct {
	MinSupport int
	MinPurity  float64
}

func (e PurityEvaluator) Evaluate(a Artifact) ([]Candidate, string) {
	var accepted []Candidate
	for _, c := range a.Candidates {
		if c.Support >= e.MinSupport && c.Purity >= e.MinPurity {
			accepted = append(accepted, c)
		}
	}
	return accepted, fmt.Sprintf("accepted %d/%d candidates (min support %d, min purity %.2f)",
		len(accepted), len(a.Candidates), e.MinSupport, e.MinPurity)
}

// compile-time interface checks
var (
	_ Mission   = LabelMappingMission{}
	_ Evaluator = PurityEvaluator{}
)
