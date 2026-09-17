package domain

// Zone is a coarse dispatch region. There is deliberately no lat/long, no
// routing engine, no map (DEC-010) — travel time is a static lookup, which
// keeps dispatch.Select pure and exactly testable.
type Zone struct {
	ID   string
	Name string
}

// ZoneTravelTime is one entry of the seeded travel-time matrix between two
// zones. The matrix is symmetric and complete (S1 criterion 4): every
// (from, to) pair, including from == to, has an entry.
type ZoneTravelTime struct {
	FromZoneID    string
	ToZoneID      string
	TravelSeconds int
}
