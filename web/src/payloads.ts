// Mirrors services/fleet/internal/actions/payload.go's JSON shapes exactly
// (DEC-030: payload is opaque String! on the wire, parsed only here).
import type { ActionType } from './types';

export interface AssignCallPayload {
  callId: string;
  unitId: string;
}

export interface RerouteUnitPayload {
  unitId: string;
  callId: string;
}

export interface AssignBackupUnitPayload {
  callId: string;
  unitId: string;
}

export interface ResolveEventPayload {
  dispatchEventId: string;
}

export type ParsedPayload =
  | AssignCallPayload
  | RerouteUnitPayload
  | AssignBackupUnitPayload
  | ResolveEventPayload;

export function parsePayload(type: ActionType, raw: string): ParsedPayload {
  return JSON.parse(raw) as ParsedPayload;
}

export function payloadSummary(type: ActionType, raw: string): string {
  const parsed = parsePayload(type, raw);
  switch (type) {
    case 'ASSIGN_CALL':
      return `Assign unit ${(parsed as AssignCallPayload).unitId} to call ${(parsed as AssignCallPayload).callId}`;
    case 'REROUTE_UNIT':
      return `Reroute unit ${(parsed as RerouteUnitPayload).unitId} to call ${(parsed as RerouteUnitPayload).callId}`;
    case 'ASSIGN_BACKUP_UNIT':
      return `Assign backup unit ${(parsed as AssignBackupUnitPayload).unitId} to call ${(parsed as AssignBackupUnitPayload).callId}`;
    case 'RESOLVE_EVENT':
      return `Resolve dispatch event ${(parsed as ResolveEventPayload).dispatchEventId}`;
    default:
      return raw;
  }
}
