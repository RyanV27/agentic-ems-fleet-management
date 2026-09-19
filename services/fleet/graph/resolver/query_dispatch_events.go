package resolver

import (
	"context"

	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/graph/generated"
	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/domain"
)

func (r *queryResolver) DispatchEvents(ctx context.Context, filter *generated.DispatchEventsFilter) ([]*generated.DispatchEvent, error) {
	events, err := r.Store.ListDispatchEvents(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*generated.DispatchEvent, 0, len(events))
	for _, e := range events {
		if filter != nil && filter.Status != nil && e.Status != domain.DispatchEventStatus(*filter.Status) {
			continue
		}
		out = append(out, mapDispatchEvent(e))
	}
	return out, nil
}
