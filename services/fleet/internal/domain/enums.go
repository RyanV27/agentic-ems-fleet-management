// Package domain holds the entities, enums, and value objects for the EMS
// fleet system. It imports nothing from the rest of the repository
// (ROADMAP.md S1 criterion 7): no store, no GraphQL, no HTTP.
package domain

// UnitStatus is the operational state of an ambulance.
type UnitStatus string

const (
	UnitStatusAvailable    UnitStatus = "AVAILABLE"
	UnitStatusEnRoute      UnitStatus = "EN_ROUTE"
	UnitStatusOnScene      UnitStatus = "ON_SCENE"
	UnitStatusTransporting UnitStatus = "TRANSPORTING"
	UnitStatusOutOfService UnitStatus = "OUT_OF_SERVICE"
)

// Valid reports whether s is one of the defined UnitStatus values.
func (s UnitStatus) Valid() bool {
	switch s {
	case UnitStatusAvailable, UnitStatusEnRoute, UnitStatusOnScene, UnitStatusTransporting, UnitStatusOutOfService:
		return true
	}
	return false
}

// Capability is the clinical capability level of a unit. P1 calls require
// ALS; P2/P3 accept either (DEC-014).
type Capability string

const (
	CapabilityBLS Capability = "BLS"
	CapabilityALS Capability = "ALS"
)

// Valid reports whether c is one of the defined Capability values.
func (c Capability) Valid() bool {
	switch c {
	case CapabilityBLS, CapabilityALS:
		return true
	}
	return false
}

// CallStatus is the lifecycle state of a dispatch call. There is
// deliberately no PENDING value (DEC-021): intake resolves every call to
// PENDING_APPROVAL or ESCALATED within the intake transaction.
type CallStatus string

const (
	CallStatusPendingApproval CallStatus = "PENDING_APPROVAL"
	CallStatusEscalated       CallStatus = "ESCALATED"
	CallStatusAssigned        CallStatus = "ASSIGNED"
	CallStatusEnRoute         CallStatus = "EN_ROUTE"
	CallStatusOnScene         CallStatus = "ON_SCENE"
	CallStatusTransporting    CallStatus = "TRANSPORTING"
	CallStatusClosed          CallStatus = "CLOSED"
	CallStatusCancelled       CallStatus = "CANCELLED"
)

// Valid reports whether s is one of the defined CallStatus values.
func (s CallStatus) Valid() bool {
	switch s {
	case CallStatusPendingApproval, CallStatusEscalated, CallStatusAssigned, CallStatusEnRoute,
		CallStatusOnScene, CallStatusTransporting, CallStatusClosed, CallStatusCancelled:
		return true
	}
	return false
}

// ActionType is the kind of state-mutating action a PendingAction proposes.
type ActionType string

const (
	ActionTypeAssignCall       ActionType = "ASSIGN_CALL"
	ActionTypeRerouteUnit      ActionType = "REROUTE_UNIT"
	ActionTypeAssignBackupUnit ActionType = "ASSIGN_BACKUP_UNIT"
	ActionTypeResolveEvent     ActionType = "RESOLVE_EVENT"
)

// Valid reports whether t is one of the defined ActionType values.
func (t ActionType) Valid() bool {
	switch t {
	case ActionTypeAssignCall, ActionTypeRerouteUnit, ActionTypeAssignBackupUnit, ActionTypeResolveEvent:
		return true
	}
	return false
}

// ActionStatus is the lifecycle state of a PendingAction.
type ActionStatus string

const (
	ActionStatusProposed ActionStatus = "PROPOSED"
	ActionStatusApproved ActionStatus = "APPROVED"
	ActionStatusExecuted ActionStatus = "EXECUTED"
	ActionStatusRejected ActionStatus = "REJECTED"
	ActionStatusExpired  ActionStatus = "EXPIRED"
	ActionStatusFailed   ActionStatus = "FAILED"
)

// Valid reports whether s is one of the defined ActionStatus values.
func (s ActionStatus) Valid() bool {
	switch s {
	case ActionStatusProposed, ActionStatusApproved, ActionStatusExecuted, ActionStatusRejected,
		ActionStatusExpired, ActionStatusFailed:
		return true
	}
	return false
}

// ProposedBy identifies who authored a PendingAction (DEC-016): the
// deterministic rules engine, the LLM agent, or a human operator.
type ProposedBy string

const (
	ProposedBySystem   ProposedBy = "SYSTEM"
	ProposedByAgent    ProposedBy = "AGENT"
	ProposedByOperator ProposedBy = "OPERATOR"
)

// Valid reports whether p is one of the defined ProposedBy values.
func (p ProposedBy) Valid() bool {
	switch p {
	case ProposedBySystem, ProposedByAgent, ProposedByOperator:
		return true
	}
	return false
}

// ExpiredReason records why a PROPOSED action was invalidated (DEC-019).
type ExpiredReason string

