package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"

	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/domain"
)

func (s *Store) InsertExtraction(ctx context.Context, e domain.Extraction) error {
	keywordsJSON, err := marshalStrings(e.Keywords)
	if err != nil {
		return fmt.Errorf("sqlite: insert extraction for call %s: %w", e.CallID, err)
	}
	_, err = s.q(ctx).ExecContext(ctx,
		`INSERT INTO extractions
		 (id, call_id, incident_type, severity, zone_id, keywords, needs_transport, confidence,
		  failure_reason, model, latency_ms, prompt_tokens, completion_tokens, raw_response, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		uuid.NewString(), e.CallID, e.IncidentType, e.Severity, e.ZoneID, keywordsJSON,
		boolToInt(e.NeedsTransport), e.Confidence, nullableStringValue(e.FailureReason),
		nullableStringValue(e.Model), nullableInt64Value(e.LatencyMs),
		nullableIntValue(e.PromptTokens), nullableIntValue(e.CompletionTokens),
		nullableStringValue(e.RawResponse), timeToString(e.CreatedAt))
	if err != nil {
		return fmt.Errorf("sqlite: insert extraction for call %s: %w", e.CallID, err)
	}
	return nil
}

func (s *Store) GetExtractionByCallID(ctx context.Context, callID string) (domain.Extraction, error) {
	row := s.q(ctx).QueryRowContext(ctx,
		`SELECT call_id, incident_type, severity, zone_id, keywords, needs_transport, confidence,
		        failure_reason, model, latency_ms, prompt_tokens, completion_tokens, raw_response, created_at
		 FROM extractions WHERE call_id = ?`, callID)

	var e domain.Extraction
	var keywordsJSON string
	var needsTransport int64
	var failureReason, model, rawResponse sql.NullString
	var latencyMs sql.NullInt64
	var promptTokens, completionTokens sql.NullInt64
	var createdAt string
	if err := row.Scan(&e.CallID, &e.IncidentType, &e.Severity, &e.ZoneID, &keywordsJSON,
		&needsTransport, &e.Confidence, &failureReason, &model, &latencyMs,
		&promptTokens, &completionTokens, &rawResponse, &createdAt); err != nil {
		return domain.Extraction{}, fmt.Errorf("sqlite: get extraction for call %s: %w", callID, mapNoRows(err))
	}
	var err error
	if e.Keywords, err = unmarshalStrings(keywordsJSON); err != nil {
		return domain.Extraction{}, fmt.Errorf("sqlite: extraction for call %s: %w", callID, err)
	}
	e.NeedsTransport = needsTransport != 0
	e.FailureReason = failureReason.String
	e.Model = model.String
	e.RawResponse = rawResponse.String
	e.LatencyMs = latencyMs.Int64
	e.PromptTokens = int(promptTokens.Int64)
	e.CompletionTokens = int(completionTokens.Int64)
	if e.CreatedAt, err = stringToTime(createdAt); err != nil {
		return domain.Extraction{}, fmt.Errorf("sqlite: extraction for call %s: %w", callID, err)
	}
	return e, nil
}

func nullableStringValue(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

func nullableInt64Value(i int64) sql.NullInt64 {
	if i == 0 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: i, Valid: true}
}

func nullableIntValue(i int) sql.NullInt64 {
	if i == 0 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(i), Valid: true}
}
