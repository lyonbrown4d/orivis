package notification_test

import (
	"encoding/json"
	"net/http"
	"sync"
	"testing"
	"time"
)

type alertmanagerPayloadEvent struct {
	Status   string            `json:"status"`
	Labels   map[string]string `json:"labels"`
	StartsAt string            `json:"startsAt"`
	EndsAt   string            `json:"endsAt"`
}

type alertmanagerPayloadRecorder struct {
	t      *testing.T
	mu     sync.Mutex
	events []alertmanagerPayloadEvent
}

func newAlertmanagerPayloadRecorder(t *testing.T) *alertmanagerPayloadRecorder {
	t.Helper()
	return &alertmanagerPayloadRecorder{t: t}
}

func (r *alertmanagerPayloadRecorder) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	defer closeRequestBody(r.t, req)
	var payload []alertmanagerPayloadEvent
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	r.mu.Lock()
	r.events = append(r.events, payload...)
	r.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

func (r *alertmanagerPayloadRecorder) waitPayloads(t *testing.T, count int) []alertmanagerPayloadEvent {
	t.Helper()
	deadline := time.After(2 * time.Second)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-deadline:
			t.Fatalf("expected %d alertmanager payloads, got %#v", count, r.snapshot())
		case <-ticker.C:
			if events := r.snapshot(); len(events) >= count {
				return events
			}
		}
	}
}

func (r *alertmanagerPayloadRecorder) snapshot() []alertmanagerPayloadEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]alertmanagerPayloadEvent(nil), r.events...)
}
