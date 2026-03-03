package flightrecorder

import "time"

// AuditEvent is the core Article 12 schema for the MCP flight recorder.
//
// Cryptographic blinding & GDPR boundary:
// - All plaintext PII and other sensitive user data MUST be encrypted or
//   irreversibly redacted BEFORE constructing an AuditEvent.
// - KEK/DEK envelope encryption occurs upstream of this type. By the time data
//   crosses this boundary, the DEK-encrypted ciphertext and any key metadata
//   are already encapsulated inside params_json/state_before/state_after.
// - This struct MUST NEVER carry raw DEKs, KEKs, or plaintext prompts,
//   tool parameters, or user identifiers.
type AuditEvent struct {
	// ID is a stable identifier for this audit event (for example, a UUID).
	ID string `json:"id"`

	// Timestamp is the wall-clock time at which the audited operation occurred.
	Timestamp time.Time `json:"timestamp"`

	// TenantID identifies the tenant (B2B API key scope). Set from request context by AuditMiddleware.
	TenantID string `json:"tenant_id"`

	// AgentID identifies the in-process agent or subsystem responsible for
	// the operation (e.g. "cursor", "guardrails", or a concrete agent name).
	AgentID string `json:"agent_id"`

	// Intent is a coarse-grained, human-readable description of what the
	// operation was trying to accomplish (e.g. "write_file", "run_shell").
	Intent string `json:"intent"`

	// ToolName is the logical name of the tool or capability being invoked.
	ToolName string `json:"tool_name"`

	// ParamsJSON carries the encrypted JSON payload (ciphertext) for the
	// tool invocation parameters. This should be the output of DEK-based
	// envelope encryption; when serialized, it is typically base64-encoded.
	ParamsJSON []byte `json:"params_json"`

	// EnvelopeHash is a tamper-evident hash (e.g. SHA-256) computed over the
	// encrypted envelope and any associated key identifiers or headers. It
	// allows integrity checking without exposing underlying plaintext.
	EnvelopeHash string `json:"envelope_hash"`

	// StateBefore is an encrypted or structurally redacted snapshot of the
	// relevant application state prior to the operation. Only ciphertext or
	// non-PII metadata may be stored here.
	StateBefore []byte `json:"state_before"`

	// StateAfter is an encrypted or structurally redacted snapshot of the
	// relevant application state after the operation.
	StateAfter []byte `json:"state_after"`

	// ReceiptRef is an opaque reference to a future ZKP receipt or external
	// attestation object that can be resolved by the ZKP pipeline.
	ReceiptRef string `json:"receipt_ref"`
}

// MMRLeaf models a single leaf node in the Merkle Mountain Range backing the
// flight recorder's append-only log.
type MMRLeaf struct {
	// ID is a stable identifier for this leaf within the MMR (for example,
	// a UUID or content-addressed digest).
	ID string `json:"id"`

	// Index is the monotonic position of this leaf within the logical
	// append-only sequence.
	Index uint64 `json:"index"`

	// TenantID identifies the tenant for multi-tenant isolation and querying.
	TenantID string `json:"tenant_id"`

	// EventID links this leaf back to the originating AuditEvent.ID.
	EventID string `json:"event_id"`

	// Hash is the leaf hash bytes as produced by the configured MMRHasher.
	Hash []byte `json:"hash"`
}

// MMRInclusionProof encodes the data required to prove that a particular
// MMRLeaf is included in a specific MMR root.
type MMRInclusionProof struct {
	// Leaf is the concrete leaf being proven.
	Leaf MMRLeaf `json:"leaf"`

	// SiblingPath is the sequence of sibling node hashes needed to recompute
	// the MMR root from the leaf. The exact path semantics are defined by the
 	// MMR implementation.
	SiblingPath [][]byte `json:"sibling_path,omitempty"`

	// Peaks are the set of peak hashes for the MMR forest at the time this
	// proof was generated.
	Peaks [][]byte `json:"peaks,omitempty"`

	// Root is the MMR root hash that this proof attests membership against.
	Root []byte `json:"root"`
}

