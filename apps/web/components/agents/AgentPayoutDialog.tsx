"use client";
import { FormEvent, useEffect, useState } from "react";
import { IconBuildingBank, IconCash, IconCheck, IconDeviceMobile, IconX } from "@tabler/icons-react";
import { Agent, AgentPayout, AgentPayoutEntry, PayoutMethod, PayoutRequest } from "@hajj-saas/proto-gen/hajj/v1/agent_pb";
import { agentClient } from "@/lib/rpc";

const rupiah = (n: number) => `Rp${n.toLocaleString("id-ID")}`;
const parseDigits = (s: string) => Number(s.replace(/[^0-9]/g, "")) || 0;

const METHODS: { value: PayoutMethod; label: string; icon: typeof IconBuildingBank }[] = [
  { value: PayoutMethod.TRANSFER, label: "Transfer Bank", icon: IconBuildingBank },
  { value: PayoutMethod.CASH, label: "Tunai", icon: IconCash },
  { value: PayoutMethod.EWALLET, label: "E-Wallet", icon: IconDeviceMobile },
];

type Props = { open: boolean; agent?: Agent; summary?: AgentPayout; onClose: () => void; onPaid: (amount: number) => void; onRequestsChanged: () => void };

export default function AgentPayoutDialog({ open, agent, summary, onClose, onPaid, onRequestsChanged }: Props) {
  const [amountText, setAmountText] = useState("");
  const [method, setMethod] = useState<PayoutMethod>(PayoutMethod.TRANSFER);
  const [reference, setReference] = useState("");
  const [activeRequestId, setActiveRequestId] = useState("");
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [saving, setSaving] = useState(false);
  const [history, setHistory] = useState<AgentPayoutEntry[]>([]);
  const [loadingHistory, setLoadingHistory] = useState(false);
  const [requests, setRequests] = useState<PayoutRequest[]>([]);
  const [rejectingId, setRejectingId] = useState("");
  const [rejectNote, setRejectNote] = useState("");
  const [rejecting, setRejecting] = useState(false);

  const outstanding = Number(summary?.outstandingIdr ?? 0);

  const loadRequests = () => { if (agent) agentClient.listPayoutRequests({ agentId: agent.id }).then((r) => setRequests(r.requests)).catch(() => {}); };

  useEffect(() => {
    if (!open || !agent) return;
    setAmountText(outstanding > 0 ? String(outstanding) : "");
    setMethod(PayoutMethod.TRANSFER);
    setReference("");
    setActiveRequestId("");
    setErrors({});
    setRejectingId("");
    setRejectNote("");
    setLoadingHistory(true);
    agentClient.listAgentPayoutHistory({ agentId: agent.id }).then((r) => setHistory(r.entries)).catch(() => setHistory([])).finally(() => setLoadingHistory(false));
    loadRequests();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, agent?.id]);

  useEffect(() => {
    const onEsc = (e: KeyboardEvent) => e.key === "Escape" && !saving && onClose();
    if (open) window.addEventListener("keydown", onEsc);
    return () => window.removeEventListener("keydown", onEsc);
  }, [open, saving, onClose]);

  if (!open || !agent) return null;

  const amount = parseDigits(amountText);

  function useRequest(request: PayoutRequest) {
    setAmountText(String(Number(request.amountIdr)));
    setReference(request.note);
    setActiveRequestId(request.id);
    setErrors({});
  }

  async function submit(e: FormEvent) {
    e.preventDefault();
    const errs: Record<string, string> = {};
    if (amount <= 0) errs.amount = "Jumlah harus lebih dari Rp0.";
    else if (amount > outstanding) errs.amount = `Jumlah melebihi sisa tertunda (${rupiah(outstanding)}).`;
    if (Object.keys(errs).length) { setErrors(errs); return; }
    setSaving(true);
    try {
      await agentClient.recordAgentPayout({ agentId: agent!.id, amountIdr: BigInt(amount), note: reference.trim(), method, requestId: activeRequestId });
      onPaid(amount);
      if (activeRequestId) onRequestsChanged();
      onClose();
    } catch (err) {
      setErrors({ _form: err instanceof Error ? err.message : "Gagal mencatat pembayaran." });
    } finally {
      setSaving(false);
    }
  }

  async function reject(requestId: string) {
    if (!rejectNote.trim()) return;
    setRejecting(true);
    try {
      await agentClient.rejectPayoutRequest({ requestId, note: rejectNote.trim() });
      setRejectingId("");
      setRejectNote("");
      loadRequests();
      onRequestsChanged();
    } catch {
      // surfaced inline via the requests list not refreshing; operator can retry
    } finally {
      setRejecting(false);
    }
  }

  return (
    <div style={o}>
      <aside style={s}>
        <div style={h}>
          <div><p style={ey}>PENCAIRAN KOMISI</p><h2 style={{ margin: 0 }}>Bayar {agent.name}</h2></div>
          <button className="btn-close-sheet" onClick={() => !saving && onClose()} style={x}><IconX size={18} /></button>
        </div>
        <div style={b}>
          <div style={summaryCard}>
            <SummaryRow label="Total komisi (order terbayar)" value={rupiah(Number(summary?.totalCommissionIdr ?? 0))} />
            <SummaryRow label="Sudah dibayar" value={rupiah(Number(summary?.totalDisbursedIdr ?? 0))} />
            <div className="gold-divider" style={{ margin: "6px 0" }} />
            <SummaryRow label="Sisa tertunda" value={rupiah(outstanding)} emphasis />
          </div>

          {!!requests.length && (
            <div style={{ marginTop: 20 }}>
              <p style={sec}>PERMINTAAN PENCAIRAN DARI TOUR LEADER</p>
              <div style={{ display: "grid", gap: 8, marginTop: 10 }}>
                {requests.map((request) => (
                  <div key={request.id} style={requestCard}>
                    <div style={{ display: "flex", justifyContent: "space-between", gap: 12 }}>
                      <div>
                        <b>{rupiah(Number(request.amountIdr))}</b>
                        {request.note && <span style={{ display: "block", color: "var(--color-warm-500)", fontSize: 12 }}>{request.note}</span>}
                        <span style={{ display: "block", color: "var(--color-warm-400)", fontSize: 11 }}>Diajukan {request.requestedAt?.toDate().toLocaleDateString("id-ID", { day: "2-digit", month: "short", year: "numeric" })}</span>
                      </div>
                      {rejectingId !== request.id && (
                        <div style={{ display: "flex", gap: 6, alignItems: "flex-start" }}>
                          <button type="button" onClick={() => useRequest(request)} style={approveBtn}><IconCheck size={14} />Setujui</button>
                          <button type="button" onClick={() => { setRejectingId(request.id); setRejectNote(""); }} style={rejectBtn}>Tolak</button>
                        </div>
                      )}
                    </div>
                    {rejectingId === request.id && (
                      <div style={{ marginTop: 10, display: "grid", gap: 8 }}>
                        <input className="safrat-input" placeholder="Alasan penolakan (wajib)" value={rejectNote} onChange={(e) => setRejectNote(e.target.value)} style={i} />
                        <div style={{ display: "flex", gap: 8 }}>
                          <button type="button" disabled={rejecting || !rejectNote.trim()} onClick={() => void reject(request.id)} style={{ ...rejectBtn, flex: 1, justifyContent: "center" }}>{rejecting ? "Memproses..." : "Konfirmasi Tolak"}</button>
                          <button type="button" onClick={() => setRejectingId("")} style={{ ...chip, flex: 1 }}>Batal</button>
                        </div>
                      </div>
                    )}
                  </div>
                ))}
              </div>
            </div>
          )}

          {outstanding <= 0 ? (
            <p style={{ color: "var(--color-warm-500)", textAlign: "center", padding: "24px 0" }}>Tidak ada komisi tertunda untuk tour leader ini.</p>
          ) : (
            <form id="payout-form" onSubmit={submit} style={{ display: "grid", gap: 16, marginTop: 20 }}>
              <div style={{ display: "flex", justifyContent: "space-between", alignItems: "baseline" }}>
                <p style={sec}>DETAIL PEMBAYARAN</p>
                {activeRequestId && <button type="button" onClick={() => setActiveRequestId("")} style={{ ...chip, fontSize: 11 }}>Batalkan, lagi selesaikan permintaan ini</button>}
              </div>
              <label style={{ display: "grid", gap: 6 }}>
                <span style={lab}>Jumlah</span>
                <div style={amountWrap}>
                  <span style={amountPrefix}>Rp</span>
                  <input className="safrat-input" inputMode="numeric" value={amountText ? Number(amountText).toLocaleString("id-ID") : ""} onChange={(e) => setAmountText(String(parseDigits(e.target.value)))} style={{ ...i, paddingInlineStart: 40 }} />
                </div>
                <div style={{ display: "flex", gap: 8 }}>
                  <button type="button" onClick={() => setAmountText(String(outstanding))} style={chip}>Bayar penuh ({rupiah(outstanding)})</button>
                </div>
                {errors.amount && <small style={{ color: "var(--color-danger-600)" }}>{errors.amount}</small>}
              </label>

              <label style={{ display: "grid", gap: 6 }}>
                <span style={lab}>Metode pembayaran</span>
                <div style={{ display: "grid", gridTemplateColumns: "repeat(3,1fr)", gap: 8 }}>
                  {METHODS.map(({ value, label, icon: Icon }) => (
                    <button key={value} type="button" onClick={() => setMethod(value)} style={method === value ? methodBtnActive : methodBtn}>
                      <Icon size={20} />
                      <span style={{ fontSize: 12 }}>{label}</span>
                    </button>
                  ))}
                </div>
              </label>

              <label style={{ display: "grid", gap: 6 }}>
                <span style={lab}>Referensi / catatan (opsional)</span>
                <input className="safrat-input" placeholder="mis. No. referensi transfer" value={reference} onChange={(e) => setReference(e.target.value)} style={i} />
              </label>

              {errors._form && <p style={err}>{errors._form}</p>}
            </form>
          )}

          <div style={{ marginTop: 28 }}>
            <p style={sec}>RIWAYAT PEMBAYARAN</p>
            {loadingHistory ? (
              <p style={{ color: "var(--color-warm-500)", fontSize: 13, marginTop: 10 }}>Memuat riwayat...</p>
            ) : history.length ? (
              <div style={{ display: "grid", gap: 8, marginTop: 10 }}>
                {history.map((entry) => (
                  <div key={entry.id} style={historyRow}>
                    <div>
                      <b>{rupiah(Number(entry.amountIdr))}</b>
                      <span style={{ color: "var(--color-warm-400)", fontSize: 12, marginInlineStart: 8 }}>{METHODS.find((m) => m.value === entry.method)?.label ?? "-"}</span>
                      {entry.note && <span style={{ display: "block", color: "var(--color-warm-500)", fontSize: 12 }}>{entry.note}</span>}
                    </div>
                    <div style={{ textAlign: "end", color: "var(--color-warm-400)", fontSize: 12 }}>
                      <span>{entry.createdAt?.toDate().toLocaleDateString("id-ID", { day: "2-digit", month: "short", year: "numeric" })}</span>
                      <span style={{ display: "block" }}>oleh {entry.paidByName}</span>
                    </div>
                  </div>
                ))}
              </div>
            ) : (
              <p style={{ color: "var(--color-warm-500)", fontSize: 13, marginTop: 10 }}>Belum ada pembayaran tercatat.</p>
            )}
          </div>
        </div>
        {outstanding > 0 && (
          <div style={foot}>
            <button form="payout-form" disabled={saving} style={primary}>{saving ? "Memproses..." : `Catat Pembayaran ${amount > 0 ? rupiah(amount) : ""}`}</button>
          </div>
        )}
      </aside>
    </div>
  );
}

