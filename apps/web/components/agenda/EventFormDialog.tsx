"use client";

import { FormEvent, useEffect, useState } from "react";
import { Timestamp } from "@bufbuild/protobuf";
import { IconX } from "@tabler/icons-react";
import type { Branch } from "@hajj-saas/proto-gen/hajj/v1/branch_pb";
import { agendaClient } from "@/lib/rpc";

// Structural rather than the generated AgendaEvent class, so ListAgenda's
// merged AgendaItem rows (which carry the same fields for an INTERNAL kind)
// can be edited here too without a round trip to fetch the "real" event.
type EditableEvent = {
  id: string; title: string; branchId: string; location: string;
  startsAt?: Timestamp; endsAt?: Timestamp; notes: string;
};

type Props = { open: boolean; seasonId: string; branches: Branch[]; event?: EditableEvent; onClose: () => void; onSaved: () => void };

const toLocalInput = (d: Date) => {
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
};

export default function EventFormDialog({ open, seasonId, branches, event, onClose, onSaved }: Props) {
  const [form, setForm] = useState({ title: "", branchId: "", location: "", startsAt: "", endsAt: "", notes: "" });
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (!open) return;
    setError("");
    setForm(event ? {
      title: event.title, branchId: event.branchId, location: event.location,
      startsAt: event.startsAt ? toLocalInput(event.startsAt.toDate()) : "",
      endsAt: event.endsAt ? toLocalInput(event.endsAt.toDate()) : "",
      notes: event.notes,
    } : { title: "", branchId: "", location: "", startsAt: "", endsAt: "", notes: "" });
  }, [open, event]);

  if (!open) return null;

  const update = (key: keyof typeof form, value: string) => setForm((c) => ({ ...c, [key]: value }));

  async function submit(e: FormEvent) {
    e.preventDefault();
    setError("");
    if (!form.title.trim() || !form.startsAt) { setError("Judul dan waktu mulai wajib diisi."); return; }
    setSaving(true);
    try {
      const payload = {
        branchId: form.branchId, title: form.title.trim(), location: form.location.trim(),
        startsAt: Timestamp.fromDate(new Date(form.startsAt)),
        endsAt: form.endsAt ? Timestamp.fromDate(new Date(form.endsAt)) : undefined,
        notes: form.notes.trim(),
      };
      if (event) {
        await agendaClient.updateAgendaEvent({ eventId: event.id, seasonId, ...payload });
      } else {
        await agendaClient.createAgendaEvent({ seasonId, ...payload });
      }
      onSaved();
      onClose();
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "Gagal menyimpan acara.");
    } finally {
      setSaving(false);
    }
  }

  return (
    <div style={overlay}>
      <aside style={sheet}>
        <div style={head}>
          <h2 style={{ margin: 0, fontSize: 18 }}>{event ? "Ubah Acara" : "Acara Baru"}</h2>
          <button onClick={onClose} style={closeBtn} aria-label="Tutup"><IconX size={18} /></button>
        </div>
        <form onSubmit={submit} style={body}>
          <label style={label}><span>Judul Acara</span><input style={input} value={form.title} onChange={(e) => update("title", e.target.value)} /></label>
          <label style={label}>
            <span>Cabang (opsional)</span>
            <select style={input} value={form.branchId} onChange={(e) => update("branchId", e.target.value)}>
              <option value="">— pusat —</option>
              {branches.map((b) => <option key={b.id} value={b.id}>{b.name}</option>)}
            </select>
          </label>
          <div style={grid2}>
            <label style={label}><span>Mulai</span><input type="datetime-local" style={input} value={form.startsAt} onChange={(e) => update("startsAt", e.target.value)} /></label>
            <label style={label}><span>Selesai (opsional)</span><input type="datetime-local" style={input} value={form.endsAt} onChange={(e) => update("endsAt", e.target.value)} /></label>
          </div>
          <label style={label}><span>Lokasi</span><input style={input} value={form.location} onChange={(e) => update("location", e.target.value)} /></label>
          <label style={label}><span>Catatan</span><input style={input} value={form.notes} onChange={(e) => update("notes", e.target.value)} /></label>
          {error && <p style={{ margin: 0, color: "var(--color-danger-600)", fontSize: 13 }}>{error}</p>}
          <button disabled={saving} style={primary}>{saving ? "Menyimpan..." : "Simpan Acara"}</button>
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
