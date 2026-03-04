"use client";

import { useCallback, useState } from "react";
import { motion, AnimatePresence } from "framer-motion";

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
    <motion.section
      className="relative overflow-hidden rounded-2xl border bg-zinc-900/40 p-6 backdrop-blur-xl"
      style={{
        boxShadow: state === "blocked" ? "0 0 0 2px #ef4444, 0 0 24px rgba(239, 68, 68, 0.25)" : "0 0 0 1px rgba(255,255,255,0.06), 0 0 40px rgba(59, 130, 246, 0.06)",
        borderColor: state === "blocked" ? "#ef4444" : "rgba(255,255,255,0.08)",
      }}
      animate={state === "blocked" ? { x: [-10, 10, -10, 10, 0] } : {}}
      transition={{ type: "keyframes", duration: 0.4 }}
    >
      {/* macOS-style terminal chrome */}
      <div className="mb-4 flex items-center gap-2">
        <span className="h-3 w-3 rounded-full bg-red-500/80" />
        <span className="h-3 w-3 rounded-full bg-amber-500/80" />
        <span className="h-3 w-3 rounded-full bg-emerald-500/80" />
        <span className="ml-2 font-mono text-xs text-zinc-500">Agent Terminal</span>
      </div>

      <h2 className="mb-1 font-mono text-sm font-medium uppercase tracking-wider text-zinc-400">
        Kill-Switch Demo
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
          className="terminal-input w-full rounded-lg border border-zinc-600/80 bg-zinc-800/80 px-4 py-3 text-sm text-zinc-100 placeholder-zinc-500 focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
          aria-label="Agent action intent"
        />
        <button
          type="submit"
          className="rounded-lg bg-zinc-700 px-4 py-2 font-mono text-sm font-medium text-zinc-100 hover:bg-zinc-600 focus:outline-none focus:ring-2 focus:ring-blue-500"
        >
          Submit intent
        </button>
      </form>

      <AnimatePresence mode="wait">
        {state === "approved" && (
          <motion.div
            key="approved"
            initial={{ opacity: 0, height: 0 }}
            animate={{ opacity: 1, height: "auto" }}
            exit={{ opacity: 0, height: 0 }}
            transition={{ duration: 0.25 }}
            className="mt-4 overflow-hidden rounded-lg border border-emerald-500/30 bg-emerald-500/10 p-4"
            role="status"
          >
            <p className="font-mono text-sm font-medium text-emerald-400">
              Action Approved. Merkle Proof Generated:
            </p>
            <code className="mt-2 block break-all font-mono text-xs text-emerald-300/90" data-crypto>
              {mockHash}
            </code>
          </motion.div>
        )}

        {state === "blocked" && (
          <motion.div
            key="blocked"
            initial={{ opacity: 0, height: 0 }}
            animate={{ opacity: 1, height: "auto" }}
            exit={{ opacity: 0, height: 0 }}
            transition={{ duration: 0.25 }}
            className="mt-4 overflow-hidden rounded-lg border-2 border-[#ef4444] bg-red-500/10 p-4"
            role="alert"
          >
            <p className="font-mono text-sm font-semibold text-red-400">
              🛑 GUARDRAIL INTERVENTION. Action Blocked. Article 14 Log Appended.
            </p>
          </motion.div>
        )}
      </AnimatePresence>
    </motion.section>
  );
}
