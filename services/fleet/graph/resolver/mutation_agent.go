package resolver

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/graph/generated"
	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/domain"
)

// These three resolvers are the write side of AgentRun/ToolCallLog telemetry
// (S7, ROADMAP S7 c5). They are the agent's own bookkeeping, not a fleet-state
// mutation, so — unlike every other resolver in this package — they call
// r.Store directly instead of going through actions.Manager (DEC-041).

func (r *mutationResolver) StartAgentRun(ctx context.Context, input generated.StartAgentRunInput) (*generated.AgentRun, error) {
	run := domain.AgentRun{
		ID:        uuid.NewString(),
		CallID:    input.CallID,
		Trigger:   input.Trigger,
		Attempt:   input.Attempt,
		Status:    domain.AgentRunStatusRunning,
		StartedAt: r.Clock.Now(),
	}
	if err := r.Store.InsertAgentRun(ctx, run); err != nil {
		return nil, fmt.Errorf("start agent run: %w", err)
	}
	return mapAgentRun(run), nil
}

func (r *mutationResolver) RecordToolCallLog(ctx context.Context, input generated.RecordToolCallLogInput) (*generated.ToolCallLog, error) {
	log := domain.ToolCallLog{
		ID:         uuid.NewString(),
		AgentRunID: input.AgentRunID,
		Seq:        input.Seq,
		ToolName:   input.ToolName,
		Input:      []byte(input.Input),
		Output:     []byte(input.Output),
		LatencyMs:  int64(input.LatencyMs),
		Error:      input.Error,
	}
	if err := r.Store.InsertToolCallLog(ctx, log); err != nil {
		return nil, fmt.Errorf("record tool call log: %w", err)
	}
	return mapToolCallLog(log), nil
}

func (r *mutationResolver) CompleteAgentRun(ctx context.Context, input generated.CompleteAgentRunInput) (*generated.AgentRun, error) {
	run, err := r.Store.GetAgentRun(ctx, input.AgentRunID)
	if err != nil {
		return nil, fmt.Errorf("complete agent run: %w", err)
	}
	endedAt := r.Clock.Now()
	run.Status = domain.AgentRunStatus(input.Status)
	run.EndedAt = &endedAt
	latencyMs := int64(input.LatencyMs)
	run.LatencyMs = &latencyMs
	run.PromptTokens = input.PromptTokens
	run.CompletionTokens = input.CompletionTokens
	run.CostUsd = input.CostUsd
	if input.StopReason != nil {
		stopReason := domain.AgentRunStopReason(*input.StopReason)
		run.StopReason = &stopReason
	}
	if err := r.Store.UpdateAgentRun(ctx, run); err != nil {
		return nil, fmt.Errorf("complete agent run: %w", err)
	}
	return mapAgentRun(run), nil
}
