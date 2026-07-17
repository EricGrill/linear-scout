package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/EricGrill/linear-scout/internal/config"
	"github.com/EricGrill/linear-scout/internal/store"
)

func newProfileCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "profile", Short: "Inspect, export, or delete the local learning profile"}

	inspect := &cobra.Command{
		Use:   "inspect",
		Short: "Show learned profile summary",
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, err := profileStore()
			if err != nil {
				return err
			}
			lp, err := s.LoadLearned()
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "App mappings: %d\nHistory entries: %d\n", len(lp.AppMappings), len(lp.History))
			return nil
		},
	}
	export := &cobra.Command{
		Use:   "export",
		Short: "Export learned profile JSON to stdout",
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, err := profileStore()
			if err != nil {
				return err
			}
			return s.Export(cmd.OutOrStdout())
		},
	}
	del := &cobra.Command{
		Use:   "delete",
		Short: "Delete the learned profile",
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, err := profileStore()
			if err != nil {
				return err
			}
			if err := s.Delete(); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Deleted learned profile.")
			return nil
		},
	}
	cmd.AddCommand(inspect, export, del)
	return cmd
}

func profileStore() (*store.Store, error) {
	dir, err := config.ProfileDir()
	if err != nil {
		return nil, err
	}
	return store.New(dir), nil
}
