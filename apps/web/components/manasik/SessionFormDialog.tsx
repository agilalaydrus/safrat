"use client";

import { FormEvent, useEffect, useState } from "react";
import { Timestamp } from "@bufbuild/protobuf";
import { IconX } from "@tabler/icons-react";
import type { ManasikCurriculum, ManasikSession } from "@hajj-saas/proto-gen/hajj/v1/manasik_pb";
import type { Kloter } from "@hajj-saas/proto-gen/hajj/v1/kloter_pb";
import { manasikClient } from "@/lib/rpc";

type Props = { open: boolean; seasonId: string; curricula: ManasikCurriculum[]; kloters: Kloter[]; session?: ManasikSession; onClose: () => void; onSaved: () => void };

const toLocalInput = (d: Date) => {
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
};

export default function SessionFormDialog({ open, seasonId, curricula, kloters, session, onClose, onSaved }: Props) {
  const [form, setForm] = useState({ title: "", curriculumId: "", kloterId: "", location: "", instructorName: "", scheduledAt: "", durationMinutes: "60", capacity: "", notes: "" });
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (!open) return;
    setError("");
    setForm(session ? {
      title: session.title, curriculumId: session.curriculumId, kloterId: session.kloterId,
      location: session.location, instructorName: session.instructorName,
      scheduledAt: session.scheduledAt ? toLocalInput(session.scheduledAt.toDate()) : "",
      durationMinutes: String(session.durationMinutes), capacity: String(session.capacity || ""), notes: session.notes,
    } : { title: "", curriculumId: "", kloterId: "", location: "", instructorName: "", scheduledAt: "", durationMinutes: "60", capacity: "", notes: "" });
  }, [open, session]);

  if (!open) return null;

  const update = (key: keyof typeof form, value: string) => setForm((c) => ({ ...c, [key]: value }));

  async function submit(event: FormEvent) {
    event.preventDefault();
    setError("");
    if (!form.title.trim() || !form.scheduledAt) { setError("Judul dan waktu sesi wajib diisi."); return; }
    setSaving(true);
    try {
      const scheduledAt = new Date(form.scheduledAt);
      const payload = {
        curriculumId: form.curriculumId, kloterId: form.kloterId, title: form.title.trim(),
        location: form.location.trim(), instructorName: form.instructorName.trim(),
        scheduledAt: Timestamp.fromDate(scheduledAt),
        durationMinutes: Number(form.durationMinutes) || 60, capacity: Number(form.capacity) || 0,
        notes: form.notes.trim(),
      };
      if (session) {
        await manasikClient.updateManasikSession({ sessionId: session.id, ...payload });
      } else {
        await manasikClient.createManasikSession({ seasonId, ...payload });
      }
      onSaved();
      onClose();
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "Gagal menyimpan sesi.");
    } finally {
      setSaving(false);
    }
  }

  return (
    <div style={overlay}>
      <aside style={sheet}>
        <div style={head}>
          <h2 style={{ margin: 0, fontSize: 18 }}>{session ? "Ubah Sesi" : "Sesi Baru"}</h2>
          <button onClick={onClose} style={closeBtn} aria-label="Tutup"><IconX size={18} /></button>
        </div>
        <form onSubmit={submit} style={body}>
          <label style={label}><span>Judul Sesi</span><input style={input} value={form.title} onChange={(e) => update("title", e.target.value)} /></label>
          <label style={label}>
            <span>Kurikulum (opsional)</span>
            <select style={input} value={form.curriculumId} onChange={(e) => update("curriculumId", e.target.value)}>
              <option value="">— tanpa kurikulum —</option>
              {curricula.map((c) => <option key={c.id} value={c.id}>{c.title}</option>)}
            </select>
          </label>
          <label style={label}>
            <span>Kloter (opsional)</span>
            <select style={input} value={form.kloterId} onChange={(e) => update("kloterId", e.target.value)}>
              <option value="">— seluruh musim —</option>
              {kloters.map((k) => <option key={k.id} value={k.id}>{k.code}</option>)}
            </select>
          </label>
          <div style={grid2}>
            <label style={label}><span>Waktu</span><input type="datetime-local" style={input} value={form.scheduledAt} onChange={(e) => update("scheduledAt", e.target.value)} /></label>
            <label style={label}><span>Durasi (menit)</span><input type="number" style={input} value={form.durationMinutes} onChange={(e) => update("durationMinutes", e.target.value)} /></label>
          </div>
          <div style={grid2}>
            <label style={label}><span>Lokasi</span><input style={input} value={form.location} onChange={(e) => update("location", e.target.value)} /></label>
            <label style={label}><span>Kapasitas (opsional)</span><input type="number" style={input} value={form.capacity} onChange={(e) => update("capacity", e.target.value)} /></label>
          </div>
          <label style={label}><span>Pengajar</span><input style={input} value={form.instructorName} onChange={(e) => update("instructorName", e.target.value)} /></label>
          <label style={label}><span>Catatan</span><input style={input} value={form.notes} onChange={(e) => update("notes", e.target.value)} /></label>
          {error && <p style={{ margin: 0, color: "var(--color-danger-600)", fontSize: 13 }}>{error}</p>}
          <button disabled={saving} style={primary}>{saving ? "Menyimpan..." : "Simpan Sesi"}</button>
        </form>
      </aside>
    </div>
  );
}

const overlay: React.CSSProperties = { position: "fixed", inset: 0, zIndex: 30, display: "flex", justifyContent: "flex-end", background: "rgba(15,23,42,.48)" };
const sheet: React.CSSProperties = { width: "min(480px,100%)", height: "100vh", background: "#fff", display: "flex", flexDirection: "column" };
const head: React.CSSProperties = { display: "flex", justifyContent: "space-between", alignItems: "center", padding: "20px 24px 16px", borderBottom: "1px solid var(--color-cream-300)" };
const closeBtn: React.CSSProperties = { width: 36, height: 36, borderRadius: "50%", border: "1px solid var(--color-cream-400)", background: "transparent", color: "var(--color-warm-400)" };
const body: React.CSSProperties = { flex: 1, overflowY: "auto", padding: 24, display: "grid", gap: 14 };
const grid2: React.CSSProperties = { display: "grid", gridTemplateColumns: "1fr 1fr", gap: 12 };
const label: React.CSSProperties = { display: "flex", flexDirection: "column", gap: 6, fontSize: 13, fontWeight: 600, color: "var(--color-warm-700)" };
const input: React.CSSProperties = { minHeight: 40, border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 10px", font: "inherit", background: "#fff" };
const primary: React.CSSProperties = { minHeight: 46, border: 0, borderRadius: 10, background: "var(--color-gold-500)", color: "#fff", fontWeight: 700 };
