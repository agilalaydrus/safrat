"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { IconAlertTriangle, IconSend } from "@tabler/icons-react";
import type { SupportTicket, SupportTicketMessage } from "@hajj-saas/proto-gen/hajj/v1/support_pb";
import { DetailDrawer } from "@/components/ui/DetailDrawer";
import { EmptyState } from "@/components/ui/EmptyState";
import { PageHeader } from "@/components/ui/PageHeader";
import { platformClient } from "@/lib/rpc";

const STATUS_LABEL: Record<string, string> = { OPEN: "Menunggu", IN_PROGRESS: "Diproses", RESOLVED: "Selesai", CLOSED: "Ditutup" };
const PRIORITY_LABEL: Record<string, string> = { LOW: "Rendah", MEDIUM: "Sedang", HIGH: "Tinggi", URGENT: "Mendesak" };
const STATUSES: [string, string][] = [["", "Semua"], ["OPEN", "Menunggu"], ["IN_PROGRESS", "Diproses"], ["RESOLVED", "Selesai"], ["CLOSED", "Ditutup"]];
const PRIORITIES: [string, string][] = [["", "Semua"], ["URGENT", "Mendesak"], ["HIGH", "Tinggi"], ["MEDIUM", "Sedang"], ["LOW", "Rendah"]];
// OPEN/IN_PROGRESS/RESOLVED only — CLOSED is deliberately not offered here.
// It belongs to the operator's own decision (SupportService.CloseSupportTicket),
// not the platform's; the RPC itself refuses the value too, this is just so
// the screen never suggests it is possible.
const SETTABLE_STATUSES: [string, string][] = [["OPEN", "Menunggu"], ["IN_PROGRESS", "Diproses"], ["RESOLVED", "Selesai"]];

const timeOf = (at?: { toDate(): Date }) =>
  at ? at.toDate().toLocaleString("id-ID", { day: "2-digit", month: "short", hour: "2-digit", minute: "2-digit" }) : "";

