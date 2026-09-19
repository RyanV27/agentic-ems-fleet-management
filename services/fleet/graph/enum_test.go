package graph_test

import (
	"sort"
	"testing"

	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/graph/generated"
	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/domain"
	"github.com/stretchr/testify/require"
)

// TestEnums_MirrorDomainExactly is S2 c2: the GraphQL enums generated from
// schema.query.graphqls must be exactly the domain enums in internal/domain,
// so this fails the moment one drifts from the other.
func TestEnums_MirrorDomainExactly(t *testing.T) {
	cases := []struct {
		name      string
		domain    []string
		generated []string
	}{
		{"UnitStatus", stringsOf(
			domain.UnitStatusAvailable, domain.UnitStatusEnRoute, domain.UnitStatusOnScene,
			domain.UnitStatusTransporting, domain.UnitStatusOutOfService,
		), enumStrings(generated.AllUnitStatus)},
		{"Capability", stringsOf(
			domain.CapabilityBLS, domain.CapabilityALS,
		), enumStrings(generated.AllCapability)},
		{"CallStatus", stringsOf(
			domain.CallStatusPendingApproval, domain.CallStatusEscalated, domain.CallStatusAssigned,
			domain.CallStatusEnRoute, domain.CallStatusOnScene, domain.CallStatusTransporting,
			domain.CallStatusClosed, domain.CallStatusCancelled,
		), enumStrings(generated.AllCallStatus)},
		{"ActionType", stringsOf(
			domain.ActionTypeAssignCall, domain.ActionTypeRerouteUnit,
			domain.ActionTypeAssignBackupUnit, domain.ActionTypeResolveEvent,
		), enumStrings(generated.AllActionType)},
		{"ActionStatus", stringsOf(
			domain.ActionStatusProposed, domain.ActionStatusApproved, domain.ActionStatusExecuted,
			domain.ActionStatusRejected, domain.ActionStatusExpired, domain.ActionStatusFailed,
		), enumStrings(generated.AllActionStatus)},
		{"ProposedBy", stringsOf(
			domain.ProposedBySystem, domain.ProposedByAgent, domain.ProposedByOperator,
		), enumStrings(generated.AllProposedBy)},
		{"ExpiredReason", stringsOf(
			domain.ExpiredReasonUnitReassigned, domain.ExpiredReasonUnitOutOfService,
			domain.ExpiredReasonCallClosed, domain.ExpiredReasonSuperseded, domain.ExpiredReasonTTL,
		), enumStrings(generated.AllExpiredReason)},
		{"RejectionReason", stringsOf(
			domain.RejectionReasonWrongUnit, domain.RejectionReasonInsufficientInfo,
			domain.RejectionReasonUnsafeTiming, domain.RejectionReasonOther,
		), enumStrings(generated.AllRejectionReason)},
		{"DispatchEventType", stringsOf(
			domain.DispatchEventTypeDelay, domain.DispatchEventTypeBreakdown,
			domain.DispatchEventTypeRerouteNeeded, domain.DispatchEventTypeSeverityChange,
			domain.DispatchEventTypeUnitUnavailable,
		), enumStrings(generated.AllDispatchEventType)},
		{"DispatchEventStatus", stringsOf(
			domain.DispatchEventStatusOpen, domain.DispatchEventStatusResolved,
		), enumStrings(generated.AllDispatchEventStatus)},
		{"RoutingPath", stringsOf(
			domain.RoutingPathDeterministic, domain.RoutingPathEscalated,
		), enumStrings(generated.AllRoutingPath)},
		{"AgentRunStatus", stringsOf(
			domain.AgentRunStatusRunning, domain.AgentRunStatusCompleted, domain.AgentRunStatusFailed,
		), enumStrings(generated.AllAgentRunStatus)},
		{"AgentRunStopReason", stringsOf(
			domain.AgentRunStopReasonCompleted, domain.AgentRunStopReasonMaxSteps,
			domain.AgentRunStopReasonInvalidOutput, domain.AgentRunStopReasonError,
		), enumStrings(generated.AllAgentRunStopReason)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sort.Strings(tc.domain)
			sort.Strings(tc.generated)
			require.Equal(t, tc.domain, tc.generated)
		})
	}
}

func stringsOf[T ~string](values ...T) []string {
	return enumStrings(values)
}

func enumStrings[T ~string](values []T) []string {
	out := make([]string, len(values))
	for i, v := range values {
		out[i] = string(v)
	}
	return out
}
