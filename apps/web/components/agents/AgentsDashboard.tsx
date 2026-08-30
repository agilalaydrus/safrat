"use client";
import { useEffect, useState } from "react";
import { IconCheck, IconClockHour4, IconCopy, IconMail, IconPencil, IconPhone, IconPlus, IconShieldCheck, IconTrash, IconUserDollar, IconX } from "@tabler/icons-react";
import { Agent, AgentPayout, PayoutMethod, PayoutRequest } from "@hajj-saas/proto-gen/hajj/v1/agent_pb";
import { agentClient, operatorClient } from "@/lib/rpc";
import AgentFormDialog from "./AgentFormDialog";
import AgentPayoutDialog from "./AgentPayoutDialog";
import AgentKycDialog from "./AgentKycDialog";
import { RoleGate } from "@/components/auth/RoleGate";
import { buildTenantLink } from "@/lib/tenant-link";

const rupiah = (n: number) => `Rp${n.toLocaleString("id-ID")}`;
const tierBadge: Record<string, React.CSSProperties> = { GOLD: { background: "var(--color-gold-50)", color: "var(--color-gold-800)" }, SILVER: { background: "var(--color-cream-200)", color: "var(--color-warm-700)" }, BRONZE: { background: "var(--color-emerald-50)", color: "var(--color-emerald-900)" } };
const KYC_LABEL: Record<string, string> = { UNVERIFIED: "Belum Diisi", PENDING_REVIEW: "Menunggu Verifikasi", VERIFIED: "Terverifikasi", REJECTED: "Ditolak" };
function kycBadge(status: string): React.CSSProperties {
  const map: Record<string, [string, string]> = { PENDING_REVIEW: ["var(--color-gold-50)", "var(--color-gold-800)"], VERIFIED: ["var(--color-emerald-50)", "var(--color-emerald-900)"], REJECTED: ["var(--color-danger-100)", "var(--color-danger-600)"] };
  const [bg, color] = map[status] ?? ["var(--color-cream-200)", "var(--color-warm-500)"];
  return { background: bg, color, borderRadius: 8, padding: "6px 10px", justifySelf: "start" };
}

