package domain

import "time"

// DispatchEvent is an operational disruption — a breakdown, a delay, a
// reroute need — distinct from the medical incident (Call) it may relate to.
type DispatchEvent struct {
	ID            string
	Type          DispatchEventType
	CallID        *string
	UnitID        *string
	SeverityDelta *int
	Status        DispatchEventStatus
	Note          string
	CreatedAt     time.Time
	ResolvedAt    *time.Time
}
