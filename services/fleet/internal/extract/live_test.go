//go:build live

// live_test.go runs real transcripts against the real OpenRouter model
// configured by OPENROUTER_EXTRACTION_MODEL (ROADMAP.md S4 c8). It asserts
// structural invariants and severity bands, never exact strings, since a
// free-tier model's exact wording is not something to pin a test to
// (CLAUDE.md "Prefer deterministic assertions over LLM-as-judge" — the
// gate here is the range check, not a judge).
package extract

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/config"
)

var liveValidZoneIDs = []string{"zone-1", "zone-2", "zone-3", "zone-4", "zone-5", "zone-6"}

func TestExtract_Live(t *testing.T) {
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		t.Skip("OPENROUTER_API_KEY not set; run under make test:live")
	}

	cfg, err := config.Load()
	require.NoError(t, err)
	client := NewClient(cfg, liveValidZoneIDs)

	tests := []struct {
		transcriptFile string
		handLabeled    int
	}{
		{"testdata/transcripts/cardiac.txt", 1},
		{"testdata/transcripts/unresponsive_overdose.txt", 1},
		{"testdata/transcripts/breathing_difficulty.txt", 2},
		{"testdata/transcripts/laceration.txt", 3},
		{"testdata/transcripts/fall_minor.txt", 3},
	}

	for _, tt := range tests {
		t.Run(tt.transcriptFile, func(t *testing.T) {
			transcript, err := os.ReadFile("../../" + tt.transcriptFile)
			require.NoError(t, err)

			got, err := client.Extract(t.Context(), string(transcript))
			require.NoError(t, err)

			require.Equal(t, "", got.FailureReason, "raw response: %s", got.RawResponse)
			assert.Greater(t, got.Confidence, 0.5)
			assert.InDelta(t, tt.handLabeled, got.Severity, 1)
			assert.Contains(t, liveValidZoneIDs, got.ZoneID)
			assert.NotEmpty(t, got.IncidentType)
		})
	}
}