export default function AgentsDashboard() {
  const [a, setA] = useState<Agent[]>([]);
  const [payouts, setPayouts] = useState<AgentPayout[]>([]);
  const [requests, setRequests] = useState<PayoutRequest[]>([]);
  const [open, setOpen] = useState(false);
  const [edit, setEdit] = useState<Agent | undefined>();
  const [payoutTarget, setPayoutTarget] = useState<Agent | undefined>();
  const [kycTarget, setKycTarget] = useState<Agent | undefined>();
  const [note, setNote] = useState("");
  const [applyLink, setApplyLink] = useState("");
  const [workingRequestId, setWorkingRequestId] = useState("");
  const [rejectingId, setRejectingId] = useState("");
  const [rejectNote, setRejectNote] = useState("");

  const load = () => agentClient.listAgents({}).then((r) => setA(r.agents)).catch(() => setNote("Gagal memuat data tour leader."));
  const loadPayouts = () => agentClient.listAgentPayouts({}).then((r) => setPayouts(r.payouts)).catch(() => {});
  const loadRequests = () => agentClient.listPayoutRequests({}).then((r) => setRequests(r.requests)).catch(() => {});

  useEffect(() => {
    void load();
    void loadPayouts();
    void loadRequests();
    operatorClient.getMyOperator({}).then((op) => setApplyLink(buildTenantLink(op.slug, "/apply"))).catch(() => {});
  }, []);

  const pending = a.filter((x) => !x.isActive);
  const activeAgents = a.filter((x) => x.isActive);
  const payoutByAgent = new Map(payouts.map((p) => [p.agentId, p]));
  const totalOwed = payouts.reduce((n, p) => n + Number(p.outstandingIdr), 0);
  const requestsByAgent = new Map<string, PayoutRequest[]>();
  for (const r of requests) requestsByAgent.set(r.agentId, [...(requestsByAgent.get(r.agentId) ?? []), r]);

  async function approveAgent(x: Agent) {
    await agentClient.updateAgent({ agentId: x.id, name: x.name, phone: x.phone, email: x.email, commissionRate: x.commissionRate, notes: x.notes, isActive: true });
    setNote(`${x.name} disetujui.`);
    void load();
  }

  async function approveRequest(request: PayoutRequest) {
    setWorkingRequestId(request.id);
    setNote("");
    try {
      await agentClient.recordAgentPayout({ agentId: request.agentId, amountIdr: request.amountIdr, note: "Disetujui dari permintaan", method: PayoutMethod.TRANSFER, requestId: request.id });
      setNote(`Pencairan ${rupiah(Number(request.amountIdr))} untuk ${request.agentName} berhasil diproses.`);
      void loadRequests();
      void loadPayouts();
    } catch (error) {
      setNote(error instanceof Error ? error.message : "Gagal memproses permintaan.");
    } finally {
      setWorkingRequestId("");
    }
  }

  async function rejectRequest(request: PayoutRequest) {
    if (!rejectNote.trim()) return;
    setWorkingRequestId(request.id);
    try {
      await agentClient.rejectPayoutRequest({ requestId: request.id, note: rejectNote.trim() });
      setRejectingId("");
      setRejectNote("");
      setNote(`Permintaan pencairan ${request.agentName} ditolak.`);
      void loadRequests();
    } catch (error) {
      setNote(error instanceof Error ? error.message : "Gagal menolak permintaan.");
    } finally {
      setWorkingRequestId("");
    }
  }

  return (
    <main style={page}>
      <header style={head}>
        <div><p style={ey}>OPERASIONAL / TOUR LEADER</p><h1 style={title}>Tour Leader</h1><p style={{color:"var(--color-warm-500)",margin:0}}>{activeAgents.length} aktif · {pending.length} menunggu · {rupiah(totalOwed)} komisi tertunda</p></div>
        <button style={emerald} onClick={() => { setEdit(undefined); setOpen(true); }}><IconPlus size={18} />Tambah Tour Leader</button>
      </header>
      <div className="gold-divider" />
      {applyLink && <p style={applyRow}>Tautan pendaftaran publik: <code style={code}>{applyLink}</code><button style={ghost} onClick={() => navigator.clipboard.writeText(applyLink)}><IconCopy size={15} />Salin</button></p>}
      {note && <p>{note}</p>}
      <div style={stats}>
        {([["Total Tour Leader", activeAgents.length], ["Pendaftaran Tertunda", pending.length], ["Total Jamaah Dirujuk", activeAgents.reduce((n, x) => n + x.pilgrimCount, 0)], ["Total Komisi Belum Dibayar", rupiah(totalOwed)], ["Permintaan Pencairan", requests.length]] as const).map((x) => (
          <div style={stat} key={String(x[0])}><small>{x[0]}</small><strong>{x[1]}</strong></div>
        ))}
      </div>

      {!!requests.length && (
        <section style={{ marginBottom: 24 }}>
          <h2 style={{ fontSize: 18 }}>Semua permintaan pencairan ({requests.length})</h2>
          <div style={{ display: "grid", gap: 10 }}>
            {requests.map((r) => (
              <article key={r.id} style={requestCard}>
                <div style={row}>
                  <div>
                    <strong>{r.agentName}</strong>
                    <p style={{ margin: "2px 0 0", fontSize: 12, color: "var(--color-warm-400)" }}>{r.requestedAt?.toDate().toLocaleDateString("id-ID", { day: "2-digit", month: "short", year: "numeric" })}</p>
                  </div>
                  <b style={{ fontSize: 16, color: "var(--color-gold-800)" }}>{rupiah(Number(r.amountIdr))}</b>
                </div>
                {r.note && <p style={{ margin: "8px 0 0", fontSize: 13, color: "var(--color-warm-600)", background: "var(--color-cream-100)", padding: "8px 12px", borderRadius: 6 }}>{r.note}</p>}
                {rejectingId === r.id ? (
                  <div style={{ display: "grid", gap: 8, marginTop: 12 }}>
                    <input value={rejectNote} onChange={(e) => setRejectNote(e.target.value)} placeholder="Alasan penolakan (wajib)" style={rejectInput} />
                    <div style={{ display: "flex", gap: 8 }}>
                      <button disabled={workingRequestId === r.id || !rejectNote.trim()} onClick={() => void rejectRequest(r)} style={{ ...ghostDanger, flex: 1, justifyContent: "center" }}>
                        {workingRequestId === r.id ? "Memproses..." : "Konfirmasi Tolak"}
                      </button>
                      <button onClick={() => { setRejectingId(""); setRejectNote(""); }} style={{ ...ghost, flex: 1, justifyContent: "center" }}>Batal</button>
                    </div>
                  </div>
                ) : (
                  <div style={{ display: "flex", gap: 8, marginTop: 12 }}>
                    <button disabled={workingRequestId === r.id} onClick={() => void approveRequest(r)} style={emerald}><IconCheck size={15} />{workingRequestId === r.id ? "Memproses..." : "Setujui & Bayar"}</button>
                    <button disabled={workingRequestId === r.id} onClick={() => setRejectingId(r.id)} style={ghostDanger}><IconX size={15} />Tolak</button>
                  </div>
                )}
              </article>
            ))}
          </div>
        </section>
      )}

      {!!pending.length && (
        <section style={{ marginBottom: 24 }}>
          <h2 style={{ fontSize: 18 }}>Pendaftaran tertunda</h2>
          <div style={grid}>
            {pending.map((x) => (
              <article style={card} key={x.id}>
                <div style={row}><h2 style={{ margin: 0, fontSize: 18 }}>{x.name}</h2></div>
                <p style={info}><IconPhone size={15} />{x.phone || "Tanpa telepon"}</p>
                <p style={info}><IconMail size={15} />{x.email || "Tanpa email"}</p>
                <button style={emerald} onClick={() => void approveAgent(x)}><IconCheck size={15} />Setujui</button>
              </article>
            ))}
          </div>
        </section>
      )}

      {activeAgents.length ? (
        <div style={grid}>
          {activeAgents.map((x) => {
            const payout = payoutByAgent.get(x.id);
            const agentRequests = requestsByAgent.get(x.id) ?? [];
            return (
              <article style={card} key={x.id}>
                <div style={row}><h2 style={{ margin: 0, fontSize: 18 }}>{x.name}</h2><b style={rate}>{x.commissionRate.toFixed(2)}%</b></div>
                <p style={info}><IconPhone size={15} />{x.phone || "Tanpa telepon"}</p>
                <p style={info}><IconMail size={15} />{x.email || "Tanpa email"}</p>
                <p style={{ color: "var(--color-emerald-900)", fontWeight: 600 }}>{x.pilgrimCount} jamaah dirujuk</p>
                <p style={{ color: "var(--color-gold-800)", fontWeight: 600 }}>{rupiah(Number(payout?.totalCommissionIdr ?? 0))} komisi dari {payout?.paidOrderCount ?? 0} order terbayar</p>
                <p style={{ color: "var(--color-warm-500)", fontSize: 13 }}>{rupiah(Number(payout?.totalDisbursedIdr ?? 0))} sudah dibayar · <b style={{ color: "var(--color-danger-600)" }}>{rupiah(Number(payout?.outstandingIdr ?? 0))} tertunda</b></p>
                {!!agentRequests.length && <p style={requestFlag}><IconClockHour4 size={14} />{agentRequests.length} permintaan pencairan menunggu ({rupiah(agentRequests.reduce((n, r) => n + Number(r.amountIdr), 0))})</p>}
                <div style={row}><span style={{ ...badge, ...tierBadge[x.tier] }}>{x.tier} · {x.referralCode}</span><span style={active}>Aktif</span></div>
                <button style={{ ...ghost, ...kycBadge(x.kycStatus) }} onClick={() => setKycTarget(x)}><IconShieldCheck size={15} />KYC: {KYC_LABEL[x.kycStatus] ?? "Belum Diisi"}</button>
                <div style={row}>
                  <span>
                    <button style={ghost} onClick={() => { setEdit(x); setOpen(true); }}><IconPencil size={15} />Ubah</button>
                    <RoleGate require={["owner", "admin"]}>
                      <button style={{ ...ghost, color: "var(--color-danger-600)" }} onClick={async () => { if (window.confirm(`Hapus tour leader ${x.name}? Tindakan ini tidak dapat dibatalkan.`)) { await agentClient.deleteAgent({ agentId: x.id }); void load(); } }}><IconTrash size={15} />Hapus</button>
                    </RoleGate>
                  </span>
                  <button style={emerald} onClick={() => setPayoutTarget(x)} disabled={!Number(payout?.outstandingIdr ?? 0)}><IconUserDollar size={15} />Bayar</button>
                </div>
              </article>
            );
          })}
        </div>
      ) : (
        <section style={empty}>
          <IconUserDollar size={48} color="var(--color-warm-400)" />
          <h2 style={{ margin: 0 }}>Belum ada tour leader</h2>
          <p>Undang tour leader rujukan untuk melacak jamaah yang mereka referensikan.</p>
          <button style={gold} onClick={() => setOpen(true)}>Tambah Tour Leader</button>
        </section>
      )}

      <AgentFormDialog open={open} initial={edit} onClose={() => setOpen(false)} onSaved={(n) => { setNote(`${n} berhasil disimpan.`); void load(); }} />
      <AgentPayoutDialog open={!!payoutTarget} agent={payoutTarget} summary={payoutTarget ? payoutByAgent.get(payoutTarget.id) : undefined} onClose={() => setPayoutTarget(undefined)} onPaid={(amount) => { setNote(`Pembayaran ${rupiah(amount)} untuk ${payoutTarget?.name} dicatat.`); void loadPayouts(); }} onRequestsChanged={() => void loadRequests()} />
      <AgentKycDialog open={!!kycTarget} agent={kycTarget} onClose={() => setKycTarget(undefined)} onUpdated={(updated) => { setKycTarget(updated); void load(); }} />
    </main>
  );
}

