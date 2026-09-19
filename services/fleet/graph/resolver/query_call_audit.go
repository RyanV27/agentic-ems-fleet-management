package resolver

import (
	"context"
	"errors"
	"sort"

	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/graph/generated"
	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/store"
)

// CallAudit answers W5: transcript (via call), routing decision(s), every
// agent run, and every tool call log across those runs ordered by seq.
func (r *queryResolver) CallAudit(ctx context.Context, id string) (*generated.CallAudit, error) {
	call, err := r.getCall(ctx, id)
	if err != nil {
		return nil, err
	}
	if call == nil {
		return nil, nil
	}

	decisions, err := r.Store.ListRoutingDecisionsByCallID(ctx, id)
	if err != nil {
		return nil, err
	}
	runs, err := r.Store.ListAgentRunsByCallID(ctx, id)
	if err != nil {
		return nil, err
	}

	mappedDecisions := make([]*generated.RoutingDecision, len(decisions))
	for i, d := range decisions {
		mappedDecisions[i] = mapRoutingDecision(d)
	}

	mappedRuns := make([]*generated.AgentRun, len(runs))
	var allLogs []*generated.ToolCallLog
	for i, run := range runs {
		mappedRuns[i] = mapAgentRun(run)
		logs, err := r.Store.ListToolCallLogsByAgentRunID(ctx, run.ID)
		if err != nil {
			return nil, err
		}
		for _, l := range logs {
			allLogs = append(allLogs, mapToolCallLog(l))
		}
	}
	sort.Slice(allLogs, func(i, j int) bool { return allLogs[i].Seq < allLogs[j].Seq })

	return &generated.CallAudit{
		Call:             call,
		RoutingDecisions: mappedDecisions,
		AgentRuns:        mappedRuns,
		ToolCallLogs:     allLogs,
	}, nil
}

// getCall returns nil, nil on ErrNotFound rather than propagating it, so
// callers can treat a missing call as an absent optional field.
func (r *queryResolver) getCall(ctx context.Context, id string) (*generated.Call, error) {
	c, err := r.Store.GetCall(ctx, id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return r.mapCallWithExtraction(ctx, c)
}
