package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	sqlite3 "modernc.org/sqlite"
	sqlite3lib "modernc.org/sqlite/lib"

	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/domain"
)

// InsertPendingAction inserts a PROPOSED row. On an idempotency_key
// collision it returns the existing row instead of an error (FR-15, SEC-5).
func (s *Store) InsertPendingAction(ctx context.Context, a domain.PendingAction) (domain.PendingAction, error) {
	if err := checkEnum("action type", a.Type); err != nil {
		return domain.PendingAction{}, err
	}
	if err := checkEnum("action status", a.Status); err != nil {
		return domain.PendingAction{}, err
	}
	if err := checkEnum("proposed by", a.ProposedBy); err != nil {
		return domain.PendingAction{}, err
	}

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO pending_actions
		 (id, type, payload, status, idempotency_key, proposed_by, rationale, expired_reason,
		  agent_run_id, rejection_reason, rejection_note, parent_action_id, created_at, decided_at, decided_by)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, string(a.Type), string(a.Payload), string(a.Status), a.IdempotencyKey,
		string(a.ProposedBy), a.Rationale, nullableExpiredReason(a.ExpiredReason),
		nullableString(a.AgentRunID), nullableRejectionReason(a.RejectionReason),
		nullableString(a.RejectionNote), nullableString(a.ParentActionID),
		timeToString(a.CreatedAt), nullableTimeToString(a.DecidedAt), nullableString(a.DecidedBy))
	if err != nil {
		if isUniqueConstraintViolation(err) {
			existing, getErr := s.getPendingActionByIdempotencyKey(ctx, a.IdempotencyKey)
			if getErr != nil {
				return domain.PendingAction{}, fmt.Errorf("sqlite: idempotency collision for %s, fetch existing: %w", a.IdempotencyKey, getErr)
			}
			return existing, nil
		}
		return domain.PendingAction{}, fmt.Errorf("sqlite: insert pending action %s: %w", a.ID, err)
	}
	return a, nil
}

func (s *Store) getPendingActionByIdempotencyKey(ctx context.Context, key string) (domain.PendingAction, error) {
	row := s.db.QueryRowContext(ctx, pendingActionSelect+` WHERE idempotency_key = ?`, key)
	return scanPendingAction(row.Scan)
}

func (s *Store) GetPendingAction(ctx context.Context, id string) (domain.PendingAction, error) {
	row := s.db.QueryRowContext(ctx, pendingActionSelect+` WHERE id = ?`, id)
	return scanPendingAction(row.Scan)
}

func (s *Store) ListPendingActions(ctx context.Context) ([]domain.PendingAction, error) {
	rows, err := s.db.QueryContext(ctx, pendingActionSelect+` ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list pending actions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var actions []domain.PendingAction
	for rows.Next() {
		a, err := scanPendingAction(rows.Scan)
		if err != nil {
			return nil, err
		}
		actions = append(actions, a)
	}
	return actions, rows.Err()
}

func (s *Store) UpdatePendingAction(ctx context.Context, a domain.PendingAction) error {
	if err := checkEnum("action status", a.Status); err != nil {
		return err
	}
	res, err := s.db.ExecContext(ctx,
		`UPDATE pending_actions
		 SET status = ?, expired_reason = ?, rejection_reason = ?, rejection_note = ?, decided_at = ?, decided_by = ?
		 WHERE id = ?`,
		string(a.Status), nullableExpiredReason(a.ExpiredReason), nullableRejectionReason(a.RejectionReason),
		nullableString(a.RejectionNote), nullableTimeToString(a.DecidedAt), nullableString(a.DecidedBy), a.ID)
	if err != nil {
		return fmt.Errorf("sqlite: update pending action %s: %w", a.ID, err)
	}
	return checkRowsAffected(res, "pending action", a.ID)
}

const pendingActionSelect = `SELECT id, type, payload, status, idempotency_key, proposed_by, rationale,
	expired_reason, agent_run_id, rejection_reason, rejection_note, parent_action_id, created_at, decided_at, decided_by
	FROM pending_actions`

func scanPendingAction(scan func(dest ...any) error) (domain.PendingAction, error) {
	var a domain.PendingAction
	var actionType, payload, status, proposedBy string
	var expiredReason, agentRunID, rejectionReason, rejectionNote, parentActionID sql.NullString
	var createdAt string
	var decidedAt, decidedBy sql.NullString
	if err := scan(&a.ID, &actionType, &payload, &status, &a.IdempotencyKey, &proposedBy, &a.Rationale,
		&expiredReason, &agentRunID, &rejectionReason, &rejectionNote, &parentActionID,
		&createdAt, &decidedAt, &decidedBy); err != nil {
		return domain.PendingAction{}, fmt.Errorf("sqlite: scan pending action: %w", mapNoRows(err))
	}
	a.Type = domain.ActionType(actionType)
	if err := checkEnum("action type", a.Type); err != nil {
		return domain.PendingAction{}, err
	}
	a.Payload = []byte(payload)
	a.Status = domain.ActionStatus(status)
	if err := checkEnum("action status", a.Status); err != nil {
		return domain.PendingAction{}, err
	}
	a.ProposedBy = domain.ProposedBy(proposedBy)
	if err := checkEnum("proposed by", a.ProposedBy); err != nil {
		return domain.PendingAction{}, err
	}
	var err error
	if a.ExpiredReason, err = scanExpiredReason(expiredReason); err != nil {
		return domain.PendingAction{}, err
	}
	a.AgentRunID = stringOrNil(agentRunID)
	if a.RejectionReason, err = scanRejectionReason(rejectionReason); err != nil {
		return domain.PendingAction{}, err
	}
	a.RejectionNote = stringOrNil(rejectionNote)
	a.ParentActionID = stringOrNil(parentActionID)
	if a.CreatedAt, err = stringToTime(createdAt); err != nil {
		return domain.PendingAction{}, fmt.Errorf("sqlite: pending action %s: %w", a.ID, err)
	}
	if a.DecidedAt, err = stringToNullableTime(decidedAt); err != nil {
		return domain.PendingAction{}, fmt.Errorf("sqlite: pending action %s: %w", a.ID, err)
	}
	a.DecidedBy = stringOrNil(decidedBy)
	return a, nil
}

func nullableExpiredReason(r *domain.ExpiredReason) sql.NullString {
	if r == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: string(*r), Valid: true}
}

func scanExpiredReason(ns sql.NullString) (*domain.ExpiredReason, error) {
	if !ns.Valid {
		return nil, nil
	}
	r := domain.ExpiredReason(ns.String)
	if err := checkEnum("expired reason", r); err != nil {
		return nil, err
	}
	return &r, nil
}

func nullableRejectionReason(r *domain.RejectionReason) sql.NullString {
	if r == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: string(*r), Valid: true}
}

func scanRejectionReason(ns sql.NullString) (*domain.RejectionReason, error) {
	if !ns.Valid {
		return nil, nil
	}
	r := domain.RejectionReason(ns.String)
	if err := checkEnum("rejection reason", r); err != nil {
		return nil, err
	}
	return &r, nil
}

func isUniqueConstraintViolation(err error) bool {
	var sqliteErr *sqlite3.Error
	if errors.As(err, &sqliteErr) {
		return sqliteErr.Code() == sqlite3lib.SQLITE_CONSTRAINT_UNIQUE
	}
	return false
}
