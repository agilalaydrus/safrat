"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { IconCheck, IconClockHour4, IconExternalLink, IconRefresh, IconWallet, IconX } from "@tabler/icons-react";
import {
  RefundBeneficiaryKind,
  RefundPayoutAction,
  RefundPayoutMethod,
  RefundPayoutRequest,
  RefundPayoutStatus,
} from "@hajj-saas/proto-gen/hajj/v1/refund_payout_pb";
import { refundPayoutClient } from "@/lib/rpc";
import { getBearerToken } from "@/lib/transport";

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "";

const money = (value: bigint) => new Intl.NumberFormat("id-ID", { style: "currency", currency: "IDR", maximumFractionDigits: 0 }).format(Number(value));
const stamp = (value?: Date) => value?.toLocaleString("id-ID", { day: "2-digit", month: "short", year: "numeric", hour: "2-digit", minute: "2-digit" }) ?? "—";
const methodLabel: Record<number, string> = {
  [RefundPayoutMethod.BANK_TRANSFER]: "Transfer bank",
  [RefundPayoutMethod.EWALLET]: "E-wallet",
  [RefundPayoutMethod.CASH]: "Tunai",
};
const statusInfo: Record<number, { label: string; color: string; background: string }> = {
  [RefundPayoutStatus.REQUESTED]: { label: "Baru", color: "#b45309", background: "var(--color-gold-50)" },
  [RefundPayoutStatus.PROCESSING]: { label: "Diproses", color: "#1d4ed8", background: "#eff6ff" },
  [RefundPayoutStatus.PAID]: { label: "Dibayar", color: "var(--color-emerald-900)", background: "var(--color-emerald-50)" },
  [RefundPayoutStatus.FAILED]: { label: "Gagal", color: "var(--color-danger-600)", background: "#fff1f2" },
  [RefundPayoutStatus.REVERSED]: { label: "Dibalik gateway", color: "#7c3aed", background: "#f5f3ff" },
};

type Resolution = { requestId: string; action: RefundPayoutAction; note: string; paymentReference: string };

