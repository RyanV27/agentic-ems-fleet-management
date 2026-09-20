// Mirrors services/fleet/graph/schema.query.graphqls / schema.mutation.graphqls
// enums and types 1:1. JSON string fields (payload, inputSnapshot, input,
// output) stay typed as string and are parsed only at the render site
// (DEC-030) — this module never assumes their shape.

export type UnitStatus = 'AVAILABLE' | 'EN_ROUTE' | 'ON_SCENE' | 'TRANSPORTING' | 'OUT_OF_SERVICE';
export type Capability = 'BLS' | 'ALS';
export type CallStatus =
  | 'PENDING_APPROVAL'
  | 'ESCALATED'
  | 'ASSIGNED'
  | 'EN_ROUTE'
  | 'ON_SCENE'
  | 'TRANSPORTING'
  | 'CLOSED'
  | 'CANCELLED';
export type ActionType = 'ASSIGN_CALL' | 'REROUTE_UNIT' | 'ASSIGN_BACKUP_UNIT' | 'RESOLVE_EVENT';
export type ActionStatus = 'PROPOSED' | 'APPROVED' | 'EXECUTED' | 'REJECTED' | 'EXPIRED' | 'FAILED';
export type ProposedBy = 'SYSTEM' | 'AGENT' | 'OPERATOR';
export type ExpiredReason = 'UNIT_REASSIGNED' | 'UNIT_OUT_OF_SERVICE' | 'CALL_CLOSED' | 'SUPERSEDED' | 'TTL';
export type RejectionReason = 'WRONG_UNIT' | 'INSUFFICIENT_INFO' | 'UNSAFE_TIMING' | 'OTHER';
export type DispatchEventType = 'DELAY' | 'BREAKDOWN' | 'REROUTE_NEEDED' | 'SEVERITY_CHANGE' | 'UNIT_UNAVAILABLE';
export type DispatchEventStatus = 'OPEN' | 'RESOLVED';
export type RoutingPath = 'DETERMINISTIC' | 'ESCALATED';
export type AgentRunStatus = 'RUNNING' | 'COMPLETED' | 'FAILED';
export type AgentRunStopReason = 'COMPLETED' | 'MAX_STEPS' | 'INVALID_OUTPUT' | 'ERROR';

export const REJECTION_REASONS: readonly RejectionReason[] = [
  'WRONG_UNIT',
  'INSUFFICIENT_INFO',
  'UNSAFE_TIMING',
  'OTHER',
];

export interface Unit {
  id: string;
  callsign: string;
  status: UnitStatus;
  capability: Capability;
  zoneId: string;
  currentCallId: string | null;
  statusChangedAt: string;
}

export interface Extraction {
  callId: string;
  incidentType: string;
  severity: number;
  zoneId: string;
  keywords: string[];
  needsTransport: boolean;
  confidence: number;
  failureReason: string | null;
  model: string;
  latencyMs: number;
  promptTokens: number;
  completionTokens: number;
  createdAt: string;
}

export interface Call {
  id: string;
  transcript: string;
  extraction: Extraction | null;
  severity: number;
  zoneId: string;
  status: CallStatus;
  assignedUnitId: string | null;
  destinationHospitalId: string | null;
  createdAt: string;
  enqueuedAt: string | null;
  priorityScore: number | null;
}

export interface Zone {
  id: string;
  name: string;
}

export interface Hospital {
  id: string;
  name: string;
  zoneId: string;
  capabilities: string[];
  accepting: boolean;
}

export interface DispatchEvent {
  id: string;
  type: DispatchEventType;
  callId: string | null;
  unitId: string | null;
  severityDelta: number | null;
  status: DispatchEventStatus;
  note: string;
  createdAt: string;
  resolvedAt: string | null;
}

export interface PendingAction {
  id: string;
  type: ActionType;
  payload: string;
  status: ActionStatus;
  idempotencyKey: string;
  proposedBy: ProposedBy;
  rationale: string;
  expiredReason: ExpiredReason | null;
  agentRunId: string | null;
  rejectionReason: RejectionReason | null;
  rejectionNote: string | null;
  parentActionId: string | null;
  createdAt: string;
  decidedAt: string | null;
  decidedBy: string | null;
}

export interface RoutingDecision {
  id: string;
  callId: string;
  path: RoutingPath;
  matchedRuleId: string;
  inputSnapshot: string;
  decidedAt: string;
}

export interface AgentRun {
  id: string;
  callId: string;
  trigger: string;
  attempt: number;
  status: AgentRunStatus;
  startedAt: string;
  endedAt: string | null;
  latencyMs: number | null;
  promptTokens: number;
  completionTokens: number;
  costUsd: number;
  stopReason: AgentRunStopReason | null;
}

export interface ToolCallLog {
  id: string;
  agentRunId: string;
  seq: number;
  toolName: string;
  input: string;
  output: string;
  latencyMs: number;
  error: string | null;
}

export interface FleetStatus {
  units: Unit[];
  calls: Call[];
  dispatchEvents: DispatchEvent[];
}

export interface CallAudit {
  call: Call;
  routingDecisions: RoutingDecision[];
  agentRuns: AgentRun[];
  toolCallLogs: ToolCallLog[];
}
