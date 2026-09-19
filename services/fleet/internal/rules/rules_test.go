package rules_test

import (
	"fmt"
	"go/build"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/dispatch"
	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/domain"
	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/rules"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testRulesConfig() rules.RulesConfig {
	return rules.RulesConfig{
		ConfidenceThreshold: 0.75,
		Priority: dispatch.PriorityConfig{
			SeverityWeights: map[int]float64{1: 1000, 2: 100, 3: 10},
			AgingRate:       1.0,
		},
	}
}

func alsUnit(id, zoneID string) dispatch.UnitSnapshot {
	return dispatch.UnitSnapshot{ID: id, Status: domain.UnitStatusAvailable, Capability: domain.CapabilityALS, ZoneID: zoneID}
}

func blsUnit(id, zoneID string) dispatch.UnitSnapshot {
	return dispatch.UnitSnapshot{ID: id, Status: domain.UnitStatusAvailable, Capability: domain.CapabilityBLS, ZoneID: zoneID}
}

// expectedDecision is the spec's truth table, computed independently of
// rules.Decide's implementation, for TestDecide_FullCrossProduct to assert
// against (ROADMAP S3 criterion 2).
func expectedDecision(confidenceOK, available, competing bool) (domain.RoutingPath, string) {
	if !confidenceOK {
		return domain.RoutingPathEscalated, rules.RuleLowConfidence
	}
	if !available {
		return domain.RoutingPathEscalated, rules.RuleNoUnitAvailable
	}
	if competing {
		return domain.RoutingPathEscalated, rules.RuleUnitContested
	}
	return domain.RoutingPathDeterministic, rules.RuleDeterministicDispatch
}

// TestDecide_FullCrossProduct covers severity {1,2,3} x {suitable unit
// available, none} x {competing higher-priority call, none} x {confidence
// above threshold, below} — ROADMAP S3 criterion 2. Every case asserts both
// the path and the matched rule id.
func TestDecide_FullCrossProduct(t *testing.T) {
	cfg := testRulesConfig()
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	for _, severity := range []int{1, 2, 3} {
		for _, available := range []bool{true, false} {
			for _, competing := range []bool{true, false} {
				for _, confidenceOK := range []bool{true, false} {
					name := fmt.Sprintf("severity=%d/available=%v/competing=%v/confidenceOK=%v", severity, available, competing, confidenceOK)
					t.Run(name, func(t *testing.T) {
						confidence := 0.9
						if !confidenceOK {
							confidence = 0.5
						}
						extraction := domain.Extraction{Severity: severity, ZoneID: "zone-1", Confidence: confidence}

						var units []dispatch.UnitSnapshot
						if available {
							// ALS satisfies every severity (DEC-014), so this
							// axis stays independent of the capability axis
							// covered by TestDecide_NoSuitableUnitDistinctCauses.
							units = []dispatch.UnitSnapshot{alsUnit("unit-1", "zone-1")}
						}

						var pending []rules.PendingCall
						if competing {
							// Same severity, already waited 1s: strictly
							// higher score than the brand-new call, and the
							// lone unit above means it fully occupies the pool.
							pending = []rules.PendingCall{{Severity: severity, EnqueuedAt: now.Add(-1 * time.Second)}}
						}

						snapshot := rules.FleetSnapshot{Units: units, PendingCalls: pending, Now: now}
						got := rules.Decide(extraction, snapshot, cfg)

						wantPath, wantRule := expectedDecision(confidenceOK, available, competing)
						assert.Equal(t, wantPath, got.Path, name)
						assert.Equal(t, wantRule, got.MatchedRuleID, name)
						require.NotEmpty(t, got.MatchedRuleID, "every return path must set a matched rule id")
					})
				}
			}
		}
	}
}

// TestDecide_LowConfidenceEscalatesRegardless is FR-4 as its own test:
// ESCALATED fires whenever confidence < threshold, regardless of every
// other input, including a fully suitable and uncontested unit.
func TestDecide_LowConfidenceEscalatesRegardless(t *testing.T) {
	cfg := testRulesConfig()
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		name     string
		snapshot rules.FleetSnapshot
		severity int
	}{
		{
			name:     "ideal fleet state, still escalates",
			severity: 1,
			snapshot: rules.FleetSnapshot{Units: []dispatch.UnitSnapshot{alsUnit("unit-1", "zone-1")}, Now: now},
		},
		{
			name:     "no units at all, still escalates via low confidence not no-unit",
			severity: 2,
			snapshot: rules.FleetSnapshot{Now: now},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			extraction := domain.Extraction{Severity: tc.severity, ZoneID: "zone-1", Confidence: cfg.ConfidenceThreshold - 0.01}
			got := rules.Decide(extraction, tc.snapshot, cfg)
			assert.Equal(t, domain.RoutingPathEscalated, got.Path)
			assert.Equal(t, rules.RuleLowConfidence, got.MatchedRuleID)
		})
	}
}

