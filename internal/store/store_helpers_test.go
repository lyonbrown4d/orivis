package store_test

import (
	"context"
	"testing"

	"github.com/lyonbrown4d/orivis/internal/store"
)

func queryMonitorOwnerCount(t *testing.T, storage *store.Store, monitorID string) int {
	t.Helper()
	var count int
	if err := storage.DB.QueryRowContext(
		context.Background(),
		"SELECT COUNT(1) FROM monitor_agents WHERE monitor_id = ?",
		monitorID,
	).Scan(&count)
	err != nil {
		t.Fatalf("count monitor owners: %v", err)
	}
	return count
}

func queryMonitorOwnerAgentID(t *testing.T, storage *store.Store, monitorID string) string {
	t.Helper()
	var agentID string
	err := storage.DB.QueryRowContext(
		context.Background(),
		"SELECT agent_id FROM monitor_agents WHERE monitor_id = ?",
		monitorID,
	).Scan(&agentID)
	if err != nil {
		return ""
	}
	return agentID
}

