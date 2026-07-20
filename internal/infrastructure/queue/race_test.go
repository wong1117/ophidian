package queue

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestQueue_Race_EnqueueDequeue(t *testing.T) {
	q := NewPriorityQueue(nil)
	k := 200

	var wg sync.WaitGroup
	for i := 0; i < k; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			q.Enqueue(&Job{ID: fmt.Sprintf("rq-%d", idx), Handler: "test", Priority: idx % 10})
		}(i)
	}

	var wg2 sync.WaitGroup
	for i := 0; i < k/2; i++ {
		wg2.Add(1)
		go func() {
			defer wg2.Done()
			for j := 0; j < 5; j++ {
				job, _ := q.Dequeue(nil)
				if job != nil {
					q.Ack(nil, job.ID)
				}
			}
		}()
	}
	wg.Wait()
	wg2.Wait()
}

func TestQueue_Race_Delayed(t *testing.T) {
	q := NewPriorityQueue(nil)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			q.Enqueue(&Job{ID: fmt.Sprintf("delayed-%d", idx), Handler: "test", Delay: time.Nanosecond})
		}(i)
	}
	wg.Wait()
	for i := 0; i < 50; i++ {
		q.Dequeue(nil)
	}
}

func TestQueue_Race_NackRetry(t *testing.T) {
	q := NewPriorityQueue(nil)
	for i := 0; i < 100; i++ {
		q.Enqueue(&Job{ID: fmt.Sprintf("nr-%d", i), Handler: "test", MaxRetries: 5, Priority: i % 10})
	}

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				job, _ := q.Dequeue(nil)
				if job != nil {
					q.Nack(nil, job.ID, fmt.Errorf("error %s", job.ID))
				}
			}
		}()
	}
	wg.Wait()
}

func TestQueue_Race_Stats(t *testing.T) {
	q := NewPriorityQueue(nil)
	for i := 0; i < 100; i++ {
		q.Enqueue(&Job{ID: fmt.Sprintf("st-%d", i), Handler: "test"})
	}

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			q.Stats()
		}()
	}

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			q.Enqueue(&Job{ID: fmt.Sprintf("extra-%d", idx), Handler: "test"})
		}(i)
	}
	wg.Wait()
}
