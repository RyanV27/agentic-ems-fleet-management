import { useEffect, useMemo, useState } from 'react';
import { loadConfig } from './config';
import { createGqlClient } from './gql/client';
import { CALL_AUDIT_QUERY } from './gql/documents';
import { ApprovalPanel } from './panels/ApprovalPanel';
import { CallAuditDrawer } from './panels/CallAuditDrawer';
import { CallQueue } from './panels/CallQueue';
import { EventLog } from './panels/EventLog';
import { FleetBoard } from './panels/FleetBoard';
import { parsePayload } from './payloads';
import { useFleetPolling } from './hooks/useFleetPolling';
import type { CallAudit, ToolCallLog } from './types';

const config = loadConfig();
const client = createGqlClient(config.fleetGraphqlUrl);

function actionCallId(action: { type: string; payload: string }): string | null {
  if (action.type === 'RESOLVE_EVENT') return null;
  try {
    return (parsePayload(action.type as never, action.payload) as { callId?: string }).callId ?? null;
  } catch {
    return null;
  }
}

export function App() {
  const { fleetStatus, pendingActions, refetch } = useFleetPolling(client, config.pollIntervalMs);
  const [selectedCallId, setSelectedCallId] = useState<string | null>(null);
  const [toolCallLogsByAgentRunId, setToolCallLogsByAgentRunId] = useState<Record<string, ToolCallLog[]>>({});

  const agentCallIds = useMemo(() => {
    const ids = new Set<string>();
    for (const action of pendingActions) {
      if (action.proposedBy !== 'AGENT') continue;
      const callId = actionCallId(action);
      if (callId !== null) ids.add(callId);
    }
    return Array.from(ids);
  }, [pendingActions]);

  useEffect(() => {
    agentCallIds.forEach((callId) => {
      client
        .request<{ callAudit: CallAudit | null }>(CALL_AUDIT_QUERY, { id: callId })
        .then((res) => {
          if (res.callAudit === null) return;
          setToolCallLogsByAgentRunId((prev) => {
            const next = { ...prev };
            for (const log of res.callAudit!.toolCallLogs) {
              next[log.agentRunId] = [...(next[log.agentRunId] ?? []), log];
            }
            return next;
          });
        })
        .catch(() => {
          // Audit enrichment is best-effort; the approval panel still renders
          // without a tool-call list if this fails.
        });
    });
  }, [agentCallIds]);

  if (fleetStatus === null) {
    return <p>Loading fleet status…</p>;
  }

  return (
    <main>
      <h1>EMS Fleet Operator Dashboard</h1>
      <FleetBoard
        units={fleetStatus.units}
        calls={fleetStatus.calls}
        dispatchEvents={fleetStatus.dispatchEvents}
      />
      <CallQueue calls={fleetStatus.calls} selectedCallId={selectedCallId} onSelectCall={setSelectedCallId} />
      <ApprovalPanel
        actions={pendingActions}
        units={fleetStatus.units}
        toolCallLogsByAgentRunId={toolCallLogsByAgentRunId}
        client={client}
        onActionUpdated={refetch}
      />
      <EventLog dispatchEvents={fleetStatus.dispatchEvents} />
      <CallAuditDrawer client={client} callId={selectedCallId} onClose={() => setSelectedCallId(null)} />
    </main>
  );
}
