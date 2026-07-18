package concurrency_test

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lyonbrown4d/orivis/internal/concurrency"
)

const defaultPoolWorkers = 1

type submitResult struct {
	index int
	err   error
}

type boundedConcurrencyTestHarness struct {
	submitWG      sync.WaitGroup
	submitResults chan submitResult
	started       chan struct{}
	done          chan struct{}
	release       chan struct{}
	running       atomic.Int64
	maxRunning    atomic.Int64
	breached      atomic.Bool
	submitted     atomic.Int64
	workers       int
}

type workerPoolSubmitter interface {
	Submit(task func()) error
}

func waitForSignals(t *testing.T, ch <-chan struct{}, n int, timeout time.Duration, message string) {
	t.Helper()

	timeoutCh := time.After(timeout)
	for range n {
		select {
		case <-ch:
		case <-timeoutCh:
			t.Fatal(message)
		}
	}
}

func collectSubmitResults(t *testing.T, results <-chan submitResult, expected int, timeout time.Duration) {
	t.Helper()

	timeoutCh := time.After(timeout)
	for range expected {
		select {
		case result := <-results:
			if result.err != nil {
				t.Errorf("task %d: unexpected submit error: %v", result.index, result.err)
			}
		case <-timeoutCh:
			t.Fatalf("did not observe all submit results within %s", timeout)
		}
	}
}

func waitForTaskCompletions(t *testing.T, done <-chan struct{}, expected int, timeout time.Duration) {
	t.Helper()

	timeoutCh := time.After(timeout)
	for range expected {
		select {
		case <-done:
		case <-timeoutCh:
			t.Fatalf("task did not complete within %s", timeout)
		}
	}
}

func newBoundedConcurrencyTestHarness(
	t *testing.T,
	pool workerPoolSubmitter,
	workers,
	tasks int,
) *boundedConcurrencyTestHarness {
	t.Helper()

	h := &boundedConcurrencyTestHarness{
		submitResults: make(chan submitResult, tasks),
		started:       make(chan struct{}, tasks),
		done:          make(chan struct{}, tasks),
		release:       make(chan struct{}),
		workers:       workers,
	}
	h.submitWG.Add(tasks)
	for i := range tasks {
		h.submitWG.Go(func() {
			h.submitTask(t, pool, i)
		})
	}
	return h
}

func (h *boundedConcurrencyTestHarness) submitTask(t *testing.T, pool workerPoolSubmitter, index int) {
	t.Helper()

	defer h.submitWG.Done()

	err := pool.Submit(func() {
		current := h.running.Add(1)
		h.recordMaxRunning(current)
		if current > int64(h.workers) {
			h.breached.Store(true)
		}

		h.started <- struct{}{}
		<-h.release
		h.running.Add(-1)
		h.done <- struct{}{}
	})
	h.submitResults <- submitResult{index: index, err: err}
	if err == nil {
		h.submitted.Add(1)
	}
}

func (h *boundedConcurrencyTestHarness) recordMaxRunning(current int64) {
	for {
		currentMax := h.maxRunning.Load()
		if current <= currentMax {
			return
		}
		if h.maxRunning.CompareAndSwap(currentMax, current) {
			return
		}
	}
}

func (h *boundedConcurrencyTestHarness) waitForSubmitCompletion(t *testing.T, timeout time.Duration) {
	t.Helper()

	done := make(chan struct{})
	go func() {
		defer close(done)
		h.submitWG.Wait()
	}()

	select {
	case <-done:
	case <-time.After(timeout):
		t.Fatalf("submitters did not finish within %s", timeout)
	}
}

func (h *boundedConcurrencyTestHarness) releaseWorkers() {
	close(h.release)
}

func (h *boundedConcurrencyTestHarness) submittedCount() int {
	return int(h.submitted.Load())
}

func (h *boundedConcurrencyTestHarness) assertWorkerLimitNotBreached(t *testing.T) {
	t.Helper()

	if h.breached.Load() {
		t.Fatalf("observed %d concurrent workers, want <= %d", h.maxRunning.Load(), h.workers)
	}
}

func TestNewWorkerPool_NormalizesBoundaryAndIllegalSizes(t *testing.T) {
	t.Helper()

	tests := []struct {
		name    string
		workers int
		wantCap int
	}{
		{
			name:    "zero_workers",
			workers: 0,
			wantCap: defaultPoolWorkers,
		},
		{
			name:    "negative_workers",
			workers: -9,
			wantCap: defaultPoolWorkers,
		},
		{
			name:    "single_worker",
			workers: 1,
			wantCap: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			pool, err := concurrency.NewWorkerPool(tt.workers)
			if err != nil {
				t.Fatalf("NewWorkerPool(%d): unexpected error: %v", tt.workers, err)
			}
			defer pool.Release()

			if got := pool.Cap(); got != tt.wantCap {
				t.Fatalf("NewWorkerPool(%d).Cap() = %d, want %d", tt.workers, got, tt.wantCap)
			}
		})
	}
}

func TestWorkerPoolSubmitExecutesTask(t *testing.T) {
	pool, err := concurrency.NewWorkerPool(1)
	if err != nil {
		t.Fatalf("NewWorkerPool(1): unexpected error: %v", err)
	}
	defer pool.Release()

	var ran atomic.Int64

	if err := pool.Submit(func() {
		ran.Add(1)
	}); err != nil {
		t.Fatalf("Submit: unexpected error: %v", err)
	}

	deadline := time.After(time.Second)
	for ran.Load() == 0 {
		select {
		case <-deadline:
			t.Fatalf("task did not execute within 1s")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func TestWorkerPoolSubmitHonorsWorkerLimit(t *testing.T) {
	t.Parallel()

	const (
		workers = 2
		tasks   = 8
	)

	pool, err := concurrency.NewWorkerPool(workers)
	if err != nil {
		t.Fatalf("NewWorkerPool(%d): unexpected error: %v", workers, err)
	}
	defer pool.Release()

	h := newBoundedConcurrencyTestHarness(t, pool, workers, tasks)
	waitForSignals(t, h.started, workers, time.Second, "no worker started within 1s")
	h.releaseWorkers()

	h.waitForSubmitCompletion(t, 2*time.Second)
	collectSubmitResults(t, h.submitResults, tasks, time.Second)
	waitForTaskCompletions(t, h.done, h.submittedCount(), time.Second)
	h.assertWorkerLimitNotBreached(t)
}

func TestWorkerPoolReleaseIdempotentAndRejectsSubmit(t *testing.T) {
	pool, err := concurrency.NewWorkerPool(1)
	if err != nil {
		t.Fatalf("NewWorkerPool(1): unexpected error: %v", err)
	}

	pool.Release()
	pool.Release()

	if !pool.IsClosed() {
		t.Fatalf("pool.IsClosed() = false after Release, want true")
	}

	if err := pool.Submit(func() {}); err == nil {
		t.Fatalf("Submit after Release should return error")
	}
}
