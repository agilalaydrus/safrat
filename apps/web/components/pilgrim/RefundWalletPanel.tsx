"use client";

import Link from "next/link";
import { useCallback, useEffect, useMemo, useState } from "react";
import { IconArrowDown, IconClockHour4, IconShieldLock, IconWallet } from "@tabler/icons-react";
import {
  RefundPayoutMethod,
  RefundPayoutRequest,
  RefundPayoutStatus,
  RefundWallet,
} from "@hajj-saas/proto-gen/hajj/v1/refund_payout_pb";
import { refundPayoutClient } from "@/lib/rpc";

const money = (value: bigint) => new Intl.NumberFormat("id-ID", { style: "currency", currency: "IDR", maximumFractionDigits: 0 }).format(Number(value));
const date = (value?: Date) => value?.toLocaleString("id-ID", { day: "2-digit", month: "short", year: "numeric", hour: "2-digit", minute: "2-digit" }) ?? "";

const payoutStatus: Record<number, { label: string; color: string; background: string }> = {
  [RefundPayoutStatus.REQUESTED]: { label: "Menunggu diproses", color: "#b45309", background: "var(--color-gold-50)" },
  [RefundPayoutStatus.PROCESSING]: { label: "Sedang diproses", color: "#1d4ed8", background: "#eff6ff" },
  [RefundPayoutStatus.PAID]: { label: "Sudah dibayar", color: "var(--color-emerald-900)", background: "var(--color-emerald-50)" },
  [RefundPayoutStatus.FAILED]: { label: "Tidak berhasil", color: "var(--color-danger-600)", background: "#fff1f2" },
  [RefundPayoutStatus.REVERSED]: { label: "Dikembalikan gateway", color: "#7c3aed", background: "#f5f3ff" },
};

const methodLabel: Record<number, string> = {
  [RefundPayoutMethod.BANK_TRANSFER]: "Transfer bank",
  [RefundPayoutMethod.EWALLET]: "E-wallet",
  [RefundPayoutMethod.CASH]: "Tunai",
};

const bankChannels = [
  ["CENAIDJA", "BCA"], ["BRINIDJA", "BRI"], ["BNINIDJA", "BNI"],
  ["BMRIIDJA", "Mandiri"], ["BNIAIDJA", "CIMB Niaga"], ["BBBAIDJA", "Permata"],
] as const;
const walletChannels = [["ID_DANA", "DANA"], ["ID_OVO", "OVO"], ["ID_GOPAY", "GoPay"], ["ID_SHOPEEPAY", "ShopeePay"], ["ID_LINKAJA", "LinkAja"]] as const;

