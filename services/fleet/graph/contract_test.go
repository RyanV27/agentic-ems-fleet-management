package graph_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/stretchr/testify/require"

	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/graph/generated"
	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/graph/resolver"
	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/actions"
	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/dispatch"
	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/domain"
	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/store"
	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/store/sqlite"
)

const testOperatorID = "operator-1"
const testTTLSeconds = 600

// fixedClock always returns the same instant, so priorityScore assertions
// in these tests are deterministic.
type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

var testPriorityConfig = dispatch.PriorityConfig{
	SeverityWeights: map[int]float64{1: 1000, 2: 100, 3: 10},
	AgingRate:       1.0,
}

// newTestServer starts a real GraphQL server (httptest) backed by a seeded
// in-memory SQLite DB, per S2 c3 — contract tests run the whole stack, not
// just the resolver function in isolation.
func newTestServer(t *testing.T, now time.Time) (*httptest.Server, store.Store) {
	t.Helper()
	s, err := sqlite.Open(fmt.Sprintf("file:contract-test-%d?mode=memory&cache=shared", time.Now().UnixNano()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })

	ctx := context.Background()
	require.NoError(t, s.Migrate(ctx))
	require.NoError(t, store.Seed(ctx, s))

	q := dispatch.NewQueue(testPriorityConfig)
	clock := fixedClock{now: now}
	mgr := actions.NewManager(s, q, clock, testOperatorID, testTTLSeconds)

	res := &resolver.Resolver{
		Store:          s,
		Clock:          clock,
		PriorityConfig: testPriorityConfig,
		Manager:        mgr,
	}
	srv := handler.NewDefaultServer(generated.NewExecutableSchema(generated.Config{Resolvers: res}))
	return httptest.NewServer(srv), s
}

type gqlResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

func execQuery(t *testing.T, server *httptest.Server, query string, variables map[string]any) gqlResponse {
	t.Helper()
	body, err := json.Marshal(map[string]any{"query": query, "variables": variables})
	require.NoError(t, err)

	resp, err := http.Post(server.URL, "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	var out gqlResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))
	return out
}

func TestContract_Zones(t *testing.T) {
	server, _ := newTestServer(t, time.Now())
	resp := execQuery(t, server, `{ zones { id name } }`, nil)
	require.Empty(t, resp.Errors)

	var payload struct {
		Zones []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"zones"`
	}
	require.NoError(t, json.Unmarshal(resp.Data, &payload))
	require.Len(t, payload.Zones, 6)
}

func TestContract_Hospitals(t *testing.T) {
	server, _ := newTestServer(t, time.Now())
	resp := execQuery(t, server, `{ hospitals { id name zoneId capabilities accepting } }`, nil)
	require.Empty(t, resp.Errors)

	var payload struct {
		Hospitals []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"hospitals"`
	}
	require.NoError(t, json.Unmarshal(resp.Data, &payload))
	require.Len(t, payload.Hospitals, 3)
}

func TestContract_Units(t *testing.T) {
	server, _ := newTestServer(t, time.Now())
	resp := execQuery(t, server, `{ units { id callsign status capability zoneId statusChangedAt } }`, nil)
	require.Empty(t, resp.Errors)

	var payload struct {
		Units []struct {
			ID         string `json:"id"`
			Capability string `json:"capability"`
			Status     string `json:"status"`
		} `json:"units"`
	}
	require.NoError(t, json.Unmarshal(resp.Data, &payload))
	require.Len(t, payload.Units, 12)

	var als, bls int
	for _, u := range payload.Units {
		switch u.Capability {
		case "ALS":
			als++
		case "BLS":
			bls++
		}
		require.Equal(t, "AVAILABLE", u.Status)
	}
	require.Equal(t, 7, als)
	require.Equal(t, 5, bls)
}

func TestContract_FleetStatus(t *testing.T) {
	server, _ := newTestServer(t, time.Now())
	resp := execQuery(t, server, `{ fleetStatus { units { id } calls { id } dispatchEvents { id } } }`, nil)
	require.Empty(t, resp.Errors)

	var payload struct {
		FleetStatus struct {
			Units          []struct{ ID string } `json:"units"`
			Calls          []struct{ ID string } `json:"calls"`
			DispatchEvents []struct{ ID string } `json:"dispatchEvents"`
		} `json:"fleetStatus"`
	}
	require.NoError(t, json.Unmarshal(resp.Data, &payload))
	require.Len(t, payload.FleetStatus.Units, 12)
	require.Empty(t, payload.FleetStatus.Calls)
	require.Empty(t, payload.FleetStatus.DispatchEvents)
}

func TestContract_UnknownFieldReturnsGraphQLErrorNot500(t *testing.T) {
	server, _ := newTestServer(t, time.Now())
	resp, err := http.Post(server.URL, "application/json", bytes.NewReader(mustJSON(t, map[string]any{
		"query": `{ units { id notAField } }`,
	})))
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	// gqlgen returns 200 with a populated errors array for a validation
	// failure (unknown field), never a 500 (S2 c6).
	require.NotEqual(t, http.StatusInternalServerError, resp.StatusCode)
	var out gqlResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))
	require.NotEmpty(t, out.Errors)
}

