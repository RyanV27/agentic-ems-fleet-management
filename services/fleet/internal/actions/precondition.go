package actions

import (
	"fmt"

	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/dispatch"
	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/domain"
)

// callOpen reports whether a call is still awaiting a dispatch decision —
// the only states an assignment may target (DEC-021).
func callOpen(c domain.Call) bool {
	return c.Status == domain.CallStatusPendingApproval || c.Status == domain.CallStatusEscalated
}

// unitAssignable reports whether a unit may be newly assigned to a call.
func unitAssignable(u domain.Unit) bool {
	return u.Status == domain.UnitStatusAvailable
}

func unitCapableFor(u domain.Unit, severity int) bool {
	for _, c := range dispatch.RequiredCapabilities(severity) {
		if u.Capability == c {
			return true
		}
	}
	return false
}

// checkAssignCall revalidates ASSIGN_CALL's preconditions against live
// state: the call must still be open and the unit must still be available
// and capability-sufficient (ROADMAP S6 c4, DEC-014).
func checkAssignCall(call domain.Call, unit domain.Unit) error {
	if !callOpen(call) {
		return &PreconditionError{Reason: fmt.Sprintf("call %s is not open (status %s)", call.ID, call.Status)}
	}
	if !unitAssignable(unit) {
		return &PreconditionError{Reason: fmt.Sprintf("unit %s is not available (status %s)", unit.ID, unit.Status)}
	}
	if !unitCapableFor(unit, call.Severity) {
		return &PreconditionError{Reason: fmt.Sprintf("unit %s (capability %s) is not sufficient for call %s severity %d", unit.ID, unit.Capability, call.ID, call.Severity)}
	}
	return nil
}

// checkRerouteUnit revalidates REROUTE_UNIT: the unit must still exist and
// carry a call (it is being pulled off something), the target call must
// still be open and distinct from the unit's current call, and the unit
// must be capability-sufficient for the target call's severity. The unit's
// operational status is not required to be AVAILABLE — rerouting moves a
// unit that is already EN_ROUTE/ON_SCENE/TRANSPORTING onto a different call,
// which is the entire point of the action (see DEC-037).
func checkRerouteUnit(call domain.Call, unit domain.Unit) error {
	if !callOpen(call) {
		return &PreconditionError{Reason: fmt.Sprintf("call %s is not open (status %s)", call.ID, call.Status)}
	}
	if unit.CurrentCallID == nil {
		return &PreconditionError{Reason: fmt.Sprintf("unit %s has no current call to reroute from", unit.ID)}
	}
	if *unit.CurrentCallID == call.ID {
		return &PreconditionError{Reason: fmt.Sprintf("unit %s is already assigned to call %s", unit.ID, call.ID)}
	}
	if unit.Status == domain.UnitStatusOutOfService {
		return &PreconditionError{Reason: fmt.Sprintf("unit %s is out of service", unit.ID)}
	}
	if !unitCapableFor(unit, call.Severity) {
		return &PreconditionError{Reason: fmt.Sprintf("unit %s (capability %s) is not sufficient for call %s severity %d", unit.ID, unit.Capability, call.ID, call.Severity)}
	}
	return nil
}

// checkAssignBackupUnit revalidates ASSIGN_BACKUP_UNIT: the call must not be
// closed or cancelled (it may already be ASSIGNED — that is the case this
// action exists for, replacing a compromised unit) and the backup unit must
// be available and capability-sufficient.
func checkAssignBackupUnit(call domain.Call, unit domain.Unit) error {
	if call.Status == domain.CallStatusClosed || call.Status == domain.CallStatusCancelled {
		return &PreconditionError{Reason: fmt.Sprintf("call %s is closed (status %s)", call.ID, call.Status)}
	}
	if !unitAssignable(unit) {
		return &PreconditionError{Reason: fmt.Sprintf("backup unit %s is not available (status %s)", unit.ID, unit.Status)}
	}
	if !unitCapableFor(unit, call.Severity) {
		return &PreconditionError{Reason: fmt.Sprintf("backup unit %s (capability %s) is not sufficient for call %s severity %d", unit.ID, unit.Capability, call.ID, call.Severity)}
	}
	return nil
}

// checkResolveEvent revalidates RESOLVE_EVENT: the event must still be OPEN.
func checkResolveEvent(e domain.DispatchEvent) error {
	if e.Status != domain.DispatchEventStatusOpen {
		return &PreconditionError{Reason: fmt.Sprintf("dispatch event %s is not open (status %s)", e.ID, e.Status)}
	}
	return nil
}
