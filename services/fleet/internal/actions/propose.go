package actions

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/domain"
)

// ProposeInput is common to every Propose* call. IdempotencyKey may be left
// empty to use the type's default derivation (type + subject ids), which is
// enough for retry-safety of a single proposer re-evaluating the same
// situation; callers that need a different notion of "the same proposal"
// (e.g. the agent re-running a tool call) may supply their own.
type ProposeInput struct {
	ProposedBy     domain.ProposedBy
	Rationale      string
	IdempotencyKey string
	AgentRunID     *string
	ParentActionID *string
}

// ProposeAssignCall inserts a PROPOSED ASSIGN_CALL row. It does not mutate
// any fleet state or any other pending_actions row (ROADMAP S6 c1) — the
// call and unit are read only to fail fast on an obviously nonexistent
// subject; full precondition revalidation happens at executeAction time.
func (m *Manager) ProposeAssignCall(ctx context.Context, callID, unitID string, in ProposeInput) (domain.PendingAction, error) {
	if _, err := m.Store.GetCall(ctx, callID); err != nil {
		return domain.PendingAction{}, fmt.Errorf("actions: propose assign call: %w", err)
	}
	if _, err := m.Store.GetUnit(ctx, unitID); err != nil {
		return domain.PendingAction{}, fmt.Errorf("actions: propose assign call: %w", err)
	}
	if in.IdempotencyKey == "" {
		in.IdempotencyKey = fmt.Sprintf("%s:%s:%s", domain.ActionTypeAssignCall, callID, unitID)
	}
	payload, err := marshalPayload(AssignCallPayload{CallID: callID, UnitID: unitID})
	if err != nil {
		return domain.PendingAction{}, err
	}
	return m.insertProposal(ctx, domain.ActionTypeAssignCall, payload, in)
}

// ProposeRerouteUnit inserts a PROPOSED REROUTE_UNIT row.
func (m *Manager) ProposeRerouteUnit(ctx context.Context, unitID, callID string, in ProposeInput) (domain.PendingAction, error) {
	if _, err := m.Store.GetUnit(ctx, unitID); err != nil {
		return domain.PendingAction{}, fmt.Errorf("actions: propose reroute unit: %w", err)
	}
	if _, err := m.Store.GetCall(ctx, callID); err != nil {
		return domain.PendingAction{}, fmt.Errorf("actions: propose reroute unit: %w", err)
	}
	if in.IdempotencyKey == "" {
		in.IdempotencyKey = fmt.Sprintf("%s:%s:%s", domain.ActionTypeRerouteUnit, unitID, callID)
	}
	payload, err := marshalPayload(RerouteUnitPayload{UnitID: unitID, CallID: callID})
	if err != nil {
		return domain.PendingAction{}, err
	}
	return m.insertProposal(ctx, domain.ActionTypeRerouteUnit, payload, in)
}

// ProposeAssignBackupUnit inserts a PROPOSED ASSIGN_BACKUP_UNIT row.
func (m *Manager) ProposeAssignBackupUnit(ctx context.Context, callID, unitID string, in ProposeInput) (domain.PendingAction, error) {
	if _, err := m.Store.GetCall(ctx, callID); err != nil {
		return domain.PendingAction{}, fmt.Errorf("actions: propose assign backup unit: %w", err)
	}
	if _, err := m.Store.GetUnit(ctx, unitID); err != nil {
		return domain.PendingAction{}, fmt.Errorf("actions: propose assign backup unit: %w", err)
	}
	if in.IdempotencyKey == "" {
		in.IdempotencyKey = fmt.Sprintf("%s:%s:%s", domain.ActionTypeAssignBackupUnit, callID, unitID)
	}
	payload, err := marshalPayload(AssignBackupUnitPayload{CallID: callID, UnitID: unitID})
	if err != nil {
		return domain.PendingAction{}, err
	}
	return m.insertProposal(ctx, domain.ActionTypeAssignBackupUnit, payload, in)
}

// ProposeResolveEvent inserts a PROPOSED RESOLVE_EVENT row.
func (m *Manager) ProposeResolveEvent(ctx context.Context, dispatchEventID string, in ProposeInput) (domain.PendingAction, error) {
	if _, err := m.Store.GetDispatchEvent(ctx, dispatchEventID); err != nil {
		return domain.PendingAction{}, fmt.Errorf("actions: propose resolve event: %w", err)
	}
	if in.IdempotencyKey == "" {
		in.IdempotencyKey = fmt.Sprintf("%s:%s", domain.ActionTypeResolveEvent, dispatchEventID)
	}
	payload, err := marshalPayload(ResolveEventPayload{DispatchEventID: dispatchEventID})
	if err != nil {
		return domain.PendingAction{}, err
	}
	return m.insertProposal(ctx, domain.ActionTypeResolveEvent, payload, in)
}

func (m *Manager) insertProposal(ctx context.Context, actionType domain.ActionType, payload []byte, in ProposeInput) (domain.PendingAction, error) {
	a := domain.PendingAction{
		ID:             uuid.NewString(),
		Type:           actionType,
		Payload:        payload,
		Status:         domain.ActionStatusProposed,
		IdempotencyKey: in.IdempotencyKey,
		ProposedBy:     in.ProposedBy,
		Rationale:      in.Rationale,
		AgentRunID:     in.AgentRunID,
		ParentActionID: in.ParentActionID,
		CreatedAt:      m.Clock.Now(),
	}
	inserted, err := m.Store.InsertPendingAction(ctx, a)
	if err != nil {
		return domain.PendingAction{}, fmt.Errorf("actions: insert proposal: %w", err)
	}
	return inserted, nil
}
