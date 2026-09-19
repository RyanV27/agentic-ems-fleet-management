package extract

// Failure reasons populate domain.Extraction.FailureReason when Extract
// cannot produce a usable extraction. Every failure mode gets its own value
// so tests and operators can tell them apart (ROADMAP.md S4 c2-3).
const (
	FailureReasonTimeout           = "timeout"
	FailureReasonHTTPError         = "http_error"
	FailureReasonRateLimited       = "rate_limited"
	FailureReasonInvalidJSON       = "invalid_json"
	FailureReasonInvalidSeverity   = "invalid_severity"
	FailureReasonInvalidZone       = "invalid_zone"
	FailureReasonInvalidConfidence = "invalid_confidence"
	FailureReasonMissingField      = "missing_field"
)

// rawExtraction is the model's structured JSON response, before range and
// zone validation (ARCHITECTURE.md §4 Extraction).
type rawExtraction struct {
	IncidentType   string   `json:"incidentType"`
	Severity       int      `json:"severity"`
	ZoneID         string   `json:"zoneId"`
	Keywords       []string `json:"keywords"`
	NeedsTransport bool     `json:"needsTransport"`
	Confidence     float64  `json:"confidence"`
}

// responseJSONSchema is the JSON-schema-constrained response format sent to
// OpenRouter (ROADMAP.md S4 c1). Field order matches rawExtraction.
func responseJSONSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"incidentType": map[string]any{"type": "string"},
			"severity":     map[string]any{"type": "integer"},
			"zoneId":       map[string]any{"type": "string"},
			"keywords": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "string"},
			},
			"needsTransport": map[string]any{"type": "boolean"},
			"confidence":     map[string]any{"type": "number"},
		},
		"required":             []string{"incidentType", "severity", "zoneId", "keywords", "needsTransport", "confidence"},
		"additionalProperties": false,
	}
}

// validate range-checks and zone-checks a rawExtraction (ARCHITECTURE.md
// SEC-6, ROADMAP.md S4 c2). It returns a failure reason string on the first
// violation found, or "" if the extraction is usable.
func validate(raw rawExtraction, validZoneIDs []string) string {
	if raw.IncidentType == "" || raw.ZoneID == "" {
		return FailureReasonMissingField
	}
	if raw.Severity < 1 || raw.Severity > 3 {
		return FailureReasonInvalidSeverity
	}
	if raw.Confidence < 0 || raw.Confidence > 1 {
		return FailureReasonInvalidConfidence
	}
	if !zoneKnown(raw.ZoneID, validZoneIDs) {
		return FailureReasonInvalidZone
	}
	return ""
}

func zoneKnown(zoneID string, validZoneIDs []string) bool {
	for _, id := range validZoneIDs {
		if id == zoneID {
			return true
		}
	}
	return false
}
