// Package concurrency provides shared helpers for managing bounded worker pools.
package concurrency

import (
	"fmt"
	"runtime"

	"github.com/panjf2000/ants/v2"
)

const defaultPoolWorkers = 1

func NewWorkerPool(workers int) (*ants.Pool, error) {
	if workers <= 0 {
		workers = defaultPoolWorkers
	}
	pool, err := ants.NewPool(workers)
	if err != nil {
		return nil, fmt.Errorf("create worker pool: %w", err)
	}
	return pool, nil
}

func DefaultWorkerPool() (*ants.Pool, error) {
	return NewWorkerPool(runtime.NumCPU())
}
