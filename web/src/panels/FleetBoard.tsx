import type { Call, DispatchEvent, Unit } from '../types';

export interface FleetBoardProps {
  units: Unit[];
  calls: Call[];
  dispatchEvents: DispatchEvent[];
}

function activeCalls(calls: Call[]): Call[] {
  return calls.filter((c) => c.status !== 'CLOSED' && c.status !== 'CANCELLED');
}

function openEvents(events: DispatchEvent[]): DispatchEvent[] {
  return events.filter((e) => e.status === 'OPEN');
}

function waitSeconds(call: Call): number | null {
  if (!call.enqueuedAt) return null;
  const enqueuedAtMs = new Date(call.enqueuedAt).getTime();
  return Math.max(0, Math.floor((Date.now() - enqueuedAtMs) / 1000));
}

export function FleetBoard({ units, calls, dispatchEvents }: FleetBoardProps) {
  return (
    <section aria-label="Fleet board">
      <h2>Fleet Board</h2>

      <table aria-label="Units">
        <thead>
          <tr>
            <th>Callsign</th>
            <th>Status</th>
            <th>Zone</th>
          </tr>
        </thead>
        <tbody>
          {units.map((unit) => (
            <tr key={unit.id} data-testid={`unit-row-${unit.id}`}>
              <td>{unit.callsign}</td>
              <td>{unit.status}</td>
              <td>{unit.zoneId}</td>
            </tr>
          ))}
        </tbody>
      </table>

      <table aria-label="Active calls">
        <thead>
          <tr>
            <th>Call</th>
            <th>Severity</th>
            <th>Wait</th>
          </tr>
        </thead>
        <tbody>
          {activeCalls(calls).map((call) => (
            <tr key={call.id} data-testid={`call-row-${call.id}`}>
              <td>{call.id}</td>
              <td>P{call.severity}</td>
              <td>{waitSeconds(call) === null ? '—' : `${waitSeconds(call)}s`}</td>
            </tr>
          ))}
        </tbody>
      </table>

      <table aria-label="Open dispatch events">
        <thead>
          <tr>
            <th>Type</th>
            <th>Note</th>
          </tr>
        </thead>
        <tbody>
          {openEvents(dispatchEvents).map((event) => (
            <tr key={event.id} data-testid={`event-row-${event.id}`}>
              <td>{event.type}</td>
              <td>{event.note}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </section>
  );
}
