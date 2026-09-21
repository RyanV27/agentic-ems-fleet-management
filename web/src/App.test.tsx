import { render, screen } from '@testing-library/react';
import { act } from 'react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import units from '../test/fixtures/units.json';
import calls from '../test/fixtures/calls.json';
import dispatchEvents from '../test/fixtures/dispatchEvents.json';
import pendingActions from '../test/fixtures/pendingActions.json';
import { App } from './App';

// Guards against a real bug: every other test mounts a panel directly with a
// mocked client, so none of them ever exercised the top-level App render
// with live data. That render path is where App wrapped every panel in
// CopilotKit's <CopilotKit> provider, which throws a ConfigurationError with
// no runtimeUrl/publicApiKey/publicLicenseKey/local agents — crashing to a
// blank page in a real browser. This test mounts the real App with a mocked
// fetch so a future regression that reintroduces a component requiring
// config this app never provides fails here instead of only in a browser.
describe('App', () => {
  let fetchSpy: ReturnType<typeof vi.spyOn>;

  beforeEach(() => {
    fetchSpy = vi.spyOn(global, 'fetch').mockImplementation(async (_url, init) => {
      const body = JSON.parse((init as RequestInit).body as string) as { query: string };
      const data = body.query.includes('pendingActions')
        ? { pendingActions }
        : body.query.includes('callAudit')
          ? { callAudit: null }
          : { fleetStatus: { units, calls, dispatchEvents } };
      return new Response(JSON.stringify({ data }), { status: 200, headers: { 'Content-Type': 'application/json' } });
    });
  });

  afterEach(() => {
    fetchSpy.mockRestore();
  });

  it('renders the dashboard from live data with no CopilotKit configuration required', async () => {
    await act(async () => {
      render(<App />);
      await Promise.resolve();
      await Promise.resolve();
    });

    expect(screen.getByRole('heading', { name: 'EMS Fleet Operator Dashboard' })).toBeInTheDocument();
    expect(screen.getByTestId('unit-row-unit-01')).toBeInTheDocument();
  });
});
