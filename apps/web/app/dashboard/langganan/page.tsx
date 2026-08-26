"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { ConnectError } from "@connectrpc/connect";
import {
  IconArrowLeft, IconBuildingBank, IconCheck, IconCopy, IconCreditCard,
  IconInfoCircle, IconLock, IconSparkles,
} from "@tabler/icons-react";
import type { GetMySubscriptionResponse, SubscriptionInvoice } from "@hajj-saas/proto-gen/hajj/v1/subscription_pb";
import { PaymentChannel } from "@hajj-saas/proto-gen/hajj/v1/subscription_pb";
import { subscriptionClient } from "@/lib/rpc";

const PLANS = [
  { id: "STARTER", name: "Starter PPIU", price: "Rp589.000", note: "Halaman travel di subdomain, portal jamaah, tour leader, dan agen." },
  { id: "GROWTH", name: "Growth Enterprise", price: "Rp789.000", note: "Semua fitur Starter, tampil di domain travel Anda sendiri." },
  { id: "PRO", name: "Custom Enterprises", price: "Rp2.489.000", note: "Server terpisah dan pengembangan fitur khusus." },
] as const;

const rupiah = (amount: bigint | number) => "Rp" + Number(amount).toLocaleString("id-ID");

function daysLeft(until?: Date) {
  if (!until) return 0;
  return Math.ceil((until.getTime() - Date.now()) / 86_400_000);
}

/**
 * Deliberately outside the dashboard shell. The shell loads operator and season
 * data on mount, and both are gated — so a locked operator would land on a page
 * that cannot render itself. This screen calls SubscriptionService only, which
 * stays reachable exactly so there is always a way back in.
 */
