module v3r1c0r3.local/api

go 1.22

require (
	github.com/go-chi/chi/v5 v5.0.12
	go.opentelemetry.io/otel v1.32.0
	v3r1c0r3.local/auth v0.0.0
	v3r1c0r3.local/db v0.0.0
	v3r1c0r3.local/guardrails v0.0.0
	v3r1c0r3.local/kms v0.0.0
	v3r1c0r3.local/mcp-flight-recorder v0.0.0
	v3r1c0r3.local/pqc v0.0.0
	v3r1c0r3.local/telemetry v0.0.0
	v3r1c0r3.local/webhooks v0.0.0
)

replace (
	v3r1c0r3.local/auth => ../../packages/auth
	v3r1c0r3.local/db => ../../packages/db
	v3r1c0r3.local/guardrails => ../../packages/guardrails
	v3r1c0r3.local/kms => ../../packages/kms
	v3r1c0r3.local/mcp-flight-recorder => ../../packages/mcp-flight-recorder
	v3r1c0r3.local/pqc => ../../packages/pqc
	v3r1c0r3.local/telemetry => ../../packages/telemetry
	v3r1c0r3.local/webhooks => ../../packages/webhooks
)

