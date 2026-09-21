// Command server is the entry point for the EMS fleet dispatch service. It
// wires config, store, clock, rules, dispatch queue, approval-gate manager,
// intake pipeline, and the GraphQL resolver into a real HTTP+GraphQL server
// (ROADMAP.md S7 c9 needs a runnable service to triage against).
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/99designs/gqlgen/graphql/handler"

	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/graph/generated"
	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/graph/resolver"
	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/actions"
	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/agentclient"
	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/config"
	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/dispatch"
	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/extract"
	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/intake"
	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/rules"
	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/store"
	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/store/sqlite"
)

// sweepInterval is how often the approval-gate backstop expires stale
// PROPOSED actions (internal/actions.Manager.SweepExpired is a function, not
// a scheduler — see docs/slices/S6-approval-gate-codebase-guide.md). No
// interval was specified anywhere in the docs, so 5s is chosen here as a
// production default: frequent enough that a TTL of 600s (DEC-019) expires
// within a fraction of a percent of its budget, cheap enough to run forever.
const sweepInterval = 5 * time.Second

// realClock reads the wall clock. internal/simclock (S10) does not exist yet
// (DEC-029); every package that needs "now" defines its own minimal Clock
// interface, so this single type satisfies all of them structurally.
type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

// app is the fully wired set of components run() serves over HTTP. It is
// split out from run() so tests can exercise the real wiring (seeding,
// resolver construction, GraphQL routing) via httptest without going through
// ListenAndServe/signal handling.
type app struct {
	store   store.Store
	clock   realClock
	manager *actions.Manager
	handler http.Handler
}

func buildApp(ctx context.Context, cfg config.Config) (*app, error) {
	st, err := sqlite.Open(cfg.FleetDBPath)
	if err != nil {
		return nil, err
	}

	if err := st.Migrate(ctx); err != nil {
		return nil, err
	}
	if err := seedIfEmpty(ctx, st); err != nil {
		return nil, err
	}

	clock := realClock{}
	priorityCfg := dispatch.PriorityConfig{
		SeverityWeights: cfg.SeverityWeights,
		AgingRate:       cfg.AgingRate,
	}
	queue := dispatch.NewQueue(priorityCfg)
	mgr := actions.NewManager(st, queue, clock, cfg.OperatorID, cfg.PendingActionTTLSeconds)

	zones, err := st.ListZones(ctx)
	if err != nil {
		return nil, err
	}
	validZoneIDs := make([]string, 0, len(zones))
	for _, z := range zones {
		validZoneIDs = append(validZoneIDs, z.ID)
	}
	travelRows, err := st.ListZoneTravelTimes(ctx)
	if err != nil {
		return nil, err
	}
	travelTimes := dispatch.NewTravelTimeTable(travelRows)

	extractor := extract.NewClient(cfg, validZoneIDs)
	agentClient := agentclient.NewClient(cfg.AgentBaseURL)
	rulesCfg := rules.RulesConfig{
		ConfidenceThreshold: cfg.ExtractionConfidenceThreshold,
		Priority:            priorityCfg,
	}
	intakeHandler := intake.NewHandler(mgr, extractor, agentClient, rulesCfg, travelTimes, cfg.AgentEnabled, cfg.AgentMaxConcurrentTriage)

	res := &resolver.Resolver{
		Store:          st,
		Clock:          clock,
		PriorityConfig: priorityCfg,
		Manager:        mgr,
		Intake:         intakeHandler,
	}
	srv := handler.NewDefaultServer(generated.NewExecutableSchema(generated.Config{Resolvers: res}))

	mux := http.NewServeMux()
	mux.Handle("/query", srv)

	return &app{store: st, clock: clock, manager: mgr, handler: withCORS(mux)}, nil
}

// withCORS allows any origin to call the GraphQL endpoint. The web dashboard
// (Vite, port 5173) and the fleet service (port 8080) are different origins,
// so a browser blocks the dashboard's fetch calls without this. There is no
// auth or cookie-based session anywhere in this system and no real patient
// data (CLAUDE.md hard rule 4), so a permissive origin has no confidentiality
// cost here.
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	a, err := buildApp(ctx, cfg)
	if err != nil {
		return err
	}
	defer func() { _ = a.store.Close() }()

	httpServer := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.FleetPort),
		Handler:           a.handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go runSweeper(ctx, a.manager, a.clock)

	errCh := make(chan error, 1)
	go func() {
		log.Printf("fleet server listening on :%d", cfg.FleetPort)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return httpServer.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}

// seedIfEmpty seeds the fixed demo zones/hospitals/units on first boot only.
// store.Seed is not idempotent — it unconditionally inserts fixed-ID rows —
// so calling it against an already-seeded persistent DB would violate a
// primary key. ListZones is empty only before the first-ever seed.
func seedIfEmpty(ctx context.Context, s store.Store) error {
	zones, err := s.ListZones(ctx)
	if err != nil {
		return err
	}
	if len(zones) > 0 {
		return nil
	}
	return store.Seed(ctx, s)
}

// runSweeper periodically expires stale PROPOSED actions past their TTL
// (DEC-019) until ctx is cancelled. See sweepInterval for why 5s.
func runSweeper(ctx context.Context, mgr *actions.Manager, clock interface{ Now() time.Time }) {
	ticker := time.NewTicker(sweepInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := mgr.SweepExpired(ctx, clock.Now()); err != nil {
				log.Printf("sweep expired actions: %v", err)
			}
		}
	}
}
