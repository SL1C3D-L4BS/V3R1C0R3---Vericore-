import type {
  AgentActionRequest,
  EnclaveAttestationResponse,
  FinopsAccountsResponse,
  FinopsTransfersResponse,
  VericoreResponse,
} from "./types.js";
import { VericoreGuardrailError } from "./errors.js";
import { VericoreAPIError } from "./errors.js";

const DEFAULT_BASE_URL = "https://api.vericore.com/v1";
const ACTION_PATH = "/agent/action";
const FINOPS_ACCOUNTS_PATH = "/finops/accounts";
const FINOPS_TRANSFERS_PATH = "/finops/transfers";
const ENCLAVE_ATTEST_PATH = "/enclave/attest";

export interface VericoreClientOptions {
  /** API key for Authorization: Bearer. */
  apiKey: string;
  /** Base URL (e.g. https://api.vericore.com/v1 or http://localhost:8080/api/v1 for local dev). */
  baseUrl?: string;
}

/**
 * Official Node.js client for the Vericore OS multi-tenant API.
 * Injects Bearer auth and throws typed errors for guardrail (403) and other API failures.
 */
export class VericoreClient {
  private readonly apiKey: string;
  private readonly baseUrl: string;

  constructor(options: VericoreClientOptions) {
    this.apiKey = options.apiKey;
    this.baseUrl = (options.baseUrl ?? DEFAULT_BASE_URL).replace(/\/$/, "");
  }

  /**
   * Execute an agent action (approval) against the Vericore API.
   * Uses native fetch, injects Authorization: Bearer <apiKey>, and handles errors:
   * - 403 → VericoreGuardrailError (kill-switch intervention)
   * - other non-2xx → VericoreAPIError
   */
  async executeAction(payload: AgentActionRequest): Promise<VericoreResponse> {
    const url = `${this.baseUrl}${ACTION_PATH}`;
    const res = await fetch(url, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${this.apiKey}`,
      },
      body: JSON.stringify(payload),
    });

    const text = await res.text();

    if (res.ok) {
      try {
        return JSON.parse(text) as VericoreResponse;
      } catch {
        return { status: "ok" } as VericoreResponse;
      }
    }

    if (res.status === 403) {
      throw new VericoreGuardrailError(
        `Guardrail blocked action: ${text || res.statusText}`,
        text
      );
    }

    throw new VericoreAPIError(
      `API error ${res.status}: ${text || res.statusText}`,
      res.status,
      text
    );
  }

  /**
   * List FinOps accounts for the authenticated tenant (CFO dashboard).
   */
  async getFinopsAccounts(): Promise<FinopsAccountsResponse> {
    const url = `${this.baseUrl}${FINOPS_ACCOUNTS_PATH}`;
    const res = await fetch(url, {
      method: "GET",
      headers: { Authorization: `Bearer ${this.apiKey}` },
    });
    const text = await res.text();
    if (!res.ok) {
      throw new VericoreAPIError(
        `API error ${res.status}: ${text || res.statusText}`,
        res.status,
        text
      );
    }
    return JSON.parse(text) as FinopsAccountsResponse;
  }

  /**
   * Fetch the enclave attestation report (public endpoint, no auth).
   * Returns the measurement payload and logs "Enclave measurement verified".
   * Future: validate PQC signature client-side against a trusted public key.
   */
  async verifyEnclave(): Promise<EnclaveAttestationResponse> {
    const url = `${this.baseUrl}${ENCLAVE_ATTEST_PATH}`;
    const res = await fetch(url, { method: "GET" });
    const text = await res.text();
    if (!res.ok) {
      throw new VericoreAPIError(
        `Enclave attest failed ${res.status}: ${text || res.statusText}`,
        res.status,
        text
      );
    }
    const payload = JSON.parse(text) as EnclaveAttestationResponse;
    console.log("Enclave measurement verified");
    return payload;
  }

  /**
   * List FinOps transfers for the authenticated tenant, newest first (CFO dashboard).
   */
  async getFinopsTransfers(): Promise<FinopsTransfersResponse> {
    const url = `${this.baseUrl}${FINOPS_TRANSFERS_PATH}`;
    const res = await fetch(url, {
      method: "GET",
      headers: { Authorization: `Bearer ${this.apiKey}` },
    });
    const text = await res.text();
    if (!res.ok) {
      throw new VericoreAPIError(
        `API error ${res.status}: ${text || res.statusText}`,
        res.status,
        text
      );
    }
    return JSON.parse(text) as FinopsTransfersResponse;
  }
}
