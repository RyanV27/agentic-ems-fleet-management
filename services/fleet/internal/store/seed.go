package store

import (
	"context"
	"fmt"
	"time"

	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/domain"
)

// seedEpoch is the fixed instant every seeded Unit's StatusChangedAt is set
// to. It is a constant, not time.Now(), specifically so that two seeded
// databases are byte-identical (S1 c4) — the seed must never depend on wall
// clock time.
var seedEpoch = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

// seedZoneIDs and seedTravelHopSeconds define a deterministic, complete,
// symmetric travel-time matrix: travel(zi, zj) = |i-j| * seedTravelHopSeconds,
// which is 0 on the diagonal and satisfies the triangle inequality.
var seedZoneIDs = []string{"zone-1", "zone-2", "zone-3", "zone-4", "zone-5", "zone-6"}

const seedTravelHopSeconds = 120

var seedZoneNames = map[string]string{
	"zone-1": "Downtown",
	"zone-2": "Uptown",
	"zone-3": "Riverside",
	"zone-4": "Eastside",
	"zone-5": "Westside",
	"zone-6": "Suburbs",
}

type seedUnit struct {
	id         string
	callsign   string
	capability domain.Capability
	zoneID     string
}

// seedUnits is the fixed 12-unit fleet (7 ALS / 5 BLS, OQ-2/DEC-014).
// zone-3 is deliberately given exactly one unit, a BLS unit, and no ALS
// unit — every other zone has at least one ALS unit — so a P1 call in
// zone-3 has a nearest unit that is BLS and an unsuitable choice, with the
// nearest ALS unit strictly farther away. That is what makes golden
// scenario G4 exercise capability filtering rather than only availability
// (DEC-014, ROADMAP.md S1 c4).
var seedUnits = []seedUnit{
	{"unit-01", "Medic 1", domain.CapabilityALS, "zone-1"},
	{"unit-02", "Medic 2", domain.CapabilityALS, "zone-1"},
	{"unit-03", "Medic 3", domain.CapabilityALS, "zone-2"},
	{"unit-04", "Medic 4", domain.CapabilityBLS, "zone-2"},
	{"unit-05", "Medic 5", domain.CapabilityBLS, "zone-3"},
	{"unit-06", "Medic 6", domain.CapabilityALS, "zone-4"},
	{"unit-07", "Medic 7", domain.CapabilityBLS, "zone-4"},
	{"unit-08", "Medic 8", domain.CapabilityALS, "zone-5"},
	{"unit-09", "Medic 9", domain.CapabilityBLS, "zone-5"},
	{"unit-10", "Medic 10", domain.CapabilityALS, "zone-6"},
	{"unit-11", "Medic 11", domain.CapabilityALS, "zone-6"},
	{"unit-12", "Medic 12", domain.CapabilityBLS, "zone-6"},
}

type seedHospital struct {
	id           string
	name         string
	zoneID       string
	capabilities []string
}

var seedHospitals = []seedHospital{
	{"hospital-1", "St. Mercy Downtown", "zone-1", []string{"TRAUMA", "CARDIAC"}},
	{"hospital-2", "Riverside General", "zone-3", []string{"GENERAL"}},
	{"hospital-3", "Westside Regional", "zone-5", []string{"TRAUMA", "PEDIATRIC"}},
}

// Seed populates an empty (migrated) store with the fixed Phase 1 demo
// dataset: 12 units (7 ALS / 5 BLS), 6 zones with a complete symmetric
// travel-time matrix, and 3 hospitals (OQ-2). Every seeded value is a
// constant, so calling Seed against two freshly migrated stores produces
// byte-identical content (S1 c4).
func Seed(ctx context.Context, s Store) error {
	for _, zoneID := range seedZoneIDs {
		if err := s.InsertZone(ctx, domain.Zone{ID: zoneID, Name: seedZoneNames[zoneID]}); err != nil {
			return fmt.Errorf("seed: zone %s: %w", zoneID, err)
		}
	}

	for i, from := range seedZoneIDs {
		for j, to := range seedZoneIDs {
			seconds := absInt(i-j) * seedTravelHopSeconds
			tt := domain.ZoneTravelTime{FromZoneID: from, ToZoneID: to, TravelSeconds: seconds}
			if err := s.InsertZoneTravelTime(ctx, tt); err != nil {
				return fmt.Errorf("seed: travel time %s->%s: %w", from, to, err)
			}
		}
	}

	for _, h := range seedHospitals {
		hospital := domain.Hospital{
			ID:           h.id,
			Name:         h.name,
			ZoneID:       h.zoneID,
			Capabilities: h.capabilities,
			Accepting:    true,
		}
		if err := s.InsertHospital(ctx, hospital); err != nil {
			return fmt.Errorf("seed: hospital %s: %w", h.id, err)
		}
	}

	for _, u := range seedUnits {
		unit := domain.Unit{
			ID:              u.id,
			Callsign:        u.callsign,
			Status:          domain.UnitStatusAvailable,
			Capability:      u.capability,
			ZoneID:          u.zoneID,
			CurrentCallID:   nil,
			StatusChangedAt: seedEpoch,
		}
		if err := s.InsertUnit(ctx, unit); err != nil {
			return fmt.Errorf("seed: unit %s: %w", u.id, err)
		}
	}

	return nil
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
