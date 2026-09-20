package actions

import (
	"context"
	"fmt"
	"time"

	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/domain"
)

// SweepExpired is the TTL backstop (DEC-019 mechanism 2): it expires every
// PROPOSED action older than TTLSeconds that no event-driven invalidation
// rule already caught. It is a safety net, not the primary mechanism — a
// TTL expiry is a signal an invalidation rule is missing, not expected
// steady-state behavior, so callers should treat a non-empty result as
// worth logging. It never dequeues (ROADMAP S6 c8b): the call an expired
// action targeted is still open and still needs a decision.
func (m *Manager) SweepExpired(ctx context.Context, now time.Time) ([]domain.PendingAction, error) {
	all, err := m.Store.ListPendingActions(ctx)
	if err != nil {
		return nil, fmt.Errorf("actions: sweep expired: list pending actions: %w", err)
	}

	ttl := time.Duration(m.TTLSeconds) * time.Second
	var expired []domain.PendingAction
	for _, a := range all {
		if a.Status != domain.ActionStatusProposed {
			continue
		}
		if now.Sub(a.CreatedAt) < ttl {
			continue
		}
		reason := domain.ExpiredReasonTTL
		a.Status = domain.ActionStatusExpired
		a.ExpiredReason = &reason
		decidedAt := now
		a.DecidedAt = &decidedAt
		if err := m.Store.UpdatePendingAction(ctx, a); err != nil {
			return nil, fmt.Errorf("actions: sweep expired: expire action %s: %w", a.ID, err)
		}
		expired = append(expired, a)
	}
	return expired, nil
}
