// Package store defines the persistence boundary for the fleet service: one
// Store interface (ARCHITECTURE.md §8 — no repository-per-entity layering)
// backed by the sqlite subpackage. Postgres remains a later swap behind this
// interface (DEC-002).
package store

import (
	"context"
	"errors"
	"time"

	"github.com/ryansamvarghese/agentic-fleet-management/services/fleet/internal/domain"
)

// ErrNotFound is returned when a lookup by id finds no row.
var ErrNotFound = errors.New("store: not found")

// Store is every read and write the fleet service needs against SQLite.
// Resolvers, rules, and intake orchestration depend on this interface, never
// on the sqlite package directly (ARCHITECTURE.md §3).
type Store interface {
	Migrate(ctx context.Context) error

	// Zones and hospitals.
	InsertZone(ctx context.Context, z domain.Zone) error
	ListZones(ctx context.Context) ([]domain.Zone, error)
	InsertZoneTravelTime(ctx context.Context, t domain.ZoneTravelTime) error
	TravelSeconds(ctx context.Context, fromZoneID, toZoneID string) (int, error)
	ListZoneTravelTimes(ctx context.Context) ([]domain.ZoneTravelTime, error)
	InsertHospital(ctx context.Context, h domain.Hospital) error
	ListHospitals(ctx context.Context) ([]domain.Hospital, error)

	// Units.
	InsertUnit(ctx context.Context, u domain.Unit) error
	GetUnit(ctx context.Context, id string) (domain.Unit, error)
	ListUnits(ctx context.Context) ([]domain.Unit, error)
	UpdateUnit(ctx context.Context, u domain.Unit) error

	// Calls and extractions.
	InsertCall(ctx context.Context, c domain.Call) error
	GetCall(ctx context.Context, id string) (domain.Call, error)
	ListCalls(ctx context.Context) ([]domain.Call, error)
	UpdateCall(ctx context.Context, c domain.Call) error
	// SetCallEnqueuedAt sets Call.EnqueuedAt to now iff it is not already
	// set, and reports whether this call actually set it. EnqueuedAt is the
	// sole aging input and must be written exactly once (S1 c8, DEC-021).
	SetCallEnqueuedAt(ctx context.Context, callID string, now time.Time) (bool, error)
	InsertExtraction(ctx context.Context, e domain.Extraction) error
	GetExtractionByCallID(ctx context.Context, callID string) (domain.Extraction, error)

	// Dispatch events.
	InsertDispatchEvent(ctx context.Context, e domain.DispatchEvent) error
	GetDispatchEvent(ctx context.Context, id string) (domain.DispatchEvent, error)
	ListDispatchEvents(ctx context.Context) ([]domain.DispatchEvent, error)
	UpdateDispatchEvent(ctx context.Context, e domain.DispatchEvent) error

	// Pending actions — the approval gate (DEC-016).
	// InsertPendingAction enforces the idempotency_key UNIQUE constraint: on
	// a collision it returns the existing row rather than an error (FR-15,
	// SEC-5).
	InsertPendingAction(ctx context.Context, a domain.PendingAction) (domain.PendingAction, error)
	GetPendingAction(ctx context.Context, id string) (domain.PendingAction, error)
	ListPendingActions(ctx context.Context) ([]domain.PendingAction, error)
	UpdatePendingAction(ctx context.Context, a domain.PendingAction) error

	// Audit trail.
	InsertRoutingDecision(ctx context.Context, d domain.RoutingDecision) error
	ListRoutingDecisionsByCallID(ctx context.Context, callID string) ([]domain.RoutingDecision, error)
	InsertAgentRun(ctx context.Context, r domain.AgentRun) error
	UpdateAgentRun(ctx context.Context, r domain.AgentRun) error
	GetAgentRun(ctx context.Context, id string) (domain.AgentRun, error)
	ListAgentRunsByCallID(ctx context.Context, callID string) ([]domain.AgentRun, error)
	InsertToolCallLog(ctx context.Context, l domain.ToolCallLog) error
	ListToolCallLogsByAgentRunID(ctx context.Context, agentRunID string) ([]domain.ToolCallLog, error)

	Close() error
}
