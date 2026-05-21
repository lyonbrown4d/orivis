package collector

import "context"

func (r *Runner) closeDiscovery(ctx context.Context) {
	if r.discovery != nil {
		if err := r.discovery.Close(ctx); err != nil {
			r.logger.Warn("close monitor discovery failed", "error", err)
		}
	}
}

func (r *Runner) closeResultBuffer() {
	if r.results != nil {
		if err := r.results.Close(); err != nil {
			r.logger.Warn("close result buffer failed", "error", err)
		}
	}
}

func (r *Runner) closeChecker() {
	if r.checker != nil {
		if err := r.checker.Close(); err != nil {
			r.logger.Warn("close probe checker failed", "error", err)
		}
	}
}
