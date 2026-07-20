package scheduler

import (
	"testing"
	"time"
)

func FuzzNextCronTime(f *testing.F) {
	seeds := []string{
		"0 12 * * *",
		"30 14 15 * *",
		"0 0 1 1 *",
		"*/5 * * * *",
		"* * * * *",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, expr string) {
		now := time.Now()
		result, err := NextCronTime(expr, now)
		if err != nil {
			return
		}
		if result.Before(now.Add(-time.Minute)) {
			t.Errorf("cron time %v is before input time %v", result, now)
		}
		if result.After(now.Add(365 * 24 * time.Hour)) {
			t.Errorf("cron time %v is more than 1 year in the future", result)
		}
	})
}
