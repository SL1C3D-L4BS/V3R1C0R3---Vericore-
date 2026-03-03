"use client";

import { useEffect } from "react";
import {
  WebTracerProvider,
  SimpleSpanProcessor,
} from "@opentelemetry/sdk-trace-web";
import { ZoneContextManager } from "@opentelemetry/context-zone";
import { registerInstrumentations } from "@opentelemetry/instrumentation";
import { FetchInstrumentation } from "@opentelemetry/instrumentation-fetch";

let registered = false;

/** No-op exporter: we only need trace context propagation, not span export. */
const noopExporter = {
  export(_spans: unknown, resultCallback: (result: { code: number }) => void) {
    resultCallback({ code: 0 });
  },
};

/**
 * Registers @opentelemetry/instrumentation-fetch so that outbound fetch()
 * requests include the traceparent header (W3C Trace Context) for the Go API.
 */
function registerFetchInstrumentation() {
  if (registered || typeof window === "undefined") return;
  registered = true;

  const provider = new WebTracerProvider({
    spanProcessors: [new SimpleSpanProcessor(noopExporter as never)],
  });
  provider.register({
    contextManager: new ZoneContextManager(),
  });

  registerInstrumentations({
    instrumentations: [new FetchInstrumentation()],
  });
}

export function TelemetryProvider({ children }: { children: React.ReactNode }) {
  useEffect(() => {
    registerFetchInstrumentation();
  }, []);
  return <>{children}</>;
}
