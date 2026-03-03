"use client";

import { useCallback, useState } from "react";

const DANGEROUS_PATTERNS = [
  /drop\s+table/i,
  /delete\s+from\s+\w+/i,
  /truncate/i,
  /bypass\s+authorization/i,
  /disable\s+security/i,
  /root\s+access/i,
  /sudo\s+rm\s+-rf/i,
  /inject\s+sql/i,
  /exec\s*\(/i,
  /eval\s*\(/i,
];

function isDangerous(intent: string): boolean {
  const t = intent.trim();
  if (!t) return false;
  return DANGEROUS_PATTERNS.some((p) => p.test(t));
}

function mockMerkleHash(): string {
  const chars = "0123456789abcdef";
  let h = "";
  for (let i = 0; i < 64; i++) h += chars[Math.floor(Math.random() * 16)];
  return h;
}

type DemoState = "idle" | "approved" | "blocked";

export default function KillSwitchDemo() {
  const [intent, setIntent] = useState("");
  const [state, setState] = useState<DemoState>("idle");
  const [mockHash, setMockHash] = useState("");

  const handleSubmit = useCallback(
    (e: React.FormEvent) => {
      e.preventDefault();
      if (!intent.trim()) {
        setState("idle");
        return;
      }
      if (isDangerous(intent)) {
        setState("blocked");
      } else {
        setMockHash(mockMerkleHash());
        setState("approved");
      }
    },
    [intent]
  );

  return (
    <section className="rounded-xl border border-zinc-700/50 bg-zinc-900/50 p-6 backdrop-blur">
      <h2 className="mb-1 font-mono text-sm font-medium uppercase tracking-wider text-zinc-400">
        Agent Terminal
      </h2>
      <p className="mb-4 text-sm text-zinc-500">
        Simulated guardrail — try a safe action or a restricted one.
      </p>
      <form onSubmit={handleSubmit} className="space-y-4">
        <input
          type="text"
          value={intent}
          onChange={(e) => {
            setIntent(e.target.value);
            setState("idle");
          }}
          placeholder="Enter an agent action intent..."
          className="w-full rounded-lg border border-zinc-600 bg-zinc-800/80 px-4 py-3 font-mono text-sm text-zinc-100 placeholder-zinc-500 focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
          aria-label="Agent action intent"
        />
        <button
          type="submit"
          className="rounded-lg bg-zinc-700 px-4 py-2 font-mono text-sm font-medium text-zinc-100 hover:bg-zinc-600 focus:outline-none focus:ring-2 focus:ring-blue-500"
        >
          Submit intent
        </button>
      </form>

      {state === "approved" && (
        <div
          className="mt-4 rounded-lg border border-emerald-500/30 bg-emerald-500/10 p-4"
          role="status"
        >
          <p className="font-mono text-sm font-medium text-emerald-400">
            Action Approved. Merkle Proof Generated:
          </p>
          <code className="mt-2 block break-all font-mono text-xs text-emerald-300/90">
            {mockHash}
          </code>
        </div>
      )}

      {state === "blocked" && (
        <div
          className="mt-4 rounded-lg border border-red-500/40 bg-red-500/10 p-4"
          role="alert"
        >
          <p className="font-mono text-sm font-semibold text-red-400">
            🛑 GUARDRAIL INTERVENTION. Action Blocked. Article 14 Log Appended.
          </p>
        </div>
      )}
    </section>
  );
}
