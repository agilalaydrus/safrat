"use client";

import { FormEvent, useState } from "react";
import { Timestamp } from "@bufbuild/protobuf";
import { IconX } from "@tabler/icons-react";
import { transportClient } from "@/lib/rpc";

type Props = { open: boolean; seasonId: string; onClose: () => void; onSaved: () => void };

const MODES = [["BUS", "Bus"], ["FLIGHT", "Pesawat"], ["TRAIN", "Kereta Cepat"]] as const;

export default function MovementFormDialog({ open, seasonId, onClose, onSaved }: Props) {
  const [form, setForm] = useState({ name: "", origin: "", destination: "", scheduledAt: "", mode: "BUS" });
  const [error, setError] = useState("");

  if (!open) return null;

  const update = (key: keyof typeof form, value: string) => setForm((current) => ({ ...current, [key]: value }));

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    const scheduledAt = new Date(form.scheduledAt);
    if (!form.name || !form.origin || !form.destination || Number.isNaN(scheduledAt.valueOf()) || scheduledAt <= new Date()) {
      setError("Lengkapi semua kolom dan pilih waktu keberangkatan di masa depan.");
      return;
    }
    try {
      await transportClient.createMovement({ seasonId, name: form.name, origin: form.origin, destination: form.destination, scheduledAt: Timestamp.fromDate(scheduledAt), mode: form.mode });
      onSaved();
      onClose();
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "Gagal menambahkan jadwal.");
    }
  }

  return (
    <div role="dialog" aria-modal="true" aria-label="Tambah jadwal" style={overlay}>
      <aside style={sheet}>
        <div style={stickyHeader}>
          <div><p style={eyebrow}>TRANSPORTASI</p><h2 style={{ margin: 0 }}>Tambah jadwal</h2></div>
          <button type="button" className="btn-close-sheet" onClick={onClose} style={closeBtn} aria-label="Tutup"><IconX size={18} /></button>
        </div>
        <div style={formBody}>
          <form id="movement-form" onSubmit={submit} style={{ display: "grid", gap: 20 }}>
            <Section title="Detail jadwal">
              <Field label="Moda transportasi">
                <select className="safrat-input" value={form.mode} onChange={(event) => update("mode", event.target.value)} style={input}>
                  {MODES.map(([value, label]) => <option key={value} value={value}>{label}</option>)}
                </select>
              </Field>
              <Field label="Nama jadwal"><input className="safrat-input" required value={form.name} onChange={(event) => update("name", event.target.value)} style={input} /></Field>
              <Field label={form.mode === "FLIGHT" ? "Bandara Asal" : form.mode === "TRAIN" ? "Stasiun Asal" : "Asal"}><input className="safrat-input" required value={form.origin} onChange={(event) => update("origin", event.target.value)} style={input} /></Field>
              <Field label={form.mode === "FLIGHT" ? "Bandara Tujuan" : form.mode === "TRAIN" ? "Stasiun Tujuan" : "Tujuan"}><input className="safrat-input" required value={form.destination} onChange={(event) => update("destination", event.target.value)} style={input} /></Field>
              <Field label="Dijadwalkan pada"><input className="safrat-input" required type="datetime-local" value={form.scheduledAt} onChange={(event) => update("scheduledAt", event.target.value)} style={input} /></Field>
            </Section>
            {error && <p role="alert" style={formError}>{error}</p>}
          </form>
        </div>
        <div style={stickyFooter}><button form="movement-form" style={primary}>Tambah jadwal</button></div>
      </aside>
    </div>
  );
}

function Section({ title, children }: { title: string; children: React.ReactNode }) { return <section style={{ display: "grid", gap: 16 }}><p style={sectionTitle}>{title}</p>{children}</section>; }
function Field({ label, children }: { label: string; children: React.ReactNode }) { return <label style={{ display: "grid", gap: 6 }}><span style={fieldLabel}>{label}</span>{children}</label>; }

const overlay: React.CSSProperties = { position: "fixed", inset: 0, zIndex: 30, display: "flex", justifyContent: "flex-end", background: "rgba(26,20,16,.48)", backdropFilter: "blur(2px)", WebkitBackdropFilter: "blur(2px)" };
const sheet: React.CSSProperties = { width: "min(560px,100%)", height: "100vh", display: "flex", flexDirection: "column", background: "#ffffff", boxShadow: "-6px 0 32px rgba(26,20,16,.12)", borderRadius: "16px 0 0 16px", animation: "sheet-in .22s cubic-bezier(0,0,.2,1)", overflow: "hidden" };
const stickyHeader: React.CSSProperties = { position: "sticky", top: 0, zIndex: 10, background: "#ffffff", borderBottom: "1px solid var(--color-cream-300)", padding: "20px 24px 16px", display: "flex", alignItems: "center", justifyContent: "space-between", gap: 16, flexShrink: 0 };
const closeBtn: React.CSSProperties = { width: 40, height: 40, borderRadius: "50%", border: "1px solid var(--color-cream-400)", background: "transparent", color: "var(--color-warm-400)", display: "flex", alignItems: "center", justifyContent: "center", cursor: "pointer", flexShrink: 0, transition: "background .15s, color .15s" };
const formBody: React.CSSProperties = { flex: 1, overflowY: "auto", padding: "24px", display: "grid", gap: 0 };
const stickyFooter: React.CSSProperties = { position: "sticky", bottom: 0, background: "#ffffff", borderTop: "1px solid var(--color-cream-300)", padding: "16px 24px", flexShrink: 0 };
const eyebrow: React.CSSProperties = { margin: "0 0 6px", color: "var(--color-gold-800)", fontSize: 11, fontWeight: 700, letterSpacing: ".08em" };
const input: React.CSSProperties = { minHeight: 48, width: "100%", border: "1.5px solid var(--color-cream-400)", borderRadius: 10, padding: "0 14px", background: "#ffffff", font: "inherit", color: "var(--color-warm-900)", outline: "none", transition: "border-color .15s, box-shadow .15s" };
const primary: React.CSSProperties = { minHeight: 48, border: 0, borderRadius: 10, background: "var(--color-gold-500)", color: "var(--color-warm-900)", fontWeight: 700, padding: "0 20px", cursor: "pointer", fontFamily: "'Plus Jakarta Sans', sans-serif", fontSize: 14, width: "100%" };
const sectionTitle: React.CSSProperties = { margin: 0, fontSize: 11, fontWeight: 700, letterSpacing: ".1em", textTransform: "uppercase", color: "var(--color-warm-400)", paddingBottom: 8, borderBottom: "1px solid var(--color-cream-300)", fontFamily: "'Plus Jakarta Sans', sans-serif" };
const fieldLabel: React.CSSProperties = { fontSize: 13, fontWeight: 600, color: "var(--color-warm-700)", display: "block", marginBottom: 6 };
const formError: React.CSSProperties = { margin: 0, fontSize: 13, color: "var(--color-danger-600)", background: "#fdf0f0", padding: "10px 12px", borderRadius: 8 };