export default function SubscriptionPage() {
  const [subscription, setSubscription] = useState<GetMySubscriptionResponse>();
  const [invoices, setInvoices] = useState<SubscriptionInvoice[]>([]);
  const [plan, setPlan] = useState<string>("STARTER");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [copied, setCopied] = useState("");

  const load = useCallback(async () => {
    try {
      const [current, history] = await Promise.all([
        subscriptionClient.getMySubscription({}),
        subscriptionClient.listMyInvoices({ limit: 12 }),
      ]);
      setSubscription(current);
      setInvoices(history.invoices);
      if (current.plan) setPlan(current.plan);
    } catch (caught) {
      setError(ConnectError.from(caught).rawMessage);
    }
  }, []);

  useEffect(() => { void load(); }, [load]);

  const createInvoice = (channel: PaymentChannel) => async () => {
    setBusy(true);
    setError("");
    try {
      const invoice = await subscriptionClient.createInvoice({ plan, channel });
      // The gateway hands back a hosted checkout; send the operator straight to it.
      if (invoice.checkoutUrl) window.location.href = invoice.checkoutUrl;
      else await load();
    } catch (caught) {
      setError(ConnectError.from(caught).rawMessage);
    } finally {
      setBusy(false);
    }
  };

  const copy = async (value: string) => {
    try {
      await navigator.clipboard.writeText(value);
      setCopied(value);
      window.setTimeout(() => setCopied(""), 1800);
    } catch {
      // Clipboard can be denied; the value stays selectable on screen.
    }
  };

  const pending = subscription?.pendingInvoice;
  const until = subscription?.accessUntil?.toDate();
  const remaining = daysLeft(until);
  const locked = subscription ? !subscription.active : false;
  const trialing = subscription?.status === "TRIALING";

  return <main style={page}>
    <div style={wrap}>
      <Link href="/dashboard" style={backLink}><IconArrowLeft size={16} /> Kembali ke dashboard</Link>

      <header style={{ marginTop: 18 }}>
        <p style={eyebrow}>LANGGANAN</p>
        <h1 style={title}>Langganan TawafiqHub</h1>
      </header>

      {subscription && <section style={locked ? statusCardLocked : trialing ? statusCardTrial : statusCardActive}>
        <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
          {locked ? <IconLock size={20} /> : trialing ? <IconSparkles size={20} /> : <IconCheck size={20} />}
          <strong style={{ fontSize: 15 }}>
            {locked ? "Dashboard terkunci" : trialing ? `Masa uji coba — sisa ${remaining} hari` : `Aktif — sisa ${remaining} hari`}
          </strong>
        </div>
        <p style={statusNote}>
          {locked
            ? "Halaman travel, pendaftaran jamaah, dan portal jamaah serta tour leader Anda tetap berjalan normal. Yang terkunci hanya dashboard operator."
            : `Paket ${subscription.plan} berlaku sampai ${until?.toLocaleDateString("id-ID", { day: "numeric", month: "long", year: "numeric" })}.`}
        </p>
      </section>}

      {error && <p style={errorBox}>{error}</p>}

      {pending
        ? <section style={card}>
            <p style={eyebrow}>TAGIHAN BERJALAN</p>
            <h2 style={sectionTitle}>Selesaikan pembayaran</h2>
            {pending.channel === PaymentChannel.BANK_TRANSFER ? <>
              <p style={muted}>
                Transfer <strong>tepat sampai digit terakhir</strong>. Tiga angka terakhir adalah kode unik yang
                membuat pembayaran Anda dikenali otomatis — nominal yang dibulatkan tidak bisa dicocokkan.
              </p>
              <div style={amountBox}>
                <span style={amountLabel}>Nominal transfer</span>
                <span style={amountValue}>{rupiah(pending.amountIdr)}</span>
                <button type="button" onClick={() => void copy(String(pending.amountIdr))} style={copyButton} aria-label="Salin nominal">
                  {copied === String(pending.amountIdr) ? <IconCheck size={16} /> : <IconCopy size={16} />}
                </button>
              </div>
              <div style={bankGrid}>
                <span style={bankLabel}>Bank</span><span style={bankValue}>{subscription?.transferBankName || "—"}</span>
                <span style={bankLabel}>Nomor rekening</span><span style={bankValue}>{subscription?.transferAccountNumber || "—"}</span>
                <span style={bankLabel}>Atas nama</span><span style={bankValue}>{subscription?.transferAccountHolder || "—"}</span>
              </div>
              <p style={hint}><IconInfoCircle size={15} /> Berlaku sampai {pending.dueAt?.toDate().toLocaleDateString("id-ID", { day: "numeric", month: "long" })}. Setelah lewat, kode unik dilepas dan Anda perlu membuat tagihan baru.</p>
            </> : <>
              <p style={muted}>Tagihan pembayaran otomatis sudah dibuat. Lanjutkan ke halaman pembayaran untuk menyelesaikannya.</p>
              {pending.checkoutUrl && <a href={pending.checkoutUrl} style={primaryButton}>Lanjutkan pembayaran</a>}
            </>}
          </section>
        : <section style={card}>
            <p style={eyebrow}>PILIH PAKET</p>
            <h2 style={sectionTitle}>{locked ? "Aktifkan kembali langganan" : "Perpanjang atau ubah paket"}</h2>
            <div style={planGrid}>
              {PLANS.map((option) => <button
                key={option.id}
                type="button"
                onClick={() => setPlan(option.id)}
                style={plan === option.id ? planCardActive : planCard}
              >
                <span style={planName}>{option.name}</span>
                <span style={planPrice}>{option.price}<small style={planUnit}> / bulan</small></span>
                <span style={planNote}>{option.note}</span>
              </button>)}
            </div>

            <p style={{ ...muted, marginTop: 4 }}>Pilih cara pembayaran:</p>
            <div style={payRow}>
              {/* A method that cannot complete is hidden rather than shown and
                  rejected on click: an operator picking "transfer bank" before
                  an account is configured would get a unique amount with
                  nowhere to send it. */}
              {subscription?.bankTransferAvailable && <button type="button" onClick={() => void createInvoice(PaymentChannel.BANK_TRANSFER)()} disabled={busy} style={payButton}>
                <IconBuildingBank size={18} /> Transfer bank
              </button>}
              {subscription?.gatewayAvailable && <button type="button" onClick={() => void createInvoice(PaymentChannel.GATEWAY)()} disabled={busy} style={payButtonPrimary}>
                <IconCreditCard size={18} /> QRIS / Kartu
              </button>}
            </div>
            {subscription && !subscription.bankTransferAvailable && !subscription.gatewayAvailable &&
              <p style={hint}><IconInfoCircle size={15} /> Pembayaran mandiri belum tersedia. Hubungi tim TawafiqHub untuk mengaktifkan langganan Anda.</p>}
          </section>}

      {invoices.length > 0 && <section style={card}>
        <p style={eyebrow}>RIWAYAT</p>
        <h2 style={sectionTitle}>Tagihan sebelumnya</h2>
        <ul style={list}>
          {invoices.map((invoice) => <li key={invoice.id} style={invoiceRow}>
            <div style={{ display: "grid", gap: 2 }}>
              <strong style={{ fontSize: 13 }}>{rupiah(invoice.amountIdr)}</strong>
              <span style={invoiceMeta}>
                {invoice.plan} · {invoice.channel === PaymentChannel.BANK_TRANSFER ? "Transfer bank" : "Otomatis"} ·{" "}
                {invoice.periodStart?.toDate().toLocaleDateString("id-ID", { day: "numeric", month: "short", year: "numeric" })}
              </span>
            </div>
            <span style={invoice.status === "PAID" ? badgePaid : invoice.status === "PENDING" ? badgePending : badgeStale}>
              {invoice.status === "PAID" ? "Lunas" : invoice.status === "PENDING" ? "Menunggu" : "Kedaluwarsa"}
            </span>
          </li>)}
        </ul>
      </section>}
    </div>
  </main>;
}

