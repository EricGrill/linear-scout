package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadWorkspaceDefaults(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "linear-scout.yaml")
	os.WriteFile(p, []byte("group_by: project\nformats: [markdown, json]\nrubric:\n  min_confidence: 0.5\n  require_evidence: true\n"), 0o644)
	w, err := LoadWorkspace(p)
	if err != nil {
		t.Fatal(err)
	}
	if w.GroupBy != "project" {
		t.Fatalf("GroupBy=%q", w.GroupBy)
	}
	if !w.Rubric.RequireEvidence || w.Rubric.MinConfidence != 0.5 {
		t.Fatalf("rubric=%+v", w.Rubric)
	}
}

func TestProfileDirRespectsXDG(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdgtest")
	got, err := ProfileDir()
	if err != nil {
		t.Fatal(err)
	}
	if got != "/tmp/xdgtest/linear-scout" {
		t.Fatalf("ProfileDir=%q", got)
	}
}

func TestLoadProfileReadsSecrets(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "profile.yaml"),
		[]byte("linear_token: lin_abc\nopenai_key: sk_xyz\n"), 0o600)
	prof, err := LoadProfile(dir)
	if err != nil {
		t.Fatal(err)
	}
	if prof.LinearToken != "lin_abc" || prof.OpenAIKey != "sk_xyz" {
		t.Fatalf("profile=%+v", prof)
	}
}
