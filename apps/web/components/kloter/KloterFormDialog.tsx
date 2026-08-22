"use client";
import { FormEvent, useEffect, useState } from "react";
import { Timestamp } from "@bufbuild/protobuf";
import { IconPlane, IconTrash, IconX } from "@tabler/icons-react";
import { Kloter } from "@hajj-saas/proto-gen/hajj/v1/kloter_pb";
import { Movement } from "@hajj-saas/proto-gen/hajj/v1/transport_pb";
import { kloterClient, transportClient } from "@/lib/rpc";

type Props = { open: boolean; seasonId: string; initial?: Kloter; onClose: () => void; onSaved: (code: string) => void };

const TRIP_LEGS = [["", "Bagian lain"], ["DEPARTURE", "Keberangkatan"], ["RETURN", "Kepulangan"]] as const;
const TRIP_LEG_LABEL: Record<string, string> = { DEPARTURE: "Keberangkatan", RETURN: "Kepulangan" };

export default function KloterFormDialog({ open, seasonId, initial, onClose, onSaved }: Props) {
  const [form, setForm] = useState({ code: "", embarkation: "", flightNumber: "", departureDate: "", capacity: "" });
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);

  const [legs, setLegs] = useState<Movement[]>([]);
  const [legForm, setLegForm] = useState({ origin: "", destination: "", scheduledAt: "", airline: "", flightNumber: "", tripLeg: "" });
  const [legError, setLegError] = useState("");
  const [legSaving, setLegSaving] = useState(false);

  const loadLegs = () => {
    if (!initial || !seasonId) { setLegs([]); return; }
    transportClient.listMovements({ seasonId }).then((r) => setLegs(r.movements.filter((m) => m.kloterId === initial.id && m.mode === "FLIGHT").sort((a, b) => (a.scheduledAt?.toDate().getTime() ?? 0) - (b.scheduledAt?.toDate().getTime() ?? 0)))).catch(() => setLegs([]));
  };

  useEffect(() => {
    if (!open) return;
    setError("");
    setLegForm({ origin: "", destination: "", scheduledAt: "", airline: "", flightNumber: "", tripLeg: "" });
    setLegError("");
    if (initial) {
      setForm({
        code: initial.code,
        embarkation: initial.embarkation,
        flightNumber: initial.flightNumber,
        departureDate: initial.departureDate ? initial.departureDate.toDate().toISOString().slice(0, 10) : "",
        capacity: initial.capacity ? String(initial.capacity) : "",
      });
      loadLegs();
    } else {
      setForm({ code: "", embarkation: "", flightNumber: "", departureDate: "", capacity: "" });
      setLegs([]);
    }
  }, [open, initial]);

  if (!open) return null;

  const update = (key: keyof typeof form, value: string) => setForm((current) => ({ ...current, [key]: value }));
  const updateLeg = (key: keyof typeof legForm, value: string) => setLegForm((current) => ({ ...current, [key]: value }));

  async function addLeg() {
    if (!initial) return;
    setLegError("");
    const scheduled = new Date(legForm.scheduledAt);
    if (!legForm.origin.trim() || !legForm.destination.trim() || Number.isNaN(scheduled.valueOf())) {
      setLegError("Lengkapi asal, tujuan, dan jadwal penerbangan.");
      return;
    }
    setLegSaving(true);
    try {
      const legLabel = TRIP_LEG_LABEL[legForm.tripLeg] ?? "Penerbangan";
      await transportClient.createMovement({
        seasonId, kloterId: initial.id, mode: "FLIGHT", tripLeg: legForm.tripLeg,
        name: `${initial.code} - ${legLabel}`, origin: legForm.origin.trim(), destination: legForm.destination.trim(),
        scheduledAt: Timestamp.fromDate(scheduled), airline: legForm.airline.trim(), flightNumber: legForm.flightNumber.trim().toUpperCase(),
      });
      setLegForm({ origin: "", destination: "", scheduledAt: "", airline: "", flightNumber: "", tripLeg: "" });
      loadLegs();
    } catch (caught) {
      setLegError(caught instanceof Error ? caught.message : "Gagal menambahkan jadwal penerbangan.");
    } finally {
      setLegSaving(false);
    }
  }

  async function removeLeg(leg: Movement) {
    if (!window.confirm(`Hapus jadwal ${leg.name}?`)) return;
    await transportClient.deleteMovement({ movementId: leg.id });
    loadLegs();
  }

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
          <Field label="Nomor penerbangan (ringkas)" hint="Ditampilkan hanya jika belum ada jadwal penerbangan rinci di bawah — sebaiknya isi jadwal penerbangan rinci untuk keberangkatan/transit/kepulangan."><input className="safrat-input" value={form.flightNumber} onChange={(e) => update("flightNumber", e.target.value.toUpperCase())} style={input} /></Field>
          <Field label="Tanggal keberangkatan"><input className="safrat-input" type="date" value={form.departureDate} onChange={(e) => update("departureDate", e.target.value)} style={input} /></Field>
          <Field label="Kapasitas"><input className="safrat-input" type="number" min="0" value={form.capacity} onChange={(e) => update("capacity", e.target.value)} style={input} /></Field>
          {error && <p role="alert" style={formError}>{error}</p>}
          <button disabled={saving} style={primary}>{saving ? "Menyimpan..." : initial ? "Simpan perubahan" : "Tambah kloter"}</button>
        </form>

        {initial && (
          <section style={legsSection}>
            <div className="gold-divider" />
            <h3 style={legsTitle}><IconPlane size={16} color="var(--color-emerald-800)" />Jadwal Penerbangan ({legs.length})</h3>
            <p style={{ margin: "0 0 12px", fontSize: 12, color: "var(--color-warm-400)" }}>Tambahkan setiap kaki penerbangan — keberangkatan, transit (tambahkan lagi dengan bagian yang sama), dan kepulangan.</p>
            {legs.length > 0 && (
              <div style={{ display: "grid", gap: 8, marginBottom: 16 }}>
                {legs.map((leg) => (
                  <div key={leg.id} style={legRow}>
                    <div style={{ flex: 1, minWidth: 0 }}>
                      <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
                        {leg.tripLeg && <span style={legBadge}>{TRIP_LEG_LABEL[leg.tripLeg] ?? leg.tripLeg}</span>}
                        <span style={{ fontSize: 13 }}>{leg.origin} → {leg.destination}</span>
                      </div>
                      <p style={{ margin: "2px 0 0", fontSize: 11, color: "var(--color-warm-400)" }}>{leg.scheduledAt?.toDate().toLocaleString("id-ID", { day: "2-digit", month: "short", year: "numeric", hour: "2-digit", minute: "2-digit" })}{(leg.airline || leg.flightNumber) && ` · ${[leg.airline, leg.flightNumber].filter(Boolean).join(" ")}`}</p>
                    </div>
                    <button type="button" onClick={() => void removeLeg(leg)} style={legDeleteBtn} aria-label="Hapus jadwal"><IconTrash size={15} /></button>
                  </div>
                ))}
              </div>
            )}
            <div style={legForm2}>
              <div style={twoCol}>
                <Field label="Asal"><input className="safrat-input" value={legForm.origin} onChange={(e) => updateLeg("origin", e.target.value)} style={input} /></Field>
                <Field label="Tujuan"><input className="safrat-input" value={legForm.destination} onChange={(e) => updateLeg("destination", e.target.value)} style={input} /></Field>
              </div>
              <Field label="Dijadwalkan pada"><input className="safrat-input" type="datetime-local" value={legForm.scheduledAt} onChange={(e) => updateLeg("scheduledAt", e.target.value)} style={input} /></Field>
              <div style={twoCol}>
                <Field label="Maskapai"><input className="safrat-input" value={legForm.airline} onChange={(e) => updateLeg("airline", e.target.value)} placeholder="mis. Garuda Indonesia" style={input} /></Field>
                <Field label="Nomor Penerbangan"><input className="safrat-input" value={legForm.flightNumber} onChange={(e) => updateLeg("flightNumber", e.target.value)} placeholder="mis. GA980" style={input} /></Field>
              </div>
              <Field label="Bagian perjalanan">
                <select className="safrat-input" value={legForm.tripLeg} onChange={(e) => updateLeg("tripLeg", e.target.value)} style={input}>
                  {TRIP_LEGS.map(([value, label]) => <option key={value} value={value}>{label}</option>)}
                </select>
              </Field>
              {legError && <p role="alert" style={formError}>{legError}</p>}
              <button type="button" disabled={legSaving} onClick={() => void addLeg()} style={legAddBtn}>{legSaving ? "Menambahkan..." : "+ Tambah Jadwal Penerbangan"}</button>
            </div>
          </section>
        )}
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
const legsSection: React.CSSProperties = { marginTop: 24 };
const legsTitle: React.CSSProperties = { display: "flex", alignItems: "center", gap: 8, margin: "16px 0 4px", fontSize: 15 };
const legRow: React.CSSProperties = { display: "flex", alignItems: "flex-start", gap: 8, padding: "10px 12px", background: "var(--color-cream-100)", border: "1px solid var(--color-cream-300)", borderRadius: 8 };
const legBadge: React.CSSProperties = { padding: "3px 8px", borderRadius: 99, background: "var(--color-emerald-50)", color: "var(--color-emerald-900)", fontSize: 10, fontWeight: 700, whiteSpace: "nowrap" };
const legDeleteBtn: React.CSSProperties = { border: 0, background: "transparent", color: "var(--color-danger-600)", flexShrink: 0, cursor: "pointer" };
const legForm2: React.CSSProperties = { display: "grid", gap: 12, padding: 16, background: "var(--color-cream-100)", borderRadius: 10, border: "1px dashed var(--color-cream-400)" };
const twoCol: React.CSSProperties = { display: "grid", gridTemplateColumns: "1fr 1fr", gap: 12 };
const legAddBtn: React.CSSProperties = { minHeight: 44, border: "1px solid var(--color-emerald-800)", borderRadius: 8, background: "transparent", color: "var(--color-emerald-900)", fontWeight: 700 };