const page: React.CSSProperties = { minHeight: "100dvh", background: "var(--color-cream-100)", padding: "28px 16px 64px" };
const wrap: React.CSSProperties = { maxWidth: 720, margin: "0 auto", display: "grid", gap: 18 };
const backLink: React.CSSProperties = { display: "inline-flex", alignItems: "center", gap: 6, color: "var(--color-warm-500)", fontSize: 13, fontWeight: 600, textDecoration: "none" };
const eyebrow: React.CSSProperties = { margin: 0, color: "var(--color-warm-400)", fontSize: 11, fontWeight: 800, letterSpacing: "0.14em" };
const title: React.CSSProperties = { margin: "6px 0 0", fontFamily: "'Playfair Display',serif", fontSize: 30, fontWeight: 700, color: "var(--color-emerald-900)" };
const sectionTitle: React.CSSProperties = { margin: "6px 0 10px", fontSize: 18, fontWeight: 800, color: "var(--color-warm-900)" };
const card: React.CSSProperties = { background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 14, padding: 18, display: "grid", gap: 8 };
const statusCardBase: React.CSSProperties = { borderRadius: 14, padding: 16, display: "grid", gap: 6 };
const statusCardActive: React.CSSProperties = { ...statusCardBase, background: "var(--color-emerald-50)", border: "1px solid var(--color-emerald-200)", color: "var(--color-emerald-900)" };
const statusCardTrial: React.CSSProperties = { ...statusCardBase, background: "#fffbeb", border: "1px solid #fcd34d", color: "#b45309" };
const statusCardLocked: React.CSSProperties = { ...statusCardBase, background: "var(--color-danger-100)", border: "1px solid var(--color-danger-600)", color: "var(--color-danger-600)" };
const statusNote: React.CSSProperties = { margin: 0, fontSize: 13, lineHeight: 1.6, opacity: 0.9 };
const muted: React.CSSProperties = { margin: 0, color: "var(--color-warm-500)", fontSize: 13, lineHeight: 1.65 };
const hint: React.CSSProperties = { display: "flex", alignItems: "center", gap: 6, margin: 0, color: "var(--color-warm-400)", fontSize: 12 };
const amountBox: React.CSSProperties = { display: "flex", alignItems: "center", gap: 12, background: "var(--color-emerald-50)", border: "1px solid var(--color-emerald-200)", borderRadius: 12, padding: "14px 16px", marginTop: 4 };
const amountLabel: React.CSSProperties = { color: "var(--color-emerald-900)", fontSize: 12, fontWeight: 700 };
const amountValue: React.CSSProperties = { flex: 1, color: "var(--color-emerald-900)", fontSize: 24, fontWeight: 850, letterSpacing: "-0.01em" };
const copyButton: React.CSSProperties = { display: "grid", width: 36, height: 36, placeItems: "center", border: "1px solid var(--color-emerald-200)", borderRadius: 9, background: "#fff", color: "var(--color-emerald-900)", cursor: "pointer" };
const bankGrid: React.CSSProperties = { display: "grid", gap: "6px 14px", gridTemplateColumns: "auto 1fr", marginTop: 6 };
const bankLabel: React.CSSProperties = { color: "var(--color-warm-400)", fontSize: 12 };
const bankValue: React.CSSProperties = { color: "var(--color-warm-900)", fontSize: 13, fontWeight: 700 };
const planGrid: React.CSSProperties = { display: "grid", gap: 10, marginTop: 4 };
const planCard: React.CSSProperties = { display: "grid", gap: 3, border: "1px solid var(--color-cream-400)", borderRadius: 12, background: "#fff", padding: 14, textAlign: "left", cursor: "pointer" };
const planCardActive: React.CSSProperties = { ...planCard, border: "1px solid var(--color-emerald-800)", background: "var(--color-emerald-50)" };
const planName: React.CSSProperties = { fontSize: 14, fontWeight: 800, color: "var(--color-warm-900)" };
const planPrice: React.CSSProperties = { fontSize: 18, fontWeight: 850, color: "var(--color-emerald-900)" };
const planUnit: React.CSSProperties = { fontSize: 12, fontWeight: 600, color: "var(--color-warm-400)" };
const planNote: React.CSSProperties = { fontSize: 12, color: "var(--color-warm-500)", lineHeight: 1.5 };
const payRow: React.CSSProperties = { display: "flex", gap: 10, flexWrap: "wrap", marginTop: 4 };
const payButton: React.CSSProperties = { display: "inline-flex", flex: "1 1 180px", minHeight: 46, alignItems: "center", justifyContent: "center", gap: 8, border: "1px solid var(--color-emerald-800)", borderRadius: 10, background: "transparent", color: "var(--color-emerald-900)", fontSize: 14, fontWeight: 800, cursor: "pointer" };
const payButtonPrimary: React.CSSProperties = { ...payButton, border: "1px solid var(--color-emerald-900)", background: "var(--color-emerald-900)", color: "#fff" };
const primaryButton: React.CSSProperties = { display: "inline-flex", minHeight: 46, alignItems: "center", justifyContent: "center", borderRadius: 10, background: "var(--color-emerald-900)", color: "#fff", fontSize: 14, fontWeight: 800, textDecoration: "none", marginTop: 6 };
const list: React.CSSProperties = { display: "grid", gap: 8, margin: 0, padding: 0, listStyle: "none" };
const invoiceRow: React.CSSProperties = { display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12, border: "1px solid var(--color-cream-400)", borderRadius: 10, padding: "10px 12px" };
const invoiceMeta: React.CSSProperties = { color: "var(--color-warm-400)", fontSize: 11.5 };
const badgeBase: React.CSSProperties = { padding: "4px 10px", borderRadius: 99, fontSize: 11, fontWeight: 800 };
const badgePaid: React.CSSProperties = { ...badgeBase, color: "var(--color-emerald-900)", background: "var(--color-emerald-50)" };
const badgePending: React.CSSProperties = { ...badgeBase, color: "#b45309", background: "#fef3c7" };
const badgeStale: React.CSSProperties = { ...badgeBase, color: "var(--color-warm-400)", background: "var(--color-cream-200)" };
const errorBox: React.CSSProperties = { margin: 0, border: "1px solid var(--color-danger-600)", borderRadius: 10, background: "#fff", padding: "10px 12px", color: "var(--color-danger-600)", fontSize: 13 };
