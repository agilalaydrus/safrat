"use client";

import { useEffect, useState } from "react";
import { IconCalendarWeek, IconPlane, IconPlus, IconSchool, IconTrash, IconUsers } from "@tabler/icons-react";
import type { AgendaItem } from "@hajj-saas/proto-gen/hajj/v1/agenda_pb";
import type { Branch } from "@hajj-saas/proto-gen/hajj/v1/branch_pb";
import { agendaClient, branchClient, seasonClient } from "@/lib/rpc";
import EventFormDialog from "./EventFormDialog";

const KIND_LABEL: Record<string, string> = { INTERNAL: "Internal", MANASIK: "Manasik", DEPARTURE: "Keberangkatan", RETURN: "Kepulangan" };
const KIND_COLOR: Record<string, string> = {
  INTERNAL: "var(--color-emerald-800)", MANASIK: "var(--color-gold-800)",
  DEPARTURE: "var(--color-warm-700)", RETURN: "var(--color-warm-700)",
};
const KIND_ICON: Record<string, typeof IconCalendarWeek> = { INTERNAL: IconUsers, MANASIK: IconSchool, DEPARTURE: IconPlane, RETURN: IconPlane };

const dayKey = (d: Date) => d.toISOString().slice(0, 10);
const dayLabel = (d: Date) => d.toLocaleDateString("id-ID", { weekday: "long", day: "2-digit", month: "long", year: "numeric" });

export default function AgendaDashboard() {
  const [seasons, setSeasons] = useState<{ id: string; name: string; isActive: boolean }[]>([]);
  const [seasonId, setSeasonId] = useState("");
  const [branches, setBranches] = useState<Branch[]>([]);
  const [branchId, setBranchId] = useState("");
  const [items, setItems] = useState<AgendaItem[]>([]);
  const [notice, setNotice] = useState("");
  const [formOpen, setFormOpen] = useState(false);
  const [editItem, setEditItem] = useState<AgendaItem | undefined>();

  useEffect(() => {
    seasonClient.listSeasons({}).then((r) => {
      setSeasons(r.seasons);
      setSeasonId(r.seasons.find((s) => s.isActive)?.id ?? r.seasons[0]?.id ?? "");
    }).catch(() => setNotice("Gagal memuat musim."));
    // Branches are a paid-plan feature; an operator without it simply gets an
    // empty list here and the filter stays hidden — no error shown for that.
    branchClient.listBranches({}).then((r) => setBranches(r.branches)).catch(() => setBranches([]));
  }, []);

  const refresh = () => {
    if (!seasonId) return;
    agendaClient.listAgenda({ seasonId, branchId })
      .then((r) => setItems(r.items))
      .catch((caught) => setNotice(caught instanceof Error ? caught.message : "Gagal memuat agenda."));
  };
  useEffect(refresh, [seasonId, branchId]);

  const removeEvent = async (item: AgendaItem) => {
    if (!window.confirm(`Hapus acara "${item.title}"?`)) return;
    try { await agendaClient.deleteAgendaEvent({ eventId: item.id }); refresh(); }
    catch (caught) { setNotice(caught instanceof Error ? caught.message : "Gagal menghapus acara."); }
  };

  const groups = new Map<string, { date: Date; items: AgendaItem[] }>();
  for (const item of items) {
    if (!item.startsAt) continue;
    const date = item.startsAt.toDate();
    const key = dayKey(date);
    if (!groups.has(key)) groups.set(key, { date, items: [] });
    groups.get(key)!.items.push(item);
  }
  const sortedGroups = [...groups.values()].sort((a, b) => a.date.getTime() - b.date.getTime());

  return (
    <main style={page}>
      <header>
        <p style={eyebrow}>KALENDER GABUNGAN</p>
        <h1 style={{ margin: 0, fontSize: 32 }}>Agenda</h1>
        <p style={{ margin: "4px 0 0", color: "var(--color-warm-500)" }}>
          Manasik, keberangkatan &amp; kepulangan kloter, dan acara internal dalam satu linimasa.
        </p>
      </header>
      <div style={{ marginTop: 12, display: "flex", gap: 8, flexWrap: "wrap" }}>
        <select value={seasonId} onChange={(e) => setSeasonId(e.target.value)} style={seasonSelect}>
          {seasons.map((s) => <option key={s.id} value={s.id}>{s.name}{s.isActive ? " (aktif)" : ""}</option>)}
        </select>
        {branches.length > 0 && (
          <select value={branchId} onChange={(e) => setBranchId(e.target.value)} style={seasonSelect}>
            <option value="">Semua cabang</option>
            {branches.map((b) => <option key={b.id} value={b.id}>{b.name}</option>)}
          </select>
        )}
        <button type="button" onClick={() => { setEditItem(undefined); setFormOpen(true); }} style={primaryBtn}>
          <IconPlus size={14} /> Acara Baru
        </button>
      </div>
      {branchId && (
        <p style={{ margin: "8px 0 0", fontSize: 12, color: "var(--color-warm-400)" }}>
          Manasik dan keberangkatan/kepulangan selalu tampil — keduanya bukan milik cabang manapun. Saringan cabang hanya berlaku untuk acara internal.
        </p>
      )}
      {notice && <p style={{ color: "var(--color-danger-600)" }}>{notice}</p>}
      <div className="gold-divider" />

      {sortedGroups.length ? (
        <div style={{ display: "grid", gap: 20, marginTop: 12 }}>
          {sortedGroups.map((group) => (
            <section key={dayKey(group.date)} style={card}>
              <h2 style={sectionTitle}>{dayLabel(group.date)}</h2>
              <div style={{ display: "grid", gap: 8, marginTop: 12 }}>
                {group.items
                  .sort((a, b) => (a.startsAt?.toDate().getTime() ?? 0) - (b.startsAt?.toDate().getTime() ?? 0))
                  .map((item) => {
                    const Icon = KIND_ICON[item.kind] ?? IconCalendarWeek;
                    return (
                      <div key={item.id} style={row}>
                        <div style={iconBadge}><Icon size={16} color={KIND_COLOR[item.kind]} /></div>
                        <div style={{ flex: 1, minWidth: 0 }}>
                          <div style={{ display: "flex", alignItems: "center", gap: 8, flexWrap: "wrap" }}>
                            <strong>{item.title}</strong>
                            <span style={{ ...kindBadge, color: KIND_COLOR[item.kind] }}>{KIND_LABEL[item.kind] ?? item.kind}</span>
                            {item.kloterCode && <span style={miniBadge}>{item.kloterCode}</span>}
                            {item.branchName && <span style={miniBadge}>{item.branchName}</span>}
                          </div>
                          <p style={{ margin: "2px 0 0", fontSize: 12, color: "var(--color-warm-500)" }}>
                            {item.startsAt?.toDate().toLocaleTimeString("id-ID", { hour: "2-digit", minute: "2-digit" })}
                            {item.endsAt && ` – ${item.endsAt.toDate().toLocaleTimeString("id-ID", { hour: "2-digit", minute: "2-digit" })}`}
                            {item.location && ` · ${item.location}`}
                          </p>
                        </div>
                        {item.kind === "INTERNAL" && (
                          <div style={{ display: "flex", gap: 6 }}>
                            <button type="button" onClick={() => { setEditItem(item); setFormOpen(true); }} style={ghostBtn}>Ubah</button>
                            <button type="button" onClick={() => void removeEvent(item)} style={iconBtnDanger} aria-label="Hapus"><IconTrash size={14} /></button>
                          </div>
                        )}
                      </div>
                    );
                  })}
              </div>
            </section>
          ))}
        </div>
      ) : <p style={{ color: "var(--color-warm-400)", fontSize: 13, marginTop: 12 }}>Belum ada kegiatan pada musim ini.</p>}

      <EventFormDialog
        open={formOpen}
        seasonId={seasonId}
        branches={branches}
        event={editItem ? { id: editItem.id, title: editItem.title, branchId: editItem.branchId, location: editItem.location, startsAt: editItem.startsAt, endsAt: editItem.endsAt, notes: editItem.notes } : undefined}
        onClose={() => setFormOpen(false)}
        onSaved={refresh}
      />
    </main>
  );
}

