package dispatch_test

import (
	"fmt"
	"go/build"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/dispatch"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testQueueConfig() dispatch.PriorityConfig {
	return dispatch.PriorityConfig{
		SeverityWeights: map[int]float64{1: 1000, 2: 100, 3: 10},
		AgingRate:       1.0,
	}
}

func randomQueueItems(rng *rand.Rand, n int, base time.Time) []dispatch.QueueItem {
	items := make([]dispatch.QueueItem, n)
	for i := range items {
		items[i] = dispatch.QueueItem{
			CallID:     fmt.Sprintf("call-%04d", i),
			Severity:   rng.Intn(3) + 1,
			EnqueuedAt: base.Add(-time.Duration(rng.Intn(3600)) * time.Second),
		}
	}
	return items
}

func bruteForceOrder(items []dispatch.QueueItem, now time.Time, cfg dispatch.PriorityConfig) []string {
	sorted := make([]dispatch.QueueItem, len(items))
	copy(sorted, items)
	sort.SliceStable(sorted, func(i, j int) bool {
		si := dispatch.PriorityScore(sorted[i].Severity, sorted[i].EnqueuedAt, now, cfg)
		sj := dispatch.PriorityScore(sorted[j].Severity, sorted[j].EnqueuedAt, now, cfg)
		if si != sj {
			return si > sj
		}
		return sorted[i].CallID < sorted[j].CallID
	})
	ids := make([]string, len(sorted))
	for i, item := range sorted {
		ids[i] = item.CallID
	}
	return ids
}

// TestQueue_PopOrderMatchesBruteForceSort is ROADMAP S3 property test 8a:
// across 1000 random call sets, heap pop order must equal a brute-force sort
// by PriorityScore at a fixed now.
func TestQueue_PopOrderMatchesBruteForceSort(t *testing.T) {
	cfg := testQueueConfig()
	rng := rand.New(rand.NewSource(1))
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	for trial := 0; trial < 1000; trial++ {
		n := rng.Intn(20) + 1
		items := randomQueueItems(rng, n, base)
		now := base.Add(time.Duration(rng.Intn(3600)) * time.Second)

		q := dispatch.NewQueue(cfg)
		for _, item := range items {
			q.Push(item, now)
		}

		want := bruteForceOrder(items, now, cfg)
		got := make([]string, 0, n)
		for {
			item, ok := q.Pop(now)
			if !ok {
				break
			}
			got = append(got, item.CallID)
		}

		require.Equal(t, want, got, "trial %d: heap pop order must match brute-force sort at now=%v", trial, now)
	}
}

// TestQueue_OrderingInvariantInNow is ROADMAP S3 property test 8b: because
// AgingRate is uniform and linear, pop order must not change as now varies
// — the (now * agingRate) term is identical for every element at any given
// instant. If this test ever fails after changing aging to be per-severity
// or nonlinear, that invariance no longer holds and Reheapify(now) becomes
// mandatory on the sim tick (see CLAUDE.md "Two things that look like bugs
// but are not").
func TestQueue_OrderingInvariantInNow(t *testing.T) {
	cfg := testQueueConfig()
	rng := rand.New(rand.NewSource(2))
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	for trial := 0; trial < 1000; trial++ {
		n := rng.Intn(20) + 1
		items := randomQueueItems(rng, n, base)
		nowA := base.Add(time.Duration(rng.Intn(3600)) * time.Second)
		deltaSeconds := rng.Intn(3600) + 1
		nowB := nowA.Add(time.Duration(deltaSeconds) * time.Second)

		orderA := bruteForceOrder(items, nowA, cfg)
		orderB := bruteForceOrder(items, nowB, cfg)

		require.Equal(t, orderA, orderB,
			"trial %d: pop order changed between now=%v and now=%v under uniform linear aging; "+
				"if aging was made per-severity or nonlinear, Reheapify(now) is now required",
			trial, nowA, nowB)
	}
}

