// Package report renders the surface-agnostic Report model into output formats.
package report

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/EricGrill/linear-scout/internal/model"
)

func Markdown(r model.Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# linear-scout report (%s, grouped by %s)\n\n", r.Window, r.GroupBy)
	if r.Unclassified > 0 {
		fmt.Fprintf(&b, "_%d unclassified issue(s)._\n\n", r.Unclassified)
	}
	for _, rec := range r.Recommendations {
		fmt.Fprintf(&b, "## %s\n\n", rec.Summary)
		fmt.Fprintf(&b, "**Why it matters:** %s\n\n", rec.WhyItMatters)
		fmt.Fprintf(&b, "**Confidence:** %s\n\n", rec.Confidence.Band())
		if len(rec.Evidence) > 0 {
			b.WriteString("**Evidence:**\n")
			for _, e := range rec.Evidence {
				fmt.Fprintf(&b, "- [%s](%s)\n", e.Ref, e.URL)
			}
			b.WriteString("\n")
		}
	}
	return b.String()
}

func JSON(r model.Report) (string, error) {
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", fmt.Errorf("render json: %w", err)
	}
	return string(b), nil
}

func Telegram(r model.Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "linear-scout — %s\n", r.Window)
	for _, rec := range r.Recommendations {
		fmt.Fprintf(&b, "\n• %s (%s)\n", rec.Summary, rec.Confidence.Band())
		fmt.Fprintf(&b, "  %s\n", rec.WhyItMatters)
		for _, e := range rec.Evidence {
			fmt.Fprintf(&b, "  ↳ %s %s\n", e.Ref, e.URL)
		}
	}
	return b.String()
}

func Render(r model.Report, format string) (string, error) {
	switch format {
	case "markdown", "md":
		return Markdown(r), nil
	case "json":
		return JSON(r)
	case "telegram":
		return Telegram(r), nil
	default:
		return "", fmt.Errorf("unknown format %q", format)
	}
}
