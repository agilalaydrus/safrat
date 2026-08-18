"use client";
import { FormEvent, useEffect, useState } from "react";
import { Timestamp } from "@bufbuild/protobuf";
import { IconX } from "@tabler/icons-react";
import { SeasonType } from "@hajj-saas/proto-gen/hajj/v1/season_pb";
import { seasonClient } from "@/lib/rpc";

type Props = { open: boolean; onClose: () => void; onSaved: (name: string) => void };

export default function SeasonFormDialog({ open, onClose, onSaved }: Props) {
  const [name, setName] = useState("");
  const [type, setType] = useState<"HAJJ" | "UMRAH">("HAJJ");
  const [startDate, setStartDate] = useState("");
  const [endDate, setEndDate] = useState("");
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (open) { setName(""); setType("HAJJ"); setStartDate(""); setEndDate(""); setErrors({}); }
  }, [open]);

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
      await seasonClient.createSeason({
        name: name.trim(),
        type: type === "HAJJ" ? SeasonType.HAJJ : SeasonType.UMRAH,
        startDate: Timestamp.fromDate(new Date(`${startDate}T00:00:00.000Z`)),
        endDate: Timestamp.fromDate(new Date(`${endDate}T00:00:00.000Z`)),
      });
      onSaved(name.trim());
      onClose();
    } catch (err) {
      setErrors({ _form: err instanceof Error ? err.message : "Gagal membuat musim." });
    } finally {
      setSaving(false);
    }
  }

  return (
    <div style={o}>
      <aside style={s}>
        <div style={h}>
          <div><p style={ey}>MUSIM</p><h2 style={{ margin: 0 }}>Tambah Musim</h2></div>
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
              <span style={lab}>Jenis perjalanan</span>
              <select className="safrat-input" value={type} onChange={(e) => setType(e.target.value as "HAJJ" | "UMRAH")} style={i}>
                <option value="HAJJ">Haji</option>
                <option value="UMRAH">Umrah</option>
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

const o: React.CSSProperties = { position: "fixed", inset: 0, zIndex: 30, display: "flex", justifyContent: "flex-end", background: "rgba(26,20,16,.48)", backdropFilter: "blur(2px)" };
const s: React.CSSProperties = { width: "min(480px,100%)", height: "100vh", display: "flex", flexDirection: "column", background: "#fff", borderRadius: "16px 0 0 16px", overflow: "hidden" };
const h: React.CSSProperties = { display: "flex", justifyContent: "space-between", alignItems: "center", padding: "20px 24px 16px", borderBottom: "1px solid var(--color-cream-300)" };
const b: React.CSSProperties = { flex: 1, overflowY: "auto", padding: 24 };
const foot: React.CSSProperties = { padding: "16px 24px", borderTop: "1px solid var(--color-cream-300)" };
const x: React.CSSProperties = { width: 40, height: 40, borderRadius: "50%", border: "1px solid var(--color-cream-400)", background: "transparent", color: "var(--color-warm-400)" };
const ey: React.CSSProperties = { margin: 0, color: "var(--color-gold-800)", fontSize: 11, fontWeight: 700, letterSpacing: ".1em" };
const i: React.CSSProperties = { minHeight: 48, width: "100%", border: "1.5px solid var(--color-cream-400)", borderRadius: 10, padding: "0 14px", background: "#fff", font: "inherit" };
const lab: React.CSSProperties = { fontSize: 13, fontWeight: 600, color: "var(--color-warm-700)" };
const primary: React.CSSProperties = { minHeight: 48, width: "100%", border: 0, borderRadius: 10, background: "var(--color-gold-500)", color: "var(--color-warm-900)", fontWeight: 700 };
const err: React.CSSProperties = { margin: 0, padding: 10, borderRadius: 8, background: "#fdf0f0", color: "var(--color-danger-600)" };
