package agentclient

import (
	"context"
	"sync"
)

// RecordingClient is a test double for Client: it records every Triage call
// and can be configured to block (to test intake's concurrency cap, ROADMAP
// S5 c9) or fail (to test REL-2 — an agent error must never drop the call).
// It is not a _test.go file because internal/intake's own tests need to
// import it too (ROADMAP.md S5: "a recording fake used in tests").
type RecordingClient struct {
	mu       sync.Mutex
	requests []TriageRequest

	// Err is returned by every Triage call when non-nil.
	Err error
	// Block, when non-nil, is waited on before Triage returns. Closing it
	// releases every call currently blocked inside Triage.
	Block <-chan struct{}
}

// Triage records req, waits on Block if set (or until ctx is done), and
// returns Err.
func (f *RecordingClient) Triage(ctx context.Context, req TriageRequest) error {
	f.mu.Lock()
	f.requests = append(f.requests, req)
	f.mu.Unlock()

	if f.Block != nil {
		select {
		case <-f.Block:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return f.Err
}

// Requests returns a copy of every TriageRequest recorded so far.
func (f *RecordingClient) Requests() []TriageRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]TriageRequest, len(f.requests))
	copy(out, f.requests)
	return out
}

// Count reports how many Triage calls have been recorded so far.
func (f *RecordingClient) Count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.requests)
}
