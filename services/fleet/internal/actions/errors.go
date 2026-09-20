package actions

import (
	"fmt"

	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/domain"
)

// PreconditionError is returned by executeAction (and the manual mutations)
// when the fleet state has drifted since a proposal was created, so the
// mutation can no longer be safely applied (ROADMAP S6 c4, SEC-4). The
// pending action row is still transitioned to FAILED and committed — the
// error communicates *why* to the caller, it is not itself the failure to
// persist.
type PreconditionError struct {
	Reason string
}

func (e *PreconditionError) Error() string {
	return fmt.Sprintf("precondition failed: %s", e.Reason)
}

// TransitionError is returned when an operation would move a PendingAction
// between statuses that IsLegalTransition forbids — e.g. executing or
// rejecting an action that is not PROPOSED (ROADMAP S6 c2, c8).
type TransitionError struct {
	ActionID string
	From     domain.ActionStatus
	To       domain.ActionStatus
}

func (e *TransitionError) Error() string {
	return fmt.Sprintf("action %s: illegal transition %s -> %s", e.ActionID, e.From, e.To)
}
