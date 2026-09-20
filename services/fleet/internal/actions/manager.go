// Package actions is the approval gate: every state-mutating action becomes
// a PROPOSED pending_actions row first, and only executeAction (fired by a
// human clicking Approve) or a manual operator mutation ever changes fleet
// state (hard rule 1, DEC-016). This package owns that lifecycle: propose,
// execute, reject, event-driven invalidation, and the TTL backstop sweeper
// (ROADMAP.md S6).
package actions

import (
	"time"

	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/dispatch"
	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/store"
)

// Clock supplies "now". A minimal local interface rather than
// internal/simclock (S10's package, not yet built) — see DEC-029, replicated
// here for the same reason. Structurally compatible with the fuller
// simclock.Clock once S10 lands, so no change is needed here then.
type Clock interface {
	Now() time.Time
}

// Manager holds every dependency the approval-gate lifecycle needs.
type Manager struct {
	Store store.Store
	Queue *dispatch.Queue
	Clock Clock

	// OperatorID is the actor recorded as DecidedBy on every executeAction
	// and manual mutation (OQ-7: single hardcoded operator, no auth in
	// Phase 1). Execution is always a human action regardless of who
	// proposed it (SYSTEM or AGENT) — see DEC-016 and this package's DEC
	// entry on executeAction being one code path for both origins.
	OperatorID string

	// TTLSeconds is PENDING_ACTION_TTL_SECONDS (DEC-019): the backstop
	// sweeper expires a PROPOSED action older than this many seconds only
	// if no event-driven rule already caught it.
	TTLSeconds int
}

// NewManager constructs a Manager from its dependencies.
func NewManager(st store.Store, queue *dispatch.Queue, clock Clock, operatorID string, ttlSeconds int) *Manager {
	return &Manager{
		Store:      st,
		Queue:      queue,
		Clock:      clock,
		OperatorID: operatorID,
		TTLSeconds: ttlSeconds,
	}
}
