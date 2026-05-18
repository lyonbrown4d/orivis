package collector

import (
	"hash/fnv"
	"strconv"
	"time"

	"github.com/lyonbrown4d/orivis/internal/protocol"
)

func taskInterval(task protocol.AgentTask, fallback time.Duration) time.Duration {
	if task.IntervalSeconds > 0 {
		return time.Duration(task.IntervalSeconds) * time.Second
	}
	if fallback > 0 {
		return fallback
	}
	return 30 * time.Second
}

func taskInitialJitter(task protocol.AgentTask, configured, interval time.Duration) time.Duration {
	if configured <= 0 {
		return 0
	}
	maxJitter := configured
	if interval > 0 && interval/2 < maxJitter {
		maxJitter = interval / 2
	}
	nanos := maxJitter.Nanoseconds()
	if nanos <= 0 {
		return 0
	}
	hash := fnv.New32a()
	if _, err := hash.Write([]byte(task.MonitorID + "\x00" + task.Target)); err != nil {
		return 0
	}
	return time.Duration(int64(hash.Sum32()) % nanos)
}

func taskSignature(task protocol.AgentTask) string {
	return task.Type + "\x00" + task.Target + "\x00" + strconv.Itoa(task.IntervalSeconds) + "\x00" + strconv.Itoa(task.TimeoutSeconds)
}

func taskTag(monitorID string) string {
	return "monitor:" + monitorID
}
