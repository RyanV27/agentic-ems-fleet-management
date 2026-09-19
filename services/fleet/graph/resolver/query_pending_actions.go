package resolver

import (
	"context"

	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/graph/generated"
	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/domain"
)

func (r *queryResolver) PendingActions(ctx context.Context, status *generated.ActionStatus) ([]*generated.PendingAction, error) {
	actions, err := r.Store.ListPendingActions(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*generated.PendingAction, 0, len(actions))
	for _, a := range actions {
		if status != nil && a.Status != domain.ActionStatus(*status) {
			continue
		}
		out = append(out, mapPendingAction(a))
	}
	return out, nil
}