// TestQueue_AgingPreventsStarvation is ROADMAP S3 criterion 9. The crossover
// time — how long a P3 call must wait before it outranks a brand-new P1
// call — is computed from cfg, not hardcoded, so the test still holds if
// SEVERITY_WEIGHTS or AGING_RATE change.
func TestQueue_AgingPreventsStarvation(t *testing.T) {
	cfg := testQueueConfig()
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	crossoverSeconds := (cfg.SeverityWeights[1] - cfg.SeverityWeights[3]) / cfg.AgingRate

	q := dispatch.NewQueue(cfg)
	p3 := dispatch.QueueItem{CallID: "call-p3-aged", Severity: 3, EnqueuedAt: base}
	q.Push(p3, base)

	beforeCrossover := base.Add(time.Duration(crossoverSeconds-1) * time.Second)
	q.Push(dispatch.QueueItem{CallID: "call-p1-new", Severity: 1, EnqueuedAt: beforeCrossover}, beforeCrossover)
	top, ok := q.Peek(beforeCrossover)
	require.True(t, ok)
	assert.Equal(t, "call-p1-new", top.CallID, "just before crossover, the new P1 must still outrank the aged P3")
	require.True(t, q.Remove("call-p1-new", beforeCrossover))

	afterCrossover := base.Add(time.Duration(crossoverSeconds+1) * time.Second)
	q.Push(dispatch.QueueItem{CallID: "call-p1-new-2", Severity: 1, EnqueuedAt: afterCrossover}, afterCrossover)
	top, ok = q.Peek(afterCrossover)
	require.True(t, ok)
	assert.Equal(t, "call-p3-aged", top.CallID, "past crossover, the aged P3 must outrank the new P1")
}

// TestQueue_RemoveInterleavedWithPushPop is ROADMAP S3 property test 10:
// inserting, popping, and removing at varying now must never break the heap
// invariant — every remaining item after a Remove is still poppable in
// correct priority order.
func TestQueue_RemoveInterleavedWithPushPop(t *testing.T) {
	cfg := testQueueConfig()
	rng := rand.New(rand.NewSource(3))
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	for trial := 0; trial < 200; trial++ {
		q := dispatch.NewQueue(cfg)
		alive := map[string]dispatch.QueueItem{}
		now := base

		for step := 0; step < 50; step++ {
			now = now.Add(time.Duration(rng.Intn(10)) * time.Second)
			switch rng.Intn(3) {
			case 0: // push
				id := fmt.Sprintf("trial%d-call%d", trial, step)
				item := dispatch.QueueItem{CallID: id, Severity: rng.Intn(3) + 1, EnqueuedAt: now}
				q.Push(item, now)
				alive[id] = item
			case 1: // pop
				item, ok := q.Pop(now)
				if ok {
					delete(alive, item.CallID)
				}
			case 2: // remove a random alive item
				for id := range alive {
					if q.Remove(id, now) {
						delete(alive, id)
					}
					break
				}
			}

			require.Equal(t, len(alive), q.Len(), "trial %d step %d: queue length must match tracked alive set", trial, step)
		}

		// Drain and verify against brute-force order at the final now.
		remaining := make([]dispatch.QueueItem, 0, len(alive))
		for _, item := range alive {
			remaining = append(remaining, item)
		}
		want := bruteForceOrder(remaining, now, cfg)
		got := make([]string, 0, len(remaining))
		for {
			item, ok := q.Pop(now)
			if !ok {
				break
			}
			got = append(got, item.CallID)
		}
		require.Equal(t, want, got, "trial %d: final drain order must match brute-force sort after interleaved push/pop/remove", trial)
	}
}

// TestPurity_NoForbiddenImportsOrClockReads mirrors internal/rules's purity
// test (ROADMAP S3 criterion 12 applies to both packages): no import of
// store, extract, or net/http, and no time.Now() call anywhere in dispatch.
func TestPurity_NoForbiddenImportsOrClockReads(t *testing.T) {
	pkg, err := build.ImportDir(".", 0)
	require.NoError(t, err)

	forbidden := []string{"/internal/store", "/internal/extract", "net/http"}
	allImports := append(append([]string{}, pkg.Imports...), pkg.TestImports...)
	allImports = append(allImports, pkg.XTestImports...)
	for _, imp := range allImports {
		for _, f := range forbidden {
			assert.False(t, strings.Contains(imp, f), "dispatch must not import %s (got %s)", f, imp)
		}
	}

	for _, name := range pkg.GoFiles {
		assertNoTimeNow(t, name)
	}
}

func assertNoTimeNow(t *testing.T, filename string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(".", filename))
	require.NoError(t, err)
	assert.False(t, strings.Contains(string(data), "time.Now("), "%s must not call time.Now(); now must be a parameter", filename)
}
