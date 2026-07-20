package feature

import (
	"testing"
)

func FuzzIsInRollout(f *testing.F) {
	f.Add("user-123", 50)
	f.Add("machine-456", 100)
	f.Add("agent-789", 0)

	f.Fuzz(func(t *testing.T, targetID string, pct int) {
		if pct < 0 || pct > 100 {
			return
		}
		result := isInRollout(targetID, pct)
		if pct == 0 {
			if result {
				t.Errorf("expected false for 0%% rollout, got true")
			}
		}
		if pct == 100 {
			if !result {
				t.Errorf("expected true for 100%% rollout, got false")
			}
		}
	})
}
