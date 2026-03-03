package zkp

import "context"

// Prover is the Go boundary for the ZKP pipeline. It wraps the Rust zkVM guest
// (or a remote prover such as Bonsai) and returns a receipt reference and
// public hash without exposing private payloads.
type Prover interface {
	// GenerateReceipt runs the blinded ZK proof over privatePayload. It returns
	// a receiptRef (e.g. Bonsai session ID or local receipt path) and the
	// publicHash (SHA-256 of the private input, as committed in the journal).
	// The private payload must never appear in the receipt or logs.
	GenerateReceipt(ctx context.Context, privatePayload []byte) (receiptRef string, publicHash []byte, err error)
}
