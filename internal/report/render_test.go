package report

import (
	"strings"
	"testing"
	"time"

	"github.com/EricGrill/linear-scout/internal/model"
)

func sampleReport() model.Report {
	return model.Report{
		GeneratedAt: time.Date(2026, 7, 16, 0, 0, 0, 0, time.UTC),
		Window:      "7d", GroupBy: "project",
		Groups: []model.Group{{Label: "CoreApp", Kind: "app", Confidence: 0.9}},
		Recommendations: []model.Recommendation{{
			Summary: "Fix login crash", WhyItMatters: "blocks users", Confidence: 0.8,
			Evidence: []model.EvidenceLink{{Ref: "ENG-1", URL: "https://l/ENG-1"}},
		}},
	}
}

func TestMarkdownIncludesEvidenceLink(t *testing.T) {
	out := Markdown(sampleReport())
	if !strings.Contains(out, "Fix login crash") || !strings.Contains(out, "https://l/ENG-1") {
		t.Fatalf("markdown missing content:\n%s", out)
	}
}

func TestJSONRoundTrips(t *testing.T) {
	out, err := JSON(sampleReport())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "\"Summary\": \"Fix login crash\"") {
		t.Fatalf("json missing content:\n%s", out)
	}
}

func TestTelegramIsConcise(t *testing.T) {
	out := Telegram(sampleReport())
	if !strings.Contains(out, "Fix login crash") || strings.Contains(out, "|") {
		t.Fatalf("telegram not concise/plain:\n%s", out)
	}
}

func TestRenderDispatch(t *testing.T) {
	for _, f := range []string{"markdown", "json", "telegram"} {
		if _, err := Render(sampleReport(), f); err != nil {
			t.Fatalf("format %s: %v", f, err)
		}
	}
	if _, err := Render(sampleReport(), "bogus"); err == nil {
		t.Fatal("expected error for bogus format")
	}
}
