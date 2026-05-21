package collector

import "time"

const (
	resultFlushBackoffBase = 5 * time.Second
	resultFlushBackoffMax  = 2 * time.Minute
)

type resultFlushBackoff struct {
	failures    int
	nextAttempt time.Time
}

func (b resultFlushBackoff) CanAttempt(now time.Time) bool {
	return b.nextAttempt.IsZero() || !now.Before(b.nextAttempt)
}

func (b *resultFlushBackoff) RecordFailure(now time.Time) time.Duration {
	b.failures++
	delay := resultFlushBackoffBase
	for i := 1; i < b.failures; i++ {
		delay *= 2
		if delay >= resultFlushBackoffMax {
			delay = resultFlushBackoffMax
			break
		}
	}
	b.nextAttempt = now.Add(delay)
	return delay
}

func (b *resultFlushBackoff) Reset() {
	b.failures = 0
	b.nextAttempt = time.Time{}
}