const page: React.CSSProperties = { maxWidth: 1000, margin: "0 auto", padding: "32px 24px", display: "grid", gap: 16 };
const eyebrow: React.CSSProperties = { margin: "0 0 6px", fontSize: 11, fontWeight: 700, color: "var(--color-gold-800)", letterSpacing: ".08em" };
const seasonSelect: React.CSSProperties = { minHeight: 40, border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 10px", font: "inherit", background: "#fff" };
const card: React.CSSProperties = { background: "var(--color-cream-200)", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 20 };
const sectionTitle: React.CSSProperties = { margin: 0, fontSize: 16 };
const row: React.CSSProperties = { display: "flex", alignItems: "center", gap: 10, padding: "10px 12px", background: "#fff", border: "1px solid var(--color-cream-300)", borderRadius: 8, flexWrap: "wrap" };
const iconBadge: React.CSSProperties = { width: 32, height: 32, borderRadius: 8, background: "var(--color-cream-200)", display: "grid", placeItems: "center", flexShrink: 0 };
const kindBadge: React.CSSProperties = { fontSize: 11, fontWeight: 700 };
const miniBadge: React.CSSProperties = { padding: "2px 6px", borderRadius: 99, background: "var(--color-cream-200)", color: "var(--color-warm-500)", fontSize: 10, fontWeight: 700 };
const ghostBtn: React.CSSProperties = { minHeight: 32, border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 10px", background: "transparent", color: "var(--color-emerald-900)", fontSize: 12, fontWeight: 600, display: "inline-flex", alignItems: "center", gap: 6 };
const primaryBtn: React.CSSProperties = { minHeight: 36, border: 0, borderRadius: 8, padding: "0 14px", background: "var(--color-gold-500)", color: "#fff", fontWeight: 700, display: "inline-flex", alignItems: "center", gap: 6, fontSize: 13 };
const iconBtnDanger: React.CSSProperties = { width: 28, height: 28, border: "1px solid var(--color-cream-400)", borderRadius: 6, background: "#fff", color: "var(--color-danger-600)", display: "grid", placeItems: "center" };
