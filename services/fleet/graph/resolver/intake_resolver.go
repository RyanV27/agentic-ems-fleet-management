package resolver

import (
	"context"

	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/graph/generated"
	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/dispatch"
)

// SubmitTranscript delegates the entire pipeline to internal/intake — this
// resolver holds no business logic of its own (ROADMAP.md S5, CLAUDE.md
// resolver convention).
func (r *mutationResolver) SubmitTranscript(ctx context.Context, input generated.SubmitTranscriptInput) (*generated.Call, error) {
	zoneHint := ""
	if input.ZoneHint != nil {
		zoneHint = *input.ZoneHint
	}

	call, err := r.Intake.Handle(ctx, input.Transcript, zoneHint)
	if err != nil {
		return nil, err
	}

	var extraction *generated.Extraction
	if call.Extraction != nil {
		extraction = mapExtraction(*call.Extraction)
	}

	var priorityScore *float64
	if call.EnqueuedAt != nil {
		score := dispatch.PriorityScore(call.Severity, *call.EnqueuedAt, r.Clock.Now(), r.PriorityConfig)
		priorityScore = &score
	}

	return mapCall(call, extraction, priorityScore), nil
}
