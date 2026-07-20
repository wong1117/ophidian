package scheduler

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestScheduler_Race_ScheduleConcurrent(t *testing.T) {
	store := newTestStore()
	s := NewScheduler(store)

	k := 100
	var wg sync.WaitGroup
	for i := 0; i < k; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			s.Schedule(&Job{
				ID:           fmt.Sprintf("race-%d", idx),
				ScheduleType: ScheduleOnce,
				RunAt:        time.Now().Add(time.Hour),
				Func:         func(ctx context.Context) error { return nil },
			})
		}(i)
	}
	wg.Wait()
	if got := len(s.List()); got != k {
		t.Errorf("expected %d jobs, got %d", k, got)
	}
}

func TestScheduler_Race_ListWhileScheduling(t *testing.T) {
	store := newTestStore()
	s := NewScheduler(store)

	for i := 0; i < 50; i++ {
		s.Schedule(&Job{
			ID:           fmt.Sprintf("init-%d", i),
			ScheduleType: ScheduleOnce, RunAt: time.Now().Add(time.Hour),
			Func: func(ctx context.Context) error { return nil },
		})
	}

	done := make(chan struct{})
	go func() {
		for i := 0; i < 50; i++ {
			s.Schedule(&Job{ID: fmt.Sprintf("writer-%d", i), ScheduleType: ScheduleOnce, RunAt: time.Now().Add(time.Hour),
				Func: func(ctx context.Context) error { return nil }})
			time.Sleep(time.Microsecond)
		}
		close(done)
	}()

	for i := 0; i < 100; i++ {
		s.List()
		time.Sleep(time.Microsecond)
	}
	<-done
}

func TestScheduler_Race_ExecuteAndCancel(t *testing.T) {
	store := newTestStore()
	s := NewScheduler(store)

	for i := 0; i < 20; i++ {
		s.Schedule(&Job{
			ID:           fmt.Sprintf("ec-%d", i),
			ScheduleType: ScheduleOnce, RunAt: time.Now().Add(10 * time.Millisecond),
			Func:         func(ctx context.Context) error { return nil },
		})
	}
	s.Start()
	time.Sleep(200 * time.Millisecond)
	s.Stop()
}
