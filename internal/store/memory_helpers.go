package store

import (
	"fmt"
	"sort"
	"strings"
	"time"

	collectionlist "github.com/arcgolabs/collectionx/list"
	"github.com/hashicorp/go-memdb"
	"github.com/lyonbrown4d/orivis/internal/model"
)

func ensureMemoryRegion(txn *memdb.Txn, code string, now time.Time) (string, error) {
	raw, err := txn.First(memoryTableRegions, "code", code)
	if err != nil {
		return "", fmt.Errorf("find memory region: %w", err)
	}
	if raw != nil {
		region, valueErr := memoryValue[model.Region](raw, "region")
		if valueErr != nil {
			return "", valueErr
		}
		return region.ID, nil
	}

	id, err := newID("reg")
	if err != nil {
		return "", err
	}
	region := model.Region{
		ID:        id,
		Name:      code,
		Code:      code,
		Enabled:   true,
		Source:    model.ConfigSourceAPI,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := txn.Insert(memoryTableRegions, &region); err != nil {
		return "", fmt.Errorf("insert memory region: %w", err)
	}
	return id, nil
}

func ensureMemoryEnvironments(txn *memdb.Txn, codes []string, now time.Time) ([]string, error) {
	out := make([]string, 0, len(codes))
	for _, code := range codes {
		code = normalizeCode(code)
		if code == "" {
			continue
		}
		id, err := ensureMemoryEnvironment(txn, code, now)
		if err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, nil
}

func ensureMemoryEnvironment(txn *memdb.Txn, code string, now time.Time) (string, error) {
	raw, err := txn.First(memoryTableEnvironments, "code", code)
	if err != nil {
		return "", fmt.Errorf("find memory environment: %w", err)
	}
	if raw != nil {
		environment, valueErr := memoryValue[model.Environment](raw, "environment")
		if valueErr != nil {
			return "", valueErr
		}
		return environment.ID, nil
	}

	id, err := newID("env")
	if err != nil {
		return "", err
	}
	environment := model.Environment{
		ID:        id,
		Name:      code,
		Code:      code,
		Enabled:   true,
		Source:    model.ConfigSourceAPI,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := txn.Insert(memoryTableEnvironments, &environment); err != nil {
		return "", fmt.Errorf("insert memory environment: %w", err)
	}
	return id, nil
}

func replaceMemoryAgent(txn *memdb.Txn, existing *model.Agent, next model.Agent) error {
	if err := txn.Delete(memoryTableAgents, existing); err != nil {
		return fmt.Errorf("replace memory agent: %w", err)
	}
	inserted := cloneAgent(&next)
	if err := txn.Insert(memoryTableAgents, &inserted); err != nil {
		return fmt.Errorf("insert replacement memory agent: %w", err)
	}
	return nil
}

func replaceMemoryAgentEnvironments(txn *memdb.Txn, agentID string, environmentIDs []string) error {
	it, err := txn.Get(memoryTableAgentEnvironments, "agent", agentID)
	if err != nil {
		return fmt.Errorf("list memory agent environments: %w", err)
	}
	existing := make([]any, 0)
	for raw := it.Next(); raw != nil; raw = it.Next() {
		existing = append(existing, raw)
	}
	if err := deleteMemoryRows(txn, memoryTableAgentEnvironments, existing, "memory agent environment"); err != nil {
		return err
	}
	for _, environmentID := range environmentIDs {
		link := memoryAgentEnvironment{
			ID:            memoryJoinID(agentID, environmentID),
			AgentID:       agentID,
			EnvironmentID: environmentID,
		}
		if err := txn.Insert(memoryTableAgentEnvironments, &link); err != nil {
			return fmt.Errorf("insert memory agent environment: %w", err)
		}
	}
	return nil
}

func memoryAgentEnvironmentIDs(txn *memdb.Txn, agentID string) ([]string, error) {
	it, err := txn.Get(memoryTableAgentEnvironments, "agent", agentID)
	if err != nil {
		return nil, fmt.Errorf("query memory agent environments: %w", err)
	}
	ids := make([]string, 0)
	for raw := it.Next(); raw != nil; raw = it.Next() {
		link, err := memoryValue[memoryAgentEnvironment](raw, "agent environment link")
		if err != nil {
			return nil, err
		}
		ids = append(ids, link.EnvironmentID)
	}
	sort.Strings(ids)
	return ids, nil
}

func cloneAgent(agent *model.Agent) model.Agent {
	if agent == nil {
		return model.Agent{}
	}
	out := *agent
	if agent.EnvironmentIDs != nil {
		out.EnvironmentIDs = collectionlist.NewList[string](agent.EnvironmentIDs.Values()...)
	}
	return out
}

func cloneMonitor(monitor *model.Monitor) model.Monitor {
	if monitor == nil {
		return model.Monitor{}
	}
	return *monitor
}

func cloneProbeResult(result *model.ProbeResult) model.ProbeResult {
	if result == nil {
		return model.ProbeResult{}
	}
	out := *result
	out.RawDetail = append([]byte(nil), result.RawDetail...)
	return out
}

func memoryJoinID(parts ...string) string {
	return strings.Join(parts, "\x00")
}

func memoryValue[T any](raw any, name string) (*T, error) {
	value, ok := raw.(*T)
	if !ok {
		return nil, fmt.Errorf("unexpected memory %s value %T", name, raw)
	}
	return value, nil
}
