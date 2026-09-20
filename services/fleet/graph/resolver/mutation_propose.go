package resolver

import (
	"context"

	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/graph/generated"
	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/actions"
	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/domain"
)

// These four resolvers only insert a PROPOSED row (DEC-016, ROADMAP S6 c1).
// proposedBy defaults to OPERATOR (the web dashboard's own propose flows);
// the agent (S7) is a separate process whose only tool surface is this API
// (CLAUDE.md), so it passes proposedBy: AGENT and agentRunId explicitly —
// there is no request-level auth in Phase 1 to derive caller identity from
// (OQ-7), so the caller states it (DEC-038). The deterministic rules engine
// (S3) never goes through this API; it calls internal/actions directly
// in-process with proposedBy = SYSTEM.

func (r *mutationResolver) ProposeAssignCall(ctx context.Context, input generated.ProposeAssignCallInput) (*generated.PendingAction, error) {
	a, err := r.Manager.ProposeAssignCall(ctx, input.CallID, input.UnitID, actions.ProposeInput{
		ProposedBy: domain.ProposedBy(input.ProposedBy),
		Rationale:  input.Rationale,
		AgentRunID: input.AgentRunID,
	})
	if err != nil {
		return nil, err
	}
	return mapPendingAction(a), nil
}

func (r *mutationResolver) ProposeRerouteUnit(ctx context.Context, input generated.ProposeRerouteUnitInput) (*generated.PendingAction, error) {
	a, err := r.Manager.ProposeRerouteUnit(ctx, input.UnitID, input.CallID, actions.ProposeInput{
		ProposedBy: domain.ProposedBy(input.ProposedBy),
		Rationale:  input.Rationale,
		AgentRunID: input.AgentRunID,
	})
	if err != nil {
		return nil, err
	}
	return mapPendingAction(a), nil
}

func (r *mutationResolver) ProposeAssignBackupUnit(ctx context.Context, input generated.ProposeAssignBackupUnitInput) (*generated.PendingAction, error) {
	a, err := r.Manager.ProposeAssignBackupUnit(ctx, input.CallID, input.UnitID, actions.ProposeInput{
		ProposedBy: domain.ProposedBy(input.ProposedBy),
		Rationale:  input.Rationale,
		AgentRunID: input.AgentRunID,
	})
	if err != nil {
		return nil, err
	}
	return mapPendingAction(a), nil
}

func (r *mutationResolver) ProposeResolveEvent(ctx context.Context, input generated.ProposeResolveEventInput) (*generated.PendingAction, error) {
	a, err := r.Manager.ProposeResolveEvent(ctx, input.DispatchEventID, actions.ProposeInput{
		ProposedBy: domain.ProposedBy(input.ProposedBy),
		Rationale:  input.Rationale,
		AgentRunID: input.AgentRunID,
	})
	if err != nil {
		return nil, err
	}
	return mapPendingAction(a), nil
}
