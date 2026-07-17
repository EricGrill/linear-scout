// Package config loads the shared workspace config and the user-local profile.
// Secrets live only in the profile; the workspace file must never hold them.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type RubricConfig struct {
	MinConfidence   float64 `yaml:"min_confidence"`
	RequireEvidence bool    `yaml:"require_evidence"`
}

type Workspace struct {
	GroupBy string       `yaml:"group_by"`
	Formats []string     `yaml:"formats"`
	Rubric  RubricConfig `yaml:"rubric"`
}

type Profile struct {
	LinearToken string `yaml:"linear_token"`
	OpenAIKey   string `yaml:"openai_key"`
	Dir         string `yaml:"-"`
}

// LoadWorkspace reads and parses the shared workspace config file.
func LoadWorkspace(path string) (Workspace, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Workspace{}, fmt.Errorf("read workspace config: %w", err)
	}
	var w Workspace
	if err := yaml.Unmarshal(b, &w); err != nil {
		return Workspace{}, fmt.Errorf("parse workspace config: %w", err)
	}
	if w.GroupBy == "" {
		w.GroupBy = "project"
	}
	if len(w.Formats) == 0 {
		w.Formats = []string{"markdown"}
	}
	return w, nil
}

// ProfileDir resolves the user-local profile directory.
func ProfileDir() (string, error) {
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		return filepath.Join(x, "linear-scout"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(home, ".config", "linear-scout"), nil
}

// LoadProfile reads profile.yaml (secrets) from dir.
func LoadProfile(dir string) (Profile, error) {
	b, err := os.ReadFile(filepath.Join(dir, "profile.yaml"))
	if err != nil {
		return Profile{}, fmt.Errorf("read profile: %w", err)
	}
	var p Profile
	if err := yaml.Unmarshal(b, &p); err != nil {
		return Profile{}, fmt.Errorf("parse profile: %w", err)
	}
	p.Dir = dir
	return p, nil
}
