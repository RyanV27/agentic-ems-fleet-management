package intake_test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/actions"
	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/agentclient"
	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/dispatch"
	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/domain"
	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/intake"
	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/rules"
	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/store/sqlite"
)

// --- fixtures, mirroring internal/actions/actions_test.go's pattern ---

var testDBCounter int64

func newTestStore(t *testing.T) *sqlite.Store {
	t.Helper()
	n := atomic.AddInt64(&testDBCounter, 1)
	dsn := fmt.Sprintf("file:intake-test-%d?mode=memory&cache=shared", n)
	s, err := sqlite.Open(dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	require.NoError(t, s.Migrate(context.Background()))
	return s
}

func testPriorityConfig() dispatch.PriorityConfig {
	return dispatch.PriorityConfig{
		SeverityWeights: map[int]float64{1: 1000, 2: 100, 3: 10},
		AgingRate:       1.0,
	}
}

func testRulesConfig() rules.RulesConfig {
	return rules.RulesConfig{ConfidenceThreshold: 0.75, Priority: testPriorityConfig()}
}

type fakeClock struct{ now time.Time }

func (c *fakeClock) Now() time.Time { return c.now }

const testOperatorID = "operator-1"
const testTTLSeconds = 600
const testMaxConcurrentTriage = 2

// stubExtractor is a fixed-output Extractor test double.
type stubExtractor struct {
	extraction domain.Extraction
	err        error
}

func (s stubExtractor) Extract(_ context.Context, _ string) (domain.Extraction, error) {
	return s.extraction, s.err
}

func highConfidenceExtraction(severity int, zoneID string) domain.Extraction {
	return domain.Extraction{
		IncidentType: "cardiac arrest",
		Severity:     severity,
		ZoneID:       zoneID,
		Confidence:   0.95,
	}
}

func lowConfidenceExtraction(severity int, zoneID string) domain.Extraction {
	return domain.Extraction{
		IncidentType: "unclear",
		Severity:     severity,
		ZoneID:       zoneID,
		Confidence:   0.1,
	}
}

func failedExtraction() domain.Extraction {
	return domain.Extraction{FailureReason: "invalid_json", Confidence: 0, Severity: 0, ZoneID: ""}
}

type fixture struct {
	store *sqlite.Store
	queue *dispatch.Queue
	clock *fakeClock
	mgr   *actions.Manager
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	st := newTestStore(t)
	q := dispatch.NewQueue(testPriorityConfig())
	clock := &fakeClock{now: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)}
	mgr := actions.NewManager(st, q, clock, testOperatorID, testTTLSeconds)
	return &fixture{store: st, queue: q, clock: clock, mgr: mgr}
}

func (f *fixture) newHandler(extractor intake.Extractor, agentClient intake.AgentClient, agentEnabled bool) *intake.Handler {
	return intake.NewHandler(f.mgr, extractor, agentClient, testRulesConfig(), dispatch.TravelTimeTable{}, agentEnabled, testMaxConcurrentTriage)
}

func (f *fixture) mustInsertZone(t *testing.T, id string) {
	t.Helper()
	require.NoError(t, f.store.InsertZone(context.Background(), domain.Zone{ID: id, Name: id}))
}

func (f *fixture) mustInsertUnit(t *testing.T, u domain.Unit) {
	t.Helper()
	require.NoError(t, f.store.InsertUnit(context.Background(), u))
}

func availableUnit(id, zoneID string, cap domain.Capability, now time.Time) domain.Unit {
	return domain.Unit{
		ID:              id,
		Callsign:        id,
		Status:          domain.UnitStatusAvailable,
		Capability:      cap,
		ZoneID:          zoneID,
		StatusChangedAt: now,
	}
}

// noopAgentClient never gets called in deterministic-path-only tests; it
// fails the test if it is.
type noopAgentClient struct{ t *testing.T }

func (n noopAgentClient) Triage(_ context.Context, _ agentclient.TriageRequest) error {
	n.t.Fatal("agent client Triage must not be called on the deterministic path")
	return nil
}

