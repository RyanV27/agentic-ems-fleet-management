package actions_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/actions"
	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/domain"
)

// TestIsLegalTransition_FullTable is ROADMAP c12: every ordered pair of
// ActionStatus values is checked against an independently-authored
// expectation, not derived from the production map, so this test would
// catch a typo'd or accidentally-added transition. PROPOSED is the only
// non-terminal state (DEC-016: no separate APPROVED step; executeAction goes
// PROPOSED -> EXECUTED directly). All four terminal statuses have no legal
// outgoing transitions, including into themselves.
func TestIsLegalTransition_FullTable(t *testing.T) {
	all := []domain.ActionStatus{
		domain.ActionStatusProposed,
		domain.ActionStatusApproved,
		domain.ActionStatusExecuted,
		domain.ActionStatusRejected,
		domain.ActionStatusExpired,
		domain.ActionStatusFailed,
	}

	expected := map[domain.ActionStatus]map[domain.ActionStatus]bool{
		domain.ActionStatusProposed: {
			domain.ActionStatusExecuted: true,
			domain.ActionStatusRejected: true,
			domain.ActionStatusExpired:  true,
			domain.ActionStatusFailed:   true,
		},
	}

	for _, from := range all {
		for _, to := range all {
			want := expected[from][to]
			got := actions.IsLegalTransition(from, to)
			assert.Equalf(t, want, got, "IsLegalTransition(%s, %s)", from, to)
		}
	}
}

// TestIsLegalTransition_TerminalStatesHaveNoOutgoingTransitions is a
// belt-and-suspenders check that no terminal status (everything but
// PROPOSED) permits any outgoing transition at all.
func TestIsLegalTransition_TerminalStatesHaveNoOutgoingTransitions(t *testing.T) {
	terminal := []domain.ActionStatus{
		domain.ActionStatusApproved,
		domain.ActionStatusExecuted,
		domain.ActionStatusRejected,
		domain.ActionStatusExpired,
		domain.ActionStatusFailed,
	}
	targets := []domain.ActionStatus{
		domain.ActionStatusProposed,
		domain.ActionStatusApproved,
		domain.ActionStatusExecuted,
		domain.ActionStatusRejected,
		domain.ActionStatusExpired,
		domain.ActionStatusFailed,
	}
	for _, from := range terminal {
		for _, to := range targets {
			assert.Falsef(t, actions.IsLegalTransition(from, to), "terminal status %s must have no outgoing transitions (to %s)", from, to)
		}
	}
}
