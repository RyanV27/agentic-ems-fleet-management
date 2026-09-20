package resolver

import (
	"context"

	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/graph/generated"
)

// assignCall, rerouteUnit, and resolveDispatchEvent are the one exception to
// "only executeAction mutates fleet state" (DEC-016, ROADMAP S6 c9): an
// operator-initiated manual mutation, where the click itself is the
// approval. No PROPOSED phase — each still records an EXECUTED,
// proposedBy=OPERATOR audit row via internal/actions.Manager.

func (r *mutationResolver) AssignCall(ctx context.Context, callID string, unitID string) (*generated.PendingAction, error) {
	a, err := r.Manager.ManualAssignCall(ctx, callID, unitID)
	if err != nil {
		return nil, err
	}
	return mapPendingAction(a), nil
}

func (r *mutationResolver) RerouteUnit(ctx context.Context, unitID string, callID string) (*generated.PendingAction, error) {
	a, err := r.Manager.ManualRerouteUnit(ctx, unitID, callID)
	if err != nil {
		return nil, err
	}
	return mapPendingAction(a), nil
}

func (r *mutationResolver) ResolveDispatchEvent(ctx context.Context, dispatchEventID string) (*generated.PendingAction, error) {
	a, err := r.Manager.ManualResolveDispatchEvent(ctx, dispatchEventID)
	if err != nil {
		return nil, err
	}
	return mapPendingAction(a), nil
}
