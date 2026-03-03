"use client";

import { useCallback, useEffect, useState } from "react";

const API_BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
const DEFAULT_API_KEY = process.env.NEXT_PUBLIC_PORTAL_DEMO_KEY || "sk_test_123";

interface PortalKey {
  key_prefix: string;
  created_at: string;
}

export default function PortalPage() {
  const [apiKey, setApiKey] = useState(DEFAULT_API_KEY);
  const [keys, setKeys] = useState<PortalKey[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [generatedKey, setGeneratedKey] = useState<string | null>(null);
  const [generateLoading, setGenerateLoading] = useState(false);
  const [copyFeedback, setCopyFeedback] = useState(false);

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
    <main style={{ padding: "2rem", maxWidth: "56rem" }}>
      <h1 style={{ marginBottom: "0.5rem" }}>Developer Portal</h1>
      <p style={{ marginBottom: "1.5rem", color: "#666" }}>
        Manage API keys for the Vericore OS multi-tenant API. Keys are hashed; the raw key is shown only once when created.
      </p>

      <section style={{ marginBottom: "1.5rem" }}>
        <label style={{ display: "block", marginBottom: "0.5rem", fontWeight: 600 }}>
          API Key (used to authenticate portal requests)
        </label>
        <input
          type="password"
          value={apiKey}
          onChange={(e) => setApiKey(e.target.value)}
          placeholder="sk_test_..."
          style={{ width: "100%", maxWidth: "24rem", padding: "0.5rem", fontSize: "1rem" }}
          aria-label="API Key"
        />
      </section>

      <section style={{ marginBottom: "1.5rem" }}>
        <button
          type="button"
          onClick={handleGenerate}
          disabled={generateLoading}
          style={{
            padding: "0.6rem 1.2rem",
            fontSize: "1rem",
            fontWeight: 600,
            cursor: generateLoading ? "not-allowed" : "pointer",
            opacity: generateLoading ? 0.6 : 1,
          }}
        >
          {generateLoading ? "Generating…" : "Generate New API Key"}
        </button>
      </section>

      {generatedKey && (
        <section style={{ marginBottom: "1.5rem", padding: "1rem", background: "#f0f7ff", borderRadius: "8px", border: "1px solid #b3d4fc" }}>
          <p style={{ marginBottom: "0.5rem", fontWeight: 600 }}>Your new API key (copy now — it won’t be shown again):</p>
          <code style={{ display: "block", wordBreak: "break-all", marginBottom: "0.75rem", fontSize: "0.9rem" }}>
            {generatedKey}
          </code>
          <button
            type="button"
            onClick={copyToClipboard}
            style={{ padding: "0.4rem 0.8rem", fontSize: "0.9rem", cursor: "pointer" }}
          >
            {copyFeedback ? "Copied!" : "Copy to Clipboard"}
          </button>
        </section>
      )}

      <section>
        <h2 style={{ marginBottom: "0.75rem", fontSize: "1rem" }}>Your active keys</h2>
        {loading ? (
          <p style={{ color: "#666" }}>Loading…</p>
        ) : error ? (
          <p style={{ color: "#c00" }}>{error}</p>
        ) : keys.length === 0 ? (
          <p style={{ color: "#666" }}>No keys yet. Generate one above.</p>
        ) : (
          <ul style={{ listStyle: "none" }}>
            {keys.map((k, i) => (
              <li
                key={`${k.key_prefix}-${k.created_at}-${i}`}
                style={{
                  padding: "0.75rem 1rem",
                  border: "1px solid #ddd",
                  borderRadius: "6px",
                  marginBottom: "0.5rem",
                }}
              >
                <strong>{k.key_prefix}</strong>
                <span style={{ marginLeft: "0.5rem", color: "#666" }}>created {k.created_at}</span>
              </li>
            ))}
          </ul>
        )}
      </section>
    </main>
  );
}
