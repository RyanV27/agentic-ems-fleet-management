// Package rules is PURE: no I/O, no clock reads, no DB, no logging
// (ROADMAP.md S3). It decides which of the two intake paths a call takes.
// The LLM extracts; this package decides; the human still approves before
// anything mutates (CLAUDE.md hard rules 1-2).
package rules

import (
	"time"

	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/dispatch"
	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/domain"
)

// Reason explains why a Decision came out the way it did — the fourth
// column of the audit trail alongside Path and MatchedRuleID (FR-26).
type Reason string

const (
	ReasonLowConfidence          Reason = "LOW_CONFIDENCE"
	ReasonNoUnitAvailable        Reason = "NO_UNIT_AVAILABLE"
	ReasonNoCapableUnitAvailable Reason = "NO_CAPABLE_UNIT_AVAILABLE"
	ReasonUnitContested          Reason = "UNIT_CONTESTED"
	ReasonSuitableUnitAvailable  Reason = "SUITABLE_UNIT_AVAILABLE"
)

// Matched rule ids. Every one is written to routing_decisions verbatim, so
// they are stable, human-readable strings rather than incrementing numbers.
const (
	RuleLowConfidence         = "R-01-LOW-CONFIDENCE"
	RuleNoUnitAvailable       = "R-02-NO-UNIT-AVAILABLE"
	RuleNoCapableUnit         = "R-03-NO-CAPABLE-UNIT"
	RuleUnitContested         = "R-04-UNIT-CONTESTED"
	RuleDeterministicDispatch = "R-12-DETERMINISTIC-DISPATCH"
)

// Decision is rules.Decide's entire output: the path, the rule that fired,
// and the reason. Every return path sets a non-empty MatchedRuleID.
type Decision struct {
	Path          domain.RoutingPath
	MatchedRuleID string
	Reason        Reason
}

// RulesConfig is the subset of config.Config the rules engine needs.
type RulesConfig struct {
	ConfidenceThreshold float64
	Priority            dispatch.PriorityConfig
}

// PendingCall is one call already waiting in the operator work queue,
// relevant to whether it competes with a new call for the same unit pool.
type PendingCall struct {
	Severity   int
	EnqueuedAt time.Time
}

// FleetSnapshot is everything Decide needs about current fleet and queue
// state, frozen at Now. Decide never reads a clock itself.
type FleetSnapshot struct {
	Units        []dispatch.UnitSnapshot
	PendingCalls []PendingCall
	Now          time.Time
}

// Decide returns DETERMINISTIC when the brief's rule holds (ARCHITECTURE.md
// §5): severity is actionable, a suitable unit is available (available and
// capability-sufficient, DEC-014), no higher-priority queued call is
// competing for that same unit pool, and confidence is at or above
// threshold. It never picks the unit itself — that is dispatch.Select's job,
// called separately after DETERMINISTIC is returned.
func Decide(extraction domain.Extraction, snapshot FleetSnapshot, cfg RulesConfig) Decision {
	// FR-4: below-threshold confidence escalates regardless of every other
	// input. Checked first, and only first.
	if extraction.Confidence < cfg.ConfidenceThreshold {
		return Decision{Path: domain.RoutingPathEscalated, MatchedRuleID: RuleLowConfidence, Reason: ReasonLowConfidence}
	}

	required := dispatch.RequiredCapabilities(extraction.Severity)

	anyAvailable := false
	pool := make([]dispatch.UnitSnapshot, 0, len(snapshot.Units))
	for _, u := range snapshot.Units {
		if u.Status != domain.UnitStatusAvailable {
			continue
		}
		anyAvailable = true
		if capabilitySufficient(u.Capability, required) {
			pool = append(pool, u)
		}
	}
	if !anyAvailable {
		return Decision{Path: domain.RoutingPathEscalated, MatchedRuleID: RuleNoUnitAvailable, Reason: ReasonNoUnitAvailable}
	}
	if len(pool) == 0 {
		return Decision{Path: domain.RoutingPathEscalated, MatchedRuleID: RuleNoCapableUnit, Reason: ReasonNoCapableUnitAvailable}
	}

	if contested(extraction, snapshot, pool, cfg) {
		return Decision{Path: domain.RoutingPathEscalated, MatchedRuleID: RuleUnitContested, Reason: ReasonUnitContested}
	}

	return Decision{Path: domain.RoutingPathDeterministic, MatchedRuleID: RuleDeterministicDispatch, Reason: ReasonSuitableUnitAvailable}
}

// contested reports whether enough already-queued, higher-priority calls
// would draw from the same unit pool as extraction to leave nothing for it
// — "a higher-priority call awaiting action competing for the same unit"
// (ARCHITECTURE.md §5 step 3).
//
// Every severity's required-capability set includes ALS (DEC-014: P1 is
// ALS-only, P2/P3 accept BLS-or-ALS), so any two calls' pools always
// overlap in this fleet's two-tier capability model — there is no severity
// pair whose candidate units are disjoint. Contention is therefore a
// straight comparison of priority scores against the pool size, with no
// separate capability-overlap filter needed.
func contested(extraction domain.Extraction, snapshot FleetSnapshot, pool []dispatch.UnitSnapshot, cfg RulesConfig) bool {
	currentScore := dispatch.PriorityScore(extraction.Severity, snapshot.Now, snapshot.Now, cfg.Priority)

	competing := 0
	for _, pc := range snapshot.PendingCalls {
		pcScore := dispatch.PriorityScore(pc.Severity, pc.EnqueuedAt, snapshot.Now, cfg.Priority)
		if pcScore > currentScore {
			competing++
		}
	}
	return competing >= len(pool)
}

func capabilitySufficient(capability domain.Capability, required []domain.Capability) bool {
	for _, c := range required {
		if capability == c {
			return true
		}
	}
	return false
}
