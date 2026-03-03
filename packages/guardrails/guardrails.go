package guardrails

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// Decision values for Article 14.5 non-repudiation and kill-switch enforcement.
const (
	DecisionApproved = "APPROVED"
	DecisionRejected = "REJECTED"
)

// ApprovalDecision is the statutory schema for double-verification (EU AI Act
// Article 14.5). It MUST be fully populated and validated before any
// state-changing action; FIDOSignature provides non-repudiation on approval.
type ApprovalDecision struct {
	ActionID       string `json:"action_id"`
	Decision       string `json:"decision"`
	Reasoning      string `json:"reasoning"`
	FIDOSignature  []byte `json:"fido_signature"`
}

// Validator is the kill-switch interface: it evaluates raw payloads (e.g. LLM
// or client output) and returns a structured ApprovalDecision or a hard error,
// blocking execution when the payload is invalid or not approved.
type Validator interface {
	Evaluate(ctx context.Context, rawPayload []byte) (*ApprovalDecision, error)
}

// StrictSchemaValidator unmarshals raw JSON into ApprovalDecision and strictly
// enforces presence and validity of all fields. It acts as a deterministic
// guardrail and kill-switch: invalid or non-approved payloads cause a hard error.
type StrictSchemaValidator struct{}

// NewStrictSchemaValidator returns a Validator that enforces the full
// ApprovalDecision schema and FIDO requirement on approval.
func NewStrictSchemaValidator() *StrictSchemaValidator {
	return &StrictSchemaValidator{}
}

// Evaluate implements Validator. It returns an error (kill-switch) if:
//   - JSON is invalid or any required field is missing/zero,
//   - Decision is not exactly "APPROVED" or "REJECTED",
//   - Decision is "APPROVED" but FIDOSignature is empty.
func (v *StrictSchemaValidator) Evaluate(ctx context.Context, rawPayload []byte) (*ApprovalDecision, error) {
	if ctx != nil && ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if len(rawPayload) == 0 {
		return nil, errKillSwitch("payload empty")
	}

	var d ApprovalDecision
	if err := json.Unmarshal(rawPayload, &d); err != nil {
		return nil, fmt.Errorf("%w: invalid json: %v", errKillSwitchSentinel, err)
	}

	if d.ActionID == "" {
		return nil, errKillSwitch("action_id required")
	}
	if d.Decision == "" {
		return nil, errKillSwitch("decision required")
	}
	if d.Reasoning == "" {
		return nil, errKillSwitch("reasoning required")
	}

	switch d.Decision {
	case DecisionApproved:
		if len(d.FIDOSignature) == 0 {
			return nil, errKillSwitch("fido_signature required on approval (Article 14.5 non-repudiation)")
		}
		return &d, nil
	case DecisionRejected:
		return &d, nil
	default:
		return nil, errKillSwitch("decision must be APPROVED or REJECTED, got %q", d.Decision)
	}
}

var errKillSwitchSentinel = errors.New("guardrails: kill-switch")

// errKillSwitch returns an error that marks the request as blocked by the guardrail.
func errKillSwitch(format string, args ...interface{}) error {
	return fmt.Errorf("%w: %s", errKillSwitchSentinel, fmt.Sprintf(format, args...))
}

// IsKillSwitch returns true if err was produced by the guardrail kill-switch.
func IsKillSwitch(err error) bool {
	return errors.Is(err, errKillSwitchSentinel)
}
