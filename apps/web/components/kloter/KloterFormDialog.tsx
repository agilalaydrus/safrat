"use client";
import { FormEvent, useEffect, useState } from "react";
import { Timestamp } from "@bufbuild/protobuf";
import { IconX } from "@tabler/icons-react";
import { Kloter } from "@hajj-saas/proto-gen/hajj/v1/kloter_pb";
import { kloterClient } from "@/lib/rpc";

type Props = { open: boolean; seasonId: string; initial?: Kloter; onClose: () => void; onSaved: (code: string) => void };

export default function KloterFormDialog({ open, seasonId, initial, onClose, onSaved }: Props) {
  const [form, setForm] = useState({ code: "", embarkation: "", flightNumber: "", departureDate: "", capacity: "" });
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (!open) return;
    setError("");
    if (initial) {
      setForm({
        code: initial.code,
        embarkation: initial.embarkation,
        flightNumber: initial.flightNumber,
        departureDate: initial.departureDate ? initial.departureDate.toDate().toISOString().slice(0, 10) : "",
        capacity: initial.capacity ? String(initial.capacity) : "",
      });
    } else {
      setForm({ code: "", embarkation: "", flightNumber: "", departureDate: "", capacity: "" });
    }
  }, [open, initial]);

  if (!open) return null;

  const update = (key: keyof typeof form, value: string) => setForm((current) => ({ ...current, [key]: value }));

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    if (!form.code.trim()) { setError("Kode kloter wajib diisi."); return; }
    setSaving(true);
    try {
      const departureDate = form.departureDate ? Timestamp.fromDate(new Date(`${form.departureDate}T00:00:00Z`)) : undefined;
      if (initial) {
        await kloterClient.updateKloter({ kloterId: initial.id, code: form.code.trim().toUpperCase(), embarkation: form.embarkation.trim(), flightNumber: form.flightNumber.trim().toUpperCase(), departureDate, capacity: Number(form.capacity) || 0 });
      } else {
        await kloterClient.createKloter({ seasonId, code: form.code.trim().toUpperCase(), embarkation: form.embarkation.trim(), flightNumber: form.flightNumber.trim().toUpperCase(), departureDate, capacity: Number(form.capacity) || 0 });
      }
      onSaved(form.code.trim().toUpperCase());
      onClose();
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "Gagal menyimpan kloter.");
    } finally {
      setSaving(false);
    }
  }

  return (
    <div role="dialog" aria-modal="true" aria-label={initial ? "Ubah kloter" : "Tambah kloter"} style={overlay}>
      <aside style={sheet}>
        <header style={header}>
          <div><p style={eyebrow}>KLOTER KEBERANGKATAN</p><h2 style={{ margin: 0 }}>{initial ? "Ubah kloter" : "Tambah kloter"}</h2></div>
          <button type="button" onClick={onClose} style={closeBtn} aria-label="Tutup"><IconX size={18} /></button>
        </header>
        <div className="gold-divider" />
        <form onSubmit={submit} style={{ display: "grid", gap: 16 }}>
          <Field label="Kode kloter" hint="contoh: SOC-01, JKG-15"><input className="safrat-input" required value={form.code} onChange={(e) => update("code", e.target.value.toUpperCase())} style={input} /></Field>
          <Field label="Embarkasi" hint="Kota keberangkatan, contoh: Soekarno-Hatta"><input className="safrat-input" value={form.embarkation} onChange={(e) => update("embarkation", e.target.value)} style={input} /></Field>
          <Field label="Nomor penerbangan" hint="contoh: SV 815"><input className="safrat-input" value={form.flightNumber} onChange={(e) => update("flightNumber", e.target.value.toUpperCase())} style={input} /></Field>
          <Field label="Tanggal keberangkatan"><input className="safrat-input" type="date" value={form.departureDate} onChange={(e) => update("departureDate", e.target.value)} style={input} /></Field>
          <Field label="Kapasitas"><input className="safrat-input" type="number" min="0" value={form.capacity} onChange={(e) => update("capacity", e.target.value)} style={input} /></Field>
          {error && <p role="alert" style={formError}>{error}</p>}
          <button disabled={saving} style={primary}>{saving ? "Menyimpan..." : initial ? "Simpan perubahan" : "Tambah kloter"}</button>
        </form>
      </aside>
    </div>
  );
}

function Field({ label, hint, children }: { label: string; hint?: string; children: React.ReactNode }) {
  return <label style={{ display: "grid", gap: 6 }}><span style={fieldLabel}>{label}</span>{children}{hint && <span style={{ fontSize: 11, color: "var(--color-warm-400)" }}>{hint}</span>}</label>;
}

const overlay: React.CSSProperties = { position: "fixed", inset: 0, zIndex: 30, display: "flex", justifyContent: "flex-end", background: "rgba(26,20,16,.48)" };
const sheet: React.CSSProperties = { width: "min(480px,100%)", height: "100vh", overflowY: "auto", background: "#fff", padding: 24, boxShadow: "-6px 0 32px rgba(26,20,16,.12)" };
const header: React.CSSProperties = { display: "flex", justifyContent: "space-between", alignItems: "flex-start", gap: 16 };
const eyebrow: React.CSSProperties = { margin: "0 0 6px", color: "var(--color-gold-800)", fontSize: 11, fontWeight: 700, letterSpacing: ".08em" };
const closeBtn: React.CSSProperties = { width: 40, height: 40, borderRadius: "50%", border: "1px solid var(--color-cream-400)", background: "transparent", color: "var(--color-warm-400)" };
const input: React.CSSProperties = { minHeight: 48, width: "100%", border: "1.5px solid var(--color-cream-400)", borderRadius: 10, padding: "0 14px", font: "inherit" };
const fieldLabel: React.CSSProperties = { fontSize: 13, fontWeight: 600, color: "var(--color-warm-700)" };
const primary: React.CSSProperties = { minHeight: 48, border: 0, borderRadius: 10, background: "var(--color-gold-500)", color: "var(--color-warm-900)", fontWeight: 700 };
const formError: React.CSSProperties = { margin: 0, fontSize: 13, color: "var(--color-danger-600)", background: "#fdf0f0", padding: "10px 12px", borderRadius: 8 };
