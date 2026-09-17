package sqlite

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// normalizeTime round-trips t through the same RFC3339Nano text encoding
// the store uses, so a time.Time built directly in a test compares equal to
// one read back from the DB (time.Time's internal representation is not
// guaranteed identical across construction paths for the same instant).
func normalizeTime(t time.Time) time.Time {
	nt, err := stringToTime(timeToString(t))
	if err != nil {
		panic(err)
	}
	return nt
}

var testDBCounter atomic.Int64

// newTestStore opens a uniquely-named in-memory SQLite database (so
// parallel tests never share state) and migrates it. S1 c6: every store
// test runs against file::memory:?cache=shared and leaves no files behind.
func newTestStore(t *testing.T) *Store {
	t.Helper()
	n := testDBCounter.Add(1)
	safeName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	dsn := fmt.Sprintf("file:test-%s-%d?mode=memory&cache=shared", safeName, n)
	s, err := Open(dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })

	require.NoError(t, s.Migrate(context.Background()))
	return s
}
