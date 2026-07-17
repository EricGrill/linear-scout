package store

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// AuditEntry records a single executed write for the append-only audit log.
type AuditEntry struct {
	At       time.Time `json:"at"`
	Action   string    `json:"action"`   // "create-issue" | "comment" | "add-labels"
	Target   string    `json:"target"`   // issue id/identifier or created identifier
	Summary  string    `json:"summary"`  // human description of the change
	Evidence []string  `json:"evidence"` // source Linear evidence refs
}

func (s *Store) auditPath() string { return filepath.Join(s.Dir, "audit.log") }

// AppendAudit appends one entry as a JSON line. The log is append-only.
func (s *Store) AppendAudit(e AuditEntry) error {
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		return fmt.Errorf("mkdir profile: %w", err)
	}
	f, err := os.OpenFile(s.auditPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("open audit log: %w", err)
	}
	defer f.Close()
	b, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("marshal audit entry: %w", err)
	}
	if _, err := f.Write(append(b, '\n')); err != nil {
		return fmt.Errorf("write audit entry: %w", err)
	}
	return nil
}

// ReadAudit returns all audit entries in order, or an empty slice if none.
func (s *Store) ReadAudit() ([]AuditEntry, error) {
	f, err := os.Open(s.auditPath())
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open audit log: %w", err)
	}
	defer f.Close()
	var out []AuditEntry
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var e AuditEntry
		if err := json.Unmarshal(line, &e); err != nil {
			return nil, fmt.Errorf("parse audit entry: %w", err)
		}
		out = append(out, e)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("scan audit log: %w", err)
	}
	return out, nil
}
