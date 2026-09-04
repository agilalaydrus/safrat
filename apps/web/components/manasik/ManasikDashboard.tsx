"use client";

import { useEffect, useState } from "react";
import { IconClipboardCheck, IconPlus, IconTrash } from "@tabler/icons-react";
import type { ManasikCurriculum, ManasikSession } from "@hajj-saas/proto-gen/hajj/v1/manasik_pb";
import type { Kloter } from "@hajj-saas/proto-gen/hajj/v1/kloter_pb";
import { manasikClient, seasonClient, kloterClient } from "@/lib/rpc";
import SessionFormDialog from "./SessionFormDialog";
import AttendanceDrawer from "./AttendanceDrawer";

const STATUS_LABEL: Record<string, string> = { SCHEDULED: "Terjadwal", COMPLETED: "Selesai", CANCELLED: "Dibatalkan" };
const STATUS_COLOR: Record<string, string> = { SCHEDULED: "var(--color-gold-800)", COMPLETED: "var(--color-emerald-800)", CANCELLED: "var(--color-danger-600)" };

export default function ManasikDashboard() {
  const [seasons, setSeasons] = useState<{ id: string; name: string; isActive: boolean }[]>([]);
  const [seasonId, setSeasonId] = useState("");
  const [curricula, setCurricula] = useState<ManasikCurriculum[]>([]);
  const [sessions, setSessions] = useState<ManasikSession[]>([]);
  const [kloters, setKloters] = useState<Kloter[]>([]);
  const [notice, setNotice] = useState("");
  const [curriculumForm, setCurriculumForm] = useState({ title: "", description: "" });
  const [sessionFormOpen, setSessionFormOpen] = useState(false);
  const [editSession, setEditSession] = useState<ManasikSession | undefined>();
  const [attendanceSession, setAttendanceSession] = useState<ManasikSession | undefined>();

  useEffect(() => {
    seasonClient.listSeasons({}).then((r) => {
      setSeasons(r.seasons);
      setSeasonId(r.seasons.find((s) => s.isActive)?.id ?? r.seasons[0]?.id ?? "");
    }).catch(() => setNotice("Gagal memuat musim."));
  }, []);

  const refresh = () => {
    if (!seasonId) return;
    Promise.all([
      manasikClient.listManasikCurricula({ seasonId }).then((r) => setCurricula(r.curricula)),
      manasikClient.listManasikSessions({ seasonId }).then((r) => setSessions(r.sessions)),
      kloterClient.listKloters({ seasonId }).then((r) => setKloters(r.kloters)),
    ]).catch(() => setNotice("Gagal memuat data manasik."));
  };
  useEffect(refresh, [seasonId]);

  const addCurriculum = async () => {
    if (!curriculumForm.title.trim()) { setNotice("Judul kurikulum wajib diisi."); return; }
    try {
      await manasikClient.createManasikCurriculum({ seasonId, title: curriculumForm.title.trim(), description: curriculumForm.description.trim(), sortOrder: curricula.length });
      setCurriculumForm({ title: "", description: "" });
      refresh();
    } catch (caught) { setNotice(caught instanceof Error ? caught.message : "Gagal menambah kurikulum."); }
  };

  const removeCurriculum = async (c: ManasikCurriculum) => {
    if (!window.confirm(`Hapus topik "${c.title}"?`)) return;
    try { await manasikClient.deleteManasikCurriculum({ curriculumId: c.id }); refresh(); }
    catch (caught) { setNotice(caught instanceof Error ? caught.message : "Gagal menghapus kurikulum."); }
  };

  const setSessionStatus = async (session: ManasikSession, status: string) => {
    try { await manasikClient.updateManasikSessionStatus({ sessionId: session.id, status }); refresh(); }
    catch (caught) { setNotice(caught instanceof Error ? caught.message : "Gagal mengubah status sesi."); }
  };

  const removeSession = async (session: ManasikSession) => {
    if (!window.confirm(`Hapus sesi "${session.title}"? Data absensinya juga akan hilang.`)) return;
    try { await manasikClient.deleteManasikSession({ sessionId: session.id }); refresh(); }
    catch (caught) { setNotice(caught instanceof Error ? caught.message : "Gagal menghapus sesi."); }
  };

  const upcoming = [...sessions].sort((a, b) => (a.scheduledAt?.toDate().getTime() ?? 0) - (b.scheduledAt?.toDate().getTime() ?? 0));

  return (
    <main style={page}>
      <header>
        <p style={eyebrow}>PELATIHAN RITUAL</p>
        <h1 style={{ margin: 0, fontSize: 32 }}>Manasik</h1>
        <p style={{ margin: "4px 0 0", color: "var(--color-warm-500)" }}>Kurikulum, jadwal sesi, dan absensi jamaah.</p>
      </header>
      <div style={{ marginTop: 12 }}>
        <select value={seasonId} onChange={(e) => setSeasonId(e.target.value)} style={seasonSelect}>
          {seasons.map((s) => <option key={s.id} value={s.id}>{s.name}{s.isActive ? " (aktif)" : ""}</option>)}
        </select>
      </div>
      {notice && <p style={{ color: "var(--color-danger-600)" }}>{notice}</p>}
      <div className="gold-divider" />

      <section style={card}>
        <h2 style={sectionTitle}>Kurikulum</h2>
        {curricula.length ? (
          <div style={{ display: "grid", gap: 6, marginBottom: 12 }}>
            {curricula.map((c) => (
              <div key={c.id} style={row}>
                <div style={{ flex: 1, minWidth: 0 }}>
                  <strong>{c.title}</strong>
                  {c.description && <p style={{ margin: "2px 0 0", fontSize: 12, color: "var(--color-warm-500)" }}>{c.description}</p>}
                </div>
                <button type="button" onClick={() => void removeCurriculum(c)} style={iconBtnDanger} aria-label="Hapus"><IconTrash size={14} /></button>
              </div>
            ))}
          </div>
        ) : <p style={{ color: "var(--color-warm-400)", fontSize: 13, marginBottom: 12 }}>Belum ada topik kurikulum.</p>}
        <div style={inlineForm}>
          <input placeholder="Judul topik (mis. Tawaf)" value={curriculumForm.title} onChange={(e) => setCurriculumForm({ ...curriculumForm, title: e.target.value })} style={input} />
          <input placeholder="Deskripsi (opsional)" value={curriculumForm.description} onChange={(e) => setCurriculumForm({ ...curriculumForm, description: e.target.value })} style={input} />
          <button type="button" onClick={() => void addCurriculum()} style={ghostBtn}><IconPlus size={14} /> Tambah Topik</button>
        </div>
      </section>

      <section style={card}>
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
          <h2 style={sectionTitle}>Sesi Manasik ({sessions.length})</h2>
          <button type="button" onClick={() => { setEditSession(undefined); setSessionFormOpen(true); }} style={primaryBtn}><IconPlus size={14} /> Sesi Baru</button>
        </div>
        {upcoming.length ? (
          <div style={{ display: "grid", gap: 8, marginTop: 12 }}>
            {upcoming.map((session) => (
              <div key={session.id} style={sessionRow}>
                <div style={{ flex: 1, minWidth: 0 }}>
                  <div style={{ display: "flex", alignItems: "center", gap: 8, flexWrap: "wrap" }}>
                    <strong>{session.title}</strong>
                    <span style={{ ...statusBadge, color: STATUS_COLOR[session.status] }}>{STATUS_LABEL[session.status] ?? session.status}</span>
                    {session.kloterCode && <span style={miniBadge}>{session.kloterCode}</span>}
                  </div>
                  <p style={{ margin: "2px 0 0", fontSize: 12, color: "var(--color-warm-500)" }}>
                    {session.scheduledAt?.toDate().toLocaleString("id-ID", { day: "2-digit", month: "short", year: "numeric", hour: "2-digit", minute: "2-digit" })}
                    {session.location && ` · ${session.location}`}{session.instructorName && ` · ${session.instructorName}`}
                  </p>
                </div>
                <div style={{ display: "flex", gap: 6, flexWrap: "wrap" }}>
                  <button type="button" onClick={() => setAttendanceSession(session)} style={ghostBtn}><IconClipboardCheck size={14} /> Absensi</button>
                  <button type="button" onClick={() => { setEditSession(session); setSessionFormOpen(true); }} style={ghostBtn}>Ubah</button>
                  {session.status === "SCHEDULED" && <button type="button" onClick={() => void setSessionStatus(session, "COMPLETED")} style={ghostBtn}>Selesai</button>}
                  <button type="button" onClick={() => void removeSession(session)} style={iconBtnDanger} aria-label="Hapus"><IconTrash size={14} /></button>
                </div>
              </div>
            ))}
          </div>
        ) : <p style={{ color: "var(--color-warm-400)", fontSize: 13, marginTop: 12 }}>Belum ada sesi. Tambahkan sesi pertama.</p>}
      </section>

      <SessionFormDialog open={sessionFormOpen} seasonId={seasonId} curricula={curricula} kloters={kloters} session={editSession} onClose={() => setSessionFormOpen(false)} onSaved={refresh} />
      <AttendanceDrawer open={!!attendanceSession} session={attendanceSession} seasonId={seasonId} onClose={() => setAttendanceSession(undefined)} />
    </main>
  );
}

