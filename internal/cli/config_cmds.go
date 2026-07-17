package cli

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/EricGrill/linear-scout/internal/ai"
	"github.com/EricGrill/linear-scout/internal/config"
	"github.com/EricGrill/linear-scout/internal/linear"
)

func realDeps(prof config.Profile) *deps {
	src := linear.New(linear.DefaultEndpoint, prof.LinearToken, http.DefaultClient)
	prov := ai.NewOpenAI(prof.OpenAIKey, "gpt-4o-mini")
	return &deps{source: src, provider: prov}
}

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize configuration (creates profile dir + template files)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			dir, err := config.ProfileDir()
			if err != nil {
				return err
			}
			if err := os.MkdirAll(dir, 0o700); err != nil {
				return err
			}
			pp := filepath.Join(dir, "profile.yaml")
			if _, err := os.Stat(pp); os.IsNotExist(err) {
				os.WriteFile(pp, []byte("linear_token: \"\"\nopenai_key: \"\"\n"), 0o600)
			}
			if _, err := os.Stat("linear-scout.yaml"); os.IsNotExist(err) {
				os.WriteFile("linear-scout.yaml",
					[]byte("group_by: project\nformats: [markdown, json, telegram]\nrubric:\n  min_confidence: 0.5\n  require_evidence: true\n"), 0o644)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Initialized. Edit secrets in %s and defaults in ./linear-scout.yaml\n", pp)
			return nil
		},
	}
}

func newValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate Linear and provider credentials",
		RunE: func(cmd *cobra.Command, _ []string) error {
			dir, err := config.ProfileDir()
			if err != nil {
				return err
			}
			prof, err := config.LoadProfile(dir)
			if err != nil {
				return fmt.Errorf("load profile: %w", err)
			}
			if prof.LinearToken == "" {
				return fmt.Errorf("linear_token is empty in %s", dir)
			}
			c := linear.New(linear.DefaultEndpoint, prof.LinearToken, http.DefaultClient)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if _, err := c.Issues(ctx, time.Now().Add(-time.Hour)); err != nil {
				return fmt.Errorf("linear credential check failed: %w", err)
			}
			if prof.OpenAIKey == "" {
				return fmt.Errorf("openai_key is empty in %s", dir)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Credentials OK.")
			return nil
		},
	}
}
