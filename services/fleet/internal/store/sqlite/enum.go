package sqlite

import "fmt"

// validatable is satisfied by every domain enum type (they all define
// Valid() bool). checkEnum is the one validation boundary a row must pass
// through on load — an invalid enum string fails clearly rather than
// silently propagating (S1 c5).
type validatable interface {
	Valid() bool
}

func checkEnum[T validatable](kind string, v T) error {
	if !v.Valid() {
		return fmt.Errorf("sqlite: invalid %s %q", kind, fmt.Sprintf("%v", v))
	}
	return nil
}
