/**
 * Typed errors for Vericore API failures.
 */

/** Thrown when the guardrail kill-switch blocks the action (HTTP 403). */
export class VericoreGuardrailError extends Error {
  readonly statusCode = 403;
  readonly body: string;

  constructor(message: string, body: string) {
    super(message);
    this.name = "VericoreGuardrailError";
    this.body = body;
    Object.setPrototypeOf(this, VericoreGuardrailError.prototype);
  }
}

/** Thrown on non-2xx responses other than 403 (e.g. 401, 500). */
export class VericoreAPIError extends Error {
  readonly statusCode: number;
  readonly body: string;

  constructor(message: string, statusCode: number, body: string) {
    super(message);
    this.name = "VericoreAPIError";
    this.statusCode = statusCode;
    this.body = body;
    Object.setPrototypeOf(this, VericoreAPIError.prototype);
  }
}
