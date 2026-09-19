// Package dispatch is PURE: no I/O, no clock reads, no DB (ROADMAP.md S3).
// It holds the operator work queue — the calls awaiting a human, status
// PENDING_APPROVAL or ESCALATED (DEC-021) — and unit selection. The queue
// itself (container/heap) and Select land with S3; PriorityScore lands here
// first because S2's calls query needs it too (DEC-020 requires one shared
// scoring function so the GraphQL-visible priorityScore and the heap's pop
// order can never diverge — see DECISIONS.md DEC-028).
package dispatch

import "time"

// PriorityConfig is the subset of config.Config that PriorityScore needs.
// Kept local to this package (rather than importing internal/config) so the
// package stays free of any dependency beyond the standard library.
type PriorityConfig struct {
	SeverityWeights map[int]float64
	AgingRate       float64
}

// PriorityScore implements the brief's dynamic score (DEC-020):
//
//	severityWeight(severity) + (now - enqueuedAt) * agingRate
//
// now is always a parameter — this package never calls time.Now(). The
// score is computed on every call, never stored.
func PriorityScore(severity int, enqueuedAt, now time.Time, cfg PriorityConfig) float64 {
	waitingSeconds := now.Sub(enqueuedAt).Seconds()
	return cfg.SeverityWeights[severity] + waitingSeconds*cfg.AgingRate
}
