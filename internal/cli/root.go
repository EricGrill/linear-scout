// Package cli wires cobra commands as thin adapters over the engine.
package cli

import (
	"time"

	"github.com/spf13/cobra"

	"github.com/EricGrill/linear-scout/internal/ai"
	"github.com/EricGrill/linear-scout/internal/ingest"
)

// deps lets tests inject a deterministic source/provider/clock.
type deps struct {
	source   ingest.Source
	provider ai.Provider
	now      time.Time
	mappings map[string]string
}

// testDeps, when non-nil, overrides real construction (tests only).
var testDeps *deps

func nowFn(d *deps) time.Time {
	if d != nil && !d.now.IsZero() {
		return d.now
	}
	return time.Now()
}

func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "linear-scout",
		Short: "AI-first assistant that turns Linear activity into product opportunities",
	}
	root.AddCommand(newReportCmd())
	root.AddCommand(newInitCmd())
	root.AddCommand(newValidateCmd())
	root.AddCommand(newCreateDraftsCmd())
	root.AddCommand(newPreviewCmd())
	root.AddCommand(newProfileCmd())
	return root
}

func Execute() error {
	return NewRootCmd().Execute()
}
