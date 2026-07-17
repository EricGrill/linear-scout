package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/EricGrill/linear-scout/internal/config"
	"github.com/EricGrill/linear-scout/internal/grouping"
	"github.com/EricGrill/linear-scout/internal/ingest"
	"github.com/EricGrill/linear-scout/internal/learn"
	"github.com/EricGrill/linear-scout/internal/store"
)

// storeFor returns the profile store — the injected one under test, otherwise
// one rooted at the user's profile directory. Local, non-Linear operations use
// this directly so they never require provider credentials.
func storeFor() (*store.Store, error) {
	if testDeps != nil && testDeps.profileStore != nil {
		return testDeps.profileStore, nil
	}
	dir, err := config.ProfileDir()
	if err != nil {
		return nil, err
	}
	return store.New(dir), nil
}

func newLearnCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "learn", Short: "Run or inspect learning passes over Linear activity"}
	cmd.AddCommand(newLearnRunCmd(), newLearnInspectCmd())
	return cmd
}

func newLearnRunCmd() *cobra.Command {
	var since, groupBy string
	var apply bool
	var minSupport int
	var minPurity float64
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run learning missions; propose profile updates (dry-run unless --apply)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			d, _, err := buildDeps(cmd)
			if err != nil {
				return err
			}
			st, err := storeFor()
			if err != nil {
				return err
			}
			existing, err := st.LoadLearned()
			if err != nil {
				return err
			}

			act, err := ingest.Fetch(context.Background(), d.source, since, nowFn(d))
			if err != nil {
				return err
			}
			groups, _ := grouping.Classify(act, existing.AppMappings, groupBy)

			art, accepted, reason := learn.RunPass(
				learn.LabelMappingMission{},
				learn.PurityEvaluator{MinSupport: minSupport, MinPurity: minPurity},
				learn.Input{Activity: act, Groups: groups, Existing: existing},
			)

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Mission: %s\n", art.Mission)
			for _, r := range art.Rationale {
				fmt.Fprintf(out, "  - %s\n", r)
			}
			fmt.Fprintf(out, "%s\n", reason)
			if len(accepted) == 0 {
				fmt.Fprintln(out, "No mappings met the acceptance bar.")
				return nil
			}
			fmt.Fprintln(out, "Accepted mappings:")
			for _, c := range accepted {
				fmt.Fprintf(out, "  %s → %s (support %d, purity %.2f)\n", c.Key, c.App, c.Support, c.Purity)
			}
			if !apply {
				fmt.Fprintln(out, "Re-run with --apply to write these into the local profile.")
				return nil
			}
			if err := st.MergeMappings(learn.Mappings(accepted)); err != nil {
				return err
			}
			fmt.Fprintf(out, "Applied %d mapping(s) to the local profile.\n", len(accepted))
			return nil
		},
	}
	cmd.Flags().StringVar(&since, "since", "30d", "time window to learn from")
	cmd.Flags().StringVar(&groupBy, "group-by", "project", "grouping mode")
	cmd.Flags().BoolVar(&apply, "apply", false, "write accepted mappings into the profile")
	cmd.Flags().IntVar(&minSupport, "min-support", 3, "minimum supporting issues per mapping")
	cmd.Flags().Float64Var(&minPurity, "min-purity", 0.75, "minimum purity (0..1) per mapping")
	return cmd
}

func newLearnInspectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "inspect",
		Short: "Show learned mappings and decision-history counts",
		RunE: func(cmd *cobra.Command, _ []string) error {
			st, err := storeFor()
			if err != nil {
				return err
			}
			lp, err := st.LoadLearned()
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Learned mappings (%d):\n", len(lp.AppMappings))
			for k, v := range lp.AppMappings {
				fmt.Fprintf(out, "  %s → %s\n", k, v)
			}
			accepted, rejected := 0, 0
			for _, h := range lp.History {
				if h.Accepted {
					accepted++
				} else {
					rejected++
				}
			}
			fmt.Fprintf(out, "Decisions: %d accepted, %d rejected\n", accepted, rejected)
			return nil
		},
	}
}

func newCorrectCmd() *cobra.Command {
	var label, app string
	cmd := &cobra.Command{
		Use:   "correct",
		Short: "Record a correction mapping a label to an app/product",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if label == "" || app == "" {
				return fmt.Errorf("--label and --app are required")
			}
			st, err := storeFor()
			if err != nil {
				return err
			}
			if err := st.MergeMappings(map[string]string{"label:" + label: app}); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Recorded: label %q → %s\n", label, app)
			return nil
		},
	}
	cmd.Flags().StringVar(&label, "label", "", "Linear label name (required)")
	cmd.Flags().StringVar(&app, "app", "", "app/product to map it to (required)")
	return cmd
}

func newFeedbackCmd() *cobra.Command {
	var rec string
	var accept, reject bool
	cmd := &cobra.Command{
		Use:   "feedback",
		Short: "Record acceptance or rejection of a recommendation",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if rec == "" {
				return fmt.Errorf("--rec is required")
			}
			if accept == reject { // both false or both true
				return fmt.Errorf("exactly one of --accept or --reject is required")
			}
			st, err := storeFor()
			if err != nil {
				return err
			}
			if err := st.RecordDecision(store.HistoryEntry{RecSummary: rec, Accepted: accept, At: nowFnStore()}); err != nil {
				return err
			}
			verb := "rejected"
			if accept {
				verb = "accepted"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Recorded %s recommendation: %q\n", verb, rec)
			return nil
		},
	}
	cmd.Flags().StringVar(&rec, "rec", "", "recommendation summary (required)")
	cmd.Flags().BoolVar(&accept, "accept", false, "mark the recommendation accepted")
	cmd.Flags().BoolVar(&reject, "reject", false, "mark the recommendation rejected")
	return cmd
}

// nowFnStore returns the injected clock if present, else the real time.
func nowFnStore() time.Time {
	return nowFn(testDeps)
}
