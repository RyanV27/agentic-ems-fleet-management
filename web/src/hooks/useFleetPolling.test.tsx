import { act, renderHook } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createMockGqlClient } from '../../test/mockGraphql';
import { useFleetPolling } from './useFleetPolling';

const baseFleetStatus = {
  units: [],
  calls: [],
  dispatchEvents: [],
};

describe('useFleetPolling', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('refetches on a configurable interval, surfacing a proposal that flips to EXPIRED without a reload (ROADMAP S8 c8)', async () => {
    let callCount = 0;
    const client = createMockGqlClient({
      FleetStatus: () => ({ fleetStatus: baseFleetStatus }),
      PendingActions: () => {
        callCount += 1;
        return {
          pendingActions:
            callCount === 1
              ? [{ id: 'a1', status: 'PROPOSED' }]
              : [{ id: 'a1', status: 'EXPIRED', expiredReason: 'TTL' }],
        };
      },
    });

    const { result } = renderHook(() => useFleetPolling(client, 5000));

    await act(async () => {
      await Promise.resolve();
    });
    expect(result.current.pendingActions).toHaveLength(1);
    expect(result.current.pendingActions[0]!.status).toBe('PROPOSED');

    await act(async () => {
      await vi.advanceTimersByTimeAsync(5000);
    });

    expect(result.current.pendingActions[0]!.status).toBe('EXPIRED');
  });

  it('clears the interval on unmount', () => {
    const client = createMockGqlClient({
      FleetStatus: () => ({ fleetStatus: baseFleetStatus }),
      PendingActions: () => ({ pendingActions: [] }),
    });
    const clearIntervalSpy = vi.spyOn(global, 'clearInterval');
    const { unmount } = renderHook(() => useFleetPolling(client, 5000));
    unmount();
    expect(clearIntervalSpy).toHaveBeenCalled();
  });
});
