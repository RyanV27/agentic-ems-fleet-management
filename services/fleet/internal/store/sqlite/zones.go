package sqlite

import (
	"context"
	"fmt"

	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/domain"
	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/store"
)

func (s *Store) InsertZone(ctx context.Context, z domain.Zone) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO zones (id, name) VALUES (?, ?)`, z.ID, z.Name)
	if err != nil {
		return fmt.Errorf("sqlite: insert zone %s: %w", z.ID, err)
	}
	return nil
}

func (s *Store) ListZones(ctx context.Context) ([]domain.Zone, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name FROM zones ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list zones: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var zones []domain.Zone
	for rows.Next() {
		var z domain.Zone
		if err := rows.Scan(&z.ID, &z.Name); err != nil {
			return nil, fmt.Errorf("sqlite: scan zone: %w", err)
		}
		zones = append(zones, z)
	}
	return zones, rows.Err()
}

func (s *Store) InsertZoneTravelTime(ctx context.Context, t domain.ZoneTravelTime) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO zone_travel_times (from_zone_id, to_zone_id, travel_seconds) VALUES (?, ?, ?)`,
		t.FromZoneID, t.ToZoneID, t.TravelSeconds)
	if err != nil {
		return fmt.Errorf("sqlite: insert zone travel time %s->%s: %w", t.FromZoneID, t.ToZoneID, err)
	}
	return nil
}

func (s *Store) TravelSeconds(ctx context.Context, fromZoneID, toZoneID string) (int, error) {
	var seconds int
	err := s.db.QueryRowContext(ctx,
		`SELECT travel_seconds FROM zone_travel_times WHERE from_zone_id = ? AND to_zone_id = ?`,
		fromZoneID, toZoneID).Scan(&seconds)
	if err != nil {
		return 0, fmt.Errorf("sqlite: travel seconds %s->%s: %w", fromZoneID, toZoneID, mapNoRows(err))
	}
	return seconds, nil
}

func (s *Store) ListZoneTravelTimes(ctx context.Context) ([]domain.ZoneTravelTime, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT from_zone_id, to_zone_id, travel_seconds FROM zone_travel_times ORDER BY from_zone_id, to_zone_id`)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list zone travel times: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var times []domain.ZoneTravelTime
	for rows.Next() {
		var t domain.ZoneTravelTime
		if err := rows.Scan(&t.FromZoneID, &t.ToZoneID, &t.TravelSeconds); err != nil {
			return nil, fmt.Errorf("sqlite: scan zone travel time: %w", err)
		}
		times = append(times, t)
	}
	return times, rows.Err()
}

func (s *Store) InsertHospital(ctx context.Context, h domain.Hospital) error {
	capsJSON, err := marshalStrings(h.Capabilities)
	if err != nil {
		return fmt.Errorf("sqlite: insert hospital %s: %w", h.ID, err)
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO hospitals (id, name, zone_id, capabilities, accepting) VALUES (?, ?, ?, ?, ?)`,
		h.ID, h.Name, h.ZoneID, capsJSON, boolToInt(h.Accepting))
	if err != nil {
		return fmt.Errorf("sqlite: insert hospital %s: %w", h.ID, err)
	}
	return nil
}

func (s *Store) ListHospitals(ctx context.Context) ([]domain.Hospital, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, zone_id, capabilities, accepting FROM hospitals ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list hospitals: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var hospitals []domain.Hospital
	for rows.Next() {
		var h domain.Hospital
		var capsJSON string
		var accepting int64
		if err := rows.Scan(&h.ID, &h.Name, &h.ZoneID, &capsJSON, &accepting); err != nil {
			return nil, fmt.Errorf("sqlite: scan hospital: %w", err)
		}
		h.Accepting = accepting != 0
		if h.Capabilities, err = unmarshalStrings(capsJSON); err != nil {
			return nil, fmt.Errorf("sqlite: hospital %s: %w", h.ID, err)
		}
		hospitals = append(hospitals, h)
	}
	return hospitals, rows.Err()
}

func mapNoRows(err error) error {
	if err == nil {
		return nil
	}
	if isNoRows(err) {
		return store.ErrNotFound
	}
	return err
}
