"use client";

import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  BarElement,
  ArcElement,
  Title,
  Tooltip,
  Legend,
} from "chart.js";
import { Bar, Doughnut } from "react-chartjs-2";

ChartJS.register(
  CategoryScale,
  LinearScale,
  BarElement,
  ArcElement,
  Title,
  Tooltip,
  Legend
);

export interface DailyTelemetryPoint {
  date: string;
  approved_actions: number;
  blocked_actions: number;
}

export interface TelemetryStats {
  total_events: number;
  interventions_blocked: number;
  daily_timeseries: DailyTelemetryPoint[];
}

const CHART_COLORS = {
  approved: "rgba(34, 197, 94, 0.85)",
  approvedBorder: "rgb(22, 163, 74)",
  blocked: "rgba(239, 68, 68, 0.85)",
  blockedBorder: "rgb(220, 38, 38)",
  doughnutApproved: "rgb(34, 197, 94)",
  doughnutBlocked: "rgb(239, 68, 68)",
};

interface TelemetryChartProps {
  data: TelemetryStats;
  traceparent?: string | null;
}

export function TelemetryChart({ data }: TelemetryChartProps) {
  const approvedTotal = data.total_events - data.interventions_blocked;
  const blockedTotal = data.interventions_blocked;

  const barLabels = data.daily_timeseries.map((d) => d.date);
  const barData = {
    labels: barLabels,
    datasets: [
      {
        label: "Approved actions",
        data: data.daily_timeseries.map((d) => d.approved_actions),
        backgroundColor: CHART_COLORS.approved,
        borderColor: CHART_COLORS.approvedBorder,
        borderWidth: 1,
      },
      {
        label: "Blocked (kill-switch)",
        data: data.daily_timeseries.map((d) => d.blocked_actions),
        backgroundColor: CHART_COLORS.blocked,
        borderColor: CHART_COLORS.blockedBorder,
        borderWidth: 1,
      },
    ],
  };

  const doughnutData = {
    labels: ["Approved", "Kill-switch interventions"],
    datasets: [
      {
        data: [Math.max(0, approvedTotal), blockedTotal],
        backgroundColor: [CHART_COLORS.doughnutApproved, CHART_COLORS.doughnutBlocked],
        borderWidth: 1,
        borderColor: "#fff",
      },
    ],
  };

  const barOptions = {
    responsive: true,
    maintainAspectRatio: false,
    plugins: {
      legend: { position: "top" as const },
      title: { display: true, text: "Last 7 days: Approved vs blocked actions" },
    },
    scales: {
      x: { stacked: true },
      y: { stacked: true, beginAtZero: true },
    },
  };

  const doughnutOptions = {
    responsive: true,
    maintainAspectRatio: false,
    plugins: {
      legend: { position: "bottom" as const },
      title: { display: true, text: "Overall: Successful actions vs kill-switch" },
    },
  };

  return (
    <section
      className="telemetry-dashboard"
      style={{
        display: "grid",
        gridTemplateColumns: "1fr 1fr",
        gap: "1.5rem",
        alignItems: "start",
      }}
    >
      <div
        style={{
          background: "#fff",
          borderRadius: "8px",
          padding: "1rem",
          boxShadow: "0 1px 3px rgba(0,0,0,0.08)",
          minHeight: "280px",
        }}
      >
        <div style={{ height: "260px" }}>
          <Bar data={barData} options={barOptions} />
        </div>
      </div>
      <div
        style={{
          background: "#fff",
          borderRadius: "8px",
          padding: "1rem",
          boxShadow: "0 1px 3px rgba(0,0,0,0.08)",
          minHeight: "280px",
        }}
      >
        <div style={{ height: "260px" }}>
          <Doughnut data={doughnutData} options={doughnutOptions} />
        </div>
      </div>
    </section>
  );
}
