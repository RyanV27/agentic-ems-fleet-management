package resolver

import (
	"time"

	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/graph/generated"
	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/dispatch"
	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/domain"
)

func mapUnit(u domain.Unit) *generated.Unit {
	return &generated.Unit{
		ID:              u.ID,
		Callsign:        u.Callsign,
		Status:          generated.UnitStatus(u.Status),
		Capability:      generated.Capability(u.Capability),
		ZoneID:          u.ZoneID,
		CurrentCallID:   u.CurrentCallID,
		StatusChangedAt: u.StatusChangedAt,
	}
}

func mapExtraction(e domain.Extraction) *generated.Extraction {
	var failureReason *string
	if e.FailureReason != "" {
		failureReason = &e.FailureReason
	}
	return &generated.Extraction{
		CallID:           e.CallID,
		IncidentType:     e.IncidentType,
		Severity:         e.Severity,
		ZoneID:           e.ZoneID,
		Keywords:         e.Keywords,
		NeedsTransport:   e.NeedsTransport,
		Confidence:       e.Confidence,
		FailureReason:    failureReason,
		Model:            e.Model,
		LatencyMs:        int(e.LatencyMs),
		PromptTokens:     e.PromptTokens,
		CompletionTokens: e.CompletionTokens,
		CreatedAt:        e.CreatedAt,
	}
}

// mapCall maps a domain.Call to the GraphQL model. extraction is looked up
// by the caller (a separate store call) since domain.Call.Extraction is only
// populated by callers that chose to join it; priorityScore is computed by
// the caller too, since it needs "now" and config the mapper doesn't have.
func mapCall(c domain.Call, extraction *generated.Extraction, priorityScore *float64) *generated.Call {
	return &generated.Call{
		ID:                    c.ID,
		Transcript:            c.Transcript,
		Extraction:            extraction,
		Severity:              c.Severity,
		ZoneID:                c.ZoneID,
		Status:                generated.CallStatus(c.Status),
		AssignedUnitID:        c.AssignedUnitID,
		DestinationHospitalID: c.DestinationHospitalID,
		CreatedAt:             c.CreatedAt,
		EnqueuedAt:            c.EnqueuedAt,
		PriorityScore:         priorityScore,
	}
}

func mapZone(z domain.Zone) *generated.Zone {
	return &generated.Zone{ID: z.ID, Name: z.Name}
}

func mapHospital(h domain.Hospital) *generated.Hospital {
	return &generated.Hospital{
		ID:           h.ID,
		Name:         h.Name,
		ZoneID:       h.ZoneID,
		Capabilities: h.Capabilities,
		Accepting:    h.Accepting,
	}
}

func mapDispatchEvent(e domain.DispatchEvent) *generated.DispatchEvent {
	return &generated.DispatchEvent{
		ID:            e.ID,
		Type:          generated.DispatchEventType(e.Type),
		CallID:        e.CallID,
		UnitID:        e.UnitID,
		SeverityDelta: e.SeverityDelta,
		Status:        generated.DispatchEventStatus(e.Status),
		Note:          e.Note,
		CreatedAt:     e.CreatedAt,
		ResolvedAt:    e.ResolvedAt,
	}
}

func mapPendingAction(a domain.PendingAction) *generated.PendingAction {
	var expiredReason *generated.ExpiredReason
	if a.ExpiredReason != nil {
		v := generated.ExpiredReason(*a.ExpiredReason)
		expiredReason = &v
	}
	var rejectionReason *generated.RejectionReason
	if a.RejectionReason != nil {
		v := generated.RejectionReason(*a.RejectionReason)
		rejectionReason = &v
	}
	return &generated.PendingAction{
		ID:              a.ID,
		Type:            generated.ActionType(a.Type),
		Payload:         string(a.Payload),
		Status:          generated.ActionStatus(a.Status),
		IdempotencyKey:  a.IdempotencyKey,
		ProposedBy:      generated.ProposedBy(a.ProposedBy),
		Rationale:       a.Rationale,
		ExpiredReason:   expiredReason,
		AgentRunID:      a.AgentRunID,
		RejectionReason: rejectionReason,
		RejectionNote:   a.RejectionNote,
		ParentActionID:  a.ParentActionID,
		CreatedAt:       a.CreatedAt,
		DecidedAt:       a.DecidedAt,
		DecidedBy:       a.DecidedBy,
	}
}

func mapRoutingDecision(d domain.RoutingDecision) *generated.RoutingDecision {
	return &generated.RoutingDecision{
		ID:            d.ID,
		CallID:        d.CallID,
		Path:          generated.RoutingPath(d.Path),
		MatchedRuleID: d.MatchedRuleID,
		InputSnapshot: string(d.InputSnapshot),
		DecidedAt:     d.DecidedAt,
	}
}

func mapAgentRun(r domain.AgentRun) *generated.AgentRun {
	var latencyMs *int
	if r.LatencyMs != nil {
		v := int(*r.LatencyMs)
		latencyMs = &v
	}
	var stopReason *generated.AgentRunStopReason
	if r.StopReason != nil {
		v := generated.AgentRunStopReason(*r.StopReason)
		stopReason = &v
	}
	return &generated.AgentRun{
		ID:               r.ID,
		CallID:           r.CallID,
		Trigger:          r.Trigger,
		Attempt:          r.Attempt,
		Status:           generated.AgentRunStatus(r.Status),
		StartedAt:        r.StartedAt,
		EndedAt:          r.EndedAt,
		LatencyMs:        latencyMs,
		PromptTokens:     r.PromptTokens,
		CompletionTokens: r.CompletionTokens,
		CostUsd:          r.CostUsd,
		StopReason:       stopReason,
	}
}

func mapToolCallLog(l domain.ToolCallLog) *generated.ToolCallLog {
	return &generated.ToolCallLog{
		ID:         l.ID,
		AgentRunID: l.AgentRunID,
		Seq:        l.Seq,
		ToolName:   l.ToolName,
		Input:      string(l.Input),
		Output:     string(l.Output),
		LatencyMs:  int(l.LatencyMs),
		Error:      l.Error,
	}
}

// callPriorityScore computes dispatch.PriorityScore for a call that has an
// EnqueuedAt, or returns nil for one that does not (it never entered the
// operator work queue).
func callPriorityScore(c domain.Call, now time.Time, cfg dispatch.PriorityConfig) *float64 {
	if c.EnqueuedAt == nil {
		return nil
	}
	score := dispatch.PriorityScore(c.Severity, *c.EnqueuedAt, now, cfg)
	return &score
}
