package actions

import "github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/domain"

// legalTransitions is the full PendingAction state machine (ROADMAP S6 c12,
// REL-4). PROPOSED is the only non-terminal state; APPROVED is defined on
// domain.ActionStatus but unreached by any Phase 1 code path (executeAction
// goes PROPOSED -> EXECUTED directly, per DEC-016 — there is no separate
// approve step to record before the human's one click, which itself *is*
// the transition to EXECUTED). Every other status is terminal: nothing
// transitions out of EXECUTED, REJECTED, EXPIRED, or FAILED.
var legalTransitions = map[domain.ActionStatus]map[domain.ActionStatus]bool{
	domain.ActionStatusProposed: {
		domain.ActionStatusExecuted: true,
		domain.ActionStatusRejected: true,
		domain.ActionStatusExpired:  true,
		domain.ActionStatusFailed:   true,
	},
}

// IsLegalTransition reports whether a PendingAction may move from -> to.
func IsLegalTransition(from, to domain.ActionStatus) bool {
	return legalTransitions[from][to]
}
