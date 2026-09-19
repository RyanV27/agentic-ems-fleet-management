package rules_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/dispatch"
	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/domain"
	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/rules"
)

// BenchmarkDecideAndSelect is ROADMAP S3 criterion 11: Decide followed by
// Select for a 50-unit fleet must run in well under 1ms — the whole point
// of the deterministic path being zero-LLM and sub-millisecond
// (CLAUDE.md hard rule 3). b.N iterations amortize allocation noise; the
// per-op time reported by `go test -bench=.` is what to compare against 1ms.
func BenchmarkDecideAndSelect(b *testing.B) {
	cfg := rules.RulesConfig{
		ConfidenceThreshold: 0.75,
		Priority: dispatch.PriorityConfig{
			SeverityWeights: map[int]float64{1: 1000, 2: 100, 3: 10},
			AgingRate:       1.0,
		},
	}
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	units := make([]dispatch.UnitSnapshot, 50)
	zones := []string{"zone-1", "zone-2", "zone-3", "zone-4", "zone-5"}
	for i := range units {
		capability := domain.CapabilityBLS
		if i%2 == 0 {
			capability = domain.CapabilityALS
		}
		units[i] = dispatch.UnitSnapshot{
			ID:         fmt.Sprintf("unit-%02d", i),
			Status:     domain.UnitStatusAvailable,
			Capability: capability,
			ZoneID:     zones[i%len(zones)],
		}
	}

	travelRows := make([]domain.ZoneTravelTime, 0, len(zones)*len(zones))
	for _, from := range zones {
		for _, to := range zones {
			seconds := 0
			if from != to {
				seconds = 300
			}
			travelRows = append(travelRows, domain.ZoneTravelTime{FromZoneID: from, ToZoneID: to, TravelSeconds: seconds})
		}
	}
	travelTimes := dispatch.NewTravelTimeTable(travelRows)

	pending := make([]rules.PendingCall, 10)
	for i := range pending {
		pending[i] = rules.PendingCall{Severity: 3, EnqueuedAt: now.Add(-time.Duration(i) * time.Second)}
	}

	extraction := domain.Extraction{Severity: 2, ZoneID: "zone-3", Confidence: 0.9}
	snapshot := rules.FleetSnapshot{Units: units, PendingCalls: pending, Now: now}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		decision := rules.Decide(extraction, snapshot, cfg)
		if decision.Path == domain.RoutingPathDeterministic {
			dispatch.Select(dispatch.SelectInput{
				Severity:    extraction.Severity,
				ZoneID:      extraction.ZoneID,
				Units:       units,
				TravelTimes: travelTimes,
			})
		}
	}
}