// --- c1/c2: submitTranscript creates a Call, runs extraction, decides, and
// records exactly one routing_decisions row for both paths ---

func TestHandle_DeterministicPath_CreatesCallAndRoutingDecision(t *testing.T) {
	f := newFixture(t)
	f.mustInsertZone(t, "zone-1")
	f.mustInsertUnit(t, availableUnit("unit-1", "zone-1", domain.CapabilityBLS, f.clock.now))

	h := f.newHandler(stubExtractor{extraction: highConfidenceExtraction(2, "zone-1")}, noopAgentClient{t}, true)

	call, err := h.Handle(context.Background(), "chest pain caller", "")
	require.NoError(t, err)
	assert.Equal(t, domain.CallStatusPendingApproval, call.Status)

	decisions, err := f.store.ListRoutingDecisionsByCallID(context.Background(), call.ID)
	require.NoError(t, err)
	require.Len(t, decisions, 1, "exactly one routing_decisions row per intake")
	assert.Equal(t, domain.RoutingPathDeterministic, decisions[0].Path)
	assert.Equal(t, rules.RuleDeterministicDispatch, decisions[0].MatchedRuleID)
	assert.NotEmpty(t, decisions[0].InputSnapshot)
}

func TestHandle_EscalatedPath_CreatesCallAndRoutingDecision(t *testing.T) {
	f := newFixture(t)
	f.mustInsertZone(t, "zone-1")
	rec := &agentclient.RecordingClient{}
	h := f.newHandler(stubExtractor{extraction: lowConfidenceExtraction(2, "zone-1")}, rec, true)

	call, err := h.Handle(context.Background(), "unclear transcript", "")
	require.NoError(t, err)
	assert.Equal(t, domain.CallStatusEscalated, call.Status)

	decisions, err := f.store.ListRoutingDecisionsByCallID(context.Background(), call.ID)
	require.NoError(t, err)
	require.Len(t, decisions, 1, "exactly one routing_decisions row per intake")
	assert.Equal(t, domain.RoutingPathEscalated, decisions[0].Path)
	assert.Equal(t, rules.RuleLowConfidence, decisions[0].MatchedRuleID)
}

// --- c3: deterministic path proposes; does not assign ---

func TestDeterministicPath_ProposesDoesNotAssign(t *testing.T) {
	f := newFixture(t)
	f.mustInsertZone(t, "zone-1")
	unit := availableUnit("unit-1", "zone-1", domain.CapabilityBLS, f.clock.now)
	f.mustInsertUnit(t, unit)

	otherCall := domain.Call{
		ID:         "other-call",
		Transcript: "unrelated",
		Severity:   3,
		ZoneID:     "zone-1",
		Status:     domain.CallStatusEscalated,
		CreatedAt:  f.clock.now,
	}
	require.NoError(t, f.store.InsertCall(context.Background(), otherCall))

	unitsBefore, err := f.store.ListUnits(context.Background())
	require.NoError(t, err)
	callsBefore, err := f.store.ListCalls(context.Background())
	require.NoError(t, err)

	h := f.newHandler(stubExtractor{extraction: highConfidenceExtraction(2, "zone-1")}, noopAgentClient{t}, true)
	call, err := h.Handle(context.Background(), "chest pain caller", "")
	require.NoError(t, err)

	unitsAfter, err := f.store.ListUnits(context.Background())
	require.NoError(t, err)
	assert.Equal(t, unitsBefore, unitsAfter, "no unit's fleet state may change during intake")

	callsAfter, err := f.store.ListCalls(context.Background())
	require.NoError(t, err)
	assert.Len(t, callsAfter, len(callsBefore)+1, "exactly the new call is added")
	for _, c := range callsBefore {
		found := false
		for _, ca := range callsAfter {
			if ca.ID == c.ID {
				assert.Equal(t, c, ca, "pre-existing calls must be untouched")
				found = true
			}
		}
		assert.True(t, found)
	}

	pendingActions, err := f.store.ListPendingActions(context.Background())
	require.NoError(t, err)
	require.Len(t, pendingActions, 1)
	assert.Equal(t, domain.ActionStatusProposed, pendingActions[0].Status)
	assert.Equal(t, domain.ProposedBySystem, pendingActions[0].ProposedBy)
	assert.Equal(t, domain.ActionTypeAssignCall, pendingActions[0].Type)
	assert.NotEmpty(t, pendingActions[0].Rationale)

	gotUnit, err := f.store.GetUnit(context.Background(), unit.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.UnitStatusAvailable, gotUnit.Status, "unit status must stay non-EN_ROUTE until executeAction")

	assert.Equal(t, domain.CallStatusPendingApproval, call.Status)
}

