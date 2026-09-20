package graph_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/domain"
	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/store"
)

// insertPendingCall seeds a single PENDING_APPROVAL call so mutation tests
// have a real subject, mirroring seedCalls' pattern.
func insertPendingCall(t *testing.T, s store.Store, now time.Time, id string, severity int) {
	t.Helper()
	enqueuedAt := now
	require.NoError(t, s.InsertCall(context.Background(), domain.Call{
		ID:         id,
		Transcript: "test transcript",
		Severity:   severity,
		ZoneID:     "zone-1",
		Status:     domain.CallStatusPendingApproval,
		CreatedAt:  enqueuedAt,
		EnqueuedAt: &enqueuedAt,
	}))
}

const proposeAssignCallMutation = `
mutation($input: ProposeAssignCallInput!) {
  proposeAssignCall(input: $input) {
    id
    type
    status
    proposedBy
    agentRunId
    rationale
  }
}`

func TestContractMutation_ProposeAssignCallDefaultsProposedByOperator(t *testing.T) {
	now := time.Now()
	server, s := newTestServer(t, now)
	insertPendingCall(t, s, now, "call-1", 1)

	resp := execQuery(t, server, proposeAssignCallMutation, map[string]any{
		"input": map[string]any{
			"callId":    "call-1",
			"unitId":    "unit-01",
			"rationale": "nearest ALS unit",
		},
	})
	require.Empty(t, resp.Errors)

	var payload struct {
		ProposeAssignCall struct {
			ID         string  `json:"id"`
			Type       string  `json:"type"`
			Status     string  `json:"status"`
			ProposedBy string  `json:"proposedBy"`
			AgentRunID *string `json:"agentRunId"`
			Rationale  string  `json:"rationale"`
		} `json:"proposeAssignCall"`
	}
	require.NoError(t, json.Unmarshal(resp.Data, &payload))
	require.Equal(t, "ASSIGN_CALL", payload.ProposeAssignCall.Type)
	require.Equal(t, "PROPOSED", payload.ProposeAssignCall.Status)
	require.Equal(t, "OPERATOR", payload.ProposeAssignCall.ProposedBy)
	require.Nil(t, payload.ProposeAssignCall.AgentRunID)
}

func TestContractMutation_ProposeAssignCallHonorsAgentProposedBy(t *testing.T) {
	now := time.Now()
	server, s := newTestServer(t, now)
	insertPendingCall(t, s, now, "call-1", 1)

	resp := execQuery(t, server, proposeAssignCallMutation, map[string]any{
		"input": map[string]any{
			"callId":     "call-1",
			"unitId":     "unit-01",
			"rationale":  "agent-recommended reroute",
			"proposedBy": "AGENT",
			"agentRunId": "run-123",
		},
	})
	require.Empty(t, resp.Errors)

	var payload struct {
		ProposeAssignCall struct {
			ProposedBy string  `json:"proposedBy"`
			AgentRunID *string `json:"agentRunId"`
		} `json:"proposeAssignCall"`
	}
	require.NoError(t, json.Unmarshal(resp.Data, &payload))
	require.Equal(t, "AGENT", payload.ProposeAssignCall.ProposedBy)
	require.NotNil(t, payload.ProposeAssignCall.AgentRunID)
	require.Equal(t, "run-123", *payload.ProposeAssignCall.AgentRunID)
}

func TestContractMutation_ExecuteActionAssignsUnitAndCall(t *testing.T) {
	now := time.Now()
	server, s := newTestServer(t, now)
	insertPendingCall(t, s, now, "call-1", 1)

	proposeResp := execQuery(t, server, proposeAssignCallMutation, map[string]any{
		"input": map[string]any{
			"callId":    "call-1",
			"unitId":    "unit-01",
			"rationale": "nearest ALS unit",
		},
	})
	require.Empty(t, proposeResp.Errors)
	var proposePayload struct {
		ProposeAssignCall struct {
			ID string `json:"id"`
		} `json:"proposeAssignCall"`
	}
	require.NoError(t, json.Unmarshal(proposeResp.Data, &proposePayload))
	actionID := proposePayload.ProposeAssignCall.ID
	require.NotEmpty(t, actionID)

	execResp := execQuery(t, server, `
	mutation($id: ID!) {
	  executeAction(actionId: $id) {
	    id
	    status
	    decidedBy
	  }
	}`, map[string]any{"id": actionID})
	require.Empty(t, execResp.Errors)

	var execPayload struct {
		ExecuteAction struct {
			ID        string  `json:"id"`
			Status    string  `json:"status"`
			DecidedBy *string `json:"decidedBy"`
		} `json:"executeAction"`
	}
	require.NoError(t, json.Unmarshal(execResp.Data, &execPayload))
	require.Equal(t, "EXECUTED", execPayload.ExecuteAction.Status)
	require.NotNil(t, execPayload.ExecuteAction.DecidedBy)
	require.Equal(t, testOperatorID, *execPayload.ExecuteAction.DecidedBy)

	call, err := s.GetCall(context.Background(), "call-1")
	require.NoError(t, err)
	require.Equal(t, domain.CallStatusAssigned, call.Status)

	unit, err := s.GetUnit(context.Background(), "unit-01")
	require.NoError(t, err)
	require.Equal(t, domain.UnitStatusEnRoute, unit.Status)
}

