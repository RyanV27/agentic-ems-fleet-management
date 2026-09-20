// Package resolver implements generated.ResolverRoot. Every resolver is a
// Store call plus mapping from a domain type to a generated GraphQL model —
// no business logic (ROADMAP.md S2 c5). internal/rules and internal/dispatch
// are never imported here, except dispatch.PriorityScore for Call.priorityScore
// (DEC-020), which is exactly the one shared function this criterion polices.
package resolver

import (
	"time"

	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/graph/generated"
	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/actions"
	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/dispatch"
	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/store"
)

// Resolver is the root dependency holder. Clock is a minimal local
// abstraction (not internal/simclock, which is S10's file to create) so this
// package still never calls time.Now() itself — only whatever wires up the
// server (main, or a test) supplies Clock.
type Resolver struct {
	Store store.Store
	Clock Clock
	// PriorityConfig feeds dispatch.PriorityScore for Call.priorityScore.
	PriorityConfig dispatch.PriorityConfig
	// Manager is the S6 approval gate. Every mutation resolver delegates to
	// it; no resolver ever mutates fleet state itself (DEC-016).
	Manager *actions.Manager
}

// Clock supplies "now" to resolvers that need it. Structurally compatible
// with the fuller internal/simclock.Clock interface S10 will introduce, so
// no change is needed here when that lands.
type Clock interface {
	Now() time.Time
}

// Query returns the QueryResolver implementation.
func (r *Resolver) Query() generated.QueryResolver { return &queryResolver{r} }

type queryResolver struct{ *Resolver }

// Mutation returns the MutationResolver implementation.
func (r *Resolver) Mutation() generated.MutationResolver { return &mutationResolver{r} }

type mutationResolver struct{ *Resolver }
