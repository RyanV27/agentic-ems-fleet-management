package dispatch

import "github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/domain"

// UnitSnapshot is the subset of domain.Unit that unit selection needs. Kept
// separate from domain.Unit so this package's inputs stay exactly as small
// as the pure decision requires.
type UnitSnapshot struct {
	ID         string
	Status     domain.UnitStatus
	Capability domain.Capability
	ZoneID     string
}

// TravelTimeTable is a lookup over the seeded zone-to-zone travel matrix.
type TravelTimeTable map[string]map[string]int

// NewTravelTimeTable builds a lookup table from the seeded travel-time rows.
func NewTravelTimeTable(rows []domain.ZoneTravelTime) TravelTimeTable {
	t := make(TravelTimeTable, len(rows))
	for _, row := range rows {
		if t[row.FromZoneID] == nil {
			t[row.FromZoneID] = make(map[string]int)
		}
		t[row.FromZoneID][row.ToZoneID] = row.TravelSeconds
	}
	return t
}

// Seconds returns the travel time from -> to and whether an entry exists.
func (t TravelTimeTable) Seconds(from, to string) (int, bool) {
	row, ok := t[from]
	if !ok {
		return 0, false
	}
	seconds, ok := row[to]
	return seconds, ok
}

// RequiredCapabilities is DEC-014: P1 requires ALS; P2/P3 accept BLS or ALS.
// Exported so internal/rules can reuse the exact same capability rule rather
// than re-deriving it.
func RequiredCapabilities(severity int) []domain.Capability {
	if severity == 1 {
		return []domain.Capability{domain.CapabilityALS}
	}
	return []domain.Capability{domain.CapabilityBLS, domain.CapabilityALS}
}

func capabilitySufficient(capability domain.Capability, required []domain.Capability) bool {
	for _, c := range required {
		if capability == c {
			return true
		}
	}
	return false
}

// NoSelectionReason explains why Select found no unit, distinguishing "no
// unit available at all" from "available but none capability-sufficient"
// (ROADMAP S3 criterion 5) with distinct rule-facing ids.
type NoSelectionReason string

const (
	NoSelectionReasonNoUnitAvailable        NoSelectionReason = "NO_UNIT_AVAILABLE"
	NoSelectionReasonNoCapableUnitAvailable NoSelectionReason = "NO_CAPABLE_UNIT_AVAILABLE"
)

// SelectInput is the snapshot Select decides over.
type SelectInput struct {
	Severity    int
	ZoneID      string
	Units       []UnitSnapshot
	TravelTimes TravelTimeTable
}

// SelectResult is a typed "no suitable unit" result (ROADMAP S3 criterion
// 6): Unit is nil exactly when Reason is non-empty, never a nil unit with a
// nil error.
type SelectResult struct {
	Unit   *UnitSnapshot
	Reason NoSelectionReason
}

// Found reports whether Select picked a unit.
func (r SelectResult) Found() bool { return r.Unit != nil }

// Select filters on capability before distance (DEC-014): among available,
// capability-sufficient units, it returns the one with the lowest
// TravelSeconds to the call's zone, tie-broken by ascending unit id.
// OUT_OF_SERVICE and any other non-AVAILABLE unit is never selected.
func Select(in SelectInput) SelectResult {
	available := make([]UnitSnapshot, 0, len(in.Units))
	for _, u := range in.Units {
		if u.Status == domain.UnitStatusAvailable {
			available = append(available, u)
		}
	}
	if len(available) == 0 {
		return SelectResult{Reason: NoSelectionReasonNoUnitAvailable}
	}

	required := RequiredCapabilities(in.Severity)
	capable := make([]UnitSnapshot, 0, len(available))
	for _, u := range available {
		if capabilitySufficient(u.Capability, required) {
			capable = append(capable, u)
		}
	}
	if len(capable) == 0 {
		return SelectResult{Reason: NoSelectionReasonNoCapableUnitAvailable}
	}

	best := capable[0]
	bestSeconds, _ := in.TravelTimes.Seconds(best.ZoneID, in.ZoneID)
	for _, u := range capable[1:] {
		seconds, _ := in.TravelTimes.Seconds(u.ZoneID, in.ZoneID)
		if seconds < bestSeconds || (seconds == bestSeconds && u.ID < best.ID) {
			best = u
			bestSeconds = seconds
		}
	}

	picked := best
	return SelectResult{Unit: &picked}
}
