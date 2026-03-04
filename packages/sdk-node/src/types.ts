/**
 * TypeScript types mirroring the Vericore OS Go API structs.
 */

/** Request body for POST /api/v1/agent/action (ApprovalDecision + RYOW fields). */
export interface AgentActionRequest {
  /** Unique identifier for the action being approved. */
  action_id: string;
  /** Decision from the guardrail: e.g. "APPROVED". */
  decision: string;
  /** Human-readable justification (required for approval). */
  reasoning: string;
  /** Base64-encoded FIDO2/WebAuthn signature (authenticatorData + signature). */
  fido_signature?: string;
  /** Verification queue record ID for RYOW (default "1"). */
  record_id?: string;
  /** Expected state after commit (default "committed"). */
  expected_state?: string;
}

/** Success response from the agent/action endpoint. */
export interface VericoreResponse {
  status: "ok";
}

/** FinOps account (GET /api/v1/finops/accounts). */
export interface FinopsAccount {
  id: string;
  tenant_id: string;
  name: string;
  balance_cents: number;
  currency: string;
}

/** FinOps transfer (GET /api/v1/finops/transfers). */
export interface FinopsTransfer {
  id: string;
  tenant_id: string;
  from_account: string;
  to_account: string;
  amount_cents: number;
  status: string;
  created_at: string;
  verification_queue_id?: number;
}

/** Response from GET /api/v1/finops/accounts. */
export interface FinopsAccountsResponse {
  accounts: FinopsAccount[];
}

/** Response from GET /api/v1/finops/transfers. */
export interface FinopsTransfersResponse {
  transfers: FinopsTransfer[];
}

/** Response from GET /api/v1/enclave/attest (TEE remote attestation). */
export interface EnclaveAttestationResponse {
  /** Platform Configuration Register 0 (e.g. SHA-384 hex or hardware PCR). */
  pcr0: string;
  /** Base64-encoded PQC signature over the measurement (for future client-side verification). */
  enclave_signature: string;
}
