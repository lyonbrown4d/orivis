package probe

import (
	"context"
	"strings"
	"time"

	probing "github.com/prometheus-community/pro-bing"

	"github.com/lyonbrown4d/orivis/internal/model"
	"github.com/lyonbrown4d/orivis/internal/protocol"
)

func (c *Checker) checkPing(ctx context.Context, task protocol.AgentTask) (model.Status, map[string]any, error) {
	target := strings.TrimSpace(dnsTargetHost(task.Target))
	if target == "" {
		return model.StatusDown, map[string]any{"target": task.Target}, newError("ping target is empty")
	}
	pinger, err := probing.NewPinger(target)
	if err != nil {
		return model.StatusDown, map[string]any{"target": target}, wrapError(err, "create ping probe")
	}
	timeout := taskTimeout(task)
	pinger.Count = 1
	pinger.Timeout = timeout
	pinger.ResolveTimeout = timeout
	pinger.Interval = timeout
	pinger.RecordRtts = true
	pinger.SetPrivileged(false)

	pingCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := pinger.RunWithContext(pingCtx); err != nil {
		return pingResult(target, pinger.Statistics(), err)
	}
	return pingResult(target, pinger.Statistics(), nil)
}

func pingResult(target string, stats *probing.Statistics, err error) (model.Status, map[string]any, error) {
	detail := pingDetail(target, stats)
	if stats != nil && stats.PacketsRecv > 0 {
		return model.StatusUp, detail, nil
	}
	if err != nil {
		return model.StatusDown, detail, wrapError(err, "execute ping probe")
	}
	return model.StatusDown, detail, errorf("no ping replies from %s", target)
}

func pingDetail(target string, stats *probing.Statistics) map[string]any {
	detail := map[string]any{"target": target}
	if stats == nil {
		return detail
	}
	detail["packets_sent"] = stats.PacketsSent
	detail["packets_recv"] = stats.PacketsRecv
	detail["packet_loss"] = stats.PacketLoss
	if stats.AvgRtt > 0 {
		detail["avg_rtt_ms"] = float64(stats.AvgRtt) / float64(time.Millisecond)
	}
	return detail
}