// --- c4: escalated path invokes Triage exactly once, no fleet state changes ---

func TestEscalatedPath_TriagesOnceNoFleetStateChanges(t *testing.T) {
	f := newFixture(t)
	f.mustInsertZone(t, "zone-1")
	unit := availableUnit("unit-1", "zone-1", domain.CapabilityBLS, f.clock.now)
	f.mustInsertUnit(t, unit)

	unitsBefore, err := f.store.ListUnits(context.Background())
	require.NoError(t, err)

	rec := &agentclient.RecordingClient{}
	h := f.newHandler(stubExtractor{extraction: lowConfidenceExtraction(2, "zone-1")}, rec, true)

	call, err := h.Handle(context.Background(), "unclear transcript", "")
	require.NoError(t, err)
	assert.Equal(t, domain.CallStatusEscalated, call.Status)

	require.Eventually(t, func() bool { return rec.Count() == 1 }, time.Second, time.Millisecond)
	reqs := rec.Requests()
	require.Len(t, reqs, 1)
	assert.Equal(t, call.ID, reqs[0].CallID)
	assert.Equal(t, string(rules.ReasonLowConfidence), reqs[0].Reason)

	unitsAfter, err := f.store.ListUnits(context.Background())
	require.NoError(t, err)
	assert.Equal(t, unitsBefore, unitsAfter)
}

// --- c4b: both paths enqueue; mixed batch pops in PriorityScore order ---

func TestHandle_EnqueuesOnBothPaths(t *testing.T) {
	f := newFixture(t)
	f.mustInsertZone(t, "zone-1")
	f.mustInsertUnit(t, availableUnit("unit-1", "zone-1", domain.CapabilityBLS, f.clock.now))

	h := f.newHandler(stubExtractor{extraction: highConfidenceExtraction(2, "zone-1")}, noopAgentClient{t}, true)
	call, err := h.Handle(context.Background(), "chest pain caller", "")
	require.NoError(t, err)

	require.Equal(t, 1, f.queue.Len(), "the queue is intake's own producer: exactly one entry per intake")
	item, ok := f.queue.Peek(f.clock.now)
	require.True(t, ok)
	assert.Equal(t, call.ID, item.CallID)
	require.NotNil(t, call.EnqueuedAt)
	assert.True(t, call.EnqueuedAt.Equal(f.clock.now), "enqueued_at must equal the intake instant")
}

func TestEnqueue_MixedBatchPopsInPriorityOrderNotArrivalOrder(t *testing.T) {
	f := newFixture(t)
	f.mustInsertZone(t, "zone-1")
	rec := &agentclient.RecordingClient{}

	lowPriorityHandler := f.newHandler(stubExtractor{extraction: lowConfidenceExtraction(3, "zone-1")}, rec, false)
	lowCall, err := lowPriorityHandler.Handle(context.Background(), "low priority, arrives first", "")
	require.NoError(t, err)

	highPriorityHandler := f.newHandler(stubExtractor{extraction: lowConfidenceExtraction(1, "zone-1")}, rec, false)
	highCall, err := highPriorityHandler.Handle(context.Background(), "high priority, arrives second", "")
	require.NoError(t, err)

	first, ok := f.queue.Pop(f.clock.now)
	require.True(t, ok)
	assert.Equal(t, highCall.ID, first.CallID, "higher severity must pop first despite arriving second")

	second, ok := f.queue.Pop(f.clock.now)
	require.True(t, ok)
	assert.Equal(t, lowCall.ID, second.CallID)
}

// --- c5: escalation triage is asynchronous ---

