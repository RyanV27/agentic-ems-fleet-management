import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createMockGqlClient } from '../test/mockGraphql';
import units from '../test/fixtures/units.json';
import calls from '../test/fixtures/calls.json';
import dispatchEvents from '../test/fixtures/dispatchEvents.json';
import pendingActions from '../test/fixtures/pendingActions.json';
import { ApprovalPanel } from './panels/ApprovalPanel';
import { FleetBoard } from './panels/FleetBoard';
import type { Call, DispatchEvent, PendingAction, Unit } from './types';

// ROADMAP S8 c10: the board and approval panel render correctly from
// fixtures, and the SYSTEM proposal flow is fully usable, with the agent
// process not running at all. Since S8 uses CopilotKit as UI chrome only
// (no runtimeUrl, no chat, no agent endpoint dependency — see plan), this is
// verified by asserting the fleet dashboard's fetch traffic never reaches
// any agent-hosted address regardless of whether that process is up.
describe('graceful degradation with the agent process stopped', () => {
  let fetchSpy: ReturnType<typeof vi.spyOn>;

  beforeEach(() => {
    fetchSpy = vi.spyOn(global, 'fetch').mockRejectedValue(new Error('agent process is not running'));
  });

  afterEach(() => {
    fetchSpy.mockRestore();
  });

  it('renders the fleet board and lets a SYSTEM proposal be fully approved without any fetch to the agent', async () => {
    const user = userEvent.setup();
    const client = createMockGqlClient({
      ExecuteAction: { executeAction: { id: 'action-system-1', status: 'EXECUTED' } },
    });
    const systemAction = (pendingActions as PendingAction[])[0]!;

    render(
      <>
        <FleetBoard units={units as Unit[]} calls={calls as Call[]} dispatchEvents={dispatchEvents as DispatchEvent[]} />
        <ApprovalPanel
          actions={[systemAction]}
          units={units as Unit[]}
          toolCallLogsByAgentRunId={{}}
          client={client}
          onActionUpdated={() => {}}
        />
      </>,
    );

    expect(screen.getByTestId('unit-row-unit-01')).toBeInTheDocument();
    const row = screen.getByTestId(`proposal-${systemAction.id}`);
    await user.click(within(row).getByRole('button', { name: 'Approve' }));

    expect(client.calls).toHaveLength(1);
    expect(client.calls[0]!.document).toContain('executeAction');
    expect(fetchSpy).not.toHaveBeenCalled();
  });
});
