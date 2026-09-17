package domain

import "time"

// Extraction is the LLM's entire output surface during intake (ARCHITECTURE.md
// §4). Nothing else in the intake path is model-generated.
type Extraction struct {
	CallID           string
	IncidentType     string
	Severity         int
	ZoneID           string
	Keywords         []string
	NeedsTransport   bool
	Confidence       float64
	FailureReason    string
	Model            string
	LatencyMs        int64
	PromptTokens     int
	CompletionTokens int
	RawResponse      string
	CreatedAt        time.Time
}