export default function SupportTab() {
  const [tickets, setTickets] = useState<SupportTicket[]>([]);
  const [status, setStatus] = useState("");
  const [priority, setPriority] = useState("");
  const [loading, setLoading] = useState(true);
  const [failure, setFailure] = useState("");

  const [selected, setSelected] = useState<SupportTicket>();
  const [messages, setMessages] = useState<SupportTicketMessage[]>([]);
  const [reply, setReply] = useState("");
  const [saving, setSaving] = useState(false);

  const load = useCallback(() => {
    setLoading(true);
    setFailure("");
    platformClient
      .listAllSupportTickets({ status, priority })
      .then((r) => setTickets(r.tickets))
      .catch(() => setFailure("Gagal memuat tiket dukungan."))
      .finally(() => setLoading(false));
  }, [status, priority]);

  useEffect(load, [load]);

  const openTicket = (ticket: SupportTicket) => {
    setSelected(ticket);
    platformClient
      .getSupportTicketAsPlatform({ ticketId: ticket.id })
      .then((r) => { setSelected(r.ticket); setMessages(r.messages); })
      .catch(() => setFailure("Gagal memuat detail tiket."));
  };

  const sendReply = async () => {
    if (!selected || !reply.trim()) return;
    setSaving(true);
    try {
      await platformClient.replyToSupportTicketAsPlatform({ ticketId: selected.id, body: reply.trim() });
      setReply("");
      openTicket(selected);
      load();
    } catch (caught) {
      setFailure(caught instanceof Error ? caught.message : "Gagal mengirim balasan.");
    } finally {
      setSaving(false);
    }
  };

  const setSelectedStatus = async (next: string) => {
    if (!selected) return;
    setSaving(true);
    try {
      const updated = await platformClient.setSupportTicketStatus({ ticketId: selected.id, status: next });
      setSelected(updated);
      load();
    } catch (caught) {
      setFailure(caught instanceof Error ? caught.message : "Gagal mengubah status.");
    } finally {
      setSaving(false);
    }
  };

  return (
    <section className="tw-screen">
      <PageHeader
        eyebrow="TAWAFIQHUB / SUPPORT"
        title="Support"
        subtitle="Kotak masuk tiket dukungan dari seluruh travel. Balasan di sini langsung muncul di percakapan travel yang bersangkutan."
      />

      <div className="tw-filter-bar">
        <div className="tw-segmented" role="group" aria-label="Status">
          {STATUSES.map(([value, label]) => (
            <button key={value || "all"} type="button" onClick={() => setStatus(value)} aria-pressed={status === value}
              className={status === value ? "tw-segmented__item is-active" : "tw-segmented__item"}>{label}</button>
          ))}
        </div>
        <div className="tw-segmented" role="group" aria-label="Prioritas">
          {PRIORITIES.map(([value, label]) => (
            <button key={value || "all"} type="button" onClick={() => setPriority(value)} aria-pressed={priority === value}
              className={priority === value ? "tw-segmented__item is-active" : "tw-segmented__item"}>{label}</button>
          ))}
        </div>
      </div>

      {failure && <p className="tw-inline-alert" data-tone="danger"><IconAlertTriangle size={16} />{failure}</p>}
      {loading && <p className="tw-note">Memuat…</p>}

      {!loading && tickets.length === 0 && (
        <EmptyState
          title="Tidak ada tiket yang cocok"
          cause="Saringan status dan prioritas sedang berlaku bersamaan."
          nextStep="Longgarkan salah satunya — kotak masuk ini benar-benar kosong, bukan gagal memuat."
        />
      )}

      {tickets.length > 0 && (
        <div className="tw-table-wrap">
          <table className="tw-table">
            <thead>
              <tr>{["Dibuka", "Travel", "Judul", "Prioritas", "Status", ""].map((head) => <th key={head}>{head}</th>)}</tr>
            </thead>
            <tbody>
              {tickets.map((t) => (
                <tr key={t.id} onClick={() => openTicket(t)} style={{ cursor: "pointer" }}>
                  <td style={{ whiteSpace: "nowrap" }}>{timeOf(t.createdAt)}</td>
                  <td>
                    <Link href={`/admin/tenant/${t.operatorId}`} onClick={(e) => e.stopPropagation()} style={link}>
                      {t.operatorName || t.operatorId}
                    </Link>
                  </td>
                  <td style={{ fontWeight: 600 }}>{t.subject}</td>
                  <td>{PRIORITY_LABEL[t.priority] ?? t.priority}</td>
                  <td>
                    {STATUS_LABEL[t.status] ?? t.status}
                    {t.responseOverdue && <span style={{ color: "var(--color-danger-600)", fontWeight: 700 }}> · lewat target</span>}
                  </td>
                  <td style={{ color: "var(--color-warm-400)" }}>Buka →</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <DetailDrawer open={!!selected} onClose={() => setSelected(undefined)} title={selected?.subject ?? "Tiket"}
        subtitle={selected ? <>
          {selected.operatorName || selected.operatorId} · Prioritas {PRIORITY_LABEL[selected.priority] ?? selected.priority}
          {selected.responseOverdue && <span style={{ color: "var(--color-danger-600)", fontWeight: 700 }}> · lewat target respons</span>}
        </> : undefined}>
        {selected && (
          <div style={{ display: "grid", gap: 16 }}>
            {selected.status !== "CLOSED" && (
              <div className="tw-segmented" role="group" aria-label="Ubah status">
                {SETTABLE_STATUSES.map(([value, label]) => (
                  <button key={value} type="button" disabled={saving} onClick={() => void setSelectedStatus(value)}
                    aria-pressed={selected.status === value}
                    className={selected.status === value ? "tw-segmented__item is-active" : "tw-segmented__item"}>{label}</button>
                ))}
              </div>
            )}
            <div style={{ display: "grid", gap: 10 }}>
              {messages.map((m) => (
                <div key={m.id} style={{ ...messageBubble, ...(m.authorIsPlatform ? messageBubblePlatform : {}) }}>
                  <div style={{ display: "flex", justifyContent: "space-between", gap: 8 }}>
                    <strong style={{ fontSize: 12 }}>{m.authorIsPlatform ? `${m.authorName} · TawafiqHub` : m.authorName}</strong>
                    <span style={{ fontSize: 11, color: "var(--color-warm-400)" }}>{timeOf(m.createdAt)}</span>
                  </div>
                  <p style={{ margin: "4px 0 0", fontSize: 13, whiteSpace: "pre-wrap" }}>{m.body}</p>
                </div>
              ))}
            </div>
            {selected.status !== "CLOSED" ? (
              <div style={{ display: "flex", gap: 8 }}>
                <input value={reply} onChange={(e) => setReply(e.target.value)} placeholder="Balas sebagai TawafiqHub..."
                  style={input} onKeyDown={(e) => { if (e.key === "Enter") void sendReply(); }} />
                <button type="button" onClick={() => void sendReply()} disabled={saving} className="tw-btn tw-btn--emerald"><IconSend size={14} /></button>
              </div>
            ) : (
              <p className="tw-note">Tiket ini sudah ditutup oleh travel yang bersangkutan.</p>
            )}
          </div>
        )}
      </DetailDrawer>
    </section>
  );
}

const link: React.CSSProperties = { color: "var(--color-emerald-900)", fontWeight: 700, textDecoration: "none" };
const messageBubble: React.CSSProperties = { background: "var(--color-cream-100)", border: "1px solid var(--color-cream-300)", borderRadius: 8, padding: "10px 12px" };
const messageBubblePlatform: React.CSSProperties = { background: "var(--color-emerald-100)", borderColor: "var(--color-emerald-800)" };
const input: React.CSSProperties = { minHeight: 40, border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 10px", font: "inherit", background: "#fff", flex: 1 };
