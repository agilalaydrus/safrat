"use client";

import { useEffect, useState } from "react";
import { IconAlertTriangle, IconPlus, IconSend } from "@tabler/icons-react";
import type { SupportTicket, SupportTicketMessage } from "@hajj-saas/proto-gen/hajj/v1/support_pb";
import { supportClient } from "@/lib/rpc";

const PRIORITY_LABEL: Record<string, string> = { LOW: "Rendah", MEDIUM: "Sedang", HIGH: "Tinggi", URGENT: "Mendesak" };
const PRIORITY_COLOR: Record<string, string> = { LOW: "var(--color-warm-500)", MEDIUM: "var(--color-gold-800)", HIGH: "var(--color-danger-600)", URGENT: "var(--color-danger-600)" };
const STATUS_LABEL: Record<string, string> = { OPEN: "Menunggu", IN_PROGRESS: "Diproses", RESOLVED: "Selesai", CLOSED: "Ditutup" };
const RESPONSE_TARGET_TEXT: Record<string, string> = { LOW: "3 hari", MEDIUM: "24 jam", HIGH: "4 jam", URGENT: "1 jam" };

export default function SupportDashboard() {
  const [tickets, setTickets] = useState<SupportTicket[]>([]);
  const [selected, setSelected] = useState<SupportTicket>();
  const [messages, setMessages] = useState<SupportTicketMessage[]>([]);
  const [loading, setLoading] = useState(true);
  const [notice, setNotice] = useState("");
  const [createOpen, setCreateOpen] = useState(false);
  const [form, setForm] = useState({ subject: "", priority: "MEDIUM", body: "" });
  const [reply, setReply] = useState("");
  const [saving, setSaving] = useState(false);

  const refreshList = () => {
    setLoading(true);
    supportClient.listMySupportTickets({}).then((r) => setTickets(r.tickets)).catch(() => setNotice("Gagal memuat daftar tiket.")).finally(() => setLoading(false));
  };
  useEffect(refreshList, []);

  const openTicket = (ticket: SupportTicket) => {
    setSelected(ticket);
    supportClient.getSupportTicket({ ticketId: ticket.id }).then((r) => { setSelected(r.ticket); setMessages(r.messages); }).catch(() => setNotice("Gagal memuat tiket."));
  };

  const createTicket = async () => {
    if (!form.subject.trim() || !form.body.trim()) { setNotice("Judul dan penjelasan wajib diisi."); return; }
    setSaving(true);
    try {
      const ticket = await supportClient.createSupportTicket({ subject: form.subject.trim(), priority: form.priority, body: form.body.trim() });
      setCreateOpen(false);
      setForm({ subject: "", priority: "MEDIUM", body: "" });
      refreshList();
      openTicket(ticket);
    } catch (caught) {
      setNotice(caught instanceof Error ? caught.message : "Gagal membuat tiket.");
    } finally {
      setSaving(false);
    }
  };

  const sendReply = async () => {
    if (!selected || !reply.trim()) return;
    setSaving(true);
    try {
      await supportClient.addSupportTicketMessage({ ticketId: selected.id, body: reply.trim() });
      setReply("");
      openTicket(selected);
    } catch (caught) {
      setNotice(caught instanceof Error ? caught.message : "Gagal mengirim balasan.");
    } finally {
      setSaving(false);
    }
  };

  const closeTicket = async () => {
    if (!selected) return;
    if (!window.confirm("Tutup tiket ini?")) return;
    try {
      const updated = await supportClient.closeSupportTicket({ ticketId: selected.id });
      setSelected(updated);
      refreshList();
    } catch (caught) {
      setNotice(caught instanceof Error ? caught.message : "Gagal menutup tiket.");
    }
  };

  return (
    <main style={page}>
      <header style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-end", flexWrap: "wrap", gap: 12 }}>
        <div>
          <p style={eyebrow}>BANTUAN</p>
          <h1 style={{ margin: 0, fontSize: 32 }}>Tiket Dukungan</h1>
          <p style={{ margin: "4px 0 0", color: "var(--color-warm-500)" }}>Kirim pertanyaan atau kendala ke tim TawafiqHub.</p>
        </div>
        <button type="button" onClick={() => setCreateOpen(true)} style={primaryBtn}><IconPlus size={14} /> Tiket Baru</button>
      </header>
      {notice && <p style={{ color: "var(--color-danger-600)" }}>{notice}</p>}
      <div className="gold-divider" />

      <div style={layout}>
        <section style={{ ...card, padding: 0, overflow: "hidden" }}>
          {loading ? <p style={{ padding: 20, color: "var(--color-warm-400)" }}>Memuat...</p> : tickets.length ? (
            <div>
              {tickets.map((t) => (
                <button key={t.id} type="button" onClick={() => openTicket(t)} style={{ ...ticketRow, ...(selected?.id === t.id ? ticketRowActive : {}) }}>
                  <div style={{ flex: 1, minWidth: 0, textAlign: "left" }}>
                    <div style={{ display: "flex", alignItems: "center", gap: 6, flexWrap: "wrap" }}>
                      <strong style={{ fontSize: 13 }}>{t.subject}</strong>
                      {t.responseOverdue && <IconAlertTriangle size={14} color="var(--color-danger-600)" />}
                    </div>
                    <p style={{ margin: "2px 0 0", fontSize: 11, color: "var(--color-warm-400)" }}>
                      {STATUS_LABEL[t.status] ?? t.status} · {t.createdAt?.toDate().toLocaleDateString("id-ID", { day: "2-digit", month: "short" })}
                    </p>
                  </div>
                  <span style={{ ...priorityBadge, color: PRIORITY_COLOR[t.priority] }}>{PRIORITY_LABEL[t.priority] ?? t.priority}</span>
                </button>
              ))}
            </div>
          ) : <p style={{ padding: 20, color: "var(--color-warm-400)", fontSize: 13 }}>Belum ada tiket. Buat yang pertama kalau ada kendala.</p>}
        </section>

        <section style={card}>
          {selected ? (
            <>
              <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start", gap: 8, flexWrap: "wrap" }}>
                <div>
                  <h2 style={{ margin: 0, fontSize: 18 }}>{selected.subject}</h2>
                  <p style={{ margin: "4px 0 0", fontSize: 12, color: "var(--color-warm-500)" }}>
                    Prioritas {PRIORITY_LABEL[selected.priority] ?? selected.priority} · target respons {RESPONSE_TARGET_TEXT[selected.priority] ?? "24 jam"}
                    {selected.responseOverdue && <span style={{ color: "var(--color-danger-600)", fontWeight: 700 }}> · lewat target</span>}
                  </p>
                </div>
                {(selected.status === "OPEN" || selected.status === "IN_PROGRESS") && (
                  <button type="button" onClick={() => void closeTicket()} style={ghostBtn}>Tutup Tiket</button>
                )}
              </div>
              <div style={{ display: "grid", gap: 10, marginTop: 16 }}>
                {messages.map((m) => (
                  <div key={m.id} style={{ ...messageBubble, ...(m.authorIsPlatform ? messageBubblePlatform : {}) }}>
                    <div style={{ display: "flex", justifyContent: "space-between", gap: 8 }}>
                      <strong style={{ fontSize: 12 }}>{m.authorIsPlatform ? `${m.authorName} · TawafiqHub` : m.authorName}</strong>
                      <span style={{ fontSize: 11, color: "var(--color-warm-400)" }}>{m.createdAt?.toDate().toLocaleString("id-ID", { day: "2-digit", month: "short", hour: "2-digit", minute: "2-digit" })}</span>
                    </div>
                    <p style={{ margin: "4px 0 0", fontSize: 13, whiteSpace: "pre-wrap" }}>{m.body}</p>
                  </div>
                ))}
              </div>
              {(selected.status === "OPEN" || selected.status === "IN_PROGRESS") && (
                <div style={{ display: "flex", gap: 8, marginTop: 16 }}>
                  <input value={reply} onChange={(e) => setReply(e.target.value)} placeholder="Tulis balasan..." style={{ ...input, flex: 1 }} onKeyDown={(e) => { if (e.key === "Enter") void sendReply(); }} />
                  <button type="button" onClick={() => void sendReply()} disabled={saving} style={primaryBtn}><IconSend size={14} /></button>
                </div>
              )}
            </>
          ) : <p style={{ color: "var(--color-warm-400)", fontSize: 13 }}>Pilih tiket di sebelah kiri, atau buat yang baru.</p>}
        </section>
      </div>

      {createOpen && (
        <div style={overlay}>
          <div style={dialog}>
            <h2 style={{ margin: "0 0 12px", fontSize: 18 }}>Tiket Baru</h2>
            <div style={{ display: "grid", gap: 12 }}>
              <label style={label1}><span>Judul</span><input style={input} value={form.subject} onChange={(e) => setForm({ ...form, subject: e.target.value })} /></label>
              <label style={label1}>
                <span>Prioritas</span>
                <select style={input} value={form.priority} onChange={(e) => setForm({ ...form, priority: e.target.value })}>
                  {Object.entries(PRIORITY_LABEL).map(([value, l]) => <option key={value} value={value}>{l} — target {RESPONSE_TARGET_TEXT[value]}</option>)}
                </select>
              </label>
              <label style={label1}><span>Penjelasan</span><textarea rows={4} style={{ ...input, height: "auto", padding: 10 }} value={form.body} onChange={(e) => setForm({ ...form, body: e.target.value })} /></label>
            </div>
            <div style={{ display: "flex", gap: 8, marginTop: 16 }}>
              <button type="button" onClick={() => setCreateOpen(false)} style={ghostBtn}>Batal</button>
              <button type="button" onClick={() => void createTicket()} disabled={saving} style={primaryBtn}>{saving ? "Mengirim..." : "Kirim Tiket"}</button>
            </div>
          </div>
        </div>
      )}
    </main>
  );
}

