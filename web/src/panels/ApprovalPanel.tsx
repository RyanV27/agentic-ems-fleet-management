import { useState } from 'react';
import type { GqlClient } from '../gql/client';
import {
  ASSIGN_CALL_MUTATION,
  EXECUTE_ACTION_MUTATION,
  REJECT_ACTION_MUTATION,
  REROUTE_UNIT_MUTATION,
  RESOLVE_DISPATCH_EVENT_MUTATION,
} from '../gql/documents';
import { parsePayload, payloadSummary } from '../payloads';
import { REJECTION_REASONS } from '../types';
import type { PendingAction, RejectionReason, ToolCallLog, Unit } from '../types';

export interface ApprovalPanelProps {
  actions: PendingAction[];
  units: Unit[];
  toolCallLogsByAgentRunId: Record<string, ToolCallLog[]>;
  client: GqlClient;
  onActionUpdated: () => void;
}

async function manualDispatch(
  client: GqlClient,
  action: PendingAction,
  chosenUnitId: string | null,
): Promise<void> {
  if (action.type === 'RESOLVE_EVENT') {
    const payload = parsePayload(action.type, action.payload) as { dispatchEventId: string };
    await client.request(RESOLVE_DISPATCH_EVENT_MUTATION, { dispatchEventId: payload.dispatchEventId });
    return;
  }
  const payload = parsePayload(action.type, action.payload) as { callId: string; unitId: string };
  if (chosenUnitId === null) throw new Error('a unit must be selected for manual dispatch');
  if (action.type === 'REROUTE_UNIT') {
    await client.request(REROUTE_UNIT_MUTATION, { unitId: chosenUnitId, callId: payload.callId });
  } else {
    await client.request(ASSIGN_CALL_MUTATION, { callId: payload.callId, unitId: chosenUnitId });
  }
}

function ProposalRow({
  action,
  units,
  toolCallLogsByAgentRunId,
  client,
  onActionUpdated,
}: {
  action: PendingAction;
  units: Unit[];
  toolCallLogsByAgentRunId: Record<string, ToolCallLog[]>;
  client: GqlClient;
  onActionUpdated: () => void;
}) {
  const [rejecting, setRejecting] = useState(false);
  const [dispatching, setDispatching] = useState(false);
  const [chosenUnitId, setChosenUnitId] = useState<string>('');

  const isAgent = action.proposedBy === 'AGENT';
  const isExpired = action.status === 'EXPIRED';
  const toolCalls = action.agentRunId !== null ? (toolCallLogsByAgentRunId[action.agentRunId] ?? []) : [];

  async function handleApprove() {
    await client.request(EXECUTE_ACTION_MUTATION, { actionId: action.id });
    onActionUpdated();
  }

  async function handleReject(reason: RejectionReason) {
    await client.request(REJECT_ACTION_MUTATION, { actionId: action.id, reason, note: null });
    setRejecting(false);
    onActionUpdated();
  }

  async function handleManualDispatchConfirm() {
    await manualDispatch(client, action, action.type === 'RESOLVE_EVENT' ? null : chosenUnitId || null);
    setDispatching(false);
    onActionUpdated();
  }

  return (
    <li
      data-testid={`proposal-${action.id}`}
      data-proposed-by={action.proposedBy}
      data-status={action.status}
    >
      {isAgent ? (
        <div data-testid={`proposal-expanded-${action.id}`}>
          <p>{action.rationale}</p>
          <p>{payloadSummary(action.type, action.payload)}</p>
          {toolCalls.length > 0 && (
            <ol aria-label="Tool calls" data-testid={`tool-call-list-${action.id}`}>
              {toolCalls
                .slice()
                .sort((a, b) => a.seq - b.seq)
                .map((log) => (
                  <li key={log.id}>{log.toolName}</li>
                ))}
            </ol>
          )}
        </div>
      ) : (
        <div data-testid={`proposal-compact-${action.id}`}>
          <p>{action.rationale}</p>
        </div>
      )}

      {isExpired && action.expiredReason !== null && (
        <p data-testid={`expired-reason-${action.id}`}>Expired: {action.expiredReason}</p>
      )}

      <button type="button" onClick={handleApprove} disabled={isExpired}>
        Approve
      </button>

      <button type="button" onClick={() => setRejecting((v) => !v)}>
        Reject
      </button>
      {rejecting && (
        <div role="group" aria-label={`Reject reasons for ${action.id}`}>
          {REJECTION_REASONS.map((reason) => (
            <button key={reason} type="button" onClick={() => void handleReject(reason)}>
              {reason}
            </button>
          ))}
        </div>
      )}

      <button type="button" onClick={() => setDispatching((v) => !v)}>
        Manual dispatch
      </button>
      {dispatching && (
        <div aria-label={`Manual dispatch for ${action.id}`}>
          {action.type !== 'RESOLVE_EVENT' && (
            <select
              aria-label={`Choose unit for ${action.id}`}
              value={chosenUnitId}
              onChange={(e) => setChosenUnitId(e.target.value)}
            >
              <option value="">Select unit</option>
              {units.map((unit) => (
                <option key={unit.id} value={unit.id}>
                  {unit.callsign}
                </option>
              ))}
            </select>
          )}
          <button type="button" onClick={() => void handleManualDispatchConfirm()}>
            Confirm dispatch
          </button>
        </div>
      )}
    </li>
  );
}

// The core approval surface (ROADMAP S8 c2-c7). Renders every PROPOSED/EXPIRED
// PendingAction. SYSTEM proposals render compactly with no tool-call section;
// AGENT proposals render expanded with their ordered tool-call list. All three
// controls (Approve/Reject/Manual dispatch) are always rendered and enabled,
// except Approve on an EXPIRED action (DEC-016, DEC-019).
export function ApprovalPanel({ actions, units, toolCallLogsByAgentRunId, client, onActionUpdated }: ApprovalPanelProps) {
  const visible = actions.filter((a) => a.status === 'PROPOSED' || a.status === 'EXPIRED');

  return (
    <section aria-label="Approval panel">
      <h2>Approval Panel</h2>
      <ul>
        {visible.map((action) => (
          <ProposalRow
            key={action.id}
            action={action}
            units={units}
            toolCallLogsByAgentRunId={toolCallLogsByAgentRunId}
            client={client}
            onActionUpdated={onActionUpdated}
          />
        ))}
      </ul>
    </section>
  );
}
