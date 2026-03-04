"use client";

import { useCallback, useEffect, useState } from "react";
import { motion } from "framer-motion";
import { Fingerprint, CheckCircle } from "lucide-react";
import {
  VericoreClient,
  VericoreGuardrailError,
  type FinopsAccount,
  type FinopsTransfer,
} from "@vericore/node-sdk";
import { clsx } from "clsx";
import { twMerge } from "tailwind-merge";

const API_BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
const DEFAULT_API_KEY = process.env.NEXT_PUBLIC_PORTAL_DEMO_KEY || "sk_test_123";

const vericoreClient = new VericoreClient({
  apiKey: DEFAULT_API_KEY,
  baseUrl: `${API_BASE}/api/v1`,
});

function cn(...inputs: Parameters<typeof clsx>) {
  return twMerge(clsx(inputs));
}

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
      <main className="mx-auto min-h-screen max-w-6xl bg-[var(--bg)] p-8 text-[var(--text)]">
        <h1 className="mb-2 text-2xl font-semibold">CFO FinOps Dashboard</h1>
        <p className="text-[var(--text-muted)]">Loading treasury data…</p>
      </main>
    );
  }

  if (error) {
    return (
      <main className="mx-auto min-h-screen max-w-6xl bg-[var(--bg)] p-8 text-[var(--text)]">
        <h1 className="mb-2 text-2xl font-semibold">CFO FinOps Dashboard</h1>
        <p className="mt-4 text-[var(--danger)]">{error}</p>
        <p className="mt-2 text-sm text-[var(--text-muted)]">
          Ensure the API is running at {API_BASE} and the API key is valid.
        </p>
      </main>
    );
  }

  return (
    <main className="mx-auto min-h-screen max-w-6xl bg-[var(--bg)] p-8 text-[var(--text)]">
      <h1 className="mb-1 text-2xl font-semibold">CFO FinOps Dashboard</h1>
      <p className="mb-8 text-[var(--text-muted)]">
        Autonomous Corporate Treasury — view balances and cryptographically approve high-stakes transfers.
      </p>

      {/* Treasury Balances: motion cards with hover lift + border highlight */}
      <section className="mb-10">
        <h2 className="mb-4 text-lg font-medium text-[var(--text-muted)]">Treasury Balances</h2>
        <div
          className="grid gap-4"
          style={{ gridTemplateColumns: "repeat(auto-fill, minmax(220px, 1fr))" }}
        >
          {accounts.length === 0 ? (
            <p className="col-span-full text-[var(--text-muted)]">No accounts. Seed finops_accounts for this tenant.</p>
          ) : (
            accounts.map((acct) => (
              <motion.div
                key={acct.id}
                className="rounded-lg border p-4"
                style={{
                  background: "var(--bg-elevated)",
                  borderColor: "var(--border)",
                }}
                whileHover={{ y: -4, boxShadow: "0 8px 24px rgba(0,0,0,0.25)", borderColor: "rgba(59, 130, 246, 0.4)" }}
                transition={{ type: "spring", stiffness: 300, damping: 20 }}
              >
                <div className="truncate font-mono text-sm text-[var(--text-muted)]" title={acct.id}>
                  {acct.id}
                </div>
                <div className="mt-1 font-medium">{acct.name}</div>
                <div className="mt-2 font-mono text-lg" style={{ color: "var(--success)" }}>
                  {formatCurrency(acct.balance_cents, acct.currency)}
                </div>
              </motion.div>
            ))
          )}
        </div>
      </section>

      {/* AI Transfer Ledger */}
      <section>
        <h2 className="mb-4 text-lg font-medium text-[var(--text-muted)]">AI Transfer Ledger</h2>
        <div
          className="overflow-hidden rounded-lg border"
          style={{ borderColor: "var(--border)", background: "var(--bg-elevated)" }}
        >
          {transfers.length === 0 ? (
            <div className="p-6 text-[var(--text-muted)]">No transfers yet.</div>
          ) : (
            <table className="w-full border-collapse text-left">
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
                    <td className="max-w-[120px] truncate p-3" title={t.id}>
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
                          className="inline-flex items-center gap-1.5 rounded px-2 py-0.5 text-xs font-medium"
                          style={{ background: "rgba(34, 197, 94, 0.2)", color: "var(--success)" }}
                        >
                          <CheckCircle className="h-3.5 w-3.5" />
                          AI Auto-Executed
                        </span>
                      ) : t.status === "pending_approval" ? (
                        <span
                          className="inline-flex items-center px-2 py-0.5 text-xs font-medium"
                          style={{
                            background: "rgba(245, 158, 11, 0.2)",
                            color: "#f59e0b",
                          }}
                        >
                          Requires FIDO2 Hardware Signature
                        </span>
                      ) : (
                        <span className="inline-flex items-center px-2 py-0.5 text-xs" style={{ color: "var(--text-muted)" }}>
                          {t.status}
                        </span>
                      )}
                    </td>
                    <td className="p-3 text-[var(--text-muted)]">
                      {t.created_at ? new Date(t.created_at).toLocaleString() : "—"}
                    </td>
                    <td className="p-3">
                      {t.status === "pending_approval" && t.verification_queue_id != null ? (
                        <motion.button
                          type="button"
                          disabled={approvingId === t.id}
                          onClick={() => handleApproveTransfer(t)}
                          className={cn(
                            "inline-flex items-center gap-2 rounded px-3 py-1.5 text-xs font-medium transition",
                            approvingId === t.id && "opacity-80",
                            approvingId !== t.id && "animate-pulse"
                          )}
                          style={{
                            background: "var(--accent)",
                            color: "#fff",
                            boxShadow: approvingId === t.id ? "none" : "0 0 20px rgba(245, 158, 11, 0.35)",
                          }}
                          animate={
                            t.status === "pending_approval" && approvingId !== t.id
                              ? { boxShadow: ["0 0 20px rgba(245, 158, 11, 0.35)", "0 0 28px rgba(245, 158, 11, 0.5)", "0 0 20px rgba(245, 158, 11, 0.35)"] }
                              : {}
                          }
                          transition={{ repeat: Infinity, duration: 2 }}
                        >
                          {approvingId === t.id ? (
                            <>
                              <Fingerprint className="h-4 w-4 animate-spin" />
                              Awaiting Hardware FIDO2 Signature…
                            </>
                          ) : (
                            <>
                              <Fingerprint className="h-4 w-4" />
                              Approve Transfer
                            </>
                          )}
                        </motion.button>
                      ) : null}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
        {approveStatus.type === "ok" && (
          <motion.p
            initial={{ opacity: 0, y: 4 }}
            animate={{ opacity: 1, y: 0 }}
            className="mt-4 flex items-center gap-2 text-sm"
            style={{ color: "var(--success)" }}
          >
            <CheckCircle className="h-4 w-4" />
            {approveStatus.message}
          </motion.p>
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
