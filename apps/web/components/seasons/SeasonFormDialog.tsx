"use client";
import { FormEvent, useEffect, useState } from "react";
import { Timestamp } from "@bufbuild/protobuf";
import { IconX } from "@tabler/icons-react";
import { Season, SeasonType } from "@hajj-saas/proto-gen/hajj/v1/season_pb";
import { seasonClient } from "@/lib/rpc";
import { SEASON_TYPE_OPTIONS } from "@/lib/season-types";

const toDateInput = (d?: Date) => d ? d.toISOString().slice(0, 10) : "";

type Props = { open: boolean; initial?: Season; onClose: () => void; onSaved: (name: string) => void };

export default function SeasonFormDialog({ open, initial, onClose, onSaved }: Props) {
  const [name, setName] = useState("");
  const [type, setType] = useState<SeasonType>(SeasonType.HAJJ);
  const [startDate, setStartDate] = useState("");
  const [endDate, setEndDate] = useState("");
  const [capacity, setCapacity] = useState("0");
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (!open) return;
    setName(initial?.name ?? "");
    setType(initial?.type ?? SeasonType.HAJJ);
    setStartDate(toDateInput(initial?.startDate?.toDate()));
    setEndDate(toDateInput(initial?.endDate?.toDate()));
    setCapacity(String(initial?.capacity ?? 0));
    setErrors({});
  }, [open, initial]);

  useEffect(() => {
    const onEsc = (e: KeyboardEvent) => e.key === "Escape" && !saving && onClose();
    if (open) window.addEventListener("keydown", onEsc);
    return () => window.removeEventListener("keydown", onEsc);
  }, [open, saving, onClose]);

  if (!open) return null;

  async function submit(e: FormEvent) {
    e.preventDefault();
    const errs: Record<string, string> = {};
    if (!name.trim()) errs.name = "Nama musim wajib diisi.";
    if (!startDate) errs.startDate = "Tanggal mulai wajib diisi.";
    if (!endDate) errs.endDate = "Tanggal selesai wajib diisi.";
    if (startDate && endDate && startDate > endDate) errs.endDate = "Tanggal selesai harus setelah tanggal mulai.";
    if (Object.keys(errs).length) { setErrors(errs); return; }
    setSaving(true);
    try {
      const payload = {
        name: name.trim(),
        type,
        startDate: Timestamp.fromDate(new Date(`${startDate}T00:00:00.000Z`)),
        endDate: Timestamp.fromDate(new Date(`${endDate}T00:00:00.000Z`)),
        capacity: Math.max(0, Number(capacity) || 0),
      };
      if (initial) await seasonClient.updateSeason({ ...payload, seasonId: initial.id });
      else await seasonClient.createSeason(payload);
      onSaved(name.trim());
      onClose();
    } catch (err) {
      setErrors({ _form: err instanceof Error ? err.message : "Gagal menyimpan musim." });
    } finally {
      setSaving(false);
    }
  }

  return (
    <div style={o}>
      <aside style={s}>
        <div style={h}>
          <div><p style={ey}>MUSIM</p><h2 style={{ margin: 0 }}>{initial ? "Ubah Musim" : "Tambah Musim"}</h2></div>
          <button className="btn-close-sheet" onClick={() => !saving && onClose()} style={x}><IconX size={18} /></button>
        </div>
        <div style={b}>
          <form id="season-form" onSubmit={submit} style={{ display: "grid", gap: 16 }}>
            <label style={{ display: "grid", gap: 6 }}>
              <span style={lab}>Nama musim</span>
              <input className="safrat-input" placeholder="mis. Haji 2027" value={name} onChange={(e) => setName(e.target.value)} style={i} />
              {errors.name && <small style={{ color: "var(--color-danger-600)" }}>{errors.name}</small>}
            </label>
            <label style={{ display: "grid", gap: 6 }}>
              <span style={lab}>Jenis musim</span>
              <select className="safrat-input" value={type} onChange={(e) => setType(Number(e.target.value) as SeasonType)} style={i}>
                {SEASON_TYPE_OPTIONS.map((o) => <option key={o.value} value={o.value}>{o.label}</option>)}
              </select>
            </label>
            <label style={{ display: "grid", gap: 6 }}>
              <span style={lab}>Tanggal mulai</span>
              <input className="safrat-input" type="date" value={startDate} onChange={(e) => setStartDate(e.target.value)} style={i} />
              {errors.startDate && <small style={{ color: "var(--color-danger-600)" }}>{errors.startDate}</small>}
            </label>
            <label style={{ display: "grid", gap: 6 }}>
              <span style={lab}>Tanggal selesai</span>
              <input className="safrat-input" type="date" value={endDate} onChange={(e) => setEndDate(e.target.value)} style={i} />
              {errors.endDate && <small style={{ color: "var(--color-danger-600)" }}>{errors.endDate}</small>}
            </label>
            <label style={{ display: "grid", gap: 6 }}>
              <span style={lab}>Kapasitas jamaah</span>
              <input className="safrat-input" type="number" min={0} placeholder="0 = tidak terbatas" value={capacity} onChange={(e) => setCapacity(e.target.value)} style={i} />
              <small style={{ color: "var(--color-warm-400)" }}>Saat kapasitas tercapai, pendaftar baru masuk daftar tunggu. Isi 0 untuk tidak membatasi.</small>
            </label>
            {errors._form && <p style={err}>{errors._form}</p>}
          </form>
        </div>
        <div style={foot}>
          <button form="season-form" disabled={saving} style={primary}>{saving ? "Menyimpan..." : "Simpan musim"}</button>
        </div>
      </aside>
    </div>
  );
}

const o: React.CSSProperties = { position: "fixed", inset: 0, zIndex: 30, display: "flex", justifyContent: "flex-end", background: "rgba(15,23,42,.48)", backdropFilter: "blur(2px)" };
const s: React.CSSProperties = { width: "min(480px,100%)", height: "100vh", display: "flex", flexDirection: "column", background: "#fff", borderRadius: "16px 0 0 16px", overflow: "hidden" };
const h: React.CSSProperties = { display: "flex", justifyContent: "space-between", alignItems: "center", padding: "20px 24px 16px", borderBottom: "1px solid var(--color-cream-300)" };
const b: React.CSSProperties = { flex: 1, overflowY: "auto", padding: 24 };
const foot: React.CSSProperties = { padding: "16px 24px", borderTop: "1px solid var(--color-cream-300)" };
const x: React.CSSProperties = { width: 40, height: 40, borderRadius: "50%", border: "1px solid var(--color-cream-400)", background: "transparent", color: "var(--color-warm-400)" };
const ey: React.CSSProperties = { margin: 0, color: "var(--color-gold-800)", fontSize: 11, fontWeight: 700, letterSpacing: ".1em" };
const i: React.CSSProperties = { minHeight: 48, width: "100%", border: "1.5px solid var(--color-cream-400)", borderRadius: 10, padding: "0 14px", background: "#fff", font: "inherit" };
const lab: React.CSSProperties = { fontSize: 13, fontWeight: 600, color: "var(--color-warm-700)" };
const primary: React.CSSProperties = { minHeight: 48, width: "100%", border: 0, borderRadius: 10, background: "var(--color-gold-500)", color: "#fff", fontWeight: 700 };
const err: React.CSSProperties = { margin: 0, padding: 10, borderRadius: 8, background: "#ffe4e6", color: "var(--color-danger-600)" };
