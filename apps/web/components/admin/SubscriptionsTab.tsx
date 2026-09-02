"use client";

import { Fragment, useCallback, useEffect, useMemo, useState } from "react";
import { IconAlertTriangle, IconBan } from "@tabler/icons-react";
import type { PlatformOperator, SubscriptionInvoiceRow } from "@hajj-saas/proto-gen/hajj/v1/platform_pb";
import { platformClient } from "@/lib/rpc";

const rupiah = (n: bigint | number) =>
  new Intl.NumberFormat("id-ID", { style: "currency", currency: "IDR", maximumFractionDigits: 0 }).format(Number(n));

const tanggal = (d: Date) => d.toLocaleDateString("id-ID", { day: "2-digit", month: "short", year: "numeric" });

const STAGE_LABEL: Record<string, string> = {
  H1: "Pengingat 1", H7: "Pengingat 2", H14: "Peringatan akhir", SUSPEND: "Ditangguhkan",
};

function daysOverdue(accessUntil?: Date): number {
  if (!accessUntil) return 0;
  const ms = Date.now() - accessUntil.getTime();
  return ms <= 0 ? 0 : Math.floor(ms / 86_400_000);
}

export default function SubscriptionsTab() {
  const [operators, setOperators] = useState<PlatformOperator[]>([]);
  const [invoices, setInvoices] = useState<SubscriptionInvoiceRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [notice, setNotice] = useState("");
  const [voiding, setVoiding] = useState("");
  const [reason, setReason] = useState("");

  const refresh = useCallback(() => {
    setLoading(true);
    Promise.all([platformClient.listOperators({}), platformClient.listSubscriptionInvoices({ limit: 100 })])
      .then(([operatorResponse, invoiceResponse]) => {
        setOperators(operatorResponse.operators);
        setInvoices(invoiceResponse.invoices);
      })
      .catch(() => setNotice("Gagal memuat data langganan."))
      .finally(() => setLoading(false));
  }, []);
  useEffect(refresh, [refresh]);

  const lapsed = useMemo(
    () => operators
      .filter((o) => o.accessUntil && o.accessUntil.toDate() < new Date())
      .sort((a, b) => (a.accessUntil?.toDate().getTime() ?? 0) - (b.accessUntil?.toDate().getTime() ?? 0)),
    [operators],
  );
  const suspended = useMemo(() => operators.filter((o) => o.suspendedAt).length, [operators]);
  const outstanding = useMemo(
    () => lapsed.reduce((sum, o) => sum + Number(o.outstandingIdr), 0),
    [lapsed],
  );

  async function voidInvoice(invoiceId: string) {
    try {
      await platformClient.voidSubscriptionInvoice({ invoiceId, reason: reason.trim() });
      setVoiding("");
      setReason("");
      setNotice("Invoice dibatalkan. Barisnya tetap tersimpan sebagai catatan.");
      refresh();
    } catch {
      setNotice("Gagal membatalkan. Invoice yang sudah dibayar tidak bisa dibatalkan.");
    }
  }

  if (loading) return <p style={muted}>Memuat data langganan…</p>;

  return (
    <section style={{ display: "grid", gap: 20 }}>
      <div>
        <h2 style={heading}>Langganan</h2>
        <p style={muted}>
          {operators.length} travel · {lapsed.length} lewat jatuh tempo · {suspended} ditangguhkan
          {outstanding > 0 ? ` · ${rupiah(outstanding)} belum tertagih` : ""}
        </p>
      </div>

      {notice && <p role="status" style={noticeBox}>{notice}</p>}

      {lapsed.length > 0 && (
        <p style={warnBox}>
          <IconAlertTriangle size={16} />
          {rupiah(outstanding)} belum tertagih dari {lapsed.length} travel. Pengingat berjalan otomatis;
          penangguhan menutup akses tanpa menghapus data apa pun.
        </p>
      )}

      <div>
        <h3 style={{ margin: "0 0 4px", fontSize: 15 }}>Yang lewat jatuh tempo</h3>
        {lapsed.length === 0 ? (
          <div style={emptyBox}>
            <p style={{ margin: 0, fontWeight: 700 }}>Semua langganan lancar</p>
            <p style={{ ...muted, marginTop: 6 }}>
              Tidak ada travel yang aksesnya sudah lewat. Pengingat hanya berjalan saat ada yang telat.
            </p>
          </div>
        ) : (
          <table style={table}>
            <caption style={caption}>Lintas seluruh travel · yang paling lama telat di atas</caption>
            <thead>
              <tr>{["Travel", "Paket", "Akses habis", "Telat", "Tahap pengingat", "Belum dibayar", "Status"].map((h) => <th key={h} style={th}>{h}</th>)}</tr>
            </thead>
            <tbody>
              {lapsed.map((operator) => {
                const late = daysOverdue(operator.accessUntil?.toDate());
                return (
                  <tr key={operator.id} style={operator.suspendedAt ? { ...tr, background: "var(--color-danger-100)" } : tr}>
                    <td style={{ ...td, fontWeight: 700 }}>{operator.name}</td>
                    <td style={td}>{operator.plan || "—"}</td>
                    <td style={td}>{operator.accessUntil ? tanggal(operator.accessUntil.toDate()) : "—"}</td>
                    <td style={td}>{late} hari</td>
                    <td style={td}>
                      {operator.dunningStage
                        ? STAGE_LABEL[operator.dunningStage] ?? operator.dunningStage
                        : <span style={{ color: "var(--color-warm-400)" }}>belum dikirim</span>}
                    </td>
                    <td style={td}>{operator.outstandingIdr > 0n ? rupiah(operator.outstandingIdr) : "—"}</td>
                    <td style={td}>
                      {operator.suspendedAt
                        ? <span style={{ color: "var(--color-danger-600)", fontWeight: 700 }}>Ditangguhkan</span>
                        : operator.subscriptionStatus || "—"}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        )}
      </div>

      <div>
        <h3 style={{ margin: "0 0 4px", fontSize: 15 }}>Riwayat tagihan</h3>
        <p style={muted}>
          Invoice yang dibatalkan tetap ditampilkan — ia bagian dari catatan, bukan kesalahan yang disembunyikan.
        </p>
        {invoices.length === 0 ? (
          <div style={emptyBox}>
            <p style={{ margin: 0, fontWeight: 700 }}>Belum ada tagihan</p>
            <p style={{ ...muted, marginTop: 6 }}>
              Tagihan perpanjangan terbit otomatis tujuh hari sebelum akses habis.
            </p>
          </div>
        ) : (
          <table style={table}>
            <caption style={caption}>Lintas seluruh travel · 100 terakhir</caption>
            <thead>
              <tr>{["Travel", "Paket", "Nominal", "Kanal", "Jatuh tempo", "Status", ""].map((h) => <th key={h} style={th}>{h}</th>)}</tr>
            </thead>
            <tbody>
              {invoices.map((invoice) => (
                <Fragment key={invoice.id}>
                  <tr style={tr}>
                    <td style={{ ...td, fontWeight: 700 }}>{invoice.operatorName}</td>
                    <td style={td}>{invoice.plan}</td>
                    <td style={td}>{rupiah(invoice.amountIdr)}</td>
                    <td style={td}>{invoice.channel === "BANK_TRANSFER" ? "Transfer" : "Gateway"}</td>
                    <td style={td}>{invoice.dueAt ? tanggal(invoice.dueAt.toDate()) : "—"}</td>
                    <td style={td}>
                      {invoice.voidedAt
                        ? <span style={{ color: "var(--color-warm-400)" }}>Dibatalkan</span>
                        : invoice.status === "PAID"
                          ? <span style={{ color: "var(--color-emerald-800)", fontWeight: 700 }}>Lunas</span>
                          : invoice.status}
                    </td>
                    <td style={td}>
                      {invoice.status === "PENDING" && (
                        voiding === invoice.id ? (
                          <div style={{ display: "grid", gap: 6, minWidth: 220 }}>
                            <input
                              value={reason}
                              onChange={(e) => setReason(e.target.value)}
                              style={input}
                              placeholder="Alasan pembatalan"
                              aria-label={`Alasan membatalkan invoice ${invoice.operatorName}`}
                              autoFocus
                            />
                            <div style={{ display: "flex", gap: 6 }}>
                              <button
                                onClick={() => voidInvoice(invoice.id)}
                                disabled={!reason.trim()}
                                style={{ ...danger, opacity: reason.trim() ? 1 : 0.5 }}
                              >
                                Batalkan
                              </button>
                              <button onClick={() => { setVoiding(""); setReason(""); }} style={ghost}>Tutup</button>
                            </div>
                          </div>
                        ) : (
                          <button onClick={() => { setVoiding(invoice.id); setReason(""); }} style={ghost}>
                            <IconBan size={15} />Batalkan
                          </button>
                        )
                      )}
                    </td>
                  </tr>
                  {invoice.voidedAt && invoice.voidedReason && (
                    <tr style={tr}>
                      <td colSpan={7} style={{ ...td, color: "var(--color-warm-400)", fontSize: 12 }}>
                        Dibatalkan {tanggal(invoice.voidedAt.toDate())} — {invoice.voidedReason}
                      </td>
                    </tr>
                  )}
                </Fragment>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </section>
  );
}

const muted: React.CSSProperties = { color: "var(--color-warm-500)", fontSize: 14, margin: 0 };
const heading: React.CSSProperties = { margin: "0 0 4px", fontSize: 18 };
const input: React.CSSProperties = { minHeight: 38, border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 10px", fontFamily: "inherit", fontSize: 13 };
const ghost: React.CSSProperties = { minHeight: 38, border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 14px", background: "transparent", color: "var(--color-warm-700)", fontSize: 13, display: "inline-flex", alignItems: "center", gap: 6 };
const danger: React.CSSProperties = { minHeight: 38, border: 0, borderRadius: 8, padding: "0 14px", background: "var(--color-danger-600)", color: "#fff", fontWeight: 700, fontSize: 13 };
const table: React.CSSProperties = { width: "100%", borderCollapse: "collapse", background: "#fff" };
const caption: React.CSSProperties = { captionSide: "top", textAlign: "left", padding: "0 0 8px", fontSize: 11, color: "var(--color-warm-400)", letterSpacing: "0.06em", textTransform: "uppercase" };
const th: React.CSSProperties = { textAlign: "left", padding: 12, fontSize: 11, color: "var(--color-warm-400)", borderBottom: "1px solid var(--color-cream-300)", whiteSpace: "nowrap" };
const tr: React.CSSProperties = { borderBottom: "1px solid var(--color-cream-300)" };
const td: React.CSSProperties = { padding: 12, color: "var(--color-warm-700)", verticalAlign: "top", fontSize: 13 };
const noticeBox: React.CSSProperties = { margin: 0, padding: "10px 14px", borderRadius: 8, background: "var(--color-emerald-50)", color: "var(--color-emerald-800)", fontSize: 13 };
const warnBox: React.CSSProperties = { display: "flex", alignItems: "center", gap: 8, margin: 0, padding: "12px 16px", background: "var(--color-warning-50)", border: "1px solid var(--color-warning-200)", borderRadius: 8, color: "var(--color-warning-700)", fontSize: 13, fontWeight: 600 };
const emptyBox: React.CSSProperties = { padding: "20px 18px", border: "1px dashed var(--color-cream-400)", borderRadius: 10, background: "#fff", marginTop: 10 };
