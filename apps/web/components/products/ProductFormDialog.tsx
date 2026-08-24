"use client";
import { FormEvent, useCallback, useEffect, useRef, useState } from "react";
import { IconPlus, IconTrash, IconX } from "@tabler/icons-react";
import { Product, ItineraryDay } from "@hajj-saas/proto-gen/hajj/v1/product_pb";
import { Hotel } from "@hajj-saas/proto-gen/hajj/v1/accommodation_pb";
import { Kloter } from "@hajj-saas/proto-gen/hajj/v1/kloter_pb";
import { productClient, accommodationClient, kloterClient } from "@/lib/rpc";

type Props = { open: boolean; seasonId: string; initial?: Product; onClose: () => void; onSaved: (name: string) => void };
type DayForm = { dayNumber: number; title: string; city: string; activities: string; mealBreakfast: boolean; mealLunch: boolean; mealDinner: boolean };
type Values = {
  name: string; category: string; type: string; price: string; duration: string; description: string; inclusions: string; active: boolean;
  platformMargin: string; operatorMargin: string; agentMargin: string;
  itinerary: DayForm[]; hotelIds: string[]; defaultKloterId: string;
};
const emptyDay = (dayNumber: number): DayForm => ({ dayNumber, title: "", city: "", activities: "", mealBreakfast: false, mealLunch: false, mealDinner: false });
const empty: Values = { name: "", category: "TRAVEL_PACKAGE", type: "HAJJ", price: "", duration: "", description: "", inclusions: "", active: true, platformMargin: "15", operatorMargin: "70", agentMargin: "15", itinerary: [], hotelIds: [], defaultKloterId: "" };
const CATEGORIES: [string, string][] = [
  ["TRAVEL_PACKAGE", "Paket Perjalanan"],
  ["EQUIPMENT", "Perlengkapan Umrah & Haji"],
  ["ROAMING_DATA", "Paket Data Roaming"],
  ["PPOB_CREDIT", "Pulsa & PPOB"],
];

function fromProtoDay(d: ItineraryDay): DayForm {
  return { dayNumber: d.dayNumber, title: d.title, city: d.city, activities: d.activities, mealBreakfast: d.mealBreakfast, mealLunch: d.mealLunch, mealDinner: d.mealDinner };
}

