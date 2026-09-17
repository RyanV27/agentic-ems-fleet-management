package domain

import "time"

// Unit is an ambulance in the fleet.
type Unit struct {
	ID              string
	Callsign        string
	Status          UnitStatus
	Capability      Capability
	ZoneID          string
	CurrentCallID   *string
	StatusChangedAt time.Time
}