// TestDecide_ConfidenceAtThresholdIsSufficient asserts the boundary is
// inclusive: confidence exactly equal to the threshold does not escalate on
// that basis.
func TestDecide_ConfidenceAtThresholdIsSufficient(t *testing.T) {
	cfg := testRulesConfig()
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	extraction := domain.Extraction{Severity: 2, ZoneID: "zone-1", Confidence: cfg.ConfidenceThreshold}
	snapshot := rules.FleetSnapshot{Units: []dispatch.UnitSnapshot{blsUnit("unit-1", "zone-1")}, Now: now}

	got := rules.Decide(extraction, snapshot, cfg)

	assert.Equal(t, domain.RoutingPathDeterministic, got.Path)
	assert.NotEqual(t, rules.RuleLowConfidence, got.MatchedRuleID)
}

// TestDecide_NoSuitableUnitDistinctCauses is ROADMAP S3 criterion 5: "no
// suitable unit" is tested for both causes, with distinct matched rule ids.
// A P1 call with only BLS units free escalates (no capable unit), while the
// same fleet state with a P2 call dispatches deterministically — the case
// that proves capability is really in the decision (DEC-014).
func TestDecide_NoSuitableUnitDistinctCauses(t *testing.T) {
	cfg := testRulesConfig()
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	blsOnlyFleet := rules.FleetSnapshot{Units: []dispatch.UnitSnapshot{blsUnit("unit-1", "zone-1")}, Now: now}
	emptyFleet := rules.FleetSnapshot{Now: now}

	p1 := domain.Extraction{Severity: 1, ZoneID: "zone-1", Confidence: 0.9}
	p1Decision := rules.Decide(p1, blsOnlyFleet, cfg)
	assert.Equal(t, domain.RoutingPathEscalated, p1Decision.Path)
	assert.Equal(t, rules.RuleNoCapableUnit, p1Decision.MatchedRuleID)

	p2 := domain.Extraction{Severity: 2, ZoneID: "zone-1", Confidence: 0.9}
	p2Decision := rules.Decide(p2, blsOnlyFleet, cfg)
	assert.Equal(t, domain.RoutingPathDeterministic, p2Decision.Path)
	assert.Equal(t, rules.RuleDeterministicDispatch, p2Decision.MatchedRuleID)

	noUnitsDecision := rules.Decide(p1, emptyFleet, cfg)
	assert.Equal(t, domain.RoutingPathEscalated, noUnitsDecision.Path)
	assert.Equal(t, rules.RuleNoUnitAvailable, noUnitsDecision.MatchedRuleID)

	assert.NotEqual(t, p1Decision.MatchedRuleID, noUnitsDecision.MatchedRuleID,
		"no-capacity and no-capability must be distinct rule ids")
}

// TestDecide_OutOfServiceUnitsNeverCountAsAvailable guards against a unit
// list that has entries but none AVAILABLE being mistaken for "some
// capacity exists".
func TestDecide_OutOfServiceUnitsNeverCountAsAvailable(t *testing.T) {
	cfg := testRulesConfig()
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	units := []dispatch.UnitSnapshot{
		{ID: "unit-1", Status: domain.UnitStatusOutOfService, Capability: domain.CapabilityALS, ZoneID: "zone-1"},
		{ID: "unit-2", Status: domain.UnitStatusEnRoute, Capability: domain.CapabilityALS, ZoneID: "zone-1"},
	}
	extraction := domain.Extraction{Severity: 2, ZoneID: "zone-1", Confidence: 0.9}
	snapshot := rules.FleetSnapshot{Units: units, Now: now}

	got := rules.Decide(extraction, snapshot, cfg)

	assert.Equal(t, domain.RoutingPathEscalated, got.Path)
	assert.Equal(t, rules.RuleNoUnitAvailable, got.MatchedRuleID)
}

// TestPurity_NoForbiddenImportsOrClockReads is ROADMAP S3 criterion 12: no
// import of store, extract, or net/http, and no time.Now() call anywhere —
// now is always a parameter.
func TestPurity_NoForbiddenImportsOrClockReads(t *testing.T) {
	pkg, err := build.ImportDir(".", 0)
	require.NoError(t, err)

	forbidden := []string{"/internal/store", "/internal/extract", "net/http"}
	allImports := append(append([]string{}, pkg.Imports...), pkg.TestImports...)
	allImports = append(allImports, pkg.XTestImports...)
	for _, imp := range allImports {
		for _, f := range forbidden {
			assert.False(t, strings.Contains(imp, f), "rules must not import %s (got %s)", f, imp)
		}
	}

	for _, name := range pkg.GoFiles {
		assertNoTimeNow(t, name)
	}
}

func assertNoTimeNow(t *testing.T, filename string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(".", filename))
	require.NoError(t, err)
	assert.False(t, strings.Contains(string(data), "time.Now("), "%s must not call time.Now(); now must be a parameter", filename)
}