export default function RefundWalletPanel({ appAccessCode = "", fallbackBalance = 0n, agent = false }: { appAccessCode?: string; fallbackBalance?: bigint; agent?: boolean }) {
  const [wallet, setWallet] = useState<RefundWallet>();
  const [amount, setAmount] = useState("");
  const [method, setMethod] = useState(RefundPayoutMethod.BANK_TRANSFER);
  const [note, setNote] = useState("");
  const [destinationChannel, setDestinationChannel] = useState<string>(bankChannels[0][0]);
  const [accountHolder, setAccountHolder] = useState("");
  const [accountNumber, setAccountNumber] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [idempotencyKey, setIdempotencyKey] = useState(() => crypto.randomUUID());

  const load = useCallback(async () => {
    try {
      setWallet(agent ? await refundPayoutClient.getMyAgentRefundWallet({}) : await refundPayoutClient.getMyRefundWallet({ appAccessCode }));
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "Saldo refund tidak dapat dimuat.");
    }
  }, [agent, appAccessCode]);

  useEffect(() => { void load(); }, [load]);

  const available = wallet?.availableIdr ?? fallbackBalance;
  const requestedAmount = useMemo(() => {
    const parsed = Number(amount.replace(/\D/g, ""));
    return Number.isSafeInteger(parsed) && parsed > 0 ? BigInt(parsed) : 0n;
  }, [amount]);

  async function submit() {
    if (requestedAmount <= 0n || requestedAmount > available) {
      setError("Masukkan jumlah yang tidak melebihi saldo tersedia.");
      return;
    }
    setSubmitting(true);
    setError("");
    setNotice("");
    try {
      const destination = method === RefundPayoutMethod.CASH ? {} : {
        destinationChannel, destinationAccountHolder: accountHolder.trim(), destinationAccountNumber: accountNumber.trim(),
      };
      if (method !== RefundPayoutMethod.CASH && (!accountHolder.trim() || !/^\d{7,34}$/.test(accountNumber.trim()))) {
        setError("Isi nama pemilik dan nomor rekening/e-wallet yang valid.");
        return;
      }
      if (agent) {
        await refundPayoutClient.requestAgentRefundPayout({ amountIdr: requestedAmount, method, note: note.trim(), idempotencyKey, ...destination });
      } else {
        await refundPayoutClient.requestRefundPayout({ appAccessCode, amountIdr: requestedAmount, method, note: note.trim(), idempotencyKey, ...destination });
      }
      setNotice(method === RefundPayoutMethod.CASH ? "Permintaan pencairan tunai tercatat." : "Permintaan tercatat dan akan dikirim otomatis melalui Xendit.");
      setAmount("");
      setNote("");
      setAccountNumber("");
      setIdempotencyKey(crypto.randomUUID());
      await load();
    } catch (caught) {
      const message = caught instanceof Error ? caught.message : "Permintaan pencairan gagal dikirim.";
      setError(message);
    } finally {
      setSubmitting(false);
    }
  }

  if (!wallet && fallbackBalance <= 0n && !error) return null;

  return (
    <section style={panel} aria-labelledby="refund-wallet-heading">
      <div style={panelHeader}>
        <div style={walletIcon}><IconWallet size={22} /></div>
        <div>
          <p style={sectionEyebrow}>SALDO REFUND</p>
          <h2 id="refund-wallet-heading" style={{ margin: 0, fontSize: 21 }}>Dana yang dikembalikan</h2>
        </div>
      </div>

      <div style={stats}>
        <Stat label="Saldo ledger" value={money(wallet?.balanceIdr ?? fallbackBalance)} />
        <Stat label="Sedang diproses" value={money(wallet?.reservedIdr ?? 0n)} />
        <Stat label="Bisa dicairkan" value={money(available)} accent />
      </div>

      {available > 0n && (
        <div style={requestBox}>
          <div style={{ display: "flex", gap: 8, alignItems: "center" }}>
            <IconShieldLock size={17} color="var(--color-emerald-800)" />
            <strong>Ajukan pencairan</strong>
          </div>
          <p style={help}>Akun tertaut dan 2FA aktif wajib digunakan. Transfer bank/e-wallet dikirim otomatis melalui Xendit; nomor tujuan disimpan terenkripsi.</p>
          <label style={field}>Jumlah
            <input value={amount} onChange={(event) => setAmount(event.target.value.replace(/\D/g, ""))} inputMode="numeric" placeholder={String(available)} style={input} />
          </label>
          <div style={{ display: "flex", flexWrap: "wrap", gap: 8 }}>
            <button type="button" onClick={() => setAmount(String(available))} style={smallButton}>Cairkan semua</button>
          </div>
          <label style={field}>Metode yang diinginkan
            <select value={method} onChange={(event) => { const next = Number(event.target.value) as RefundPayoutMethod; setMethod(next); setDestinationChannel(next === RefundPayoutMethod.EWALLET ? walletChannels[0][0] : bankChannels[0][0]); }} style={input}>
              <option value={RefundPayoutMethod.BANK_TRANSFER}>Transfer bank</option>
              <option value={RefundPayoutMethod.EWALLET}>E-wallet</option>
              <option value={RefundPayoutMethod.CASH}>Tunai</option>
            </select>
          </label>
          {method !== RefundPayoutMethod.CASH && <>
            <label style={field}>{method === RefundPayoutMethod.EWALLET ? "Penyedia e-wallet" : "Bank tujuan"}
              <select value={destinationChannel} onChange={(event) => setDestinationChannel(event.target.value)} style={input}>
                {(method === RefundPayoutMethod.EWALLET ? walletChannels : bankChannels).map(([value, label]) => <option key={value} value={value}>{label}</option>)}
              </select>
            </label>
            <label style={field}>Nama pemilik akun
              <input value={accountHolder} onChange={(event) => setAccountHolder(event.target.value)} maxLength={255} autoComplete="name" style={input} />
            </label>
            <label style={field}>{method === RefundPayoutMethod.EWALLET ? "Nomor ponsel e-wallet" : "Nomor rekening"}
              <input value={accountNumber} onChange={(event) => setAccountNumber(event.target.value.replace(/\D/g, ""))} maxLength={34} inputMode="numeric" autoComplete="off" style={input} />
            </label>
          </>}
          <label style={field}>Catatan untuk travel
            <textarea value={note} onChange={(event) => setNote(event.target.value)} maxLength={500} rows={3} placeholder="Contoh: hubungi lewat WhatsApp sebelum transfer" style={{ ...input, paddingTop: 12 }} />
          </label>
          <button type="button" disabled={submitting} onClick={() => void submit()} style={primaryButton}>
            <IconArrowDown size={17} />{submitting ? "Mengirim..." : "Ajukan Pencairan"}
          </button>
          <small style={{ color: "var(--color-warm-500)" }}>Belum mengaktifkan 2FA? <Link href="/keamanan" style={{ color: "var(--color-emerald-800)", fontWeight: 700 }}>Buka Keamanan Akun</Link></small>
        </div>
      )}

      {error && <p role="alert" style={errorStyle}>{error}</p>}
      {notice && <p role="status" style={noticeStyle}>{notice}</p>}

      {!!wallet?.payoutRequests.length && (
        <div style={{ display: "grid", gap: 9 }}>
          <h3 style={subheading}>Riwayat pencairan</h3>
          {wallet.payoutRequests.map((request) => <PayoutRow key={request.id} request={request} />)}
        </div>
      )}

      {!!wallet?.entries.length && (
        <details>
          <summary style={summaryStyle}>Lihat seluruh pergerakan saldo ({wallet.entries.length})</summary>
          <div style={{ display: "grid", gap: 8, marginTop: 10 }}>
            {wallet.entries.map((entry) => (
              <div key={entry.id} style={ledgerRow}>
                <div><strong>{entry.kind === "REFUND" ? "Refund masuk" : entry.kind === "WITHDRAWAL" ? "Pencairan" : entry.kind}</strong><small style={muted}>{date(entry.createdAt?.toDate())}{entry.note ? ` · ${entry.note}` : ""}</small></div>
                <strong style={{ color: entry.amountIdr >= 0n ? "var(--color-emerald-800)" : "var(--color-danger-600)" }}>{entry.amountIdr >= 0n ? "+" : "−"}{money(entry.amountIdr >= 0n ? entry.amountIdr : -entry.amountIdr)}</strong>
              </div>
            ))}
          </div>
        </details>
      )}
    </section>
  );
}

