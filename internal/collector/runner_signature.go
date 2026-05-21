package collector

import (
	"hash"
	"hash/fnv"
	"sort"
	"strconv"
	"strings"

	"github.com/lyonbrown4d/orivis/internal/protocol"
	"github.com/samber/oops"
)

func (r *Runner) shouldSyncDiscovery(signature string, count int) bool {
	r.discoverySignatureMu.Lock()
	defer r.discoverySignatureMu.Unlock()
	if count == 0 {
		if r.lastDiscoverySignature == "" {
			return false
		}
		r.lastDiscoverySignature = ""
		r.lastDiscoveryMonitorCount = 0
		return false
	}
	if r.lastDiscoverySignature == signature && r.lastDiscoveryMonitorCount == count {
		return false
	}
	return true
}

func (r *Runner) rememberDiscoverySignature(signature string, count int) {
	r.discoverySignatureMu.Lock()
	r.lastDiscoverySignature = signature
	r.lastDiscoveryMonitorCount = count
	r.discoverySignatureMu.Unlock()
}

func discoverySignature(monitors []protocol.AgentDiscoveredMonitor) string {
	if len(monitors) == 0 {
		return ""
	}
	keys := make([]string, 0, len(monitors))
	for i := range monitors {
		keys = append(keys, discoveredMonitorSignatureKey(monitors[i]))
	}
	sort.Strings(keys)

	hasher := fnv.New64a()
	for i := range keys {
		if err := signatureWrite(hasher, keys[i]); err != nil {
			return ""
		}
	}
	return strconv.FormatUint(hasher.Sum64(), 16)
}

func discoveredMonitorSignatureKey(m protocol.AgentDiscoveredMonitor) string {
	return discoveredMonitorIdentityKey(m, "")
}

func discoveredMonitorKey(m protocol.AgentDiscoveredMonitor) string {
	return discoveredMonitorIdentityKey(m, "nil")
}

func discoveredMonitorIdentityKey(m protocol.AgentDiscoveredMonitor, nilEnabled string) string {
	enabled := nilEnabled
	if m.Enabled != nil {
		enabled = strconv.FormatBool(*m.Enabled)
	}
	return strings.Join([]string{
		m.SourceKey,
		m.Name,
		m.Type,
		m.Target,
		m.GroupName,
		m.EnvironmentCode,
		strconv.Itoa(m.IntervalSeconds),
		strconv.Itoa(m.TimeoutSeconds),
		strconv.Itoa(m.RetryCount),
		m.AggregationPolicy,
		enabled,
	}, "\x1f")
}

func signatureWrite(hasher hash.Hash64, value string) error {
	if _, err := hasher.Write([]byte(value)); err != nil {
		return oops.Wrapf(err, "write signature hash value")
	}
	if _, err := hasher.Write([]byte{0}); err != nil {
		return oops.Wrapf(err, "write signature hash delimiter")
	}
	return nil
}
