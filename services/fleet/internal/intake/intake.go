// Package intake orchestrates one transcript from arrival to the approval
// gate (ARCHITECTURE.md §5, ROADMAP.md S5): extraction, rules.Decide,
// dispatch.Select, and actions.Propose, called exactly as S3/S4/S6 built
// them — this package adds no decision logic of its own (ROADMAP S5 c8).
// Every intake ends by pushing the call onto the operator work queue
// (DEC-021 c8b), whichever path it took.
package intake

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/actions"
	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/agentclient"
	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/dispatch"
	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/domain"
	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/rules"
)

// fallbackSeverity is used for queue prioritization and the stored Call
// record when extraction fails (confidence 0, severity left at its zero
// value) — OQ-14: an unknown severity is treated as the most urgent tier
// rather than the least, so a failed extraction can never silently starve
// in the operator queue. It never affects rules.Decide, which is always
// given the raw (possibly zero-severity) extraction and escalates on
// low confidence before severity is ever consulted.
const fallbackSeverity = 1

// Extractor is the LLM boundary intake depends on (ARCHITECTURE.md §6
// boundary 1). extract.Client satisfies this; tests supply a stub.
type Extractor interface {
	Extract(ctx context.Context, transcript string) (domain.Extraction, error)
}

// AgentClient is the TS agent boundary intake depends on (ARCHITECTURE.md §6
// boundary 2). agentclient.Client satisfies this; tests use
// agentclient.RecordingClient.
type AgentClient interface {
	Triage(ctx context.Context, req agentclient.TriageRequest) error
}

// Handler is the intake orchestrator. It reuses actions.Manager's Store,
// Queue, and Clock rather than duplicating them.
type Handler struct {
	Manager     *actions.Manager
	Extractor   Extractor
	AgentClient AgentClient

	RulesConfig rules.RulesConfig
	TravelTimes dispatch.TravelTimeTable

	// AgentEnabled gates the escalated path only (DEC-015 scope note,
	// ROADMAP S5 c10) — the deterministic path proposes identically either
	// way.
	AgentEnabled bool

	// triageSem bounds concurrent agentclient.Triage calls at
	// AGENT_MAX_CONCURRENT_TRIAGE (DEC-018). It is acquired inside the
	// background goroutine, never on Handle's synchronous path, so an
	// over-cap call still returns to its caller immediately (ROADMAP S5 c5,
	// c9) and simply waits its turn in the background.
	triageSem chan struct{}
}

// NewHandler builds a Handler. maxConcurrentTriage must be positive
// (config.Config.AgentMaxConcurrentTriage is range-checked at load).
func NewHandler(mgr *actions.Manager, extractor Extractor, agentClient AgentClient, rulesCfg rules.RulesConfig, travelTimes dispatch.TravelTimeTable, agentEnabled bool, maxConcurrentTriage int) *Handler {
	return &Handler{
		Manager:      mgr,
		Extractor:    extractor,
		AgentClient:  agentClient,
		RulesConfig:  rulesCfg,
		TravelTimes:  travelTimes,
		AgentEnabled: agentEnabled,
		triageSem:    make(chan struct{}, maxConcurrentTriage),
	}
}

