import { useCallback, useEffect, useState } from 'react';
import type { GqlClient } from '../gql/client';
import { FLEET_STATUS_QUERY, PENDING_ACTIONS_QUERY } from '../gql/documents';
import type { FleetStatus, PendingAction } from '../types';

interface FleetStatusResponse {
  fleetStatus: FleetStatus;
}

interface PendingActionsResponse {
  pendingActions: PendingAction[];
}

export interface FleetPollingState {
  fleetStatus: FleetStatus | null;
  pendingActions: PendingAction[];
  error: string | null;
  refetch: () => void;
}

// Refetches fleetStatus + pendingActions on a fixed interval — Phase 1 uses
// polling, not subscriptions (ROADMAP S8 scope, FR-19). Cleared on unmount.
export function useFleetPolling(client: GqlClient, pollIntervalMs: number): FleetPollingState {
  const [fleetStatus, setFleetStatus] = useState<FleetStatus | null>(null);
  const [pendingActions, setPendingActions] = useState<PendingAction[]>([]);
  const [error, setError] = useState<string | null>(null);

  const fetchOnce = useCallback(() => {
    Promise.all([
      client.request<FleetStatusResponse>(FLEET_STATUS_QUERY),
      client.request<PendingActionsResponse>(PENDING_ACTIONS_QUERY, {}),
    ])
      .then(([statusRes, actionsRes]) => {
        setFleetStatus(statusRes.fleetStatus);
        setPendingActions(actionsRes.pendingActions);
        setError(null);
      })
      .catch((err: unknown) => {
        setError(err instanceof Error ? err.message : String(err));
      });
  }, [client]);

  useEffect(() => {
    fetchOnce();
    const id = setInterval(fetchOnce, pollIntervalMs);
    return () => clearInterval(id);
  }, [fetchOnce, pollIntervalMs]);

  return { fleetStatus, pendingActions, error, refetch: fetchOnce };
}
