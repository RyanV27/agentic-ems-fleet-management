package domain

import (
	"encoding/json"
	"time"
)

// RoutingDecision is written exactly once per intake and is how hard rule 2
// ("the LLM never makes the routing decision") is proven, not just asserted
// (FR-26).
type RoutingDecision struct {
	ID            string
	CallID        string
	Path          RoutingPath
	MatchedRuleID string
	InputSnapshot json.RawMessage
	DecidedAt     time.Time
}

// AgentRun is one invocation of the TypeScript triage agent.
type AgentRun struct {
	ID               string
	CallID           string
	Trigger          string
	Attempt          int
	Status           AgentRunStatus
	StartedAt        time.Time
	EndedAt          *time.Time
	LatencyMs        *int64
	PromptTokens     int
	CompletionTokens int
	CostUsd          float64
	StopReason       *AgentRunStopReason
}

// ToolCallLog is one tool call made during an AgentRun, ordered by Seq —
// exactly what the eval suite asserts an exact sequence match against.
type ToolCallLog struct {
	ID         string
	AgentRunID string
	Seq        int
	ToolName   string
	Input      json.RawMessage
	Output     json.RawMessage
	LatencyMs  int64
	Error      *string
}
