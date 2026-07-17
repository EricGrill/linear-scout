// Package ai defines the provider interface, prompt assembly, and validation.
// The evidence layer below it makes every recommendation auditable.
package ai

import (
	"context"
	"fmt"
	"strings"

	"github.com/EricGrill/linear-scout/internal/model"
)

type Request struct {
	Groups   []model.Group
	Evidence map[string]model.EvidenceBundle
}

type Provider interface {
	Recommend(ctx context.Context, req Request) ([]model.Recommendation, error)
}

// AssemblePrompt renders a deterministic prompt from groups + evidence.
func AssemblePrompt(req Request) string {
	var b strings.Builder
	b.WriteString("You are linear-scout. Propose concrete product improvement ")
	b.WriteString("opportunities. Every recommendation MUST cite evidence refs below.\n\n")
	for _, g := range req.Groups {
		fmt.Fprintf(&b, "## Group %s (%s, confidence %s)\n", g.Label, g.Kind, g.Confidence.Band())
		for _, l := range req.Evidence[g.Key].Links {
			fmt.Fprintf(&b, "- [%s] %s — %s (%s)\n", l.Kind, l.Ref, l.Quote, l.URL)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// StubProvider is a deterministic Provider for tests and offline runs.
type StubProvider struct{ Recs []model.Recommendation }

func (s StubProvider) Recommend(context.Context, Request) ([]model.Recommendation, error) {
	return s.Recs, nil
}
