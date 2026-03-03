"use client";

import { useCallback, useEffect, useState } from "react";
import {
  VericoreClient,
  VericoreGuardrailError,
  type FinopsAccount,
  type FinopsTransfer,
} from "@vericore/node-sdk";

const API_BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
const DEFAULT_API_KEY = process.env.NEXT_PUBLIC_PORTAL_DEMO_KEY || "sk_test_123";

const vericoreClient = new VericoreClient({
  apiKey: DEFAULT_API_KEY,
  baseUrl: `${API_BASE}/api/v1`,
});

function formatCurrency(cents: number, currency = "USD"): string {
  return new Intl.NumberFormat("en-US", {
    style: "currency",
    currency,
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(cents / 100);
}

function bufferToBase64(buffer: ArrayBuffer): string {
  const bytes = new Uint8Array(buffer);
  let binary = "";
  for (let i = 0; i < bytes.length; i++) {
    binary += String.fromCharCode(bytes[i]);
  }
  return btoa(binary);
}

function encodeFidoSignature(authenticatorData: ArrayBuffer, signature: ArrayBuffer): string {
  const a = new Uint8Array(authenticatorData);
  const s = new Uint8Array(signature);
  const combined = new Uint8Array(a.length + s.length);
  combined.set(a);
  combined.set(s, a.length);
  return bufferToBase64(combined.buffer);
}

async function requestWebAuthn(): Promise<{ authenticatorData: ArrayBuffer; signature: ArrayBuffer } | null> {
  const challenge = new Uint8Array(32);
  crypto.getRandomValues(challenge);
  try {
    const cred = await navigator.credentials.get({
      publicKey: {
        challenge,
        rpId: typeof window !== "undefined" ? window.location.hostname || "localhost" : "localhost",
        timeout: 60000,
        userVerification: "required",
      },
    });
    if (!cred || !("response" in cred)) return null;
    const res = (cred as Credential & { response: AuthenticatorAssertionResponse }).response;
    return {
      authenticatorData: res.authenticatorData,
      signature: res.signature,
    };
  } catch {
    return null;
  }
}

export default function CFOFinopsDashboardPage() {
  const [accounts, setAccounts] = useState<FinopsAccount[]>([]);
  const [transfers, setTransfers] = useState<FinopsTransfer[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [approvingId, setApprovingId] = useState<string | null>(null);
  const [approveStatus, setApproveStatus] = useState<{
    type: "idle" | "ok" | "error";
    message?: string;
  }>({ type: "idle" });

  const fetchData = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [accountsRes, transfersRes] = await Promise.all([
        vericoreClient.getFinopsAccounts(),
        vericoreClient.getFinopsTransfers(),
      ]);
      setAccounts(accountsRes.accounts);
      setTransfers(transfersRes.transfers);
    } catch (e) {
      setError((e as Error).message);
      setAccounts([]);
      setTransfers([]);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  const handleApproveTransfer = useCallback(
    async (transfer: FinopsTransfer) => {
      const recordId = transfer.verification_queue_id;
      if (recordId == null) return;
      setApprovingId(transfer.id);
      setApproveStatus({ type: "idle" });
      const assertion = await requestWebAuthn();
      if (!assertion) {
        setApproveStatus({ type: "error", message: "WebAuthn failed or cancelled." });
        setApprovingId(null);
        return;
      }
      const fidoSignature = encodeFidoSignature(
        assertion.authenticatorData,
        assertion.signature
      );
      try {
        await vericoreClient.executeAction({
          action_id: transfer.id,
          decision: "APPROVED",
          reasoning: "CFO FIDO2 hardware approval for high-stakes transfer.",
          fido_signature: fidoSignature,
          record_id: String(recordId),
          expected_state: "CFO_APPROVAL",
        });
        setApproveStatus({ type: "ok", message: "Transfer approved and settled." });
        await fetchData();
      } catch (e) {
        if (e instanceof VericoreGuardrailError) {
          setApproveStatus({ type: "error", message: `Guardrail blocked: ${e.message}` });
        } else {
          setApproveStatus({ type: "error", message: (e as Error).message });
        }
      } finally {
        setApprovingId(null);
      }
    },
    [fetchData]
  );

  if (loading) {
    return (
      <main className="min-h-screen bg-[var(--bg)] text-[var(--text)] p-8 max-w-6xl mx-auto">
        <h1 className="text-2xl font-semibold mb-2">CFO FinOps Dashboard</h1>
        <p className="text-[var(--text-muted)]">Loading treasury data…</p>
      </main>
    );
  }

  if (error) {
    return (
      <main className="min-h-screen bg-[var(--bg)] text-[var(--text)] p-8 max-w-6xl mx-auto">
        <h1 className="text-2xl font-semibold mb-2">CFO FinOps Dashboard</h1>
        <p className="text-[var(--danger)] mt-4">{error}</p>
        <p className="text-[var(--text-muted)] text-sm mt-2">
          Ensure the API is running at {API_BASE} and the API key is valid.
        </p>
      </main>
    );
  }

  return (
    <main className="min-h-screen bg-[var(--bg)] text-[var(--text)] p-8 max-w-6xl mx-auto">
      <h1 className="text-2xl font-semibold mb-1">CFO FinOps Dashboard</h1>
      <p className="text-[var(--text-muted)] mb-8">
        Autonomous Corporate Treasury — view balances and cryptographically approve high-stakes transfers.
      </p>

      {/* Section 1: Treasury Balances */}
      <section className="mb-10">
        <h2 className="text-lg font-medium text-[var(--text-muted)] mb-4">Treasury Balances</h2>
        <div
          className="grid gap-4"
          style={{ gridTemplateColumns: "repeat(auto-fill, minmax(220px, 1fr))" }}
        >
          {accounts.length === 0 ? (
            <p className="text-[var(--text-muted)] col-span-full">No accounts. Seed finops_accounts for this tenant.</p>
          ) : (
            accounts.map((acct) => (
              <div
                key={acct.id}
                className="rounded-lg border p-4"
                style={{
                  background: "var(--bg-elevated)",
                  borderColor: "var(--border)",
                }}
              >
                <div
                  className="font-mono text-sm text-[var(--text-muted)] truncate"
                  title={acct.id}
                >
                  {acct.id}
                </div>
                <div className="font-medium mt-1">{acct.name}</div>
                <div className="font-mono text-lg mt-2" style={{ color: "var(--success)" }}>
                  {formatCurrency(acct.balance_cents, acct.currency)}
                </div>
              </div>
            ))
          )}
        </div>
      </section>

      {/* Section 2: AI Transfer Ledger */}
      <section>
        <h2 className="text-lg font-medium text-[var(--text-muted)] mb-4">AI Transfer Ledger</h2>
        <div
          className="rounded-lg border overflow-hidden"
          style={{ borderColor: "var(--border)", background: "var(--bg-elevated)" }}
        >
          {transfers.length === 0 ? (
            <div className="p-6 text-[var(--text-muted)]">No transfers yet.</div>
          ) : (
            <table className="w-full text-left border-collapse">
              <thead>
                <tr style={{ borderBottom: "1px solid var(--border)" }}>
                  <th className="p-3 font-medium text-[var(--text-muted)]">Transfer</th>
                  <th className="p-3 font-medium text-[var(--text-muted)]">From → To</th>
                  <th className="p-3 font-medium text-[var(--text-muted)]">Amount</th>
                  <th className="p-3 font-medium text-[var(--text-muted)]">Status</th>
                  <th className="p-3 font-medium text-[var(--text-muted)]">Date</th>
                  <th className="p-3 font-medium text-[var(--text-muted)]">Action</th>
                </tr>
              </thead>
              <tbody>
                {transfers.map((t) => (
                  <tr
                    key={t.id}
                    className="font-mono text-sm"
                    style={{ borderBottom: "1px solid var(--border)" }}
                  >
                    <td className="p-3 truncate max-w-[120px]" title={t.id}>
                      {t.id}
                    </td>
                    <td className="p-3">
                      <span className="text-[var(--text-muted)]">{t.from_account}</span>
                      <span className="mx-1 text-[var(--text-muted)]">→</span>
                      <span className="text-[var(--text-muted)]">{t.to_account}</span>
                    </td>
                    <td className="p-3">{formatCurrency(t.amount_cents)}</td>
                    <td className="p-3">
                      {t.status === "executed" ? (
                        <span
                          className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium"
                          style={{ background: "rgba(34, 197, 94, 0.2)", color: "var(--success)" }}
                        >
                          AI Auto-Executed
                        </span>
                      ) : t.status === "pending_approval" ? (
                        <span
                          className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium"
                          style={{
                            background: "rgba(245, 158, 11, 0.2)",
                            color: "#f59e0b",
                          }}
                        >
                          Requires FIDO2 Hardware Signature
                        </span>
                      ) : (
                        <span
                          className="inline-flex items-center px-2 py-0.5 rounded text-xs"
                          style={{ color: "var(--text-muted)" }}
                        >
                          {t.status}
                        </span>
                      )}
                    </td>
                    <td className="p-3 text-[var(--text-muted)]">
                      {t.created_at ? new Date(t.created_at).toLocaleString() : "—"}
                    </td>
                    <td className="p-3">
                      {t.status === "pending_approval" && t.verification_queue_id != null ? (
                        <button
                          type="button"
                          disabled={approvingId === t.id}
                          onClick={() => handleApproveTransfer(t)}
                          className="px-3 py-1.5 rounded text-xs font-medium transition opacity"
                          style={{
                            background: "var(--accent)",
                            color: "#fff",
                            opacity: approvingId === t.id ? 0.6 : 1,
                          }}
                        >
                          {approvingId === t.id ? "Requesting WebAuthn…" : "Approve Transfer"}
                        </button>
                      ) : null}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
        {approveStatus.type === "ok" && (
          <p className="mt-4 text-sm" style={{ color: "var(--success)" }}>
            {approveStatus.message}
          </p>
        )}
        {approveStatus.type === "error" && (
          <p className="mt-4 text-sm" style={{ color: "var(--danger)" }}>
            {approveStatus.message}
          </p>
        )}
      </section>
    </main>
  );
}
