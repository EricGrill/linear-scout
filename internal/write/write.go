// Package write is the surface-agnostic write engine. It mirrors the read
// engine seam: build a Plan (no side effects), preview it, then Execute it
// behind explicit confirmation, recording every change to the audit log.
//
// Only the actions represented here are possible. There is deliberately no
// plan kind for closing, reprioritizing, or deleting issues.
package write

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/EricGrill/linear-scout/internal/drafts"
	"github.com/EricGrill/linear-scout/internal/linear"
	"github.com/EricGrill/linear-scout/internal/model"
	"github.com/EricGrill/linear-scout/internal/store"
)

// Action enumerates the allowed write kinds.
type Action string

const (
	ActionCreateIssue Action = "create-issue"
	ActionComment     Action = "comment"
	ActionAddLabels   Action = "add-labels"
)

// Plan describes one intended change with no side effects.
type Plan struct {
	Action   Action
	Target   string   // issue id for comment/add-labels; empty for create
	Summary  string   // human description of the change
	Evidence []string // source Linear evidence refs

	// Payloads (only the field matching Action is meaningful).
	Issue  *linear.CreateIssueInput
	Body   string
	Labels []string
}

// Writer abstracts the Linear mutations so Execute is stubbable in tests.
// *linear.Client satisfies it.
type Writer interface {
	CreateIssue(ctx context.Context, in linear.CreateIssueInput) (linear.CreatedIssue, error)
	CreateComment(ctx context.Context, issueID, body string) (linear.CreatedComment, error)
	AddLabels(ctx context.Context, issueID string, labels []string) error
}

// AuditSink records executed writes. *store.Store satisfies it.
type AuditSink interface {
	AppendAudit(store.AuditEntry) error
}

// Result is the outcome of executing one plan.
type Result struct {
	Plan   Plan
	OK     bool
	Detail string // created identifier/url on success, error text on failure
}

// BuildIssuePlans turns draft issue metadata into create-issue plans.
func BuildIssuePlans(ds []drafts.Draft, teamID string) []Plan {
	plans := make([]Plan, 0, len(ds))
	for _, d := range ds {
		plans = append(plans, Plan{
			Action:   ActionCreateIssue,
			Summary:  "Create issue: " + d.Title,
			Evidence: refs(d.Evidence),
			Issue: &linear.CreateIssueInput{
				Title: d.Title, Description: d.Body, TeamID: teamID, Priority: d.Priority,
			},
		})
	}
	return plans
}

// Describe renders a single-line human summary of a plan for previews.
func (p Plan) Describe() string {
	ev := ""
	if len(p.Evidence) > 0 {
		ev = " [evidence: " + strings.Join(p.Evidence, ", ") + "]"
	}
	switch p.Action {
	case ActionComment:
		return fmt.Sprintf("comment on %s: %q%s", p.Target, truncate(p.Body, 60), ev)
	case ActionAddLabels:
		return fmt.Sprintf("add labels %s to %s%s", strings.Join(p.Labels, ", "), p.Target, ev)
	default:
		return fmt.Sprintf("%s%s", p.Summary, ev)
	}
}

// RenderPlans renders a dry-run preview block for a set of plans.
func RenderPlans(plans []Plan) string {
	var b strings.Builder
	fmt.Fprintf(&b, "[dry-run] %d write action(s) would be performed:\n", len(plans))
	for i, p := range plans {
		fmt.Fprintf(&b, "  %d. %s\n", i+1, p.Describe())
	}
	if len(plans) == 0 {
		b.WriteString("  (nothing to do)\n")
	}
	return b.String()
}

// Execute performs each plan in order, appending an audit entry per success.
// It never stops on a single failure; failures are reported per-plan.
func Execute(ctx context.Context, w Writer, audit AuditSink, plans []Plan, now time.Time) ([]Result, error) {
	results := make([]Result, 0, len(plans))
	for _, p := range plans {
		res := Result{Plan: p}
		var detail string
		var err error
		switch p.Action {
		case ActionCreateIssue:
			if p.Issue == nil {
				err = fmt.Errorf("create-issue plan missing payload")
				break
			}
			var created linear.CreatedIssue
			created, err = w.CreateIssue(ctx, *p.Issue)
			if err == nil {
				detail = created.Identifier + " " + created.URL
				res.Plan.Target = created.Identifier
			}
		case ActionComment:
			var created linear.CreatedComment
			created, err = w.CreateComment(ctx, p.Target, p.Body)
			if err == nil {
				detail = created.URL
			}
		case ActionAddLabels:
			err = w.AddLabels(ctx, p.Target, p.Labels)
			if err == nil {
				detail = "labels added: " + strings.Join(p.Labels, ", ")
			}
		default:
			err = fmt.Errorf("unknown action %q", p.Action)
		}

		if err != nil {
			res.OK = false
			res.Detail = err.Error()
			results = append(results, res)
			continue
		}
		res.OK = true
		res.Detail = detail
		// Record the executed write. An audit failure is surfaced but does not
		// undo the (already committed) Linear change.
		if aerr := audit.AppendAudit(store.AuditEntry{
			At: now, Action: string(p.Action), Target: res.Plan.Target,
			Summary: p.Summary, Evidence: p.Evidence,
		}); aerr != nil {
			return results, fmt.Errorf("write succeeded but audit failed: %w", aerr)
		}
		results = append(results, res)
	}
	return results, nil
}

func refs(links []model.EvidenceLink) []string {
	out := make([]string, 0, len(links))
	for _, l := range links {
		if l.Ref != "" {
			out = append(out, l.Ref)
		}
	}
	return out
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