export default function RefundPayoutDashboard() {
  const [filter, setFilter] = useState(RefundPayoutStatus.UNSPECIFIED);
  const [requests, setRequests] = useState<RefundPayoutRequest[]>([]);
  const [loading, setLoading] = useState(true);
  const [working, setWorking] = useState("");
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [resolution, setResolution] = useState<Resolution>();
  const [uploadingProof, setUploadingProof] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const response = await refundPayoutClient.listRefundPayoutRequests({ status: filter });
      setRequests(response.requests);
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "Antrean pencairan tidak dapat dimuat.");
    } finally {
      setLoading(false);
    }
  }, [filter]);

  useEffect(() => { void load(); }, [load]);

  const total = useMemo(() => requests.reduce((sum, request) => sum + request.amountIdr, 0n), [requests]);
  const activeCount = requests.filter((request) => request.status === RefundPayoutStatus.REQUESTED || request.status === RefundPayoutStatus.PROCESSING).length;

  async function transition(request: RefundPayoutRequest, action: RefundPayoutAction, note = "", paymentReference = "") {
    setWorking(request.id);
    setError("");
    setNotice("");
    try {
      await refundPayoutClient.transitionRefundPayout({ requestId: request.id, action, note: note.trim(), paymentReference: paymentReference.trim() });
      setNotice(action === RefundPayoutAction.START_PROCESSING ? `Pencairan ${request.beneficiaryName} mulai diproses.` : action === RefundPayoutAction.MARK_PAID ? `Pembayaran ${request.beneficiaryName} dicatat dan saldo ledger didebit.` : `Permintaan ${request.beneficiaryName} ditandai gagal; saldo kembali tersedia.`);
      setResolution(undefined);
      await load();
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "Status pencairan gagal diperbarui.");
    } finally {
      setWorking("");
    }
  }

  function openResolution(request: RefundPayoutRequest, action: RefundPayoutAction) {
    setResolution({ requestId: request.id, action, note: "", paymentReference: "" });
  }

  async function uploadProof(requestId: string, file?: File) {
    if (!file) return;
    setUploadingProof(true); setError("");
    try {
      const token = await getBearerToken();
      if (!token) throw new Error("Sesi tidak ditemukan. Silakan login ulang.");
      const form = new FormData(); form.append("request_id", requestId); form.append("file", file);
      const response = await fetch(`${API_URL}/upload/refund-payout-proof`, { method: "POST", headers: { Authorization: `Bearer ${token}` }, body: form });
      if (!response.ok) throw new Error(await response.text() || "Upload gagal.");
      setNotice("Bukti serah terima berhasil diunggah.");
      await load();
    } catch (caught) { setError(caught instanceof Error ? caught.message : "Bukti tidak dapat diunggah."); }
    finally { setUploadingProof(false); }
  }

  return (
    <main style={page}>
      <header style={header}>
        <div><p style={eyebrow}>KEUANGAN / REFUND</p><h1 style={title}>Pencairan Saldo Refund</h1><p style={subtitle}>{activeCount} permintaan aktif · {money(total)} pada tampilan</p></div>
        <button type="button" onClick={() => void load()} style={secondary}><IconRefresh size={16} />Muat ulang</button>
      </header>

      <div className="gold-divider" />

      <section style={stats}>
        <Stat label="Permintaan pada tampilan" value={String(requests.length)} />
        <Stat label="Masih aktif" value={String(activeCount)} />
        <Stat label="Nilai pada tampilan" value={money(total)} />
      </section>

      <div style={filters}>
        {([
          [RefundPayoutStatus.UNSPECIFIED, "Semua"],
          [RefundPayoutStatus.REQUESTED, "Baru"],
          [RefundPayoutStatus.PROCESSING, "Diproses"],
          [RefundPayoutStatus.PAID, "Dibayar"],
          [RefundPayoutStatus.FAILED, "Gagal"],
          [RefundPayoutStatus.REVERSED, "Dibalik"],
        ] as const).map(([value, label]) => <button key={value} type="button" onClick={() => setFilter(value)} style={filter === value ? filterActive : filterButton}>{label}</button>)}
      </div>

      {error && <p role="alert" style={errorStyle}>{error}</p>}
      {notice && <p role="status" style={noticeStyle}>{notice}</p>}

      {loading ? <p style={empty}>Memuat antrean pencairan...</p> : requests.length === 0 ? (
        <section style={empty}><IconWallet size={38} /><strong>Tidak ada permintaan pada status ini</strong></section>
      ) : (
        <section style={list}>
          {requests.map((request) => {
            const status = statusInfo[request.status] ?? { label: "Tidak dikenal", color: "var(--color-warm-500)", background: "var(--color-cream-200)" };
            const whatsapp = whatsappLink(request.pilgrimPhone);
            const activeResolution = resolution?.requestId === request.id ? resolution : undefined;
            return <article key={request.id} style={card}>
              <div style={cardTop}>
                <div><p style={personName}>{request.beneficiaryName || request.pilgrimName}</p><p style={meta}>{request.beneficiaryKind === RefundBeneficiaryKind.AGENT ? "Agen" : "Jamaah"} · {request.pilgrimPhone || "Nomor telepon belum tersedia"} · {stamp(request.createdAt?.toDate())}</p></div>
                <span style={{ ...badge, color: status.color, background: status.background }}>{status.label}</span>
              </div>
              <div style={amountRow}><strong style={{ fontSize: 24 }}>{money(request.amountIdr)}</strong><span style={method}>{methodLabel[request.method] ?? "Metode lain"}</span></div>
              {request.note && <p style={noteBox}>{request.note}</p>}
              {request.destinationAccountLast4 && <div style={noteBox}>Tujuan: <strong>{request.destinationChannel} · •••• {request.destinationAccountLast4}</strong> atas nama {request.destinationAccountHolder}</div>}
              {request.provider && <div style={resultBox}><span>Gateway: <strong>{request.provider}</strong> · {request.providerStatus || "menunggu"}</span>{request.providerFailureCode && <span>Kode kegagalan: {request.providerFailureCode}</span>}</div>}
              {(request.paymentReference || request.resolutionNote) && <div style={resultBox}>{request.paymentReference && <span>Referensi pembayaran: <strong>{request.paymentReference}</strong></span>}{request.resolutionNote && <span>Catatan: {request.resolutionNote}</span>}</div>}

              <div style={actions}>
                {whatsapp && <a href={whatsapp} target="_blank" rel="noreferrer" style={secondary}>Hubungi penerima <IconExternalLink size={14} /></a>}
                {request.method === RefundPayoutMethod.CASH && request.status === RefundPayoutStatus.REQUESTED && <button disabled={working === request.id} onClick={() => void transition(request, RefundPayoutAction.START_PROCESSING)} style={primary}><IconClockHour4 size={15} />Mulai proses</button>}
                {request.method === RefundPayoutMethod.CASH && request.status === RefundPayoutStatus.PROCESSING && <button disabled={working === request.id} onClick={() => openResolution(request, RefundPayoutAction.MARK_PAID)} style={primary}><IconCheck size={15} />Tandai dibayar</button>}
                {request.method === RefundPayoutMethod.CASH && (request.status === RefundPayoutStatus.REQUESTED || request.status === RefundPayoutStatus.PROCESSING) && <button disabled={working === request.id} onClick={() => openResolution(request, RefundPayoutAction.MARK_FAILED)} style={danger}><IconX size={15} />Gagal</button>}
              </div>

              {activeResolution && <div style={resolutionBox}>
                <strong>{activeResolution.action === RefundPayoutAction.MARK_PAID ? "Konfirmasi pembayaran" : "Catat kegagalan"}</strong>
                {activeResolution.action === RefundPayoutAction.MARK_PAID && <label style={field}>Bukti serah terima (PDF/JPG/PNG)
                  <input type="file" accept="application/pdf,image/jpeg,image/png" disabled={uploadingProof} onChange={(event) => void uploadProof(request.id, event.target.files?.[0])} />
                  <small style={meta}>{request.proofUrl ? "Bukti sudah tersimpan." : uploadingProof ? "Mengunggah..." : "Wajib sebelum pencairan tunai ditandai dibayar."}</small>
                </label>}
                {activeResolution.action === RefundPayoutAction.MARK_PAID && <label style={field}>Referensi pembayaran<input autoFocus value={activeResolution.paymentReference} onChange={(event) => setResolution({ ...activeResolution, paymentReference: event.target.value })} placeholder="Nomor transfer / bukti kas" maxLength={200} style={input} /></label>}
                <label style={field}>{activeResolution.action === RefundPayoutAction.MARK_FAILED ? "Alasan kegagalan" : "Catatan operasional"}<textarea autoFocus={activeResolution.action === RefundPayoutAction.MARK_FAILED} value={activeResolution.note} onChange={(event) => setResolution({ ...activeResolution, note: event.target.value })} rows={2} maxLength={500} style={{ ...input, paddingTop: 10 }} /></label>
                <div style={actions}><button onClick={() => setResolution(undefined)} style={secondary}>Batal</button><button disabled={working === request.id || uploadingProof || (activeResolution.action === RefundPayoutAction.MARK_PAID ? !activeResolution.paymentReference.trim() || !request.proofUrl : !activeResolution.note.trim())} onClick={() => void transition(request, activeResolution.action, activeResolution.note, activeResolution.paymentReference)} style={activeResolution.action === RefundPayoutAction.MARK_PAID ? primary : danger}>Simpan keputusan</button></div>
              </div>}
            </article>;
          })}
        </section>
      )}
    </main>
  );
}

