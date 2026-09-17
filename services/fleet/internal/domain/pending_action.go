package domain

import (
	"encoding/json"
	"time"
)

// PendingAction is the human-approval gate and the audit record (hard rule
// 1, DEC-016). Every state-mutating action in the system exists first as a
// PROPOSED row here; only executeAction transitions one to EXECUTED.
type PendingAction struct {
	ID              string
	Type            ActionType
	Payload         json.RawMessage
	Status          ActionStatus
	IdempotencyKey  string
	ProposedBy      ProposedBy
	Rationale       string
	ExpiredReason   *ExpiredReason
	AgentRunID      *string
	RejectionReason *RejectionReason
	RejectionNote   *string
	ParentActionID  *string
	CreatedAt       time.Time
	DecidedAt       *time.Time
	DecidedBy       *string
}