func TestHandle_EscalationIsAsync_ReturnsQuicklyEvenWhenAgentBlocks(t *testing.T) {
	f := newFixture(t)
	f.mustInsertZone(t, "zone-1")
	block := make(chan struct{})
	rec := &agentclient.RecordingClient{Block: block}
	t.Cleanup(func() { close(block) })

	h := f.newHandler(stubExtractor{extraction: lowConfidenceExtraction(2, "zone-1")}, rec, true)

	start := time.Now()
	_, err := h.Handle(context.Background(), "unclear transcript", "")
	elapsed := time.Since(start)
	require.NoError(t, err)
	assert.Less(t, elapsed, 200*time.Millisecond, "submitTranscript must not wait on agent triage")
}

// --- c6: an agent error never drops the call (REL-2) ---

func TestEscalatedPath_AgentError_CallRemainsEscalatedAndVisible(t *testing.T) {
	f := newFixture(t)
	f.mustInsertZone(t, "zone-1")
	rec := &agentclient.RecordingClient{Err: fmt.Errorf("agent down")}
	h := f.newHandler(stubExtractor{extraction: lowConfidenceExtraction(2, "zone-1")}, rec, true)

	call, err := h.Handle(context.Background(), "unclear transcript", "")
	require.NoError(t, err)

	require.Eventually(t, func() bool { return rec.Count() == 1 }, time.Second, time.Millisecond)

	got, err := f.store.GetCall(context.Background(), call.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.CallStatusEscalated, got.Status)

	calls, err := f.store.ListCalls(context.Background())
	require.NoError(t, err)
	found := false
	for _, c := range calls {
		if c.ID == call.ID && c.Status == domain.CallStatusEscalated {
			found = true
		}
	}
	assert.True(t, found, "an escalated call must remain visible via ListCalls even after a triage error")
}

// --- c7: confidence-0 extraction failure escalates with the low-confidence rule ---

func TestHandle_FailedExtraction_EscalatesWithLowConfidenceRule(t *testing.T) {
	f := newFixture(t)
	f.mustInsertZone(t, "zone-hint-1")
	rec := &agentclient.RecordingClient{}
	h := f.newHandler(stubExtractor{extraction: failedExtraction()}, rec, true)

	call, err := h.Handle(context.Background(), "garbled transcript", "zone-hint-1")
	require.NoError(t, err)
	assert.Equal(t, domain.CallStatusEscalated, call.Status)
	assert.Equal(t, 1, call.Severity, "OQ-14: unknown severity defaults to the most urgent tier")
	assert.Equal(t, "zone-hint-1", call.ZoneID, "OQ-13: zoneHint fills in when extraction yields no zone")

	decisions, err := f.store.ListRoutingDecisionsByCallID(context.Background(), call.ID)
	require.NoError(t, err)
	require.Len(t, decisions, 1)
	assert.Equal(t, rules.RuleLowConfidence, decisions[0].MatchedRuleID)
}

// --- c9: triage concurrency capped at AGENT_MAX_CONCURRENT_TRIAGE ---

// trackingAgentClient records concurrent-in-flight Triage calls, blocking
// until release is closed, to verify intake's semaphore actually bounds
// concurrency rather than just working by coincidence.
type trackingAgentClient struct {
	mu          sync.Mutex
	inFlight    int
	maxInFlight int
	requests    []agentclient.TriageRequest
	release     chan struct{}
}

func newTrackingAgentClient() *trackingAgentClient {
	return &trackingAgentClient{release: make(chan struct{})}
}

func (c *trackingAgentClient) Triage(ctx context.Context, req agentclient.TriageRequest) error {
	c.mu.Lock()
	c.inFlight++
	if c.inFlight > c.maxInFlight {
		c.maxInFlight = c.inFlight
	}
	c.requests = append(c.requests, req)
	c.mu.Unlock()

	select {
	case <-c.release:
	case <-ctx.Done():
	}

	c.mu.Lock()
	c.inFlight--
	c.mu.Unlock()
	return nil
}

func (c *trackingAgentClient) snapshot() (inFlight, total, max int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.inFlight, len(c.requests), c.maxInFlight
}