const page: React.CSSProperties = { maxWidth: 1000, margin: "0 auto", padding: "32px 24px", display: "grid", gap: 16 };
const eyebrow: React.CSSProperties = { margin: "0 0 6px", fontSize: 11, fontWeight: 700, color: "var(--color-gold-800)", letterSpacing: ".08em" };
const seasonSelect: React.CSSProperties = { minHeight: 40, border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 10px", font: "inherit", background: "#fff" };
const card: React.CSSProperties = { background: "var(--color-cream-200)", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 20 };
const sectionTitle: React.CSSProperties = { margin: 0, fontSize: 16 };
const row: React.CSSProperties = { display: "flex", alignItems: "center", gap: 10, padding: "8px 10px", background: "#fff", border: "1px solid var(--color-cream-300)", borderRadius: 8 };
const sessionRow: React.CSSProperties = { display: "flex", alignItems: "center", gap: 10, padding: "10px 12px", background: "#fff", border: "1px solid var(--color-cream-300)", borderRadius: 8, flexWrap: "wrap" };
const statusBadge: React.CSSProperties = { fontSize: 11, fontWeight: 700 };
const miniBadge: React.CSSProperties = { padding: "2px 6px", borderRadius: 99, background: "var(--color-cream-200)", color: "var(--color-warm-500)", fontSize: 10, fontWeight: 700 };
const inlineForm: React.CSSProperties = { display: "grid", gap: 8, padding: 12, background: "#fff", border: "1px solid var(--color-cream-300)", borderRadius: 8 };
const input: React.CSSProperties = { minHeight: 38, border: "1px solid var(--color-cream-400)", borderRadius: 6, padding: "0 8px", font: "inherit", background: "#fff" };
const ghostBtn: React.CSSProperties = { minHeight: 32, border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 10px", background: "transparent", color: "var(--color-emerald-900)", fontSize: 12, fontWeight: 600, display: "inline-flex", alignItems: "center", gap: 6 };
const primaryBtn: React.CSSProperties = { minHeight: 36, border: 0, borderRadius: 8, padding: "0 14px", background: "var(--color-gold-500)", color: "#fff", fontWeight: 700, display: "inline-flex", alignItems: "center", gap: 6, fontSize: 13 };
const iconBtnDanger: React.CSSProperties = { width: 28, height: 28, border: "1px solid var(--color-cream-400)", borderRadius: 6, background: "#fff", color: "var(--color-danger-600)", display: "grid", placeItems: "center" };
