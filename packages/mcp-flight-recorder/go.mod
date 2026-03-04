module v3r1c0r3.local/mcp-flight-recorder

go 1.22

require (
	go.opentelemetry.io/otel v1.32.0
	v3r1c0r3.local/auth v0.0.0
	v3r1c0r3.local/pqc v0.0.0
)

require (
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	go.opentelemetry.io/otel/metric v1.32.0 // indirect
	go.opentelemetry.io/otel/trace v1.32.0 // indirect
)

replace (
	v3r1c0r3.local/auth => ../auth
	v3r1c0r3.local/pqc => ../pqc
)
