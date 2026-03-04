"use client";

import { useCallback, useEffect, useState } from "react";
import { motion, AnimatePresence } from "framer-motion";

const API_BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
const DEFAULT_API_KEY = process.env.NEXT_PUBLIC_PORTAL_DEMO_KEY || "sk_test_123";

const CIPHER_CHARS = "abcdefghijklmnopqrstuvwxyz0123456789_";

interface PortalKey {
  key_prefix: string;
  created_at: string;
}

function useCipherReveal(finalKey: string | null, durationMs = 800) {
  const [display, setDisplay] = useState("");
  const [revealed, setRevealed] = useState(false);

  useEffect(() => {
    if (!finalKey) {
      setDisplay("");
      setRevealed(false);
      return;
    }
    setRevealed(false);
    const len = finalKey.length;
    let start = Date.now();
    const interval = 40;
    const scrambleFrames = Math.min(12, Math.floor(durationMs * 0.5 / interval));

    let frame = 0;
    const scrambleId = setInterval(() => {
      frame++;
      let s = "";
      for (let i = 0; i < len; i++) {
        s += CIPHER_CHARS[Math.floor(Math.random() * CIPHER_CHARS.length)];
      }
      setDisplay(s);
      if (frame >= scrambleFrames) {
        clearInterval(scrambleId);
        setDisplay(finalKey);
        setRevealed(true);
      }
    }, interval);

    return () => clearInterval(scrambleId);
  }, [finalKey, durationMs]);

  return { display, revealed };
}

export default function PortalPage() {
  const [apiKey, setApiKey] = useState(DEFAULT_API_KEY);
  const [keys, setKeys] = useState<PortalKey[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [generatedKey, setGeneratedKey] = useState<string | null>(null);
  const [generateLoading, setGenerateLoading] = useState(false);
  const [copyFeedback, setCopyFeedback] = useState(false);

  const { display: revealedKeyDisplay, revealed } = useCipherReveal(generatedKey);

  const fetchKeys = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await fetch(`${API_BASE}/api/v1/portal/keys`, {
        headers: { Authorization: `Bearer ${apiKey}` },
      });
      if (!res.ok) {
        const text = await res.text();
        throw new Error(`${res.status}: ${text}`);
      }
      const data = (await res.json()) as { keys: PortalKey[] };
      setKeys(data.keys || []);
    } catch (e) {
      setError((e as Error).message);
      setKeys([]);
    } finally {
      setLoading(false);
    }
  }, [apiKey]);

  useEffect(() => {
    fetchKeys();
  }, [fetchKeys]);

  const handleGenerate = useCallback(async () => {
    setGenerateLoading(true);
    setError(null);
    setGeneratedKey(null);
    try {
      const res = await fetch(`${API_BASE}/api/v1/portal/keys`, {
        method: "POST",
        headers: { Authorization: `Bearer ${apiKey}` },
      });
      if (!res.ok) {
        const text = await res.text();
        throw new Error(`${res.status}: ${text}`);
      }
      const data = (await res.json()) as { key: string };
      setGeneratedKey(data.key);
      fetchKeys();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setGenerateLoading(false);
    }
  }, [apiKey, fetchKeys]);

  const copyToClipboard = useCallback(() => {
    if (!generatedKey) return;
    void navigator.clipboard.writeText(generatedKey).then(() => {
      setCopyFeedback(true);
      setTimeout(() => setCopyFeedback(false), 2000);
    });
  }, [generatedKey]);

  return (
    <main className="mx-auto max-w-3xl bg-[var(--bg)] p-8 text-[var(--text)]">
      <h1 className="mb-2 text-2xl font-semibold">Developer Portal</h1>
      <p className="mb-6 text-[var(--text-muted)]">
        Manage API keys for the Vericore OS multi-tenant API. Keys are hashed; the raw key is shown only once when created.
      </p>

      <section className="mb-6">
        <label className="mb-2 block font-medium text-[var(--text-muted)]">
          API Key (used to authenticate portal requests)
        </label>
        <input
          type="password"
          value={apiKey}
          onChange={(e) => setApiKey(e.target.value)}
          placeholder="sk_test_..."
          className="terminal-input w-full max-w-md rounded-lg border border-zinc-600 bg-zinc-800/80 px-4 py-2.5 text-sm text-zinc-100 placeholder-zinc-500 focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
          aria-label="API Key"
        />
      </section>

      <section className="mb-6">
        <button
          type="button"
          onClick={handleGenerate}
          disabled={generateLoading}
          className="rounded-lg bg-blue-600 px-4 py-2.5 font-medium text-white transition hover:bg-blue-500 disabled:cursor-not-allowed disabled:opacity-60"
        >
          {generateLoading ? "Generating…" : "Generate New API Key"}
        </button>
      </section>

      <AnimatePresence mode="wait">
        {generatedKey && (
          <motion.section
            key="generated"
            initial={{ opacity: 0, y: 8 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0 }}
            className="mb-6 rounded-xl border border-blue-500/30 bg-blue-500/10 p-5 backdrop-blur"
          >
            <p className="mb-3 font-medium text-blue-200">
              Your new API key (copy now — it won’t be shown again):
            </p>
            <motion.code
              className="block break-all font-mono text-sm text-blue-100"
              data-crypto
              initial={{ opacity: 0.8 }}
              animate={{ opacity: revealed ? 1 : 0.9 }}
              transition={{ duration: 0.2 }}
            >
              {revealedKeyDisplay}
            </motion.code>
            <motion.button
              type="button"
              onClick={copyToClipboard}
              className="mt-3 rounded-lg bg-blue-600/80 px-3 py-1.5 text-sm font-medium text-white hover:bg-blue-500"
            >
              {copyFeedback ? "Copied!" : "Copy to Clipboard"}
            </motion.button>
          </motion.section>
        )}
      </AnimatePresence>

      <section>
        <h2 className="mb-3 text-base font-medium text-[var(--text-muted)]">Your active keys</h2>
        {loading ? (
          <p className="text-[var(--text-muted)]">Loading…</p>
        ) : error ? (
          <p className="text-[var(--danger)]">{error}</p>
        ) : keys.length === 0 ? (
          <p className="text-[var(--text-muted)]">No keys yet. Generate one above.</p>
        ) : (
          <ul className="list-none space-y-2">
            {keys.map((k, i) => (
              <li
                key={`${k.key_prefix}-${k.created_at}-${i}`}
                className="rounded-lg border border-zinc-700/50 bg-zinc-800/30 px-4 py-3"
              >
                <span className="font-mono font-medium text-zinc-200" data-crypto>{k.key_prefix}</span>
                <span className="ml-2 text-sm text-[var(--text-muted)]">created {k.created_at}</span>
              </li>
            ))}
          </ul>
        )}
      </section>
    </main>
  );
}
