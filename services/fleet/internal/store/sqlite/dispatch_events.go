package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/domain"
)

func (s *Store) InsertDispatchEvent(ctx context.Context, e domain.DispatchEvent) error {
	if err := checkEnum("dispatch event type", e.Type); err != nil {
		return err
	}
	if err := checkEnum("dispatch event status", e.Status); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO dispatch_events (id, type, call_id, unit_id, severity_delta, status, note, created_at, resolved_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.ID, string(e.Type), nullableString(e.CallID), nullableString(e.UnitID),
		nullableInt(e.SeverityDelta), string(e.Status), e.Note,
		timeToString(e.CreatedAt), nullableTimeToString(e.ResolvedAt))
	if err != nil {
		return fmt.Errorf("sqlite: insert dispatch event %s: %w", e.ID, err)
	}
	return nil
}

func (s *Store) GetDispatchEvent(ctx context.Context, id string) (domain.DispatchEvent, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, type, call_id, unit_id, severity_delta, status, note, created_at, resolved_at
		 FROM dispatch_events WHERE id = ?`, id)
	return scanDispatchEvent(row.Scan)
}

func (s *Store) ListDispatchEvents(ctx context.Context) ([]domain.DispatchEvent, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, type, call_id, unit_id, severity_delta, status, note, created_at, resolved_at
		 FROM dispatch_events ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list dispatch events: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var events []domain.DispatchEvent
	for rows.Next() {
		e, err := scanDispatchEvent(rows.Scan)
		if err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

func (s *Store) UpdateDispatchEvent(ctx context.Context, e domain.DispatchEvent) error {
	if err := checkEnum("dispatch event status", e.Status); err != nil {
		return err
	}
	res, err := s.db.ExecContext(ctx,
		`UPDATE dispatch_events SET status = ?, note = ?, resolved_at = ? WHERE id = ?`,
		string(e.Status), e.Note, nullableTimeToString(e.ResolvedAt), e.ID)
	if err != nil {
		return fmt.Errorf("sqlite: update dispatch event %s: %w", e.ID, err)
	}
	return checkRowsAffected(res, "dispatch event", e.ID)
}

func scanDispatchEvent(scan func(dest ...any) error) (domain.DispatchEvent, error) {
	var e domain.DispatchEvent
	var eventType, status string
	var callID, unitID sql.NullString
	var severityDelta sql.NullInt64
	var createdAt string
	var resolvedAt sql.NullString
	if err := scan(&e.ID, &eventType, &callID, &unitID, &severityDelta, &status, &e.Note, &createdAt, &resolvedAt); err != nil {
		return domain.DispatchEvent{}, fmt.Errorf("sqlite: scan dispatch event: %w", mapNoRows(err))
	}
	e.Type = domain.DispatchEventType(eventType)
	if err := checkEnum("dispatch event type", e.Type); err != nil {
		return domain.DispatchEvent{}, err
	}
	e.Status = domain.DispatchEventStatus(status)
	if err := checkEnum("dispatch event status", e.Status); err != nil {
		return domain.DispatchEvent{}, err
	}
	e.CallID = stringOrNil(callID)
	e.UnitID = stringOrNil(unitID)
	e.SeverityDelta = intOrNil(severityDelta)
	var err error
	if e.CreatedAt, err = stringToTime(createdAt); err != nil {
		return domain.DispatchEvent{}, fmt.Errorf("sqlite: dispatch event %s: %w", e.ID, err)
	}
	if e.ResolvedAt, err = stringToNullableTime(resolvedAt); err != nil {
		return domain.DispatchEvent{}, fmt.Errorf("sqlite: dispatch event %s: %w", e.ID, err)
	}
	return e, nil
}
