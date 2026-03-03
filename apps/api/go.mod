module v3r1c0r3.local/api

go 1.22

require (
	github.com/go-chi/chi/v5 v5.0.12
	modernc.org/sqlite v1.29.1
	v3r1c0r3.local/db v0.0.0
	v3r1c0r3.local/guardrails v0.0.0
	v3r1c0r3.local/mcp-flight-recorder v0.0.0
)

replace (
	v3r1c0r3.local/db => ../../packages/db
	v3r1c0r3.local/guardrails => ../../packages/guardrails
	v3r1c0r3.local/mcp-flight-recorder => ../../packages/mcp-flight-recorder
)