const (
	ExpiredReasonUnitReassigned   ExpiredReason = "UNIT_REASSIGNED"
	ExpiredReasonUnitOutOfService ExpiredReason = "UNIT_OUT_OF_SERVICE"
	ExpiredReasonCallClosed       ExpiredReason = "CALL_CLOSED"
	ExpiredReasonSuperseded       ExpiredReason = "SUPERSEDED"
	ExpiredReasonTTL              ExpiredReason = "TTL"
)

// Valid reports whether r is one of the defined ExpiredReason values.
func (r ExpiredReason) Valid() bool {
	switch r {
	case ExpiredReasonUnitReassigned, ExpiredReasonUnitOutOfService, ExpiredReasonCallClosed,
		ExpiredReasonSuperseded, ExpiredReasonTTL:
		return true
	}
	return false
}

// RejectionReason is the structured reason an operator gives when rejecting
// a proposal (FR-21).
type RejectionReason string

const (
	RejectionReasonWrongUnit        RejectionReason = "WRONG_UNIT"
	RejectionReasonInsufficientInfo RejectionReason = "INSUFFICIENT_INFO"
	RejectionReasonUnsafeTiming     RejectionReason = "UNSAFE_TIMING"
	RejectionReasonOther            RejectionReason = "OTHER"
)

// Valid reports whether r is one of the defined RejectionReason values.
func (r RejectionReason) Valid() bool {
	switch r {
	case RejectionReasonWrongUnit, RejectionReasonInsufficientInfo, RejectionReasonUnsafeTiming, RejectionReasonOther:
		return true
	}
	return false
}

// DispatchEventType classifies an operational disruption (not the medical
// incident itself).
type DispatchEventType string

const (
	DispatchEventTypeDelay           DispatchEventType = "DELAY"
	DispatchEventTypeBreakdown       DispatchEventType = "BREAKDOWN"
	DispatchEventTypeRerouteNeeded   DispatchEventType = "REROUTE_NEEDED"
	DispatchEventTypeSeverityChange  DispatchEventType = "SEVERITY_CHANGE"
	DispatchEventTypeUnitUnavailable DispatchEventType = "UNIT_UNAVAILABLE"
)

// Valid reports whether t is one of the defined DispatchEventType values.
func (t DispatchEventType) Valid() bool {
	switch t {
	case DispatchEventTypeDelay, DispatchEventTypeBreakdown, DispatchEventTypeRerouteNeeded,
		DispatchEventTypeSeverityChange, DispatchEventTypeUnitUnavailable:
		return true
	}
	return false
}

// DispatchEventStatus is the lifecycle state of a DispatchEvent.
type DispatchEventStatus string

const (
	DispatchEventStatusOpen     DispatchEventStatus = "OPEN"
	DispatchEventStatusResolved DispatchEventStatus = "RESOLVED"
)

// Valid reports whether s is one of the defined DispatchEventStatus values.
func (s DispatchEventStatus) Valid() bool {
	switch s {
	case DispatchEventStatusOpen, DispatchEventStatusResolved:
		return true
	}
	return false
}

// RoutingPath is which path rules.Decide chose for an intake (FR-3, FR-26).
type RoutingPath string

const (
	RoutingPathDeterministic RoutingPath = "DETERMINISTIC"
	RoutingPathEscalated     RoutingPath = "ESCALATED"
)

// Valid reports whether p is one of the defined RoutingPath values.
func (p RoutingPath) Valid() bool {
	switch p {
	case RoutingPathDeterministic, RoutingPathEscalated:
		return true
	}
	return false
}

// AgentRunStopReason records why an agent invocation ended.
type AgentRunStopReason string

const (
	AgentRunStopReasonCompleted     AgentRunStopReason = "COMPLETED"
	AgentRunStopReasonMaxSteps      AgentRunStopReason = "MAX_STEPS"
	AgentRunStopReasonInvalidOutput AgentRunStopReason = "INVALID_OUTPUT"
	AgentRunStopReasonError         AgentRunStopReason = "ERROR"
)

// Valid reports whether r is one of the defined AgentRunStopReason values.
func (r AgentRunStopReason) Valid() bool {
	switch r {
	case AgentRunStopReasonCompleted, AgentRunStopReasonMaxSteps, AgentRunStopReasonInvalidOutput, AgentRunStopReasonError:
		return true
	}
	return false
}

// AgentRunStatus is the lifecycle state of an AgentRun.
type AgentRunStatus string

const (
	AgentRunStatusRunning   AgentRunStatus = "RUNNING"
	AgentRunStatusCompleted AgentRunStatus = "COMPLETED"
	AgentRunStatusFailed    AgentRunStatus = "FAILED"
)

// Valid reports whether s is one of the defined AgentRunStatus values.
func (s AgentRunStatus) Valid() bool {
	switch s {
	case AgentRunStatusRunning, AgentRunStatusCompleted, AgentRunStatusFailed:
		return true
	}
	return false
}
