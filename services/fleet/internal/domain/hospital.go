package domain

// Hospital is a transport destination.
type Hospital struct {
	ID           string
	Name         string
	ZoneID       string
	Capabilities []string
	Accepting    bool
}
