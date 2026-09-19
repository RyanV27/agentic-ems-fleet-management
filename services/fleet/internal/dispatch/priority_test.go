package dispatch_test

import (
	"testing"
	"time"

	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/dispatch"
	"github.com/stretchr/testify/require"
)

func testConfig() dispatch.PriorityConfig {
	return dispatch.PriorityConfig{
		SeverityWeights: map[int]float64{1: 1000, 2: 100, 3: 10},
		AgingRate:       1.0,
	}
}

func TestPriorityScore_MatchesBriefFormula(t *testing.T) {
	cfg := testConfig()
	enqueuedAt := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	cases := []struct {
		name     string
		severity int
		now      time.Time
		want     float64
	}{
		{"P1 no wait", 1, enqueuedAt, 1000},
		{"P2 no wait", 2, enqueuedAt, 100},
		{"P3 no wait", 3, enqueuedAt, 10},
		{"P3 waited 90s", 3, enqueuedAt.Add(90 * time.Second), 100},
		{"P1 waited 500s", 1, enqueuedAt.Add(500 * time.Second), 1500},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := dispatch.PriorityScore(tc.severity, enqueuedAt, tc.now, cfg)
			require.InDelta(t, tc.want, got, 1e-9)
		})
	}
}

func TestPriorityScore_AgingPreventsStarvation(t *testing.T) {
	cfg := testConfig()
	now := time.Date(2026, 1, 1, 1, 0, 0, 0, time.UTC)

	// A P3 call enqueued long enough must outrank a freshly-arrived P1
	// (crossover computed from config, not hardcoded, per S3 c9).
	crossoverSeconds := (cfg.SeverityWeights[1] - cfg.SeverityWeights[3]) / cfg.AgingRate
	oldP3EnqueuedAt := now.Add(-time.Duration(crossoverSeconds+1) * time.Second)
	freshP1EnqueuedAt := now

	oldP3Score := dispatch.PriorityScore(3, oldP3EnqueuedAt, now, cfg)
	freshP1Score := dispatch.PriorityScore(1, freshP1EnqueuedAt, now, cfg)

	require.Greater(t, oldP3Score, freshP1Score)
}

func TestPriorityScore_OrderingInvariantInNow(t *testing.T) {
	cfg := testConfig()
	enqueuedA := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	enqueuedB := enqueuedA.Add(30 * time.Second)

	now1 := enqueuedB.Add(10 * time.Second)
	now2 := now1.Add(500 * time.Second)

	scoreA1 := dispatch.PriorityScore(2, enqueuedA, now1, cfg)
	scoreB1 := dispatch.PriorityScore(2, enqueuedB, now1, cfg)
	scoreA2 := dispatch.PriorityScore(2, enqueuedA, now2, cfg)
	scoreB2 := dispatch.PriorityScore(2, enqueuedB, now2, cfg)

	require.Equal(t, scoreA1 > scoreB1, scoreA2 > scoreB2)
}
