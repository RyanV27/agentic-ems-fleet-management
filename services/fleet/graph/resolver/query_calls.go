package resolver

import (
	"context"
	"errors"
	"sort"

	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/graph/generated"
	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/domain"
	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/store"
)

// Calls returns results ordered by priorityScore descending, computed at
// query time from the sim clock's now via the one shared dispatch.PriorityScore
// function (DEC-020, S2 c4) — the same function S3's queue will use, so the
// displayed order and the heap's pop order cannot diverge. Ties break on
// ascending call id.
func (r *queryResolver) Calls(ctx context.Context, filter *generated.CallsFilter) ([]*generated.Call, error) {
	calls, err := r.Store.ListCalls(ctx)
	if err != nil {
		return nil, err
	}

	filtered := make([]domain.Call, 0, len(calls))
	for _, c := range calls {
		if filter != nil && filter.Status != nil && c.Status != domain.CallStatus(*filter.Status) {
			continue
		}
		if filter != nil && filter.Severity != nil && c.Severity != *filter.Severity {
			continue
		}
		filtered = append(filtered, c)
	}

	mapped := make([]*generated.Call, len(filtered))
	for i, c := range filtered {
		gc, err := r.mapCallWithExtraction(ctx, c)
		if err != nil {
			return nil, err
		}
		mapped[i] = gc
	}

	sort.Slice(mapped, func(i, j int) bool { return lessPriority(mapped[i], mapped[j]) })
	return mapped, nil
}

func (r *queryResolver) Call(ctx context.Context, id string) (*generated.Call, error) {
	c, err := r.Store.GetCall(ctx, id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return r.mapCallWithExtraction(ctx, c)
}

// mapCallWithExtraction joins the call's Extraction (nil until extraction
// completes) and computes priorityScore.
func (r *queryResolver) mapCallWithExtraction(ctx context.Context, c domain.Call) (*generated.Call, error) {
	var extraction *generated.Extraction
	e, err := r.Store.GetExtractionByCallID(ctx, c.ID)
	switch {
	case err == nil:
		extraction = mapExtraction(e)
	case errors.Is(err, store.ErrNotFound):
		// no extraction yet — leave nil.
	default:
		return nil, err
	}

	now := r.Clock.Now()
	score := callPriorityScore(c, now, r.PriorityConfig)
	return mapCall(c, extraction, score), nil
}

// lessPriority reports whether a should sort before b: descending
// priorityScore (nil treated as -inf, since such a call never entered the
// operator work queue), tied on ascending call id — the same tie-break
// dispatch's heap will use.
func lessPriority(a, b *generated.Call) bool {
	as, bs := scoreOrNegInf(a), scoreOrNegInf(b)
	if as != bs {
		return as > bs
	}
	return a.ID < b.ID
}

func scoreOrNegInf(c *generated.Call) float64 {
	if c.PriorityScore == nil {
		return negInf
	}
	return *c.PriorityScore
}

const negInf = -1e18
