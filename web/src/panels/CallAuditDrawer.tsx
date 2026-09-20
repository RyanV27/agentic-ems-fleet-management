import { useEffect, useState } from 'react';
import type { GqlClient } from '../gql/client';
import { CALL_AUDIT_QUERY } from '../gql/documents';
import type { CallAudit } from '../types';

export interface CallAuditDrawerProps {
  client: GqlClient;
  callId: string | null;
  onClose: () => void;
}

interface CallAuditResponse {
  callAudit: CallAudit | null;
}

// Everything the audit drawer needs for one call (ROADMAP S8 c9): transcript,
// extraction + confidence, routing decision + matched rule, proposedBy (via
// the pending action, shown by ApprovalPanel), and the ordered tool-call log
// with latency and tokens.
export function CallAuditDrawer({ client, callId, onClose }: CallAuditDrawerProps) {
  const [audit, setAudit] = useState<CallAudit | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (callId === null) {
      setAudit(null);
      return;
    }
    client
      .request<CallAuditResponse>(CALL_AUDIT_QUERY, { id: callId })
      .then((res) => {
        setAudit(res.callAudit);
        setError(null);
      })
      .catch((err: unknown) => setError(err instanceof Error ? err.message : String(err)));
  }, [client, callId]);

  if (callId === null) return null;

  return (
    <aside aria-label="Call audit drawer">
      <button type="button" onClick={onClose}>
        Close
      </button>
      {error !== null && <p role="alert">{error}</p>}
      {audit !== null && (
        <div>
          <h3>Call {audit.call.id}</h3>
          <p data-testid="audit-transcript">{audit.call.transcript}</p>
          {audit.call.extraction !== null && (
            <p data-testid="audit-extraction">
              {audit.call.extraction.incidentType} — confidence {audit.call.extraction.confidence}
            </p>
          )}
          <ul aria-label="Routing decisions">
            {audit.routingDecisions.map((decision) => (
              <li key={decision.id} data-testid={`routing-decision-${decision.id}`}>
                {decision.path} — rule {decision.matchedRuleId}
              </li>
            ))}
          </ul>
          <ol aria-label="Tool call log">
            {audit.toolCallLogs
              .slice()
              .sort((a, b) => a.seq - b.seq)
              .map((log) => (
                <li key={log.id} data-testid={`tool-call-${log.id}`}>
                  {log.toolName} — {log.latencyMs}ms
                </li>
              ))}
          </ol>
        </div>
      )}
    </aside>
  );
}
