package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/domain"
)

func (s *Store) InsertRoutingDecision(ctx context.Context, d domain.RoutingDecision) error {
	if err := checkEnum("routing path", d.Path); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO routing_decisions (id, call_id, path, matched_rule_id, input_snapshot, decided_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		d.ID, d.CallID, string(d.Path), d.MatchedRuleID, string(d.InputSnapshot), timeToString(d.DecidedAt))
	if err != nil {
		return fmt.Errorf("sqlite: insert routing decision %s: %w", d.ID, err)
	}
	return nil
}

func (s *Store) ListRoutingDecisionsByCallID(ctx context.Context, callID string) ([]domain.RoutingDecision, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, call_id, path, matched_rule_id, input_snapshot, decided_at
		 FROM routing_decisions WHERE call_id = ? ORDER BY decided_at`, callID)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list routing decisions for call %s: %w", callID, err)
	}
	defer func() { _ = rows.Close() }()

	var decisions []domain.RoutingDecision
	for rows.Next() {
		var d domain.RoutingDecision
		var path, inputSnapshot, decidedAt string
		if err := rows.Scan(&d.ID, &d.CallID, &path, &d.MatchedRuleID, &inputSnapshot, &decidedAt); err != nil {
			return nil, fmt.Errorf("sqlite: scan routing decision: %w", err)
		}
		d.Path = domain.RoutingPath(path)
		if err := checkEnum("routing path", d.Path); err != nil {
			return nil, err
		}
		d.InputSnapshot = []byte(inputSnapshot)
		if d.DecidedAt, err = stringToTime(decidedAt); err != nil {
			return nil, fmt.Errorf("sqlite: routing decision %s: %w", d.ID, err)
		}
		decisions = append(decisions, d)
	}
	return decisions, rows.Err()
}

func (s *Store) InsertAgentRun(ctx context.Context, r domain.AgentRun) error {
	if err := checkEnum("agent run status", r.Status); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO agent_runs
		 (id, call_id, trigger, attempt, status, started_at, ended_at, latency_ms, prompt_tokens, completion_tokens, cost_usd, stop_reason)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID, r.CallID, r.Trigger, r.Attempt, string(r.Status), timeToString(r.StartedAt),
		nullableTimeToString(r.EndedAt), nullableInt64(r.LatencyMs), r.PromptTokens, r.CompletionTokens,
		r.CostUsd, nullableStopReason(r.StopReason))
	if err != nil {
		return fmt.Errorf("sqlite: insert agent run %s: %w", r.ID, err)
	}
	return nil
}

func (s *Store) UpdateAgentRun(ctx context.Context, r domain.AgentRun) error {
	if err := checkEnum("agent run status", r.Status); err != nil {
		return err
	}
	res, err := s.db.ExecContext(ctx,
		`UPDATE agent_runs
		 SET status = ?, ended_at = ?, latency_ms = ?, prompt_tokens = ?, completion_tokens = ?, cost_usd = ?, stop_reason = ?
		 WHERE id = ?`,
		string(r.Status), nullableTimeToString(r.EndedAt), nullableInt64(r.LatencyMs),
		r.PromptTokens, r.CompletionTokens, r.CostUsd, nullableStopReason(r.StopReason), r.ID)
	if err != nil {
		return fmt.Errorf("sqlite: update agent run %s: %w", r.ID, err)
	}
	return checkRowsAffected(res, "agent run", r.ID)
}

func (s *Store) GetAgentRun(ctx context.Context, id string) (domain.AgentRun, error) {
	row := s.db.QueryRowContext(ctx, agentRunSelect+` WHERE id = ?`, id)
	return scanAgentRun(row.Scan)
}

func (s *Store) ListAgentRunsByCallID(ctx context.Context, callID string) ([]domain.AgentRun, error) {
	rows, err := s.db.QueryContext(ctx, agentRunSelect+` WHERE call_id = ? ORDER BY started_at`, callID)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list agent runs for call %s: %w", callID, err)
	}
	defer func() { _ = rows.Close() }()

	var runs []domain.AgentRun
	for rows.Next() {
		r, err := scanAgentRun(rows.Scan)
		if err != nil {
			return nil, err
		}
		runs = append(runs, r)
	}
	return runs, rows.Err()
}

const agentRunSelect = `SELECT id, call_id, trigger, attempt, status, started_at, ended_at,
	latency_ms, prompt_tokens, completion_tokens, cost_usd, stop_reason FROM agent_runs`

func scanAgentRun(scan func(dest ...any) error) (domain.AgentRun, error) {
	var r domain.AgentRun
	var status string
	var startedAt string
	var endedAt sql.NullString
	var latencyMs sql.NullInt64
	var stopReason sql.NullString
	if err := scan(&r.ID, &r.CallID, &r.Trigger, &r.Attempt, &status, &startedAt, &endedAt,
		&latencyMs, &r.PromptTokens, &r.CompletionTokens, &r.CostUsd, &stopReason); err != nil {
		return domain.AgentRun{}, fmt.Errorf("sqlite: scan agent run: %w", mapNoRows(err))
	}
	r.Status = domain.AgentRunStatus(status)
	if err := checkEnum("agent run status", r.Status); err != nil {
		return domain.AgentRun{}, err
	}
	var err error
	if r.StartedAt, err = stringToTime(startedAt); err != nil {
		return domain.AgentRun{}, fmt.Errorf("sqlite: agent run %s: %w", r.ID, err)
	}
	if r.EndedAt, err = stringToNullableTime(endedAt); err != nil {
		return domain.AgentRun{}, fmt.Errorf("sqlite: agent run %s: %w", r.ID, err)
	}
	r.LatencyMs = int64OrNil(latencyMs)
	if r.StopReason, err = scanStopReason(stopReason); err != nil {
		return domain.AgentRun{}, err
	}
	return r, nil
}

func nullableStopReason(r *domain.AgentRunStopReason) sql.NullString {
	if r == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: string(*r), Valid: true}
}

func scanStopReason(ns sql.NullString) (*domain.AgentRunStopReason, error) {
	if !ns.Valid {
		return nil, nil
	}
	r := domain.AgentRunStopReason(ns.String)
	if err := checkEnum("agent run stop reason", r); err != nil {
		return nil, err
	}
	return &r, nil
}

func (s *Store) InsertToolCallLog(ctx context.Context, l domain.ToolCallLog) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO tool_call_logs (id, agent_run_id, seq, tool_name, input, output, latency_ms, error)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		l.ID, l.AgentRunID, l.Seq, l.ToolName, string(l.Input), nullableJSON(l.Output), l.LatencyMs, nullableString(l.Error))
	if err != nil {
		return fmt.Errorf("sqlite: insert tool call log %s: %w", l.ID, err)
	}
	return nil
}

func (s *Store) ListToolCallLogsByAgentRunID(ctx context.Context, agentRunID string) ([]domain.ToolCallLog, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, agent_run_id, seq, tool_name, input, output, latency_ms, error
		 FROM tool_call_logs WHERE agent_run_id = ? ORDER BY seq`, agentRunID)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list tool call logs for run %s: %w", agentRunID, err)
	}
	defer func() { _ = rows.Close() }()

	var logs []domain.ToolCallLog
	for rows.Next() {
		var l domain.ToolCallLog
		var input string
		var output, errStr sql.NullString
		if err := rows.Scan(&l.ID, &l.AgentRunID, &l.Seq, &l.ToolName, &input, &output, &l.LatencyMs, &errStr); err != nil {
			return nil, fmt.Errorf("sqlite: scan tool call log: %w", err)
		}
		l.Input = []byte(input)
		if output.Valid {
			l.Output = []byte(output.String)
		}
		l.Error = stringOrNil(errStr)
		logs = append(logs, l)
	}
	return logs, rows.Err()
}

func nullableJSON(v []byte) sql.NullString {
	if v == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: string(v), Valid: true}
}
