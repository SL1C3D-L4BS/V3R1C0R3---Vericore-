"use client";

import Link from "next/link";
import { motion } from "framer-motion";
import KillSwitchDemo from "./components/KillSwitchDemo";

const fadeUpTransition = { duration: 0.4 };

export default function Home() {
  return (
    <div className="min-h-screen bg-[#0a0a0a] text-zinc-100 overflow-hidden">
      {/* Glowing data floor background */}
      <div
        className="fixed inset-0 pointer-events-none z-0"
        aria-hidden
      >
        <div
          className="absolute inset-0 opacity-[0.03]"
          style={{
            backgroundImage: `
              linear-gradient(rgba(59, 130, 246, 0.15) 1px, transparent 1px),
              linear-gradient(90deg, rgba(59, 130, 246, 0.15) 1px, transparent 1px)
            `,
            backgroundSize: "48px 48px",
            animation: "dataFloorTranslate 24s linear infinite",
            willChange: "transform",
          }}
        />
        <div
          className="absolute inset-0"
          style={{
            maskImage: "radial-gradient(ellipse 80% 60% at 50% 0%, black 20%, transparent 70%)",
            WebkitMaskImage: "radial-gradient(ellipse 80% 60% at 50% 0%, black 20%, transparent 70%)",
          }}
        />
      </div>

      <header className="relative z-10 border-b border-zinc-800/80 bg-[#0a0a0a]/95 backdrop-blur">
        <div className="mx-auto max-w-5xl px-6 py-16 sm:py-24">
          <motion.div className="space-y-6">
            <motion.h1
              className="text-4xl font-semibold tracking-tight sm:text-5xl lg:text-6xl"
              initial={{ opacity: 0, y: 24 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ ...fadeUpTransition, delay: 0 }}
            >
              Legally Insurable AI Agents.
            </motion.h1>
            <motion.p
              className="max-w-2xl text-lg text-zinc-400"
              initial={{ opacity: 0, y: 24 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ ...fadeUpTransition, delay: 0.08 }}
            >
              The API-first compliance primitive for the EU AI Act. We enforce cryptographic
              guardrails so your AI can safely execute high-stakes actions.
            </motion.p>
            <motion.div
              className="flex flex-wrap gap-4 pt-4"
              initial={{ opacity: 0, y: 24 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ ...fadeUpTransition, delay: 0.16 }}
            >
              <Link
                href="/portal"
                className="inline-flex items-center rounded-lg bg-blue-600 px-5 py-2.5 font-medium text-white hover:bg-blue-500 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 focus:ring-offset-[#0a0a0a] transition-colors"
              >
                Get API Key
              </Link>
              <Link
                href="/dashboard"
                className="inline-flex items-center rounded-lg border border-zinc-600 bg-zinc-800/50 px-5 py-2.5 font-medium text-zinc-200 hover:bg-zinc-700/50 focus:outline-none focus:ring-2 focus:ring-zinc-500 focus:ring-offset-2 focus:ring-offset-[#0a0a0a] transition-colors"
              >
                View Telemetry
              </Link>
            </motion.div>
          </motion.div>
        </div>
      </header>

      <main className="relative z-10 mx-auto max-w-5xl px-6 py-16">
        <section className="mb-20">
          <KillSwitchDemo />
        </section>

        <motion.section
          className="grid gap-8 sm:grid-cols-3"
          initial={{ opacity: 0 }}
          whileInView={{ opacity: 1 }}
          viewport={{ once: true, margin: "-60px" }}
          transition={{ duration: 0.5 }}
        >
          <div className="rounded-xl border border-zinc-700/50 bg-zinc-900/30 p-6">
            <h3 className="font-mono text-sm font-semibold uppercase tracking-wider text-blue-400">
              Cryptographic Flight Recorder
            </h3>
            <p className="mt-3 text-sm text-zinc-400">
              Stateless Merkle Mountain Ranges mathematically prove agent intent.
            </p>
          </div>
          <div className="rounded-xl border border-zinc-700/50 bg-zinc-900/30 p-6">
            <h3 className="font-mono text-sm font-semibold uppercase tracking-wider text-blue-400">
              Hardware FIDO2 Approvals
            </h3>
            <p className="mt-3 text-sm text-zinc-400">
              Human-in-the-loop WebAuthn for high-stakes financial routing.
            </p>
          </div>
          <div className="rounded-xl border border-zinc-700/50 bg-zinc-900/30 p-6">
            <h3 className="font-mono text-sm font-semibold uppercase tracking-wider text-blue-400">
              Zero-Knowledge Privacy
            </h3>
            <p className="mt-3 text-sm text-zinc-400">
              GDPR Right to Erasure compliant via ZKP blinding.
            </p>
          </div>
        </motion.section>

        <footer className="mt-20 border-t border-zinc-800/80 pt-10">
          <p className="text-sm text-zinc-500">
            <Link href="/verification" className="text-zinc-400 hover:text-zinc-300">
              Verification queue
            </Link>
            {" · "}
            <Link href="/portal" className="text-zinc-400 hover:text-zinc-300">
              Developer Portal
            </Link>
          </p>
        </footer>
      </main>

    </div>
  );
}