const page: React.CSSProperties = { maxWidth: 1400, margin: "0 auto", padding: "32px 24px" };
const head: React.CSSProperties = { display: "flex", justifyContent: "space-between", alignItems: "flex-start" };
const ey: React.CSSProperties = { color: "var(--color-gold-800)", fontSize: 11, fontWeight: 700, letterSpacing: ".08em", margin: "4px 0 8px" };
const title: React.CSSProperties = { fontSize: "clamp(32px,5vw,48px)", fontWeight: 500, margin: 0 };
const emerald: React.CSSProperties = { minHeight: 48, border: 0, borderRadius: 8, padding: "0 18px", display: "inline-flex", alignItems: "center", gap: 8, background: "var(--color-emerald-900)", color: "var(--color-cream-100)", fontWeight: 700 };
const gold: React.CSSProperties = { ...emerald, background: "var(--color-gold-500)", color: "#fff" };
const stats: React.CSSProperties = { display: "grid", gridTemplateColumns: "repeat(auto-fit,minmax(190px,1fr))", gap: 14, margin: "24px 0" };
const stat: React.CSSProperties = { display: "grid", gap: 6, padding: 18, background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 10 };
const grid: React.CSSProperties = { display: "grid", gridTemplateColumns: "repeat(auto-fit,minmax(300px,1fr))", gap: 16 };
const card: React.CSSProperties = { display: "grid", gap: 12, padding: 20, background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 12 };
const requestCard: React.CSSProperties = { padding: 18, background: "#fff", border: "1px solid var(--color-gold-500)", borderRadius: 12 };
const row: React.CSSProperties = { display: "flex", justifyContent: "space-between", alignItems: "center", gap: 8 };
const rate: React.CSSProperties = { padding: "4px 8px", borderRadius: 99, background: "var(--color-gold-50)", color: "var(--color-gold-800)", fontSize: 12 };
const badge: React.CSSProperties = { padding: "4px 8px", borderRadius: 99, fontSize: 11, fontWeight: 700 };
const applyRow: React.CSSProperties = { display: "flex", alignItems: "center", gap: 8, flexWrap: "wrap", color: "var(--color-warm-500)", fontSize: 13 };
const code: React.CSSProperties = { padding: "3px 8px", borderRadius: 6, background: "var(--color-cream-200)", fontFamily: "ui-monospace, monospace", fontSize: 12 };
const info: React.CSSProperties = { display: "flex", gap: 8, alignItems: "center", margin: 0, color: "var(--color-warm-500)", fontSize: 13 };
const active: React.CSSProperties = { padding: "4px 8px", borderRadius: 99, background: "var(--color-emerald-50)", color: "var(--color-emerald-900)", fontSize: 12 };
const ghost: React.CSSProperties = { border: 0, background: "transparent", display: "inline-flex", alignItems: "center", gap: 4, color: "var(--color-warm-500)" };
const ghostDanger: React.CSSProperties = { minHeight: 40, border: "1px solid var(--color-danger-600)", borderRadius: 8, padding: "0 14px", background: "transparent", display: "inline-flex", alignItems: "center", gap: 4, color: "var(--color-danger-600)", fontWeight: 600 };
const rejectInput: React.CSSProperties = { minHeight: 40, width: "100%", border: "1px solid var(--color-cream-500)", borderRadius: 8, padding: "0 12px", background: "#fff", font: "inherit" };
const empty: React.CSSProperties = { minHeight: 280, display: "grid", placeItems: "center", alignContent: "center", gap: 12, border: "1px dashed var(--color-cream-400)", borderRadius: 12 };
const requestFlag: React.CSSProperties = { display: "flex", alignItems: "center", gap: 6, margin: 0, padding: "6px 10px", borderRadius: 8, background: "var(--color-gold-50)", color: "var(--color-gold-800)", fontSize: 12, fontWeight: 600 };
