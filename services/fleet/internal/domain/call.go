package domain

import "time"

// Call is a dispatch job. Its Extraction is nil until extraction completes.
//
// EnqueuedAt is the sim time the call entered the operator work queue (DEC-021)
// — set once, at the end of intake, and never updated afterward, since it is
// an input to every dispatch.PriorityScore comparison (DEC-020).
type Call struct {
	ID                    string
	Transcript            string
	Extraction            *Extraction
	Severity              int
	ZoneID                string
	Status                CallStatus
	AssignedUnitID        *string
	DestinationHospitalID *string
	CreatedAt             time.Time
	EnqueuedAt            *time.Time
}
