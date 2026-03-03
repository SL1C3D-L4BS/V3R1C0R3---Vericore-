/**
 * Next.js instrumentation (runs once at server startup).
 * Trace continuity: @opentelemetry/instrumentation-fetch is registered
 * in the browser by TelemetryProvider so that outbound fetch requests
 * to the Go API (e.g. POST /api/v1/agent/action) include the traceparent
 * header (W3C Trace Context).
 */
export async function register() {
  // Server-side: optional future undici instrumentation for server fetch.
  // Client-side: TelemetryProvider registers FetchInstrumentation on mount.
}
