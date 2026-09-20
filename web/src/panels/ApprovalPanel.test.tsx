import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it } from 'vitest';
import { createMockGqlClient } from '../../test/mockGraphql';
import units from '../../test/fixtures/units.json';
import pendingActions from '../../test/fixtures/pendingActions.json';
import { ApprovalPanel } from './ApprovalPanel';
import type { PendingAction, Unit } from '../types';

const actions = pendingActions as PendingAction[];
const allUnits = units as Unit[];

function renderPanel(actionsToRender: PendingAction[], responses: Record<string, unknown> = {}) {
  const client = createMockGqlClient({
    ExecuteAction: { executeAction: { id: 'x', status: 'EXECUTED' } },
    RejectAction: { rejectAction: { id: 'x', status: 'REJECTED' } },
    AssignCall: { assignCall: { id: 'x', status: 'EXECUTED' } },
    RerouteUnit: { rerouteUnit: { id: 'x', status: 'EXECUTED' } },
    ResolveDispatchEvent: { resolveDispatchEvent: { id: 'x', status: 'EXECUTED' } },
    ...responses,
  });
  render(
    <ApprovalPanel
      actions={actionsToRender}
      units={allUnits}
      toolCallLogsByAgentRunId={{}}
      client={client}
      onActionUpdated={() => {}}
    />,
  );
  return client;
}

describe('ApprovalPanel', () => {
  it('renders SYSTEM proposals compactly with no tool-call section, AGENT proposals expanded with one (ROADMAP S8 c2, c3)', () => {
    renderPanel([actions[0]!, actions[1]!]);
    expect(screen.getByTestId('proposal-compact-action-system-1')).toBeInTheDocument();
    expect(screen.queryByTestId('proposal-expanded-action-system-1')).not.toBeInTheDocument();

    expect(screen.getByTestId('proposal-expanded-action-agent-1')).toBeInTheDocument();
    expect(screen.queryByTestId('tool-call-list-action-system-1')).not.toBeInTheDocument();
  });

  it('renders all three controls enabled on every proposal, including SYSTEM (ROADMAP S8 c4)', () => {
    renderPanel([actions[0]!]);
    const row = screen.getByTestId('proposal-action-system-1');
    expect(within(row).getByRole('button', { name: 'Approve' })).toBeEnabled();
    expect(within(row).getByRole('button', { name: 'Reject' })).toBeEnabled();
    expect(within(row).getByRole('button', { name: 'Manual dispatch' })).toBeEnabled();
  });

  it('disables Approve but keeps Reject and Manual dispatch enabled on EXPIRED proposals, and shows expiredReason (ROADMAP S8 c5)', () => {
    const expiredAction = actions.find((a) => a.id === 'action-expired-unit-reassigned')!;
    renderPanel([expiredAction]);
    const row = screen.getByTestId(`proposal-${expiredAction.id}`);
    expect(within(row).getByRole('button', { name: 'Approve' })).toBeDisabled();
    expect(within(row).getByRole('button', { name: 'Reject' })).toBeEnabled();
    expect(within(row).getByRole('button', { name: 'Manual dispatch' })).toBeEnabled();
    expect(screen.getByTestId(`expired-reason-${expiredAction.id}`)).toHaveTextContent('UNIT_REASSIGNED');
  });

  it('Approve fires exactly executeAction for a SYSTEM proposal and for an AGENT proposal (ROADMAP S8 c6)', async () => {
    const user = userEvent.setup();
    const client = renderPanel([actions[0]!, actions[1]!]);

    await user.click(within(screen.getByTestId('proposal-action-system-1')).getByRole('button', { name: 'Approve' }));
    await user.click(within(screen.getByTestId('proposal-action-agent-1')).getByRole('button', { name: 'Approve' }));

    expect(client.calls).toHaveLength(2);
    for (const call of client.calls) {
      expect(call.document).toContain('executeAction');
    }
  });

  it('Reject opens a picker with exactly the four RejectionReason values, then fires rejectAction (ROADMAP S8 c6)', async () => {
    const user = userEvent.setup();
    const client = renderPanel([actions[0]!]);
    const row = screen.getByTestId('proposal-action-system-1');

    await user.click(within(row).getByRole('button', { name: 'Reject' }));
    const group = within(row).getByRole('group');
    const reasonButtons = within(group).getAllByRole('button');
    expect(reasonButtons.map((b) => b.textContent)).toEqual([
      'WRONG_UNIT',
      'INSUFFICIENT_INFO',
      'UNSAFE_TIMING',
      'OTHER',
    ]);

    await user.click(within(group).getByRole('button', { name: 'WRONG_UNIT' }));
    expect(client.calls).toHaveLength(1);
    expect(client.calls[0]!.document).toContain('rejectAction');
    expect(client.calls[0]!.variables).toMatchObject({ reason: 'WRONG_UNIT' });
  });

  it('Manual dispatch calls the manual mutation path directly and never contacts any agent endpoint (ROADMAP S8 c7)', async () => {
    const user = userEvent.setup();
    const client = renderPanel([actions[0]!]);
    const row = screen.getByTestId('proposal-action-system-1');

    await user.click(within(row).getByRole('button', { name: 'Manual dispatch' }));
    await user.selectOptions(within(row).getByRole('combobox'), 'unit-02');
    await user.click(within(row).getByRole('button', { name: 'Confirm dispatch' }));

    expect(client.calls).toHaveLength(1);
    expect(client.calls[0]!.document).toContain('assignCall');
    expect(client.calls[0]!.document).not.toContain('executeAction');
  });
});
