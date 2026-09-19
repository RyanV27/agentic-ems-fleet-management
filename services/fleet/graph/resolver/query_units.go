package resolver

import (
	"context"
	"errors"

	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/graph/generated"
	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/store"
)

func (r *queryResolver) Units(ctx context.Context) ([]*generated.Unit, error) {
	units, err := r.Store.ListUnits(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*generated.Unit, len(units))
	for i, u := range units {
		out[i] = mapUnit(u)
	}
	return out, nil
}

func (r *queryResolver) Unit(ctx context.Context, id string) (*generated.Unit, error) {
	u, err := r.Store.GetUnit(ctx, id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return mapUnit(u), nil
}
