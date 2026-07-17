// Package learn implements artifact-gated learning (SRS §6.5): each pass has a
// mission that proposes candidate profile updates, and an evaluator that gates
// which candidates are accepted. Passes are pure — applying accepted candidates
// to the user profile is the caller's explicit, separate step.
package learn

import (
	"github.com/EricGrill/linear-scout/internal/model"
	"github.com/EricGrill/linear-scout/internal/store"
)

// Candidate is one proposed profile update: a mapping key → app, with the
// evidence strength behind it.
type Candidate struct {
	Key     string  // e.g. "label:bug"
	App     string  // proposed app/product label
	Support int     // number of supporting issues
	Purity  float64 // fraction of supporting issues that agree (0..1)
}

// Artifact is the output of a mission: the candidates it proposes plus rationale.
type Artifact struct {
	Mission    string
	Candidates []Candidate
	Rationale  []string
}

// Input is the read-only context a mission learns from.
type Input struct {
	Activity model.Activity
	Groups   []model.Group
	Existing store.LearnedProfile
}

// Mission produces a candidate Artifact from Input. It must not mutate state.
type Mission interface {
	Name() string
	Run(in Input) Artifact
}

// Evaluator gates an artifact's candidates, returning the accepted subset and a
// human-readable reason for the decision.
type Evaluator interface {
	Evaluate(a Artifact) (accepted []Candidate, reason string)
}

// RunPass runs a mission then gates it with an evaluator. It has no side effects;
// it returns the full artifact, the accepted candidates, and the gate reason.
func RunPass(m Mission, e Evaluator, in Input) (Artifact, []Candidate, string) {
	a := m.Run(in)
	accepted, reason := e.Evaluate(a)
	return a, accepted, reason
}

// Mappings turns accepted candidates into a mappings delta suitable for
// store.MergeMappings.
func Mappings(accepted []Candidate) map[string]string {
	out := make(map[string]string, len(accepted))
	for _, c := range accepted {
		out[c.Key] = c.App
	}
	return out
}
