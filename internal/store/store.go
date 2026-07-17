// Package store persists user-owned state as plain, inspectable JSON files.
package store

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

type HistoryEntry struct {
	RecSummary string    `json:"rec_summary"`
	Accepted   bool      `json:"accepted"`
	At         time.Time `json:"at"`
}

type LearnedProfile struct {
	AppMappings map[string]string `json:"app_mappings"`
	History     []HistoryEntry    `json:"history"`
}

type Store struct{ Dir string }

func New(dir string) *Store { return &Store{Dir: dir} }

func (s *Store) path() string { return filepath.Join(s.Dir, "learned.json") }

// LoadLearned returns the learned profile, or an empty (initialized) one if absent.
func (s *Store) LoadLearned() (LearnedProfile, error) {
	b, err := os.ReadFile(s.path())
	if os.IsNotExist(err) {
		return LearnedProfile{AppMappings: map[string]string{}}, nil
	}
	if err != nil {
		return LearnedProfile{}, fmt.Errorf("read learned: %w", err)
	}
	var lp LearnedProfile
	if err := json.Unmarshal(b, &lp); err != nil {
		return LearnedProfile{}, fmt.Errorf("parse learned: %w", err)
	}
	if lp.AppMappings == nil {
		lp.AppMappings = map[string]string{}
	}
	return lp, nil
}

func (s *Store) SaveLearned(lp LearnedProfile) error {
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		return fmt.Errorf("mkdir profile: %w", err)
	}
	b, err := json.MarshalIndent(lp, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal learned: %w", err)
	}
	return os.WriteFile(s.path(), b, 0o600)
}

func (s *Store) Export(w io.Writer) error {
	b, err := os.ReadFile(s.path())
	if err != nil {
		return fmt.Errorf("read learned for export: %w", err)
	}
	_, err = w.Write(b)
	return err
}

func (s *Store) Delete() error {
	err := os.Remove(s.path())
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
