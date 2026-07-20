package queue

import (
	"fmt"
	"testing"
)

func BenchmarkQueue_Enqueue_LargeSet(b *testing.B) {
	q := NewPriorityQueue(nil)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < 1000; j++ {
			q.Enqueue(&Job{ID: fmt.Sprintf("be-%d-%d", i, j), Handler: "bench", Priority: j % 100})
		}
	}
}

func BenchmarkQueue_Dequeue_1000(b *testing.B) {
	for i := 0; i < b.N; i++ {
		q := NewPriorityQueue(nil)
		for j := 0; j < 1000; j++ {
			q.Enqueue(&Job{ID: fmt.Sprintf("bd-%d", j), Handler: "bench", Priority: j % 100})
		}
		for {
			j, _ := q.Dequeue(nil)
			if j == nil {
				break
			}
		}
	}
}

func BenchmarkQueue_PromoteDelayed(b *testing.B) {
	q := NewPriorityQueue(nil)
	for i := 0; i < 500; i++ {
		q.Enqueue(&Job{ID: fmt.Sprintf("del-%d", i), Handler: "bench", Delay: 0})
		q.Enqueue(&Job{ID: fmt.Sprintf("pend-%d", i), Handler: "bench", Priority: 1})
	}
	_ = q

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		q.mu.Lock()
		q.promoteDelayed()
		q.mu.Unlock()
	}
}

func BenchmarkQueue_Stats(b *testing.B) {
	q := NewPriorityQueue(nil)
	for i := 0; i < 2000; i++ {
		q.Enqueue(&Job{ID: fmt.Sprintf("st-%d", i), Handler: "bench", Priority: i % 50})
	}
	for i := 0; i < 1000; i++ {
		q.Enqueue(&Job{ID: fmt.Sprintf("del-%d", i), Handler: "bench", Delay: 1})
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		q.Stats()
	}
}
