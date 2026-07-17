package cli

import (
	"bufio"
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/EricGrill/linear-scout/internal/drafts"
	"github.com/EricGrill/linear-scout/internal/engine"
	"github.com/EricGrill/linear-scout/internal/write"
)

// confirm returns true if the user approves the write. With --yes it is
// automatic; otherwise it prompts and reads a line from stdin.
func confirm(cmd *cobra.Command, yes bool, n int) bool {
	if yes {
		return true
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Execute %d write action(s)? [y/N]: ", n)
	line, _ := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
	switch strings.TrimSpace(strings.ToLower(line)) {
	case "y", "yes":
		return true
	default:
		return false
	}
}

// runPlans previews plans and, when execute is set, confirms then executes
// them, printing per-plan results. This is the single write path all write
// commands funnel through.
func runPlans(cmd *cobra.Command, d *deps, plans []write.Plan, execute, yes bool) error {
	fmt.Fprint(cmd.OutOrStdout(), write.RenderPlans(plans))
	if !execute {
		fmt.Fprintln(cmd.OutOrStdout(), "Re-run with --execute to perform these writes.")
		return nil
	}
	if len(plans) == 0 {
		return nil
	}
	if !confirm(cmd, yes, len(plans)) {
		fmt.Fprintln(cmd.OutOrStdout(), "Aborted. Nothing was written.")
		return nil
	}
	results, err := write.Execute(context.Background(), d.writer, d.audit, plans, nowFn(d))
	if err != nil {
		return err
	}
	for _, r := range results {
		status := "OK"
		if !r.OK {
			status = "FAILED"
		}
		fmt.Fprintf(cmd.OutOrStdout(), "[%s] %s — %s\n", status, r.Plan.Describe(), r.Detail)
	}
	return nil
}

func newPreviewCmd() *cobra.Command {
	var since, groupBy, team string
	cmd := &cobra.Command{
		Use:   "preview",
		Short: "Preview the Linear writes create-issues would perform (dry-run, never writes)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			plans, d, err := buildIssuePlans(cmd, since, groupBy, team)
			if err != nil {
				return err
			}
			return runPlans(cmd, d, plans, false, false)
		},
	}
	cmd.Flags().StringVar(&since, "since", "7d", "time window")
	cmd.Flags().StringVar(&groupBy, "group-by", "project", "grouping mode")
	cmd.Flags().StringVar(&team, "team", "", "target team ID for drafted issues")
	return cmd
}

func newCreateIssuesCmd() *cobra.Command {
	var since, groupBy, team string
	var execute, yes bool
	cmd := &cobra.Command{
		Use:   "create-issues",
		Short: "Create Linear issues from AI-drafted metadata (dry-run unless --execute)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			plans, d, err := buildIssuePlans(cmd, since, groupBy, team)
			if err != nil {
				return err
			}
			return runPlans(cmd, d, plans, execute, yes)
		},
	}
	cmd.Flags().StringVar(&since, "since", "7d", "time window")
	cmd.Flags().StringVar(&groupBy, "group-by", "project", "grouping mode")
	cmd.Flags().StringVar(&team, "team", "", "target team ID for created issues (required)")
	cmd.Flags().BoolVar(&execute, "execute", false, "actually create issues (default is dry-run)")
	cmd.Flags().BoolVar(&yes, "yes", false, "skip the confirmation prompt")
	return cmd
}

// buildIssuePlans runs the read pipeline, drafts issues, and turns them into
// create-issue write plans.
func buildIssuePlans(cmd *cobra.Command, since, groupBy, team string) ([]write.Plan, *deps, error) {
	if team == "" {
		return nil, nil, fmt.Errorf("--team is required (target team ID for created issues)")
	}
	d, rubric, err := buildDeps(cmd)
	if err != nil {
		return nil, nil, err
	}
	rep, err := engine.Run(context.Background(), d.source, d.provider, engine.Options{
		Window: since, GroupBy: groupBy, Now: nowFn(d), Rubric: rubric, Mappings: d.mappings,
	})
	if err != nil {
		return nil, nil, err
	}
	return write.BuildIssuePlans(drafts.FromReport(rep), team), d, nil
}

func newCommentCmd() *cobra.Command {
	var issue, body string
	var execute, yes bool
	cmd := &cobra.Command{
		Use:   "comment",
		Short: "Add a comment to a Linear issue (dry-run unless --execute)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if issue == "" || body == "" {
				return fmt.Errorf("--issue and --body are required")
			}
			d, _, err := buildDeps(cmd)
			if err != nil {
				return err
			}
			plans := []write.Plan{{
				Action: write.ActionComment, Target: issue, Body: body,
				Summary: "Comment on " + issue,
			}}
			return runPlans(cmd, d, plans, execute, yes)
		},
	}
	cmd.Flags().StringVar(&issue, "issue", "", "issue ID or identifier (required)")
	cmd.Flags().StringVar(&body, "body", "", "comment body (required)")
	cmd.Flags().BoolVar(&execute, "execute", false, "actually post the comment (default is dry-run)")
	cmd.Flags().BoolVar(&yes, "yes", false, "skip the confirmation prompt")
	return cmd
}

func newLabelCmd() *cobra.Command {
	var issue, labels string
	var execute, yes bool
	cmd := &cobra.Command{
		Use:   "label",
		Short: "Add labels to a Linear issue (dry-run unless --execute)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			names := splitCSV(labels)
			if issue == "" || len(names) == 0 {
				return fmt.Errorf("--issue and --labels are required")
			}
			d, _, err := buildDeps(cmd)
			if err != nil {
				return err
			}
			plans := []write.Plan{{
				Action: write.ActionAddLabels, Target: issue, Labels: names,
				Summary: fmt.Sprintf("Add labels %s to %s", strings.Join(names, ", "), issue),
			}}
			return runPlans(cmd, d, plans, execute, yes)
		},
	}
	cmd.Flags().StringVar(&issue, "issue", "", "issue ID or identifier (required)")
	cmd.Flags().StringVar(&labels, "labels", "", "comma-separated label names (required)")
	cmd.Flags().BoolVar(&execute, "execute", false, "actually add the labels (default is dry-run)")
	cmd.Flags().BoolVar(&yes, "yes", false, "skip the confirmation prompt")
	return cmd
}

func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
