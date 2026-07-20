package queue

import (
	"fmt"
	"testing"
	"time"
)

func FuzzPriorityQueue(f *testing.F) {
	f.Add(int64(1), int64(10), "handler-a")
	f.Add(int64(5), int64(0), "handler-b")

	f.Fuzz(func(t *testing.T, seedA int64, seedB int64, handler string) {
		if handler == "" {
			return
		}
		q := NewPriorityQueue(nil)

		q.Enqueue(&Job{
			ID:           "job-a",
			Handler:      handler,
			Priority:     int(seedA) % 100,
			MaxRetries:   0,
		})
		q.Enqueue(&Job{
			ID:           "job-b",
			Handler:      handler,
			Priority:     int(seedB) % 100,
			MaxRetries:   0,
		})

		j1, _ := q.Dequeue(nil)
		j2, _ := q.Dequeue(nil)

		if j1 == nil || j2 == nil {
			t.Errorf("dequeued nil job")
			return
		}

		q.Ack(nil, j1.ID)
		q.Ack(nil, j2.ID)

		if q.Size() != 0 {
			t.Errorf("queue not empty after ack: %d", q.Size())
		}
	})
}

func FuzzQueueRetry(f *testing.F) {
	f.Add("job-1", int64(3))

	f.Fuzz(func(t *testing.T, jobID string, retries int64) {
		if jobID == "" {
			return
		}
		r := int(retries) % 10
		if r < 0 { r = -r }

		q := NewPriorityQueue(nil)
		q.Enqueue(&Job{ID: jobID, Handler: "test", MaxRetries: int(r)})

		for i := 0; i <= r; i++ {
			job, _ := q.Dequeue(nil)
			if job == nil {
				t.Errorf("job disappeared after %d dequeues", i)
				return
			}
			q.Nack(nil, jobID, fmt.Errorf("retry error"))
		}

		job, _ := q.Dequeue(nil)
		if job != nil {
			t.Errorf("job should be dead-lettered after %d retries", r)
		}
	})
}

func FuzzQueueDelay(f *testing.F) {
	f.Add("delayed-job", int64(50))

	f.Fuzz(func(t *testing.T, jobID string, delayMs int64) {
		if jobID == "" { return }
		delay := time.Duration(delayMs%1000) * time.Millisecond
		if delay < 0 { delay = -delay }

		q := NewPriorityQueue(nil)
		q.Enqueue(&Job{ID: jobID, Handler: "test", Delay: delay})

		if delay > 0 {
			job, _ := q.Dequeue(nil)
			if job != nil {
				t.Errorf("delayed job should not be dequeueable immediately")
			}
		}
	})
}