func TestContractMutation_RejectActionRecordsReason(t *testing.T) {
	now := time.Now()
	server, s := newTestServer(t, now)
	insertPendingCall(t, s, now, "call-1", 1)

	proposeResp := execQuery(t, server, proposeAssignCallMutation, map[string]any{
		"input": map[string]any{
			"callId":    "call-1",
			"unitId":    "unit-01",
			"rationale": "nearest ALS unit",
		},
	})
	require.Empty(t, proposeResp.Errors)
	var proposePayload struct {
		ProposeAssignCall struct {
			ID string `json:"id"`
		} `json:"proposeAssignCall"`
	}
	require.NoError(t, json.Unmarshal(proposeResp.Data, &proposePayload))
	actionID := proposePayload.ProposeAssignCall.ID

	rejectResp := execQuery(t, server, `
	mutation($id: ID!, $reason: RejectionReason!, $note: String) {
	  rejectAction(actionId: $id, reason: $reason, note: $note) {
	    id
	    status
	    rejectionReason
	    rejectionNote
	  }
	}`, map[string]any{"id": actionID, "reason": "WRONG_UNIT", "note": "closer unit available"})
	require.Empty(t, rejectResp.Errors)

	var rejectPayload struct {
		RejectAction struct {
			Status          string  `json:"status"`
			RejectionReason *string `json:"rejectionReason"`
			RejectionNote   *string `json:"rejectionNote"`
		} `json:"rejectAction"`
	}
	require.NoError(t, json.Unmarshal(rejectResp.Data, &rejectPayload))
	require.Equal(t, "REJECTED", rejectPayload.RejectAction.Status)
	require.NotNil(t, rejectPayload.RejectAction.RejectionReason)
	require.Equal(t, "WRONG_UNIT", *rejectPayload.RejectAction.RejectionReason)
	require.Equal(t, "closer unit available", *rejectPayload.RejectAction.RejectionNote)

	// Rejection mutates no fleet state (schema doc comment on rejectAction).
	call, err := s.GetCall(context.Background(), "call-1")
	require.NoError(t, err)
	require.Equal(t, domain.CallStatusPendingApproval, call.Status)
}

func TestContractMutation_AssignCallIsManualAndImmediate(t *testing.T) {
	now := time.Now()
	server, s := newTestServer(t, now)
	insertPendingCall(t, s, now, "call-1", 1)

	resp := execQuery(t, server, `
	mutation($callId: ID!, $unitId: ID!) {
	  assignCall(callId: $callId, unitId: $unitId) {
	    id
	    status
	    proposedBy
	    decidedBy
	  }
	}`, map[string]any{"callId": "call-1", "unitId": "unit-01"})
	require.Empty(t, resp.Errors)

	var payload struct {
		AssignCall struct {
			Status     string  `json:"status"`
			ProposedBy string  `json:"proposedBy"`
			DecidedBy  *string `json:"decidedBy"`
		} `json:"assignCall"`
	}
	require.NoError(t, json.Unmarshal(resp.Data, &payload))
	require.Equal(t, "EXECUTED", payload.AssignCall.Status)
	require.Equal(t, "OPERATOR", payload.AssignCall.ProposedBy)
	require.NotNil(t, payload.AssignCall.DecidedBy)

	call, err := s.GetCall(context.Background(), "call-1")
	require.NoError(t, err)
	require.Equal(t, domain.CallStatusAssigned, call.Status)
}

func TestContractMutation_ProposeAssignCallUnknownCallReturnsGraphQLErrorNot500(t *testing.T) {
	now := time.Now()
	server, _ := newTestServer(t, now)

	resp := execQuery(t, server, proposeAssignCallMutation, map[string]any{
		"input": map[string]any{
			"callId":    "no-such-call",
			"unitId":    "unit-01",
			"rationale": "nearest ALS unit",
		},
	})
	require.NotEmpty(t, resp.Errors)
}
