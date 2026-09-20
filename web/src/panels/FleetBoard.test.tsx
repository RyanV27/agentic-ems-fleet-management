import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { FleetBoard } from './FleetBoard';
import units from '../../test/fixtures/units.json';
import calls from '../../test/fixtures/calls.json';
import dispatchEvents from '../../test/fixtures/dispatchEvents.json';
import type { Call, DispatchEvent, Unit } from '../types';

describe('FleetBoard', () => {
  it('renders units with status and zone, active calls with severity and wait, open dispatch events (ROADMAP S8 c1)', () => {
    render(
      <FleetBoard
        units={units as Unit[]}
        calls={calls as Call[]}
        dispatchEvents={dispatchEvents as DispatchEvent[]}
      />,
    );

    expect(screen.getByTestId('unit-row-unit-01')).toHaveTextContent('Medic 1');
    expect(screen.getByTestId('unit-row-unit-01')).toHaveTextContent('AVAILABLE');
    expect(screen.getByTestId('unit-row-unit-01')).toHaveTextContent('zone-1');

    expect(screen.getByTestId('call-row-call-1')).toHaveTextContent('P1');
    expect(screen.getByTestId('call-row-call-1')).toHaveTextContent('s');

    expect(screen.getByTestId('event-row-event-1')).toHaveTextContent('BREAKDOWN');
  });

  it('excludes CLOSED and CANCELLED calls from the active calls table', () => {
    const closedCall: Call = { ...(calls as Call[])[0]!, id: 'call-closed', status: 'CLOSED' };
    render(<FleetBoard units={[]} calls={[closedCall]} dispatchEvents={[]} />);
    expect(screen.queryByTestId('call-row-call-closed')).not.toBeInTheDocument();
  });
});
