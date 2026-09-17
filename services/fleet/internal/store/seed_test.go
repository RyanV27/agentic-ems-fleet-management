package store_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ryansamvarghese/agentic-fleet-management/services/fleet/internal/domain"
	"github.com/ryansamvarghese/agentic-fleet-management/services/fleet/internal/store"
	"github.com/ryansamvarghese/agentic-fleet-management/services/fleet/internal/store/sqlite"
)

func newSeededStore(t *testing.T, name string) *sqlite.Store {
	t.Helper()
	s, err := sqlite.Open(fmt.Sprintf("file:seed-test-%s?mode=memory&cache=shared", name))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })

	ctx := context.Background()
	require.NoError(t, s.Migrate(ctx))
	require.NoError(t, store.Seed(ctx, s))
	return s
}

func TestSeed_TwelveUnitsSevenALSFiveBLS(t *testing.T) {
	s := newSeededStore(t, "fleet-mix")
	units, err := s.ListUnits(context.Background())
	require.NoError(t, err)
	require.Len(t, units, 12)

	var als, bls int
	for _, u := range units {
		switch u.Capability {
		case domain.CapabilityALS:
			als++
		case domain.CapabilityBLS:
			bls++
		default:
			t.Fatalf("unexpected capability %q on unit %s", u.Capability, u.ID)
		}
	}
	assert.Equal(t, 7, als)
	assert.Equal(t, 5, bls)
}

func TestSeed_SixZonesCompleteSymmetricTravelMatrix(t *testing.T) {
	s := newSeededStore(t, "travel-matrix")
	ctx := context.Background()

	zones, err := s.ListZones(ctx)
	require.NoError(t, err)
	require.Len(t, zones, 6)

	times, err := s.ListZoneTravelTimes(ctx)
	require.NoError(t, err)
	assert.Len(t, times, 6*6, "every ordered pair, including from==to, must have an entry")

	for _, a := range zones {
		for _, b := range zones {
			ab, err := s.TravelSeconds(ctx, a.ID, b.ID)
			require.NoError(t, err)
			ba, err := s.TravelSeconds(ctx, b.ID, a.ID)
			require.NoError(t, err)
			assert.Equal(t, ab, ba, "travel time %s->%s must equal %s->%s (symmetric)", a.ID, b.ID, b.ID, a.ID)
			if a.ID == b.ID {
				assert.Zero(t, ab, "same-zone travel time must be zero")
			}
		}
	}
}

func TestSeed_ThreeHospitals(t *testing.T) {
	s := newSeededStore(t, "hospitals")
	hospitals, err := s.ListHospitals(context.Background())
	require.NoError(t, err)
	assert.Len(t, hospitals, 3)
	for _, h := range hospitals {
		assert.True(t, h.Accepting)
	}
}

// TestSeed_HasBLSNearestZone is the placement half of S1 c4 / DEC-014: at
// least one zone's nearest unit (by travel time, ties aside) must be BLS,
// which is what makes golden scenario G4 exercise capability rather than
// only availability.
func TestSeed_HasBLSNearestZone(t *testing.T) {
	s := newSeededStore(t, "bls-nearest")
	ctx := context.Background()

	units, err := s.ListUnits(ctx)
	require.NoError(t, err)

	zoneHasALS := map[string]bool{}
	zoneHasBLS := map[string]bool{}
	for _, u := range units {
		if u.Capability == domain.CapabilityALS {
			zoneHasALS[u.ZoneID] = true
		} else {
			zoneHasBLS[u.ZoneID] = true
		}
	}

	found := false
	for zoneID, hasBLS := range zoneHasBLS {
		if hasBLS && !zoneHasALS[zoneID] {
			found = true
			break
		}
	}
	assert.True(t, found, "at least one zone must have a BLS unit and no ALS unit, so its nearest (own-zone) unit is BLS")
}

// TestSeed_TwoDatabasesAreByteIdentical is S1 criterion 4's determinism
// claim: seeding two freshly migrated stores produces identical content.
func TestSeed_TwoDatabasesAreByteIdentical(t *testing.T) {
	ctx := context.Background()
	a := newSeededStore(t, "identical-a")
	b := newSeededStore(t, "identical-b")

	zonesA, err := a.ListZones(ctx)
	require.NoError(t, err)
	zonesB, err := b.ListZones(ctx)
	require.NoError(t, err)
	assert.Equal(t, zonesA, zonesB)

	timesA, err := a.ListZoneTravelTimes(ctx)
	require.NoError(t, err)
	timesB, err := b.ListZoneTravelTimes(ctx)
	require.NoError(t, err)
	assert.Equal(t, timesA, timesB)

	hospitalsA, err := a.ListHospitals(ctx)
	require.NoError(t, err)
	hospitalsB, err := b.ListHospitals(ctx)
	require.NoError(t, err)
	assert.Equal(t, hospitalsA, hospitalsB)

	unitsA, err := a.ListUnits(ctx)
	require.NoError(t, err)
	unitsB, err := b.ListUnits(ctx)
	require.NoError(t, err)
	assert.Equal(t, unitsA, unitsB)
}