function SummaryRow({ label, value, emphasis }: { label: string; value: string; emphasis?: boolean }) {
  return <div style={{ display: "flex", justifyContent: "space-between", alignItems: "baseline" }}><span style={{ color: "var(--color-warm-500)", fontSize: emphasis ? 14 : 13 }}>{label}</span><b style={{ fontSize: emphasis ? 22 : 15, color: emphasis ? "var(--color-danger-600)" : "var(--color-warm-900)" }}>{value}</b></div>;
}

const o: React.CSSProperties = { position: "fixed", inset: 0, zIndex: 30, display: "flex", justifyContent: "flex-end", background: "rgba(15,23,42,.48)", backdropFilter: "blur(2px)" };
const s: React.CSSProperties = { width: "min(560px,100%)", height: "100vh", display: "flex", flexDirection: "column", background: "#fff", borderRadius: "16px 0 0 16px", overflow: "hidden" };
const h: React.CSSProperties = { display: "flex", justifyContent: "space-between", alignItems: "center", padding: "20px 24px 16px", borderBottom: "1px solid var(--color-cream-300)" };
const b: React.CSSProperties = { flex: 1, overflowY: "auto", padding: 24 };
const foot: React.CSSProperties = { padding: "16px 24px", borderTop: "1px solid var(--color-cream-300)" };
const x: React.CSSProperties = { width: 40, height: 40, borderRadius: "50%", border: "1px solid var(--color-cream-400)", background: "transparent", color: "var(--color-warm-400)" };
const ey: React.CSSProperties = { margin: 0, color: "var(--color-gold-800)", fontSize: 11, fontWeight: 700, letterSpacing: ".1em" };
const sec: React.CSSProperties = { margin: 0, fontSize: 11, fontWeight: 700, letterSpacing: ".1em", color: "var(--color-warm-400)", paddingBottom: 8, borderBottom: "1px solid var(--color-cream-300)" };
const i: React.CSSProperties = { minHeight: 48, width: "100%", border: "1.5px solid var(--color-cream-400)", borderRadius: 10, padding: "0 14px", background: "#fff", font: "inherit" };
const lab: React.CSSProperties = { fontSize: 13, fontWeight: 600, color: "var(--color-warm-700)" };
const primary: React.CSSProperties = { minHeight: 48, width: "100%", border: 0, borderRadius: 10, background: "var(--color-gold-500)", color: "#fff", fontWeight: 700 };
const err: React.CSSProperties = { margin: 0, padding: 10, borderRadius: 8, background: "#ffe4e6", color: "var(--color-danger-600)" };
const summaryCard: React.CSSProperties = { display: "grid", gap: 8, padding: 18, background: "var(--color-cream-200)", borderRadius: 12, border: "1px solid var(--color-cream-400)" };
const amountWrap: React.CSSProperties = { position: "relative" };
const amountPrefix: React.CSSProperties = { position: "absolute", insetInlineStart: 14, top: 0, height: 48, display: "flex", alignItems: "center", color: "var(--color-warm-500)", fontWeight: 600, pointerEvents: "none" };
const chip: React.CSSProperties = { minHeight: 32, border: "1px solid var(--color-cream-400)", borderRadius: 999, padding: "0 12px", background: "transparent", color: "var(--color-emerald-900)", fontSize: 12, fontWeight: 600 };
const methodBtn: React.CSSProperties = { minHeight: 64, display: "grid", justifyItems: "center", gap: 6, border: "1.5px solid var(--color-cream-400)", borderRadius: 10, background: "#fff", color: "var(--color-warm-500)" };
const methodBtnActive: React.CSSProperties = { ...methodBtn, border: "1.5px solid var(--color-gold-500)", background: "var(--color-gold-50)", color: "var(--color-gold-800)", fontWeight: 700 };
const historyRow: React.CSSProperties = { display: "flex", justifyContent: "space-between", gap: 12, padding: "10px 12px", border: "1px solid var(--color-cream-300)", borderRadius: 10, background: "#fff" };
const requestCard: React.CSSProperties = { padding: 12, border: "1px solid var(--color-gold-500)", borderRadius: 10, background: "var(--color-gold-50)" };
const approveBtn: React.CSSProperties = { minHeight: 32, border: 0, borderRadius: 8, padding: "0 10px", background: "var(--color-emerald-900)", color: "#fff", fontSize: 12, fontWeight: 600, display: "inline-flex", alignItems: "center", gap: 4, whiteSpace: "nowrap" };
const rejectBtn: React.CSSProperties = { minHeight: 32, border: "1px solid var(--color-danger-600)", borderRadius: 8, padding: "0 10px", background: "transparent", color: "var(--color-danger-600)", fontSize: 12, fontWeight: 600, display: "inline-flex", alignItems: "center", justifyContent: "center", gap: 4, whiteSpace: "nowrap" };