function Stat({ label, value, accent = false }: { label: string; value: string; accent?: boolean }) {
  return <div style={{ ...stat, background: accent ? "var(--color-emerald-50)" : "#fff" }}><small style={muted}>{label}</small><strong style={{ fontSize: 17, color: accent ? "var(--color-emerald-900)" : "var(--color-warm-800)" }}>{value}</strong></div>;
}

function PayoutRow({ request }: { request: RefundPayoutRequest }) {
  const status = payoutStatus[request.status] ?? { label: "Status tidak dikenal", color: "var(--color-warm-500)", background: "var(--color-cream-200)" };
  return <div style={payoutRow}>
    <div><strong>{money(request.amountIdr)}</strong><small style={muted}>{methodLabel[request.method] ?? "Metode lain"} · {date(request.createdAt?.toDate())}</small></div>
    <span style={{ ...statusBadge, color: status.color, background: status.background }}><IconClockHour4 size={13} />{status.label}</span>
    {(request.destinationAccountLast4 || request.paymentReference || request.resolutionNote) && <small style={{ ...muted, gridColumn: "1 / -1" }}>{request.destinationAccountLast4 ? `${request.destinationChannel} · •••• ${request.destinationAccountLast4}` : ""}{request.paymentReference ? ` · Referensi: ${request.paymentReference}` : request.resolutionNote ? ` · ${request.resolutionNote}` : ""}</small>}
  </div>;
}