func TestContract_MalformedDocumentReturnsGraphQLErrorNot500(t *testing.T) {
	server, _ := newTestServer(t, time.Now())
	resp, err := http.Post(server.URL, "application/json", bytes.NewReader(mustJSON(t, map[string]any{
		"query": `{ units { id `, // unterminated selection set
	})))
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	require.NotEqual(t, http.StatusInternalServerError, resp.StatusCode)
	var out gqlResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))
	require.NotEmpty(t, out.Errors)
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return b
}

func TestContract_CallsOrderedByPriorityScoreDescending(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	server, s := newTestServer(t, now)
	seedCalls(t, s, now)

	resp := execQuery(t, server, `{ calls { id severity priorityScore } }`, nil)
	require.Empty(t, resp.Errors)

	var payload struct {
		Calls []struct {
			ID            string  `json:"id"`
			PriorityScore float64 `json:"priorityScore"`
		} `json:"calls"`
	}
	require.NoError(t, json.Unmarshal(resp.Data, &payload))
	require.Len(t, payload.Calls, 3)

	// Assert the resolver's order matches a brute-force sort by
	// dispatch.PriorityScore for the same snapshot and now — the one
	// shared function this criterion is really policing (S2 c4).
	for i := 1; i < len(payload.Calls); i++ {
		require.GreaterOrEqual(t, payload.Calls[i-1].PriorityScore, payload.Calls[i].PriorityScore)
	}
	require.Equal(t, []string{"call-p3-old", "call-p1-fresh", "call-p2-mid"}, ids(payload.Calls))
}

// seedCalls inserts three PENDING_APPROVAL calls with distinct severities
// and enqueuedAt offsets from now, chosen so aging makes the P3 call
// outrank the fresher P1 call (P3 score 10+1500=1510, P1 score 1000+0=1000,
// P2 score 100+200=300, using testPriorityConfig).
func seedCalls(t *testing.T, s store.Store, now time.Time) {
	t.Helper()
	ctx := context.Background()

	calls := []struct {
		id         string
		severity   int
		enqueuedAt time.Time
	}{
		{"call-p1-fresh", 1, now},
		{"call-p3-old", 3, now.Add(-1500 * time.Second)},
		{"call-p2-mid", 2, now.Add(-200 * time.Second)},
	}
	for _, c := range calls {
		enqueuedAt := c.enqueuedAt
		require.NoError(t, s.InsertCall(ctx, domain.Call{
			ID:         c.id,
			Transcript: "test transcript",
			Severity:   c.severity,
			ZoneID:     "zone-1",
			Status:     domain.CallStatusPendingApproval,
			CreatedAt:  enqueuedAt,
			EnqueuedAt: &enqueuedAt,
		}))
	}
}

func ids(calls []struct {
	ID            string  `json:"id"`
	PriorityScore float64 `json:"priorityScore"`
}) []string {
	out := make([]string, len(calls))
	for i, c := range calls {
		out[i] = c.ID
	}
	return out
}