export default function ProductFormDialog({ open, seasonId, initial, onClose, onSaved }: Props) {
  const [form, setForm] = useState<Values>(empty);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [saving, setSaving] = useState(false);
  const [hotels, setHotels] = useState<Hotel[]>([]);
  const [kloters, setKloters] = useState<Kloter[]>([]);
  const initialRef = useRef(empty);

  const close = useCallback(() => { if (JSON.stringify(form) !== JSON.stringify(initialRef.current) && !window.confirm("Ada perubahan yang belum disimpan. Batalkan?")) return; onClose(); }, [form, onClose]);

  useEffect(() => {
    if (!open) return;
    const v: Values = initial ? {
      name: initial.name, category: initial.category || "TRAVEL_PACKAGE", type: initial.type || "HAJJ",
      price: String(initial.priceIdr), duration: String(initial.durationDays), description: initial.description,
      inclusions: initial.inclusions.join("\n"), active: initial.isActive,
      platformMargin: String(Math.round(initial.platformMarginPct * 100)), operatorMargin: String(Math.round(initial.operatorMarginPct * 100)), agentMargin: String(Math.round(initial.agentMarginPct * 100)),
      itinerary: initial.itineraryDays.map(fromProtoDay), hotelIds: initial.hotelIds, defaultKloterId: initial.defaultKloterId,
    } : { ...empty };
    initialRef.current = v;
    setForm(v);
    setErrors({});
  }, [open, initial]);

  useEffect(() => {
    if (!open || !seasonId) return;
    accommodationClient.listHotels({ seasonId }).then((r) => setHotels(r.hotels)).catch(() => setHotels([]));
    kloterClient.listKloters({ seasonId }).then((r) => setKloters(r.kloters)).catch(() => setKloters([]));
  }, [open, seasonId]);

  useEffect(() => {
    if (!open) return;
    const h = (e: KeyboardEvent) => { if (e.key === "Escape") close(); };
    window.addEventListener("keydown", h);
    return () => window.removeEventListener("keydown", h);
  }, [close, open]);

  const update = (key: keyof Values, value: Values[keyof Values]) => setForm((v) => ({ ...v, [key]: value }));
  const isTravelPackage = form.category === "TRAVEL_PACKAGE";
  const marginSum = (Number(form.platformMargin) || 0) + (Number(form.operatorMargin) || 0) + (Number(form.agentMargin) || 0);

  function addDay() { update("itinerary", [...form.itinerary, emptyDay(form.itinerary.length + 1)]); }
  function removeDay(index: number) { update("itinerary", form.itinerary.filter((_, i) => i !== index).map((d, i) => ({ ...d, dayNumber: i + 1 }))); }
  function updateDay(index: number, patch: Partial<DayForm>) { update("itinerary", form.itinerary.map((d, i) => (i === index ? { ...d, ...patch } : d))); }
  function toggleHotel(hotelId: string) { update("hotelIds", form.hotelIds.includes(hotelId) ? form.hotelIds.filter((id) => id !== hotelId) : [...form.hotelIds, hotelId]); }

  async function submit(e: FormEvent) {
    e.preventDefault();
    const x: Record<string, string> = {};
    if (!form.name.trim()) x.name = "Nama wajib diisi.";
    if (!form.price || Number(form.price) < 0) x.price = "Masukkan harga yang valid.";
    if (!form.duration || Number(form.duration) < 1) x.duration = "Durasi minimal 1 hari.";
    if (marginSum > 100) x.margins = "Total margin tidak boleh lebih dari 100%.";
    if (Object.keys(x).length) { setErrors(x); return; }
    setSaving(true);
    try {
      const payload = {
        name: form.name.trim(), category: form.category, type: isTravelPackage ? form.type : "",
        priceIdr: BigInt(form.price), durationDays: Number(form.duration), description: form.description.trim(),
        inclusions: form.inclusions.split("\n").map((v) => v.trim()).filter(Boolean),
        platformMarginPct: (Number(form.platformMargin) || 0) / 100, operatorMarginPct: (Number(form.operatorMargin) || 0) / 100, agentMarginPct: (Number(form.agentMargin) || 0) / 100,
        itineraryDays: isTravelPackage ? form.itinerary : [], hotelIds: isTravelPackage ? form.hotelIds : [], defaultKloterId: isTravelPackage ? form.defaultKloterId : "",
      };
      if (initial) await productClient.updateProduct({ ...payload, productId: initial.id, isActive: form.active });
      else await productClient.createProduct({ ...payload, seasonId });
      onSaved(payload.name);
      onClose();
    } catch {
      setErrors({ _form: "Produk gagal disimpan. Silakan coba lagi." });
    } finally {
      setSaving(false);
    }
  }

  if (!open) return null;
  return (
    <div role="dialog" aria-modal="true" style={overlay}>
      <aside style={sheet}>
        <div style={head}>
          <div><p style={eyebrow}>PRODUK</p><h2 style={{ margin: 0 }}>{initial ? "Ubah Produk" : "Tambah Produk"}</h2></div>
          <button className="btn-close-sheet" type="button" onClick={close} style={closeBtn} aria-label="Tutup"><IconX size={18} /></button>
        </div>
        <div style={body}>
          <form id="product-form" onSubmit={submit} style={{ display: "grid", gap: 16 }}>
            <p style={section}>DETAIL PRODUK</p>
            <Field label="Nama Produk" error={errors.name}><input className="safrat-input" value={form.name} onChange={(e) => update("name", e.target.value)} style={input} aria-invalid={!!errors.name} /></Field>
            <Field label="Kategori"><select className="safrat-input" value={form.category} onChange={(e) => update("category", e.target.value)} style={input}>{CATEGORIES.map(([value, label]) => <option key={value} value={value}>{label}</option>)}</select></Field>
            {isTravelPackage && <Field label="Jenis Perjalanan"><select className="safrat-input" value={form.type} onChange={(e) => update("type", e.target.value)} style={input}><option value="HAJJ">Haji</option><option value="UMRAH">Umrah</option></select></Field>}
            <div style={cols}>
              <Field label="Harga (Rp)" error={errors.price}><input className="safrat-input" type="number" min="0" value={form.price} onChange={(e) => update("price", e.target.value)} style={input} aria-invalid={!!errors.price} /></Field>
              <Field label="Durasi (hari)" error={errors.duration}><input className="safrat-input" type="number" min="1" value={form.duration} onChange={(e) => update("duration", e.target.value)} style={input} aria-invalid={!!errors.duration} /></Field>
            </div>
            <Field label="Deskripsi"><textarea className="safrat-input" maxLength={1000} value={form.description} onChange={(e) => update("description", e.target.value)} style={{ ...input, minHeight: 100, padding: "12px 14px" }} /><small style={count}>{form.description.length}/1000</small></Field>
            <Field label="Fasilitas Termasuk (satu per baris)"><textarea className="safrat-input" value={form.inclusions} onChange={(e) => update("inclusions", e.target.value)} style={{ ...input, minHeight: 110, padding: "12px 14px" }} /></Field>

            {isTravelPackage && (
              <>
                <p style={section}>HOTEL YANG TERMASUK</p>
                {hotels.length ? (
                  <div style={hotelGrid}>
                    {hotels.map((h) => (
                      <label key={h.id} style={{ ...hotelChip, ...(form.hotelIds.includes(h.id) ? hotelChipActive : {}) }}>
                        <input type="checkbox" checked={form.hotelIds.includes(h.id)} onChange={() => toggleHotel(h.id)} style={{ display: "none" }} />
                        {h.name} <span style={{ opacity: .6 }}>({h.city})</span>
                      </label>
                    ))}
                  </div>
                ) : <p style={{ margin: 0, fontSize: 12, color: "var(--color-warm-400)" }}>Belum ada hotel untuk musim ini.</p>}

                <p style={section}>KLOTER DEFAULT (OPSIONAL)</p>
                <Field label="Jamaah yang beli paket ini otomatis ditempatkan ke kloter berikut">
                  <select className="safrat-input" value={form.defaultKloterId} onChange={(e) => update("defaultKloterId", e.target.value)} style={input}>
                    <option value="">— tidak ditentukan —</option>
                    {kloters.map((k) => <option key={k.id} value={k.id}>{k.code}</option>)}
                  </select>
                </Field>

                <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", borderBottom: "1px solid var(--color-cream-300)", paddingBottom: 8 }}>
                  <p style={{ ...section, border: 0, padding: 0 }}>ITINERARY HARIAN</p>
                  <button type="button" onClick={addDay} style={addDayBtn}><IconPlus size={14} />Tambah Hari</button>
                </div>
                <div style={{ display: "grid", gap: 10 }}>
                  {form.itinerary.map((day, i) => (
                    <div key={i} style={dayCard}>
                      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 8 }}>
                        <strong style={{ fontSize: 13 }}>Hari {day.dayNumber}</strong>
                        <button type="button" onClick={() => removeDay(i)} style={dayDeleteBtn} aria-label="Hapus hari"><IconTrash size={14} /></button>
                      </div>
                      <div style={cols}>
                        <input className="safrat-input" placeholder="Judul, mis. Tiba di Jeddah" value={day.title} onChange={(e) => updateDay(i, { title: e.target.value })} style={input} />
                        <input className="safrat-input" placeholder="Kota" value={day.city} onChange={(e) => updateDay(i, { city: e.target.value })} style={input} />
                      </div>
                      <textarea className="safrat-input" placeholder="Aktivitas hari ini" value={day.activities} onChange={(e) => updateDay(i, { activities: e.target.value })} style={{ ...input, minHeight: 60, padding: "10px 12px", marginTop: 8 }} />
                      <div style={{ display: "flex", gap: 14, marginTop: 8, fontSize: 12, color: "var(--color-warm-600)" }}>
                        <label style={mealCheck}><input type="checkbox" checked={day.mealBreakfast} onChange={(e) => updateDay(i, { mealBreakfast: e.target.checked })} />Sarapan</label>
                        <label style={mealCheck}><input type="checkbox" checked={day.mealLunch} onChange={(e) => updateDay(i, { mealLunch: e.target.checked })} />Makan Siang</label>
                        <label style={mealCheck}><input type="checkbox" checked={day.mealDinner} onChange={(e) => updateDay(i, { mealDinner: e.target.checked })} />Makan Malam</label>
                      </div>
                    </div>
                  ))}
                  {!form.itinerary.length && <p style={{ margin: 0, fontSize: 12, color: "var(--color-warm-400)" }}>Belum ada hari — klik &quot;Tambah Hari&quot; untuk mulai menyusun itinerary.</p>}
                </div>
              </>
            )}

            <p style={section}>PEMBAGIAN MARGIN (%)</p>
            <div style={cols}>
              <Field label="Platform (%)"><input className="safrat-input" type="number" min="0" max="100" value={form.platformMargin} onChange={(e) => update("platformMargin", e.target.value)} style={input} /></Field>
              <Field label="Operator (%)"><input className="safrat-input" type="number" min="0" max="100" value={form.operatorMargin} onChange={(e) => update("operatorMargin", e.target.value)} style={input} /></Field>
              <Field label="Agen (%)"><input className="safrat-input" type="number" min="0" max="100" value={form.agentMargin} onChange={(e) => update("agentMargin", e.target.value)} style={input} /></Field>
            </div>
            <p style={{ margin: 0, fontSize: 12, color: marginSum > 100 ? "var(--color-danger-600)" : "var(--color-warm-400)" }}>Total: {marginSum}% {marginSum > 100 ? "(sudah melebihi 100%)" : `(sisa ${100 - marginSum}% belum dialokasikan)`}</p>
            {errors.margins && <small style={{ color: "var(--color-danger-600)" }}>{errors.margins}</small>}
            {initial && <label style={check}><input type="checkbox" checked={form.active} onChange={(e) => update("active", e.target.checked)} /> Produk aktif</label>}
            {errors._form && <p style={error}>{errors._form}</p>}
          </form>
        </div>
        <div style={foot}><button form="product-form" disabled={saving} style={primary}>{saving ? "Menyimpan..." : "Simpan Produk"}</button></div>
      </aside>
    </div>
  );
}
function Field({ label, error, children }: { label: string; error?: string; children: React.ReactNode }) { return <label style={{ display: "grid", gap: 6 }}><span style={labelStyle}>{label}</span>{children}{error && <small style={{ color: "var(--color-danger-600)" }}>{error}</small>}</label>; }
const overlay: React.CSSProperties = { position: "fixed", inset: 0, zIndex: 30, display: "flex", justifyContent: "flex-end", background: "rgba(15,23,42,.48)", backdropFilter: "blur(2px)" };
const sheet: React.CSSProperties = { width: "min(560px,100%)", height: "100vh", display: "flex", flexDirection: "column", background: "#fff", borderRadius: "16px 0 0 16px", overflow: "hidden", boxShadow: "-6px 0 32px rgba(15,23,42,.12)" };
const head: React.CSSProperties = { padding: "20px 24px 16px", display: "flex", justifyContent: "space-between", alignItems: "center", borderBottom: "1px solid var(--color-cream-300)" };
const body: React.CSSProperties = { flex: 1, overflowY: "auto", padding: 24 };
const foot: React.CSSProperties = { padding: "16px 24px", borderTop: "1px solid var(--color-cream-300)" };
const closeBtn: React.CSSProperties = { width: 40, height: 40, borderRadius: "50%", border: "1px solid var(--color-cream-400)", background: "transparent", color: "var(--color-warm-400)", display: "grid", placeItems: "center" };
const eyebrow: React.CSSProperties = { margin: 0, color: "var(--color-gold-800)", fontSize: 11, fontWeight: 700, letterSpacing: ".1em" };
const section: React.CSSProperties = { margin: 0, fontSize: 11, fontWeight: 700, letterSpacing: ".1em", color: "var(--color-warm-400)", paddingBottom: 8, borderBottom: "1px solid var(--color-cream-300)" };
const input: React.CSSProperties = { minHeight: 48, width: "100%", border: "1.5px solid var(--color-cream-400)", borderRadius: 10, padding: "0 14px", background: "#fff", font: "inherit", outline: "none" };
const labelStyle: React.CSSProperties = { fontSize: 13, fontWeight: 600, color: "var(--color-warm-700)" };
const cols: React.CSSProperties = { display: "grid", gridTemplateColumns: "repeat(auto-fit,minmax(180px,1fr))", gap: 12 };
const primary: React.CSSProperties = { minHeight: 48, width: "100%", border: 0, borderRadius: 10, background: "var(--color-gold-500)", color: "#fff", fontWeight: 700 };
const count: React.CSSProperties = { textAlign: "right", color: "var(--color-warm-400)" };
const check: React.CSSProperties = { color: "var(--color-warm-700)", fontSize: 13 };
const error: React.CSSProperties = { margin: 0, color: "var(--color-danger-600)", background: "#ffe4e6", padding: 10, borderRadius: 8 };
const hotelGrid: React.CSSProperties = { display: "flex", flexWrap: "wrap", gap: 8 };
const hotelChip: React.CSSProperties = { minHeight: 36, display: "inline-flex", alignItems: "center", padding: "0 12px", borderRadius: 99, border: "1px solid var(--color-cream-400)", background: "#fff", color: "var(--color-warm-600)", fontSize: 13, cursor: "pointer" };
const hotelChipActive: React.CSSProperties = { background: "var(--color-emerald-900)", borderColor: "var(--color-emerald-900)", color: "#fff" };
const addDayBtn: React.CSSProperties = { minHeight: 32, border: "1px solid var(--color-emerald-800)", borderRadius: 8, padding: "0 10px", background: "transparent", color: "var(--color-emerald-900)", fontSize: 12, fontWeight: 700, display: "inline-flex", alignItems: "center", gap: 4 };
const dayCard: React.CSSProperties = { background: "var(--color-cream-100)", border: "1px solid var(--color-cream-300)", borderRadius: 10, padding: 12 };
const dayDeleteBtn: React.CSSProperties = { border: 0, background: "transparent", color: "var(--color-danger-600)", cursor: "pointer" };
const mealCheck: React.CSSProperties = { display: "inline-flex", alignItems: "center", gap: 4 };
