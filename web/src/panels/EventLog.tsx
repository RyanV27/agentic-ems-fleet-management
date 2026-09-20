import type { DispatchEvent } from '../types';

export interface EventLogProps {
  dispatchEvents: DispatchEvent[];
}

// Read-only. Resolving an event flows through the same proposal→approve path
// as other action types (RESOLVE_EVENT payloads render via ApprovalPanel) —
// this panel never itself mutates.
export function EventLog({ dispatchEvents }: EventLogProps) {
  return (
    <section aria-label="Event log">
      <h2>Event Log</h2>
      <ul>
        {dispatchEvents.map((event) => (
          <li key={event.id} data-testid={`event-log-${event.id}`}>
            [{event.status}] {event.type} — {event.note}
          </li>
        ))}
      </ul>
    </section>
  );
}
