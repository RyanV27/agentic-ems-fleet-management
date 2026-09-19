package dispatch_test

import (
	"testing"

	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/dispatch"
	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func travelTable() dispatch.TravelTimeTable {
	return dispatch.TravelTimeTable{
		"zone-1": {"zone-1": 0, "zone-2": 300, "zone-3": 600},
		"zone-2": {"zone-1": 300, "zone-2": 0, "zone-3": 200},
		"zone-3": {"zone-1": 600, "zone-2": 200, "zone-3": 0},
	}
}

// TestSelect_CapabilityBeforeDistance is ROADMAP S3 criterion 4: a closer
// unit that fails the capability requirement must lose to a farther unit
// that satisfies it, with a deliberate tie at the distance metric between
// two capability-sufficient units broken by ascending unit ID.
func TestSelect_CapabilityBeforeDistance(t *testing.T) {
	units := []dispatch.UnitSnapshot{
		{ID: "unit-close-bls", Status: domain.UnitStatusAvailable, Capability: domain.CapabilityBLS, ZoneID: "zone-1"},
		{ID: "unit-far-als-b", Status: domain.UnitStatusAvailable, Capability: domain.CapabilityALS, ZoneID: "zone-3"},
		{ID: "unit-far-als-a", Status: domain.UnitStatusAvailable, Capability: domain.CapabilityALS, ZoneID: "zone-3"},
	}
	in := dispatch.SelectInput{
		Severity:    1, // P1 requires ALS (DEC-014): the closer BLS unit is disqualified
		ZoneID:      "zone-1",
		Units:       units,
		TravelTimes: travelTable(),
	}

	got := dispatch.Select(in)

	require.True(t, got.Found())
	assert.Equal(t, "unit-far-als-a", got.Unit.ID, "tie between equidistant capable units breaks on ascending ID")
}

// TestSelect_NonAvailableUnitsNeverSelected is ROADMAP S3 criterion 6:
// OUT_OF_SERVICE and any other non-AVAILABLE status is never selected, even
// when it is the closest and most capable unit in the fleet.
func TestSelect_NonAvailableUnitsNeverSelected(t *testing.T) {
	units := []dispatch.UnitSnapshot{
		{ID: "unit-oos", Status: domain.UnitStatusOutOfService, Capability: domain.CapabilityALS, ZoneID: "zone-1"},
		{ID: "unit-enroute", Status: domain.UnitStatusEnRoute, Capability: domain.CapabilityALS, ZoneID: "zone-1"},
		{ID: "unit-available", Status: domain.UnitStatusAvailable, Capability: domain.CapabilityALS, ZoneID: "zone-3"},
	}
	in := dispatch.SelectInput{
		Severity:    1,
		ZoneID:      "zone-1",
		Units:       units,
		TravelTimes: travelTable(),
	}

	got := dispatch.Select(in)

	require.True(t, got.Found())
	assert.Equal(t, "unit-available", got.Unit.ID)
}

// TestSelect_TypedNoSuitableUnitReasons is ROADMAP S3 criteria 5-6: the two
// distinct causes of "no suitable unit" carry distinct, typed reasons, and
// Unit is nil exactly when Reason is set.
func TestSelect_TypedNoSuitableUnitReasons(t *testing.T) {
	t.Run("no unit available at all", func(t *testing.T) {
		in := dispatch.SelectInput{
			Severity: 2,
			ZoneID:   "zone-1",
			Units: []dispatch.UnitSnapshot{
				{ID: "unit-1", Status: domain.UnitStatusOutOfService, Capability: domain.CapabilityALS, ZoneID: "zone-1"},
			},
			TravelTimes: travelTable(),
		}

		got := dispatch.Select(in)

		assert.False(t, got.Found())
		assert.Nil(t, got.Unit)
		assert.Equal(t, dispatch.NoSelectionReasonNoUnitAvailable, got.Reason)
	})

	t.Run("available but none capability-sufficient", func(t *testing.T) {
		in := dispatch.SelectInput{
			Severity: 1, // P1 requires ALS
			ZoneID:   "zone-1",
			Units: []dispatch.UnitSnapshot{
				{ID: "unit-1", Status: domain.UnitStatusAvailable, Capability: domain.CapabilityBLS, ZoneID: "zone-1"},
			},
			TravelTimes: travelTable(),
		}

		got := dispatch.Select(in)

		assert.False(t, got.Found())
		assert.Nil(t, got.Unit)
		assert.Equal(t, dispatch.NoSelectionReasonNoCapableUnitAvailable, got.Reason)
	})
}
