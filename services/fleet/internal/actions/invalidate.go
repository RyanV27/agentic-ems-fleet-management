package actions

import (
	"context"
	"fmt"
	"time"

	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/domain"
)

// expireMatching transitions every PROPOSED action (other than excludeID)
// whose payload subject satisfies match to EXPIRED with reason, recording
// the invalidating cause (DEC-019). This is the one place invalidation
// writes pending_actions, shared by every trigger in this file.
func (m *Manager) expireMatching(ctx context.Context, reason domain.ExpiredReason, excludeID string, match func(subjectRefs) bool, now time.Time) error {
	all, err := m.Store.ListPendingActions(ctx)
	if err != nil {
		return fmt.Errorf("actions: expire matching: list pending actions: %w", err)
	}
	for _, a := range all {
		if a.ID == excludeID || a.Status != domain.ActionStatusProposed {
			continue
		}
		refs, err := payloadSubject(a)
		if err != nil {
			return fmt.Errorf("actions: expire matching: %w", err)
		}
		if !match(refs) {
			continue
		}
		if !IsLegalTransition(a.Status, domain.ActionStatusExpired) {
			continue
		}
		expiredAt := now
		a.Status = domain.ActionStatusExpired
		a.ExpiredReason = &reason
		a.DecidedAt = &expiredAt
		if err := m.Store.UpdatePendingAction(ctx, a); err != nil {
			return fmt.Errorf("actions: expire action %s: %w", a.ID, err)
		}
	}
	return nil
}

// invalidateUnitReassigned expires every other PROPOSED action that targets
// unitID: the unit has just committed elsewhere, so any other proposal
// planning to use it is moot (DEC-019 trigger 1).
func (m *Manager) invalidateUnitReassigned(ctx context.Context, unitID, excludeID string, now time.Time) error {
	return m.expireMatching(ctx, domain.ExpiredReasonUnitReassigned, excludeID,
		func(r subjectRefs) bool { return r.UnitID == unitID }, now)
}

// invalidateCallSuperseded expires every other PROPOSED action that targets
// callID: the call has just been resolved by the action that executed, so
// any other proposal targeting it is superseded (DEC-019 trigger 3).
func (m *Manager) invalidateCallSuperseded(ctx context.Context, callID, excludeID string, now time.Time) error {
	return m.expireMatching(ctx, domain.ExpiredReasonSuperseded, excludeID,
		func(r subjectRefs) bool { return r.CallID == callID }, now)
}

// invalidateEventSuperseded expires every other PROPOSED action that targets
// dispatchEventID: the event has just been resolved by the action that
// executed, so any other proposal targeting it is superseded.
func (m *Manager) invalidateEventSuperseded(ctx context.Context, dispatchEventID, excludeID string, now time.Time) error {
	return m.expireMatching(ctx, domain.ExpiredReasonSuperseded, excludeID,
		func(r subjectRefs) bool { return r.EventID == dispatchEventID }, now)
}

// InvalidateUnitOutOfService expires every PROPOSED action targeting unitID
// because the unit has gone OUT_OF_SERVICE (DEC-019 trigger 1). Exposed
// directly (not gated behind a proposal) because taking a unit out of
// service is an operational fact, not a dispatch decision — see DEC-037.
func (m *Manager) InvalidateUnitOutOfService(ctx context.Context, unitID string, now time.Time) error {
	return m.expireMatching(ctx, domain.ExpiredReasonUnitOutOfService, "",
		func(r subjectRefs) bool { return r.UnitID == unitID }, now)
}

// InvalidateCallClosed expires every PROPOSED action targeting callID
// because the call has been closed or cancelled (DEC-019 trigger 2), and
// dequeues it (ROADMAP S6 c8b: close/cancel is a Remove trigger, unlike
// ordinary proposal expiry which never dequeues).
func (m *Manager) InvalidateCallClosed(ctx context.Context, callID string, now time.Time) error {
	if err := m.expireMatching(ctx, domain.ExpiredReasonCallClosed, "",
		func(r subjectRefs) bool { return r.CallID == callID }, now); err != nil {
		return err
	}
	m.Queue.Remove(callID, now)
	return nil
}