function Stat({ label, value }: { label: string; value: string }) { return <div style={stat}><small style={meta}>{label}</small><strong style={{ fontSize: 21 }}>{value}</strong></div>; }
function whatsappLink(phone: string) { const number = phone.replace(/\D/g, "").replace(/^0/, "62"); return number ? `https://wa.me/${number}` : ""; }

const page: React.CSSProperties = { maxWidth: 1180, margin: "0 auto", padding: "32px 24px 72px" };
const header: React.CSSProperties = { display: "flex", justifyContent: "space-between", alignItems: "flex-start", gap: 18, flexWrap: "wrap" };
const eyebrow: React.CSSProperties = { margin: "4px 0 8px", color: "var(--color-gold-800)", fontSize: 11, fontWeight: 800, letterSpacing: ".08em" };
const title: React.CSSProperties = { margin: 0, fontSize: "clamp(30px,4vw,46px)", fontWeight: 500 };
const subtitle: React.CSSProperties = { margin: "8px 0 0", color: "var(--color-warm-500)" };
const stats: React.CSSProperties = { display: "grid", gridTemplateColumns: "repeat(auto-fit,minmax(190px,1fr))", gap: 12, marginBottom: 18 };
const stat: React.CSSProperties = { display: "grid", gap: 5, padding: 16, border: "1px solid var(--color-cream-400)", borderRadius: 10, background: "#fff" };
const filters: React.CSSProperties = { display: "flex", flexWrap: "wrap", gap: 8, marginBottom: 18 };
const filterButton: React.CSSProperties = { minHeight: 38, padding: "0 13px", border: "1px solid var(--color-cream-400)", borderRadius: 99, background: "#fff", color: "var(--color-warm-600)", fontWeight: 700 };
const filterActive: React.CSSProperties = { ...filterButton, background: "var(--color-emerald-900)", color: "#fff", borderColor: "var(--color-emerald-900)" };
const list: React.CSSProperties = { display: "grid", gap: 13 };
const card: React.CSSProperties = { display: "grid", gap: 13, padding: 18, border: "1px solid var(--color-cream-400)", borderRadius: 12, background: "#fff" };
const cardTop: React.CSSProperties = { display: "flex", justifyContent: "space-between", alignItems: "flex-start", gap: 12 };
const personName: React.CSSProperties = { margin: 0, fontSize: 17, fontWeight: 800 };
const meta: React.CSSProperties = { margin: "3px 0 0", color: "var(--color-warm-500)", fontSize: 12 };
const badge: React.CSSProperties = { padding: "5px 9px", borderRadius: 99, fontSize: 11, fontWeight: 800 };
const amountRow: React.CSSProperties = { display: "flex", justifyContent: "space-between", alignItems: "center", gap: 12 };
const method: React.CSSProperties = { color: "var(--color-emerald-900)", fontSize: 13, fontWeight: 700 };
const noteBox: React.CSSProperties = { margin: 0, padding: 11, borderRadius: 8, background: "var(--color-cream-100)", color: "var(--color-warm-600)", fontSize: 13 };
const resultBox: React.CSSProperties = { display: "grid", gap: 4, padding: 11, borderRadius: 8, background: "var(--color-emerald-50)", color: "var(--color-emerald-900)", fontSize: 13 };
const actions: React.CSSProperties = { display: "flex", flexWrap: "wrap", gap: 8, justifyContent: "flex-end" };
const primary: React.CSSProperties = { minHeight: 40, padding: "0 13px", border: 0, borderRadius: 8, display: "inline-flex", alignItems: "center", gap: 6, justifyContent: "center", background: "var(--color-emerald-900)", color: "#fff", textDecoration: "none", fontWeight: 800 };
const secondary: React.CSSProperties = { ...primary, border: "1px solid var(--color-cream-400)", background: "#fff", color: "var(--color-emerald-900)" };
const danger: React.CSSProperties = { ...primary, background: "var(--color-danger-600)" };
const resolutionBox: React.CSSProperties = { display: "grid", gap: 11, padding: 14, border: "1px solid var(--color-cream-400)", borderRadius: 9, background: "#fffcf5" };
const field: React.CSSProperties = { display: "grid", gap: 6, color: "var(--color-warm-700)", fontSize: 12, fontWeight: 700 };
const input: React.CSSProperties = { width: "100%", minHeight: 42, border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 11px", background: "#fff", font: "inherit", fontWeight: 400 };
const empty: React.CSSProperties = { display: "grid", justifyItems: "center", gap: 8, padding: 44, color: "var(--color-warm-500)", border: "1px dashed var(--color-cream-400)", borderRadius: 12 };
const errorStyle: React.CSSProperties = { padding: 12, background: "#fff1f2", color: "var(--color-danger-600)", borderRadius: 8 };
const noticeStyle: React.CSSProperties = { padding: 12, background: "var(--color-emerald-50)", color: "var(--color-emerald-900)", borderRadius: 8 };
