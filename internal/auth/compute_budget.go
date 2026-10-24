package auth

import "time"

// This budget measures work, independently of credential failures. In particular,
// a correct password without TOTP does not erase work already admitted. Both
// password login and authenticated password checks use the same bounded table.
type computeBudget struct {
	start time.Time
	used  int
}

const maxComputeSources = 4096

func (s *Service) admitPasswordComputation(key string) bool {
	s.loginMu.Lock()
	defer s.loginMu.Unlock()
	now := s.now()
	for source, budget := range s.compute {
		if !budget.start.Add(s.config.LoginWindow).After(now) {
			delete(s.compute, source)
		}
	}
	budget, exists := s.compute[key]
	if !exists {
		if len(s.compute) >= maxComputeSources {
			return false
		}
		budget.start = now
	}
	// Two password checks are necessary for the normal password -> TOTP flow,
	// including deployments whose failure budget is configured to one.
	if budget.used >= max(2, s.config.MaxLoginFailures*2) {
		return false
	}
	budget.used++
	s.compute[key] = budget
	return true
}
