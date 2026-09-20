import { render, screen, waitFor } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { createMockGqlClient } from '../../test/mockGraphql';
import callAudit from '../../test/fixtures/callAudit.json';
import { CallAuditDrawer } from './CallAuditDrawer';

describe('CallAuditDrawer', () => {
  it('shows transcript, extraction with confidence, routing decision, and the ordered tool-call log with latency (ROADMAP S8 c9)', async () => {
    const client = createMockGqlClient({
      CallAudit: { callAudit },
    });

    render(<CallAuditDrawer client={client} callId="call-2" onClose={() => {}} />);

    await waitFor(() => expect(screen.getByTestId('audit-transcript')).toBeInTheDocument());
    expect(screen.getByTestId('audit-transcript')).toHaveTextContent('fall');
    expect(screen.getByTestId('audit-extraction')).toHaveTextContent('0.42');
    expect(screen.getByTestId('routing-decision-routing-1')).toHaveTextContent('low-confidence-extraction');

    const toolCalls = screen.getAllByRole('listitem').filter((el) => el.dataset.testid?.startsWith('tool-call-'));
    expect(toolCalls.map((el) => el.textContent)).toEqual([
      expect.stringContaining('getFleetStatus'),
      expect.stringContaining('assignBackupUnit'),
    ]);
    expect(toolCalls[0]).toHaveTextContent('45ms');
  });

  it('renders nothing when no call is selected', () => {
    const client = createMockGqlClient({});
    const { container } = render(<CallAuditDrawer client={client} callId={null} onClose={() => {}} />);
    expect(container).toBeEmptyDOMElement();
  });
});
