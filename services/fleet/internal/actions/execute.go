package actions

import (
	"context"
	"fmt"
	"time"

	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/dispatch"
	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/domain"
)

// executeOutcome is the queue side effect of a successful mutation, applied
// after the transaction commits — the queue is in-memory, not part of the
// DB (ROADMAP S6 c8b). Gathering it uniformly is what lets ExecuteAction and
// the manual mutations in manual.go share one mutation core per ActionType
// regardless of caller (ROADMAP S6 c3).
type executeOutcome struct {
	removeFromQueue []string
	pushToQueue     []dispatch.QueueItem
}

// ExecuteAction is the only mutation that transitions a PendingAction to
// EXECUTED (hard rule 1, DEC-016). It is identical regardless of
// proposedBy — SYSTEM and AGENT proposals both funnel through here
// (ROADMAP S6 c3); manual operator mutations use the same mutate* cores
// directly, skipping the PROPOSED phase, since the operator's click is
// itself the approval (DEC-016). Preconditions are revalidated against live
// state immediately before mutating (c4): if they fail, the action is
// marked FAILED, the fleet is left unchanged, and a *PreconditionError is
// returned. Everything — the read, the precondition check, the mutation,
// and the pending_actions status transition — happens in one transaction
// (c2).
func (m *Manager) ExecuteAction(ctx context.Context, actionID string) (domain.PendingAction, error) {
	now := m.Clock.Now()

	var result domain.PendingAction
	var precondErr error
	var outcome executeOutcome

	err := m.Store.WithTx(ctx, func(ctx context.Context) error {
		a, err := m.Store.GetPendingAction(ctx, actionID)
		if err != nil {
			return fmt.Errorf("actions: execute action %s: %w", actionID, err)
		}
		if a.Status != domain.ActionStatusProposed {
			return &TransitionError{ActionID: a.ID, From: a.Status, To: domain.ActionStatusExecuted}
		}

		switch a.Type {
		case domain.ActionTypeAssignCall:
			p, perr := unmarshalAssignCallPayload(a.Payload)
			if perr != nil {
				return perr
			}
			outcome, precondErr, err = m.mutateAssignCall(ctx, p.CallID, p.UnitID, a.ID, now)
		case domain.ActionTypeRerouteUnit:
			p, perr := unmarshalRerouteUnitPayload(a.Payload)
			if perr != nil {
				return perr
			}
			outcome, precondErr, err = m.mutateRerouteUnit(ctx, p.UnitID, p.CallID, a.ID, now)
		case domain.ActionTypeAssignBackupUnit:
			p, perr := unmarshalAssignBackupUnitPayload(a.Payload)
			if perr != nil {
				return perr
			}
			outcome, precondErr, err = m.mutateAssignBackupUnit(ctx, p.CallID, p.UnitID, a.ID, now)
		case domain.ActionTypeResolveEvent:
			p, perr := unmarshalResolveEventPayload(a.Payload)
			if perr != nil {
				return perr
			}
			outcome, precondErr, err = m.mutateResolveEvent(ctx, p.DispatchEventID, a.ID, now)
		default:
			err = fmt.Errorf("actions: execute action %s: unknown action type %q", a.ID, a.Type)
		}
		if err != nil {
			return err
		}

		if precondErr != nil {
			a.Status = domain.ActionStatusFailed
		} else {
			a.Status = domain.ActionStatusExecuted
		}
		decidedAt := now
		decidedBy := m.OperatorID
		a.DecidedAt = &decidedAt
		a.DecidedBy = &decidedBy
		if err := m.Store.UpdatePendingAction(ctx, a); err != nil {
			return fmt.Errorf("actions: execute action %s: update status: %w", a.ID, err)
		}
		result = a
		return nil
	})
	if err != nil {
		return domain.PendingAction{}, err
	}

	m.applyOutcome(outcome, now)

	if precondErr != nil {
		return result, precondErr
	}
	return result, nil
}

func (m *Manager) applyOutcome(outcome executeOutcome, now time.Time) {
	for _, callID := range outcome.removeFromQueue {
		m.Queue.Remove(callID, now)
	}
	for _, item := range outcome.pushToQueue {
		m.Queue.Push(item, now)
	}
}