// Handle runs one transcript through the full intake pipeline up to the
// approval gate (ROADMAP.md S5 c1): it creates the Call, runs extraction,
// calls rules.Decide, and — for the deterministic path — dispatch.Select and
// actions.Propose. It returns once the call is persisted, decided, and
// enqueued; escalated-path agent triage is asynchronous and never delays
// this return (boundary 2, c5).
//
// zoneHint is a caller-supplied fallback (OQ-13): it is used only when
// extraction fails to produce a zone at all, never to override a zone the
// transcript itself yielded.
func (h *Handler) Handle(ctx context.Context, transcript, zoneHint string) (domain.Call, error) {
	now := h.Manager.Clock.Now()
	callID := uuid.NewString()

	extraction, err := h.Extractor.Extract(ctx, transcript)
	if err != nil {
		return domain.Call{}, fmt.Errorf("intake: extract call %s: %w", callID, err)
	}
	extraction.CallID = callID

	zoneID := extraction.ZoneID
	if zoneID == "" {
		zoneID = zoneHint
	}
	severity := extraction.Severity
	if severity < 1 || severity > 3 {
		severity = fallbackSeverity
	}

	snapshot, err := h.fleetSnapshot(ctx, now)
	if err != nil {
		return domain.Call{}, fmt.Errorf("intake: snapshot for call %s: %w", callID, err)
	}

	decision := rules.Decide(extraction, snapshot, h.RulesConfig)

	enqueuedAt := now
	call := domain.Call{
		ID:         callID,
		Transcript: transcript,
		Extraction: &extraction,
		Severity:   severity,
		ZoneID:     zoneID,
		CreatedAt:  now,
		EnqueuedAt: &enqueuedAt,
	}

	var selected *dispatch.UnitSnapshot
	if decision.Path == domain.RoutingPathDeterministic {
		call.Status = domain.CallStatusPendingApproval

		result := dispatch.Select(dispatch.SelectInput{
			Severity:    extraction.Severity,
			ZoneID:      zoneID,
			Units:       snapshot.Units,
			TravelTimes: h.TravelTimes,
		})
		if !result.Found() {
			// rules.Decide only returns DETERMINISTIC when a suitable unit
			// is available under the identical capability rule (DEC-014)
			// dispatch.Select applies — a miss here means rules and
			// dispatch have diverged, which is a bug in one of those pure
			// packages, not a condition this orchestration layer routes
			// around (ROADMAP S5 c8: it calls them as-is).
			return domain.Call{}, fmt.Errorf("intake: rules.Decide returned DETERMINISTIC but dispatch.Select found no unit for call %s", callID)
		}
		selected = result.Unit
	} else {
		call.Status = domain.CallStatusEscalated
	}

	routingDecision := domain.RoutingDecision{
		ID:            uuid.NewString(),
		CallID:        callID,
		Path:          decision.Path,
		MatchedRuleID: decision.MatchedRuleID,
		InputSnapshot: buildInputSnapshot(extraction, snapshot),
		DecidedAt:     now,
	}

	err = h.Manager.Store.WithTx(ctx, func(ctx context.Context) error {
		if err := h.Manager.Store.InsertCall(ctx, call); err != nil {
			return err
		}
		if err := h.Manager.Store.InsertExtraction(ctx, extraction); err != nil {
			return err
		}
		if err := h.Manager.Store.InsertRoutingDecision(ctx, routingDecision); err != nil {
			return err
		}
		if decision.Path == domain.RoutingPathDeterministic {
			rationale := fmt.Sprintf("Deterministic dispatch (%s): nearest capable unit for severity %d in zone %s.",
				decision.MatchedRuleID, extraction.Severity, zoneID)
			if _, err := h.Manager.ProposeAssignCall(ctx, callID, selected.ID, actions.ProposeInput{
				ProposedBy: domain.ProposedBySystem,
				Rationale:  rationale,
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return domain.Call{}, fmt.Errorf("intake: persist intake for call %s: %w", callID, err)
	}

	// The queue is in-memory, not part of the DB transaction above, but it
	// is only touched once persistence has committed (DEC-021: the queue's
	// only producer).
	h.Manager.Queue.Push(dispatch.QueueItem{CallID: callID, Severity: severity, EnqueuedAt: enqueuedAt}, now)

	if decision.Path == domain.RoutingPathEscalated && h.AgentEnabled {
		h.triageAsync(callID, string(decision.Reason))
	}

	return call, nil
}

// triageAsync fires agentclient.Triage in the background, bounded by
// triageSem (DEC-018). Handle never waits on this goroutine.
func (h *Handler) triageAsync(callID, reason string) {
	go func() {
		h.triageSem <- struct{}{}
		defer func() { <-h.triageSem }()

		// A fresh context: the request that created this call has already
		// returned by the time this runs, so ctx cannot be its ctx.
		_ = h.AgentClient.Triage(context.Background(), agentclient.TriageRequest{
			CallID:  callID,
			Reason:  reason,
			Attempt: 1,
		})
		// Any error is exactly boundary 2's contract: the call simply stays
		// ESCALATED and visible for manual dispatch (REL-2). Go does not
		// retry here — retry policy, if any, is the TS agent's concern.
	}()
}

// fleetSnapshot builds rules.FleetSnapshot from current store state. Pending
// calls are read from calls in PENDING_APPROVAL/ESCALATED status rather than
// from dispatch.Queue directly, since DEC-021 defines the queue as exactly
// that set and the Queue type exposes no full iteration.
func (h *Handler) fleetSnapshot(ctx context.Context, now time.Time) (rules.FleetSnapshot, error) {
	units, err := h.Manager.Store.ListUnits(ctx)
	if err != nil {
		return rules.FleetSnapshot{}, fmt.Errorf("intake: list units: %w", err)
	}
	calls, err := h.Manager.Store.ListCalls(ctx)
	if err != nil {
		return rules.FleetSnapshot{}, fmt.Errorf("intake: list calls: %w", err)
	}

	unitSnapshots := make([]dispatch.UnitSnapshot, len(units))
	for i, u := range units {
		unitSnapshots[i] = dispatch.UnitSnapshot{ID: u.ID, Status: u.Status, Capability: u.Capability, ZoneID: u.ZoneID}
	}

	pending := make([]rules.PendingCall, 0, len(calls))
	for _, c := range calls {
		if c.EnqueuedAt == nil {
			continue
		}
		if c.Status != domain.CallStatusPendingApproval && c.Status != domain.CallStatusEscalated {
			continue
		}
		pending = append(pending, rules.PendingCall{Severity: c.Severity, EnqueuedAt: *c.EnqueuedAt})
	}

	return rules.FleetSnapshot{Units: unitSnapshots, PendingCalls: pending, Now: now}, nil
}

// inputSnapshotRecord is RoutingDecision.InputSnapshot's shape (DEC-039):
// the full extraction plus the fleet counts rules.Decide actually weighed,
// which is what proves hard rule 2 — the LLM's extraction feeding a Go
// decision, not the LLM deciding.
type inputSnapshotRecord struct {
	Extraction        domain.Extraction `json:"extraction"`
	AvailableUnits    int               `json:"availableUnits"`
	PendingCallsCount int               `json:"pendingCallsCount"`
}

func buildInputSnapshot(extraction domain.Extraction, snapshot rules.FleetSnapshot) json.RawMessage {
	available := 0
	for _, u := range snapshot.Units {
		if u.Status == domain.UnitStatusAvailable {
			available++
		}
	}
	b, err := json.Marshal(inputSnapshotRecord{
		Extraction:        extraction,
		AvailableUnits:    available,
		PendingCallsCount: len(snapshot.PendingCalls),
	})
	if err != nil {
		// extraction (plain fields/strings) plus two ints cannot fail to
		// marshal; this is unreachable in practice.
		return json.RawMessage(`{}`)
	}
	return b
}
