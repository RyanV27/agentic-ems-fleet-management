package resolver

import (
	"context"

	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/graph/generated"
)

// FleetStatus is the fleet board's single aggregate read (FR-17).
func (r *queryResolver) FleetStatus(ctx context.Context) (*generated.FleetStatus, error) {
	units, err := r.Store.ListUnits(ctx)
	if err != nil {
		return nil, err
	}
	events, err := r.Store.ListDispatchEvents(ctx)
	if err != nil {
		return nil, err
	}
	calls, err := r.Calls(ctx, nil)
	if err != nil {
		return nil, err
	}

	mappedUnits := make([]*generated.Unit, len(units))
	for i, u := range units {
		mappedUnits[i] = mapUnit(u)
	}
	mappedEvents := make([]*generated.DispatchEvent, len(events))
	for i, e := range events {
		mappedEvents[i] = mapDispatchEvent(e)
	}

	return &generated.FleetStatus{
		Units:          mappedUnits,
		Calls:          calls,
		DispatchEvents: mappedEvents,
	}, nil
}