const page: React.CSSProperties = { padding: "24px", display: "grid", gap: 16 };
const eyebrow: React.CSSProperties = { margin: "0 0 6px", fontSize: 11, fontWeight: 700, color: "var(--color-gold-800)", letterSpacing: ".08em" };
const layout: React.CSSProperties = { display: "grid", gridTemplateColumns: "300px 1fr", gap: 16, alignItems: "start" };
const card: React.CSSProperties = { background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 20, minHeight: 300 };
const ticketRow: React.CSSProperties = { display: "flex", alignItems: "center", gap: 8, width: "100%", padding: "12px 16px", border: 0, borderBottom: "1px solid var(--color-cream-200)", background: "transparent", cursor: "pointer" };
const ticketRowActive: React.CSSProperties = { background: "var(--color-cream-100)" };
const priorityBadge: React.CSSProperties = { fontSize: 10, fontWeight: 700, flexShrink: 0 };
const messageBubble: React.CSSProperties = { background: "var(--color-cream-100)", border: "1px solid var(--color-cream-300)", borderRadius: 8, padding: "10px 12px" };
const messageBubblePlatform: React.CSSProperties = { background: "var(--color-emerald-100)", borderColor: "var(--color-emerald-800)" };
const input: React.CSSProperties = { minHeight: 40, border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 10px", font: "inherit", background: "#fff", width: "100%" };
const label1: React.CSSProperties = { display: "flex", flexDirection: "column", gap: 6, fontSize: 13, fontWeight: 600, color: "var(--color-warm-700)" };
const primaryBtn: React.CSSProperties = { minHeight: 40, border: 0, borderRadius: 8, padding: "0 14px", background: "var(--color-gold-500)", color: "#fff", fontWeight: 700, display: "inline-flex", alignItems: "center", justifyContent: "center", gap: 6, fontSize: 13 };
const ghostBtn: React.CSSProperties = { minHeight: 40, border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 14px", background: "transparent", color: "var(--color-emerald-900)", fontSize: 13, fontWeight: 600 };
const overlay: React.CSSProperties = { position: "fixed", inset: 0, background: "rgba(15,23,42,.48)", display: "flex", alignItems: "center", justifyContent: "center", zIndex: 30 };
const dialog: React.CSSProperties = { background: "#fff", borderRadius: 12, padding: 24, width: "min(480px,90vw)" };
