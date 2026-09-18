package domain

import (
	"go/build"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDomain_ImportsNothingFromRepo enforces S1 criterion 7: internal/domain
// imports nothing from the rest of the repository. Checked by parsing this
// directory's imports rather than importing the package, so the assertion
// cannot be defeated by the package under test itself.
func TestDomain_ImportsNothingFromRepo(t *testing.T) {
	pkg, err := build.ImportDir(".", 0)
	require.NoError(t, err)

	const modulePrefix = "github.com/RyanV27/agentic-ems-fleet-management"
	for _, imp := range pkg.Imports {
		assert.False(t, strings.HasPrefix(imp, modulePrefix), "domain must not import %s", imp)
	}
	for _, imp := range pkg.TestImports {
		assert.False(t, strings.HasPrefix(imp, modulePrefix), "domain (including tests) must not import %s", imp)
	}
}

func TestEnums_ValidRoundTrip(t *testing.T) {
	assert.True(t, UnitStatusAvailable.Valid())
	assert.False(t, UnitStatus("BOGUS").Valid())

	assert.True(t, CapabilityALS.Valid())
	assert.True(t, CapabilityBLS.Valid())
	assert.False(t, Capability("PARAMEDIC").Valid())

	assert.True(t, CallStatusPendingApproval.Valid())
	assert.False(t, CallStatus("PENDING").Valid(), "PENDING was removed by DEC-021")

	assert.True(t, ActionTypeAssignCall.Valid())
	assert.False(t, ActionType("DELETE_CALL").Valid())

	assert.True(t, ActionStatusProposed.Valid())
	assert.False(t, ActionStatus("DRAFT").Valid())

	assert.True(t, ProposedBySystem.Valid())
	assert.False(t, ProposedBy("RULES").Valid())

	assert.True(t, ExpiredReasonTTL.Valid())
	assert.False(t, ExpiredReason("TIMED_OUT").Valid())

	assert.True(t, RejectionReasonOther.Valid())
	assert.False(t, RejectionReason("BAD").Valid())

	assert.True(t, DispatchEventTypeBreakdown.Valid())
	assert.False(t, DispatchEventType("CRASH").Valid())

	assert.True(t, DispatchEventStatusOpen.Valid())
	assert.False(t, DispatchEventStatus("PENDING").Valid())

	assert.True(t, RoutingPathDeterministic.Valid())
	assert.False(t, RoutingPath("MANUAL").Valid())

	assert.True(t, AgentRunStopReasonMaxSteps.Valid())
	assert.False(t, AgentRunStopReason("TIMEOUT").Valid())

	assert.True(t, AgentRunStatusRunning.Valid())
	assert.False(t, AgentRunStatus("QUEUED").Valid())
}