// mutateAssignCall is ASSIGN_CALL's mutation core, shared by ExecuteAction
// and ManualAssignCall. excludeActionID is the pending action performing
// this mutation (skipped by invalidation so it isn't expired by its own
// side effect); manual callers pass "" since no such row exists yet.
func (m *Manager) mutateAssignCall(ctx context.Context, callID, unitID, excludeActionID string, now time.Time) (executeOutcome, error, error) {
	call, err := m.Store.GetCall(ctx, callID)
	if err != nil {
		return executeOutcome{}, nil, fmt.Errorf("actions: assign call: %w", err)
	}
	unit, err := m.Store.GetUnit(ctx, unitID)
	if err != nil {
		return executeOutcome{}, nil, fmt.Errorf("actions: assign call: %w", err)
	}
	if err := checkAssignCall(call, unit); err != nil {
		return executeOutcome{}, err, nil
	}

	unit.Status = domain.UnitStatusEnRoute
	unit.CurrentCallID = &call.ID
	unit.StatusChangedAt = now
	if err := m.Store.UpdateUnit(ctx, unit); err != nil {
		return executeOutcome{}, nil, fmt.Errorf("actions: assign call: update unit: %w", err)
	}
	call.Status = domain.CallStatusAssigned
	call.AssignedUnitID = &unit.ID
	if err := m.Store.UpdateCall(ctx, call); err != nil {
		return executeOutcome{}, nil, fmt.Errorf("actions: assign call: update call: %w", err)
	}

	if err := m.invalidateUnitReassigned(ctx, unit.ID, excludeActionID, now); err != nil {
		return executeOutcome{}, nil, err
	}
	if err := m.invalidateCallSuperseded(ctx, call.ID, excludeActionID, now); err != nil {
		return executeOutcome{}, nil, err
	}
	return executeOutcome{removeFromQueue: []string{call.ID}}, nil, nil
}

// mutateRerouteUnit is REROUTE_UNIT's mutation core. The unit's previous
// call (read from Unit.CurrentCallID, not the caller) reopens awaiting a
// new decision; its EnqueuedAt is untouched (DEC-021 write-once aging
// input), so it re-enters the operator queue at its true wait time, not a
// reset one (see DEC-037).
func (m *Manager) mutateRerouteUnit(ctx context.Context, unitID, callID, excludeActionID string, now time.Time) (executeOutcome, error, error) {
	unit, err := m.Store.GetUnit(ctx, unitID)
	if err != nil {
		return executeOutcome{}, nil, fmt.Errorf("actions: reroute unit: %w", err)
	}
	targetCall, err := m.Store.GetCall(ctx, callID)
	if err != nil {
		return executeOutcome{}, nil, fmt.Errorf("actions: reroute unit: %w", err)
	}
	if err := checkRerouteUnit(targetCall, unit); err != nil {
		return executeOutcome{}, err, nil
	}

	previousCallID := *unit.CurrentCallID
	previousCall, err := m.Store.GetCall(ctx, previousCallID)
	if err != nil {
		return executeOutcome{}, nil, fmt.Errorf("actions: reroute unit: previous call: %w", err)
	}

	previousCall.Status = domain.CallStatusPendingApproval
	previousCall.AssignedUnitID = nil
	if err := m.Store.UpdateCall(ctx, previousCall); err != nil {
		return executeOutcome{}, nil, fmt.Errorf("actions: reroute unit: reopen previous call: %w", err)
	}

	unit.CurrentCallID = &targetCall.ID
	unit.Status = domain.UnitStatusEnRoute
	unit.StatusChangedAt = now
	if err := m.Store.UpdateUnit(ctx, unit); err != nil {
		return executeOutcome{}, nil, fmt.Errorf("actions: reroute unit: update unit: %w", err)
	}
	targetCall.Status = domain.CallStatusAssigned
	targetCall.AssignedUnitID = &unit.ID
	if err := m.Store.UpdateCall(ctx, targetCall); err != nil {
		return executeOutcome{}, nil, fmt.Errorf("actions: reroute unit: update target call: %w", err)
	}

	if err := m.invalidateUnitReassigned(ctx, unit.ID, excludeActionID, now); err != nil {
		return executeOutcome{}, nil, err
	}
	if err := m.invalidateCallSuperseded(ctx, targetCall.ID, excludeActionID, now); err != nil {
		return executeOutcome{}, nil, err
	}

	outcome := executeOutcome{removeFromQueue: []string{targetCall.ID}}
	if previousCall.EnqueuedAt != nil {
		outcome.pushToQueue = []dispatch.QueueItem{{
			CallID:     previousCall.ID,
			Severity:   previousCall.Severity,
			EnqueuedAt: *previousCall.EnqueuedAt,
		}}
	}
	return outcome, nil, nil
}