const panel: React.CSSProperties = { display: "grid", gap: 16, margin: "0 0 24px", padding: 18, border: "1px solid var(--color-cream-400)", borderRadius: 14, background: "#fffdf8" };
const panelHeader: React.CSSProperties = { display: "flex", gap: 12, alignItems: "center" };
const walletIcon: React.CSSProperties = { width: 44, height: 44, display: "grid", placeItems: "center", color: "var(--color-emerald-900)", background: "var(--color-emerald-50)", borderRadius: 12 };
const sectionEyebrow: React.CSSProperties = { color: "var(--color-gold-800)", fontSize: 10, fontWeight: 800, letterSpacing: ".09em", margin: "0 0 3px" };
const stats: React.CSSProperties = { display: "grid", gridTemplateColumns: "repeat(auto-fit,minmax(130px,1fr))", gap: 8 };
const stat: React.CSSProperties = { display: "grid", gap: 4, padding: 12, border: "1px solid var(--color-cream-300)", borderRadius: 9 };
const requestBox: React.CSSProperties = { display: "grid", gap: 11, padding: 14, background: "#fff", border: "1px solid var(--color-cream-300)", borderRadius: 10 };
const help: React.CSSProperties = { margin: 0, color: "var(--color-warm-500)", fontSize: 12, lineHeight: 1.55 };
const field: React.CSSProperties = { display: "grid", gap: 6, color: "var(--color-warm-700)", fontSize: 13, fontWeight: 700 };
const input: React.CSSProperties = { width: "100%", minHeight: 44, border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 11px", background: "#fff", color: "var(--color-warm-800)", font: "inherit", fontWeight: 400 };
const smallButton: React.CSSProperties = { minHeight: 34, border: "1px solid var(--color-cream-400)", borderRadius: 7, padding: "0 10px", background: "#fff", color: "var(--color-emerald-900)", fontWeight: 700 };
const primaryButton: React.CSSProperties = { minHeight: 46, border: 0, borderRadius: 8, display: "inline-flex", justifyContent: "center", alignItems: "center", gap: 7, background: "var(--color-emerald-900)", color: "#fff", fontWeight: 800 };
const errorStyle: React.CSSProperties = { margin: 0, padding: 11, borderRadius: 8, background: "#fff1f2", color: "var(--color-danger-600)", fontSize: 13 };
const noticeStyle: React.CSSProperties = { margin: 0, padding: 11, borderRadius: 8, background: "var(--color-emerald-50)", color: "var(--color-emerald-900)", fontSize: 13 };
const subheading: React.CSSProperties = { margin: "2px 0 0", fontSize: 14 };
const payoutRow: React.CSSProperties = { display: "grid", gridTemplateColumns: "1fr auto", gap: 8, alignItems: "center", padding: 12, border: "1px solid var(--color-cream-300)", borderRadius: 9, background: "#fff" };
const ledgerRow: React.CSSProperties = { display: "flex", justifyContent: "space-between", alignItems: "flex-start", gap: 12, padding: 10, borderBottom: "1px solid var(--color-cream-300)" };
const muted: React.CSSProperties = { display: "block", color: "var(--color-warm-500)", fontSize: 11, fontWeight: 400, marginTop: 3 };
const statusBadge: React.CSSProperties = { display: "inline-flex", alignItems: "center", gap: 4, padding: "5px 8px", borderRadius: 99, fontSize: 10, fontWeight: 800, whiteSpace: "nowrap" };
const summaryStyle: React.CSSProperties = { cursor: "pointer", color: "var(--color-emerald-900)", fontSize: 13, fontWeight: 700 };
