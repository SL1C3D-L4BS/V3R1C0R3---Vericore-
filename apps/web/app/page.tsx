import Link from "next/link";
import KillSwitchDemo from "./components/KillSwitchDemo";

export default function Home() {
  return (
    <div className="min-h-screen bg-[#0a0a0a] text-zinc-100">
      {/* Hero */}
      <header className="border-b border-zinc-800/80 bg-[#0a0a0a]/95 backdrop-blur">
        <div className="mx-auto max-w-5xl px-6 py-16 sm:py-24">
          <h1 className="text-4xl font-semibold tracking-tight sm:text-5xl lg:text-6xl">
            Legally Insurable AI Agents.
          </h1>
          <p className="mt-6 max-w-2xl text-lg text-zinc-400">
            The API-first compliance primitive for the EU AI Act. We enforce cryptographic
            guardrails so your AI can safely execute high-stakes actions.
          </p>
          <div className="mt-10 flex flex-wrap gap-4">
            <Link
              href="/portal"
              className="inline-flex items-center rounded-lg bg-blue-600 px-5 py-2.5 font-medium text-white hover:bg-blue-500 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 focus:ring-offset-[#0a0a0a]"
            >
              Get API Key
            </Link>
            <Link
              href="/dashboard"
              className="inline-flex items-center rounded-lg border border-zinc-600 bg-zinc-800/50 px-5 py-2.5 font-medium text-zinc-200 hover:bg-zinc-700/50 focus:outline-none focus:ring-2 focus:ring-zinc-500 focus:ring-offset-2 focus:ring-offset-[#0a0a0a]"
            >
              View Telemetry
            </Link>
          </div>
        </div>
      </header>

      <main className="mx-auto max-w-5xl px-6 py-16">
        {/* Interactive Kill-Switch Demo */}
        <section className="mb-20">
          <KillSwitchDemo />
        </section>

        {/* Features Grid */}
        <section className="grid gap-8 sm:grid-cols-3">
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
        </section>

        {/* Footer links */}
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
