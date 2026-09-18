package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/domain"
)

func (s *Store) InsertCall(ctx context.Context, c domain.Call) error {
	if err := checkEnum("call status", c.Status); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO calls (id, transcript, severity, zone_id, status, assigned_unit_id, destination_hospital_id, created_at, enqueued_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.ID, c.Transcript, c.Severity, c.ZoneID, string(c.Status),
		nullableString(c.AssignedUnitID), nullableString(c.DestinationHospitalID),
		timeToString(c.CreatedAt), nullableTimeToString(c.EnqueuedAt))
	if err != nil {
		return fmt.Errorf("sqlite: insert call %s: %w", c.ID, err)
	}
	return nil
}

func (s *Store) GetCall(ctx context.Context, id string) (domain.Call, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, transcript, severity, zone_id, status, assigned_unit_id, destination_hospital_id, created_at, enqueued_at
		 FROM calls WHERE id = ?`, id)
	return scanCall(row.Scan)
}

func (s *Store) ListCalls(ctx context.Context) ([]domain.Call, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, transcript, severity, zone_id, status, assigned_unit_id, destination_hospital_id, created_at, enqueued_at
		 FROM calls ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list calls: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var calls []domain.Call
	for rows.Next() {
		c, err := scanCall(rows.Scan)
		if err != nil {
			return nil, err
		}
		calls = append(calls, c)
	}
	return calls, rows.Err()
}

// UpdateCall updates every Call field except EnqueuedAt, which is
// write-once via SetCallEnqueuedAt (S1 c8, DEC-021).
func (s *Store) UpdateCall(ctx context.Context, c domain.Call) error {
	if err := checkEnum("call status", c.Status); err != nil {
		return err
	}
	res, err := s.db.ExecContext(ctx,
		`UPDATE calls SET transcript = ?, severity = ?, zone_id = ?, status = ?, assigned_unit_id = ?, destination_hospital_id = ?
		 WHERE id = ?`,
		c.Transcript, c.Severity, c.ZoneID, string(c.Status),
		nullableString(c.AssignedUnitID), nullableString(c.DestinationHospitalID), c.ID)
	if err != nil {
		return fmt.Errorf("sqlite: update call %s: %w", c.ID, err)
	}
	return checkRowsAffected(res, "call", c.ID)
}

// SetCallEnqueuedAt sets enqueued_at to now iff it is currently NULL,
// reporting whether this call actually set it.
func (s *Store) SetCallEnqueuedAt(ctx context.Context, callID string, now time.Time) (bool, error) {
	res, err := s.db.ExecContext(ctx,
		`UPDATE calls SET enqueued_at = ? WHERE id = ? AND enqueued_at IS NULL`,
		timeToString(now), callID)
	if err != nil {
		return false, fmt.Errorf("sqlite: set enqueued_at for call %s: %w", callID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("sqlite: rows affected for call %s: %w", callID, err)
	}
	return n == 1, nil
}

func scanCall(scan func(dest ...any) error) (domain.Call, error) {
	var c domain.Call
	var status string
	var assignedUnitID, destinationHospitalID sql.NullString
	var createdAt string
	var enqueuedAt sql.NullString
	if err := scan(&c.ID, &c.Transcript, &c.Severity, &c.ZoneID, &status,
		&assignedUnitID, &destinationHospitalID, &createdAt, &enqueuedAt); err != nil {
		return domain.Call{}, fmt.Errorf("sqlite: scan call: %w", mapNoRows(err))
	}
	c.Status = domain.CallStatus(status)
	if err := checkEnum("call status", c.Status); err != nil {
		return domain.Call{}, err
	}
	c.AssignedUnitID = stringOrNil(assignedUnitID)
	c.DestinationHospitalID = stringOrNil(destinationHospitalID)
	var err error
	if c.CreatedAt, err = stringToTime(createdAt); err != nil {
		return domain.Call{}, fmt.Errorf("sqlite: call %s: %w", c.ID, err)
	}
	if c.EnqueuedAt, err = stringToNullableTime(enqueuedAt); err != nil {
		return domain.Call{}, fmt.Errorf("sqlite: call %s: %w", c.ID, err)
	}
	return c, nil
}
