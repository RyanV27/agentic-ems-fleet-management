package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/domain"
	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/store"
)

func (s *Store) InsertUnit(ctx context.Context, u domain.Unit) error {
	if err := checkEnum("unit status", u.Status); err != nil {
		return err
	}
	if err := checkEnum("capability", u.Capability); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO units (id, callsign, status, capability, zone_id, current_call_id, status_changed_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		u.ID, u.Callsign, string(u.Status), string(u.Capability), u.ZoneID,
		nullableString(u.CurrentCallID), timeToString(u.StatusChangedAt))
	if err != nil {
		return fmt.Errorf("sqlite: insert unit %s: %w", u.ID, err)
	}
	return nil
}

func (s *Store) GetUnit(ctx context.Context, id string) (domain.Unit, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, callsign, status, capability, zone_id, current_call_id, status_changed_at
		 FROM units WHERE id = ?`, id)
	return scanUnit(row.Scan)
}

func (s *Store) ListUnits(ctx context.Context) ([]domain.Unit, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, callsign, status, capability, zone_id, current_call_id, status_changed_at
		 FROM units ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list units: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var units []domain.Unit
	for rows.Next() {
		u, err := scanUnit(rows.Scan)
		if err != nil {
			return nil, err
		}
		units = append(units, u)
	}
	return units, rows.Err()
}

func (s *Store) UpdateUnit(ctx context.Context, u domain.Unit) error {
	if err := checkEnum("unit status", u.Status); err != nil {
		return err
	}
	if err := checkEnum("capability", u.Capability); err != nil {
		return err
	}
	res, err := s.db.ExecContext(ctx,
		`UPDATE units SET callsign = ?, status = ?, capability = ?, zone_id = ?, current_call_id = ?, status_changed_at = ?
		 WHERE id = ?`,
		u.Callsign, string(u.Status), string(u.Capability), u.ZoneID,
		nullableString(u.CurrentCallID), timeToString(u.StatusChangedAt), u.ID)
	if err != nil {
		return fmt.Errorf("sqlite: update unit %s: %w", u.ID, err)
	}
	return checkRowsAffected(res, "unit", u.ID)
}

func scanUnit(scan func(dest ...any) error) (domain.Unit, error) {
	var u domain.Unit
	var status, capability string
	var currentCallID sql.NullString
	var statusChangedAt string
	if err := scan(&u.ID, &u.Callsign, &status, &capability, &u.ZoneID, &currentCallID, &statusChangedAt); err != nil {
		return domain.Unit{}, fmt.Errorf("sqlite: scan unit: %w", mapNoRows(err))
	}
	u.Status = domain.UnitStatus(status)
	if err := checkEnum("unit status", u.Status); err != nil {
		return domain.Unit{}, err
	}
	u.Capability = domain.Capability(capability)
	if err := checkEnum("capability", u.Capability); err != nil {
		return domain.Unit{}, err
	}
	u.CurrentCallID = stringOrNil(currentCallID)
	var err error
	if u.StatusChangedAt, err = stringToTime(statusChangedAt); err != nil {
		return domain.Unit{}, fmt.Errorf("sqlite: unit %s: %w", u.ID, err)
	}
	return u, nil
}

func checkRowsAffected(res sql.Result, kind, id string) error {
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("sqlite: rows affected for %s %s: %w", kind, id, err)
	}
	if n == 0 {
		return fmt.Errorf("sqlite: update %s %s: %w", kind, id, store.ErrNotFound)
	}
	return nil
}
