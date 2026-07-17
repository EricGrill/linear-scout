package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/EricGrill/linear-scout/internal/config"
	"github.com/EricGrill/linear-scout/internal/drafts"
	"github.com/EricGrill/linear-scout/internal/engine"
	"github.com/EricGrill/linear-scout/internal/report"
)

func buildDeps(cmd *cobra.Command) (*deps, config.RubricConfig, error) {
	if testDeps != nil {
		return testDeps, config.RubricConfig{MinConfidence: 0.5, RequireEvidence: true}, nil
	}
	// Real construction: load profile, build Linear client + OpenAI provider.
	dir, err := config.ProfileDir()
	if err != nil {
		return nil, config.RubricConfig{}, err
	}
	prof, err := config.LoadProfile(dir)
	if err != nil {
		return nil, config.RubricConfig{}, fmt.Errorf("load profile (run `linear-scout init`): %w", err)
	}
	return realDeps(prof), config.RubricConfig{MinConfidence: 0.5, RequireEvidence: true}, nil
}

func newReportCmd() *cobra.Command {
	var since, groupBy, format string
	var limit int
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Generate an AI recommendation report for a time window",
		RunE: func(cmd *cobra.Command, _ []string) error {
			d, rubric, err := buildDeps(cmd)
			if err != nil {
				return err
			}
			rep, err := engine.Run(context.Background(), d.source, d.provider, engine.Options{
				Window: since, GroupBy: groupBy, Limit: limit, Now: nowFn(d), Rubric: rubric,
			})
			if err != nil {
				return err
			}
			out, err := report.Render(rep, format)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), out)
			return nil
		},
	}
	cmd.Flags().StringVar(&since, "since", "7d", "time window, e.g. 7d, 24h, 2w")
	cmd.Flags().StringVar(&groupBy, "group-by", "project", "grouping mode: project|team")
	cmd.Flags().StringVar(&format, "format", "markdown", "output format: markdown|json|telegram")
	cmd.Flags().IntVar(&limit, "limit", 0, "max recommendations (0 = unlimited)")
	return cmd
}

func newCreateDraftsCmd() *cobra.Command {
	var since, groupBy string
	cmd := &cobra.Command{
		Use:   "create-drafts",
		Short: "Generate reviewable draft issue metadata (no writes)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			d, rubric, err := buildDeps(cmd)
			if err != nil {
				return err
			}
			rep, err := engine.Run(context.Background(), d.source, d.provider, engine.Options{
				Window: since, GroupBy: groupBy, Now: nowFn(d), Rubric: rubric,
			})
			if err != nil {
				return err
			}
			for i, dr := range drafts.FromReport(rep) {
				fmt.Fprintf(cmd.OutOrStdout(), "Draft %d: %s\n  %s\n", i+1, dr.Title, dr.Body)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&since, "since", "7d", "time window")
	cmd.Flags().StringVar(&groupBy, "group-by", "project", "grouping mode")
	return cmd
}

func newPreviewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "preview",
		Short: "Preview Linear writes (dry-run). Writes are implemented in Milestone 2.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), "[dry-run] No write actions available yet (Milestone 2). Nothing will change in Linear.")
			return nil
		},
	}
}
