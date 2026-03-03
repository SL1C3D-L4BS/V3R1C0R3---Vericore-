import { TelemetryChart } from "@/components/TelemetryChart";
import type { TelemetryStats } from "@/components/TelemetryChart";

const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

async function fetchTelemetryStats(): Promise<TelemetryStats> {
  const res = await fetch(`${API_URL}/api/v1/telemetry/stats`, {
    next: { revalidate: 10 },
    headers: {
      "Content-Type": "application/json",
    },
  });
  if (!res.ok) {
    throw new Error(`Telemetry API error: ${res.status}`);
  }
  return res.json();
}

export default async function DashboardPage() {
  let data: TelemetryStats;
  try {
    data = await fetchTelemetryStats();
  } catch (e) {
    return (
      <main style={{ padding: "2rem", maxWidth: "64rem" }}>
        <h1 style={{ marginBottom: "0.5rem" }}>Article 72 Compliance Dashboard</h1>
        <p style={{ color: "#b91c1c", marginTop: "1rem" }}>
          Failed to load telemetry: {(e as Error).message}. Ensure the Go API is running on {API_URL}.
        </p>
      </main>
    );
  }

  return (
    <main style={{ padding: "2rem", maxWidth: "64rem" }}>
      <h1 style={{ marginBottom: "0.5rem" }}>Article 72 Compliance Dashboard</h1>
      <p style={{ marginBottom: "1.5rem", color: "#555" }}>
        Post-market monitoring: MMR tree growth and guardrail interventions (no shadow blocks).
      </p>

      <div
        style={{
          display: "flex",
          gap: "1rem",
          flexWrap: "wrap",
          marginBottom: "1.5rem",
        }}
      >
        <div
          style={{
            padding: "1rem 1.25rem",
            background: "#f8fafc",
            borderRadius: "8px",
            border: "1px solid #e2e8f0",
            minWidth: "140px",
          }}
        >
          <div style={{ fontSize: "0.875rem", color: "#64748b" }}>Total events (MMR)</div>
          <div style={{ fontSize: "1.5rem", fontWeight: 700 }}>{data.total_events}</div>
        </div>
        <div
          style={{
            padding: "1rem 1.25rem",
            background: "#fef2f2",
            borderRadius: "8px",
            border: "1px solid #fecaca",
            minWidth: "140px",
          }}
        >
          <div style={{ fontSize: "0.875rem", color: "#64748b" }}>Interventions blocked</div>
          <div style={{ fontSize: "1.5rem", fontWeight: 700, color: "#b91c1c" }}>
            {data.interventions_blocked}
          </div>
        </div>
      </div>

      <TelemetryChart data={data} />
    </main>
  );
}
