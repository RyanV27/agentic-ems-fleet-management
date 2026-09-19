package resolver

import (
	"context"

	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/graph/generated"
)

func (r *queryResolver) Zones(ctx context.Context) ([]*generated.Zone, error) {
	zones, err := r.Store.ListZones(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*generated.Zone, len(zones))
	for i, z := range zones {
		out[i] = mapZone(z)
	}
	return out, nil
}

func (r *queryResolver) Hospitals(ctx context.Context) ([]*generated.Hospital, error) {
	hospitals, err := r.Store.ListHospitals(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*generated.Hospital, len(hospitals))
	for i, h := range hospitals {
		out[i] = mapHospital(h)
	}
	return out, nil
}
