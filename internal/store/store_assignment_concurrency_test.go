package store_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/lyonbrown4d/orivis/internal/model"
	"github.com/lyonbrown4d/orivis/internal/store"
)

func TestMonitorAssignmentParallelSyncIsIdempotent(t *testing.T) {
	storage := newTestStore(t)
	agentOne := registerTestAgent(t, storage, "agent-parallel-01", []string{"dev"})
	_ = registerTestAgent(t, storage, "agent-parallel-02", []string{"dev"})

	monitor := createTestMonitorWithoutAssignment(t, storage, agentOne, "shared API")
	monitorIDs := []string{monitor.ID}

	const workerCount = 2
	const roundsPerWorker = 8
	errors := make(chan error, workerCount*roundsPerWorker)
	start := make(chan struct{})

	var wg sync.WaitGroup
	for range workerCount {
		wg.Go(func() {
			<-start
			for range roundsPerWorker {
				errors <- storage.MonitorStore().AssignMonitors(context.Background(), monitorIDs)
			}
		})
	}

	close(start)
	wg.Wait()
	close(errors)

	for err := range errors {
		if err != nil {
			t.Fatalf("parallel assign monitors failed: %v", err)
		}
	}

	if got := queryMonitorOwnerCount(t, storage, monitor.ID); got != 1 {
		t.Fatalf("expected one owner row after parallel sync, got %d", got)
	}
	agentID := queryMonitorOwnerAgentID(t, storage, monitor.ID)
	if agentID == "" {
		t.Fatal("expected monitor owner after parallel sync")
	}
}

func createTestMonitorWithoutAssignment(t *testing.T, storage *store.Store, agent model.Agent, name string) model.Monitor {
	t.Helper()
	environmentID := singleEnvironmentID(t, agent)
	monitor, err := storage.MonitorStore().Create(context.Background(), store.CreateMonitorParams{
		Name:              name,
		Type:              model.MonitorHTTP,
		Target:            "https://example.com/health",
		EnvironmentID:     environmentID,
		Enabled:           true,
		Interval:          30 * time.Second,
		Timeout:           5 * time.Second,
		RetryCount:        0,
		AggregationPolicy: model.AggregationMajorityDown,
	})
	if err != nil {
		t.Fatalf("create unassigned monitor: %v", err)
	}
	return monitor
}