func TestHandle_TriageConcurrencyCappedAtMax(t *testing.T) {
	f := newFixture(t)
	f.mustInsertZone(t, "zone-1")
	client := newTrackingAgentClient()
	h := f.newHandler(stubExtractor{extraction: lowConfidenceExtraction(2, "zone-1")}, client, true)

	var calls [4]domain.Call
	for i := range calls {
		call, err := h.Handle(context.Background(), fmt.Sprintf("unclear transcript %d", i), "")
		require.NoError(t, err)
		calls[i] = call
	}

	require.Eventually(t, func() bool {
		inFlight, _, _ := client.snapshot()
		return inFlight == testMaxConcurrentTriage
	}, time.Second, time.Millisecond, "exactly AGENT_MAX_CONCURRENT_TRIAGE triages must be in flight")

	inFlight, total, maxSeen := client.snapshot()
	assert.Equal(t, testMaxConcurrentTriage, inFlight)
	assert.LessOrEqual(t, maxSeen, testMaxConcurrentTriage, "concurrency must never exceed the cap")
	assert.Equal(t, testMaxConcurrentTriage, total, "the other two calls must be queued, not dropped or rejected")

	for _, call := range calls {
		got, err := f.store.GetCall(context.Background(), call.ID)
		require.NoError(t, err)
		assert.Equal(t, domain.CallStatusEscalated, got.Status)
	}

	close(client.release)

	require.Eventually(t, func() bool {
		_, total, _ := client.snapshot()
		return total == 4
	}, time.Second, time.Millisecond, "all 4 calls must eventually be triaged")
}

// --- c10: AGENT_ENABLED scopes to the escalated path only ---

func TestHandle_AgentDisabled_EscalatedPathSkipsTriageEntirely(t *testing.T) {
	f := newFixture(t)
	f.mustInsertZone(t, "zone-1")
	rec := &agentclient.RecordingClient{}
	h := f.newHandler(stubExtractor{extraction: lowConfidenceExtraction(2, "zone-1")}, rec, false)

	call, err := h.Handle(context.Background(), "unclear transcript", "")
	require.NoError(t, err)
	assert.Equal(t, domain.CallStatusEscalated, call.Status)

	time.Sleep(20 * time.Millisecond)
	assert.Equal(t, 0, rec.Count(), "AGENT_ENABLED=false must skip Triage entirely")

	got, err := f.store.GetCall(context.Background(), call.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.CallStatusEscalated, got.Status, "call must still be ESCALATED and manually dispatchable")
}

func TestHandle_DeterministicPath_ByteIdenticalRegardlessOfAgentEnabled(t *testing.T) {
	run := func(t *testing.T, agentEnabled bool) (domain.Call, domain.PendingAction) {
		f := newFixture(t)
		f.mustInsertZone(t, "zone-1")
		f.mustInsertUnit(t, availableUnit("unit-1", "zone-1", domain.CapabilityBLS, f.clock.now))
		h := f.newHandler(stubExtractor{extraction: highConfidenceExtraction(2, "zone-1")}, noopAgentClient{t}, agentEnabled)

		call, err := h.Handle(context.Background(), "chest pain caller", "")
		require.NoError(t, err)

		actionsList, err := f.store.ListPendingActions(context.Background())
		require.NoError(t, err)
		require.Len(t, actionsList, 1)
		return call, actionsList[0]
	}

	callEnabled, actionEnabled := run(t, true)
	callDisabled, actionDisabled := run(t, false)

	assert.Equal(t, callEnabled.Status, callDisabled.Status)
	assert.Equal(t, callEnabled.Severity, callDisabled.Severity)
	assert.Equal(t, callEnabled.ZoneID, callDisabled.ZoneID)
	assert.Equal(t, actionEnabled.ProposedBy, actionDisabled.ProposedBy)
	assert.Equal(t, actionEnabled.Rationale, actionDisabled.Rationale)
	assert.Equal(t, actionEnabled.Status, actionDisabled.Status)
	assert.Equal(t, actionEnabled.Type, actionDisabled.Type)
}
