package actions

import (
	"encoding/json"
	"fmt"

	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/domain"
)

// AssignCallPayload is ActionType ASSIGN_CALL: assign UnitID to CallID.
// This is ROADMAP S6's worked example — the only payload shape the planning
// docs specify explicitly.
type AssignCallPayload struct {
	CallID string `json:"callId"`
	UnitID string `json:"unitId"`
}

// RerouteUnitPayload is ActionType REROUTE_UNIT: move UnitID off whatever
// call it currently carries and onto CallID instead. The unit's previous
// call (read from Unit.CurrentCallID at execute time, not stored on the
// payload) reverts to an open, queued state. See DEC-037.
type RerouteUnitPayload struct {
	UnitID string `json:"unitId"`
	CallID string `json:"callId"`
}

// AssignBackupUnitPayload is ActionType ASSIGN_BACKUP_UNIT: assign UnitID to
// CallID in place of whatever unit (if any) is currently assigned there —
// e.g. after a breakdown DispatchEvent makes the original unit unusable. The
// replaced unit (if any) is freed back to AVAILABLE. See DEC-037.
type AssignBackupUnitPayload struct {
	CallID string `json:"callId"`
	UnitID string `json:"unitId"`
}

// ResolveEventPayload is ActionType RESOLVE_EVENT: mark DispatchEventID
// resolved. See DEC-037.
type ResolveEventPayload struct {
	DispatchEventID string `json:"dispatchEventId"`
}

func marshalPayload(v any) (json.RawMessage, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("actions: marshal payload: %w", err)
	}
	return b, nil
}

func unmarshalAssignCallPayload(raw json.RawMessage) (AssignCallPayload, error) {
	var p AssignCallPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return AssignCallPayload{}, fmt.Errorf("actions: unmarshal ASSIGN_CALL payload: %w", err)
	}
	return p, nil
}

func unmarshalRerouteUnitPayload(raw json.RawMessage) (RerouteUnitPayload, error) {
	var p RerouteUnitPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return RerouteUnitPayload{}, fmt.Errorf("actions: unmarshal REROUTE_UNIT payload: %w", err)
	}
	return p, nil
}

func unmarshalAssignBackupUnitPayload(raw json.RawMessage) (AssignBackupUnitPayload, error) {
	var p AssignBackupUnitPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return AssignBackupUnitPayload{}, fmt.Errorf("actions: unmarshal ASSIGN_BACKUP_UNIT payload: %w", err)
	}
	return p, nil
}

func unmarshalResolveEventPayload(raw json.RawMessage) (ResolveEventPayload, error) {
	var p ResolveEventPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return ResolveEventPayload{}, fmt.Errorf("actions: unmarshal RESOLVE_EVENT payload: %w", err)
	}
	return p, nil
}

// subjectRefs is what a PendingAction's payload refers to, extracted
// uniformly across ActionTypes so invalidation and supersede lookups don't
// need a type switch at every call site. Either field may be empty.
type subjectRefs struct {
	CallID  string
	UnitID  string
	EventID string
}

// payloadSubject extracts the call/unit a PendingAction's payload targets.
func payloadSubject(a domain.PendingAction) (subjectRefs, error) {
	switch a.Type {
	case domain.ActionTypeAssignCall:
		p, err := unmarshalAssignCallPayload(a.Payload)
		if err != nil {
			return subjectRefs{}, err
		}
		return subjectRefs{CallID: p.CallID, UnitID: p.UnitID}, nil
	case domain.ActionTypeRerouteUnit:
		p, err := unmarshalRerouteUnitPayload(a.Payload)
		if err != nil {
			return subjectRefs{}, err
		}
		return subjectRefs{CallID: p.CallID, UnitID: p.UnitID}, nil
	case domain.ActionTypeAssignBackupUnit:
		p, err := unmarshalAssignBackupUnitPayload(a.Payload)
		if err != nil {
			return subjectRefs{}, err
		}
		return subjectRefs{CallID: p.CallID, UnitID: p.UnitID}, nil
	case domain.ActionTypeResolveEvent:
		p, err := unmarshalResolveEventPayload(a.Payload)
		if err != nil {
			return subjectRefs{}, err
		}
		return subjectRefs{EventID: p.DispatchEventID}, nil
	default:
		return subjectRefs{}, fmt.Errorf("actions: unknown action type %q", a.Type)
	}
}
