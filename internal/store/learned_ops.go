package store

// This file holds mutating helpers over the learned profile. Each loads the
// current profile, applies a change, and saves it back. The underlying file
// stays plain JSON so changes remain inspectable and reversible.

// MergeMappings adds/overwrites label→app mappings, preserving existing ones.
// Accepted learning candidates and user corrections both funnel through here.
func (s *Store) MergeMappings(delta map[string]string) error {
	lp, err := s.LoadLearned()
	if err != nil {
		return err
	}
	if lp.AppMappings == nil {
		lp.AppMappings = map[string]string{}
	}
	for k, v := range delta {
		lp.AppMappings[k] = v
	}
	return s.SaveLearned(lp)
}

// RecordDecision appends an accepted/rejected recommendation to the history.
func (s *Store) RecordDecision(e HistoryEntry) error {
	lp, err := s.LoadLearned()
	if err != nil {
		return err
	}
	lp.History = append(lp.History, e)
	return s.SaveLearned(lp)
}