// mutateAssignBackupUnit is ASSIGN_BACKUP_UNIT's mutation core: it replaces
// whatever unit (if any) is currently assigned to callID with unitID,
// freeing the replaced unit back to AVAILABLE (see DEC-037).
func (m *Manager) mutateAssignBackupUnit(ctx context.Context, callID, unitID, excludeActionID string, now time.Time) (executeOutcome, error, error) {
	call, err := m.Store.GetCall(ctx, callID)
	if err != nil {
		return executeOutcome{}, nil, fmt.Errorf("actions: assign backup unit: %w", err)
	}
	backup, err := m.Store.GetUnit(ctx, unitID)
	if err != nil {
		return executeOutcome{}, nil, fmt.Errorf("actions: assign backup unit: %w", err)
	}
	if err := checkAssignBackupUnit(call, backup); err != nil {
		return executeOutcome{}, err, nil
	}

	if call.AssignedUnitID != nil && *call.AssignedUnitID != backup.ID {
		previous, err := m.Store.GetUnit(ctx, *call.AssignedUnitID)
		if err != nil {
			return executeOutcome{}, nil, fmt.Errorf("actions: assign backup unit: previous unit: %w", err)
		}
		previous.Status = domain.UnitStatusAvailable
		previous.CurrentCallID = nil
		previous.StatusChangedAt = now
		if err := m.Store.UpdateUnit(ctx, previous); err != nil {
			return executeOutcome{}, nil, fmt.Errorf("actions: assign backup unit: free previous unit: %w", err)
		}
	}

	backup.Status = domain.UnitStatusEnRoute
	backup.CurrentCallID = &call.ID
	backup.StatusChangedAt = now
	if err := m.Store.UpdateUnit(ctx, backup); err != nil {
		return executeOutcome{}, nil, fmt.Errorf("actions: assign backup unit: update backup unit: %w", err)
	}
	call.Status = domain.CallStatusAssigned
	call.AssignedUnitID = &backup.ID
	if err := m.Store.UpdateCall(ctx, call); err != nil {
		return executeOutcome{}, nil, fmt.Errorf("actions: assign backup unit: update call: %w", err)
	}

	if err := m.invalidateUnitReassigned(ctx, backup.ID, excludeActionID, now); err != nil {
		return executeOutcome{}, nil, err
	}
	if err := m.invalidateCallSuperseded(ctx, call.ID, excludeActionID, now); err != nil {
		return executeOutcome{}, nil, err
	}
	return executeOutcome{removeFromQueue: []string{call.ID}}, nil, nil
}

// mutateResolveEvent is RESOLVE_EVENT's mutation core.
func (m *Manager) mutateResolveEvent(ctx context.Context, dispatchEventID, excludeActionID string, now time.Time) (executeOutcome, error, error) {
	event, err := m.Store.GetDispatchEvent(ctx, dispatchEventID)
	if err != nil {
		return executeOutcome{}, nil, fmt.Errorf("actions: resolve event: %w", err)
	}
	if err := checkResolveEvent(event); err != nil {
		return executeOutcome{}, err, nil
	}

	event.Status = domain.DispatchEventStatusResolved
	resolvedAt := now
	event.ResolvedAt = &resolvedAt
	if err := m.Store.UpdateDispatchEvent(ctx, event); err != nil {
		return executeOutcome{}, nil, fmt.Errorf("actions: resolve event: %w", err)
	}

	if err := m.invalidateEventSuperseded(ctx, event.ID, excludeActionID, now); err != nil {
		return executeOutcome{}, nil, err
	}
	return executeOutcome{}, nil, nil
}
