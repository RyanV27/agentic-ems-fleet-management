import type { Call } from '../types';

export interface CallQueueProps {
  calls: Call[];
  selectedCallId: string | null;
  onSelectCall: (callId: string) => void;
}

// The operator work queue: calls still awaiting a decision, sorted by
// priorityScore descending (highest priority first) — feeds which call's
// proposal is focused in ApprovalPanel.
export function CallQueue({ calls, selectedCallId, onSelectCall }: CallQueueProps) {
  const queued = calls
    .filter((c) => c.status === 'PENDING_APPROVAL' || c.status === 'ESCALATED')
    .slice()
    .sort((a, b) => (b.priorityScore ?? 0) - (a.priorityScore ?? 0));

  return (
    <section aria-label="Call queue">
      <h2>Call Queue</h2>
      <ul>
        {queued.map((call) => (
          <li key={call.id}>
            <button
              type="button"
              aria-pressed={call.id === selectedCallId}
              onClick={() => onSelectCall(call.id)}
            >
              {call.id} — P{call.severity} — {call.status}
            </button>
          </li>
        ))}
      </ul>
    </section>
  );
}
