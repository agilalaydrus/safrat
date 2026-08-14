"use client";

import { FormEvent, useEffect, useState } from "react";
import { Timestamp } from "@bufbuild/protobuf";
import { IconX } from "@tabler/icons-react";
import { accommodationClient } from "@/lib/rpc";

type Props = { open: boolean; seasonId: string; onClose: () => void; onSaved: () => void };

const EMPTY_FORM = { name: "", city: "", starRating: "", address: "", checkInDate: "", checkOutDate: "" };

export default function HotelFormDialog({ open, seasonId, onClose, onSaved }: Props) {
  const [form, setForm] = useState(EMPTY_FORM);
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (!open) return;
    setForm(EMPTY_FORM);
    setFieldErrors({});
  }, [open]);

  if (!open) return null;

  const update = (key: keyof typeof form, value: string) => setForm((current) => ({ ...current, [key]: value }));

  function validate(): Record<string, string> {
    const errs: Record<string, string> = {};
    if (!form.name.trim()) errs.name = "Hotel name is required.";
    if (!form.city) errs.city = "City is required.";
    if (form.checkInDate && form.checkOutDate && form.checkOutDate < form.checkInDate) {
      errs.checkOutDate = "Check-out must be on or after check-in date.";
    }
    return errs;
  }

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setFieldErrors({});
    const errs = validate();
    if (Object.keys(errs).length) {
      setFieldErrors(errs);
      return;
    }
    setSaving(true);
    try {
      await accommodationClient.createHotel({
        seasonId,
        name: form.name.trim(),
        city: form.city,
        starRating: Number(form.starRating) || 0,
        address: form.address.trim(),
        checkInDate: form.checkInDate ? Timestamp.fromDate(new Date(`${form.checkInDate}T00:00:00Z`)) : undefined,
        checkOutDate: form.checkOutDate ? Timestamp.fromDate(new Date(`${form.checkOutDate}T00:00:00Z`)) : undefined,
      });
      onSaved();
      onClose();
    } catch (caught) {
      setFieldErrors({ _form: caught instanceof Error ? caught.message : "Unable to add hotel." });
    } finally {
      setSaving(false);
    }
  }

  return (
    <div role="dialog" aria-modal="true" aria-label="Add hotel" style={overlay}>
      <aside style={sheet}>
        <div style={stickyHeader}>
          <div><p style={eyebrow}>ACCOMMODATION</p><h2 style={{ margin: 0 }}>Add hotel</h2></div>
          <button type="button" className="btn-close-sheet" onClick={onClose} style={closeBtn} aria-label="Close"><IconX size={18} /></button>
        </div>
        <div style={formBody}>
          <form id="hotel-form" onSubmit={submit} style={{ display: "grid", gap: 20 }}>
            <Section title="Hotel details">
              <Field label="Hotel name" required error={fieldErrors.name}>
                <input className="safrat-input" required value={form.name} onChange={(event) => update("name", event.target.value)} style={input} aria-invalid={Boolean(fieldErrors.name)} />
              </Field>
              <Field label="City" required error={fieldErrors.city}>
                <select className="safrat-input" required value={form.city} onChange={(event) => update("city", event.target.value)} style={input} aria-invalid={Boolean(fieldErrors.city)}>
                  <option value="">Select city</option>
                  {["makkah", "madinah", "jeddah", "mina", "arafah"].map((city) => <option key={city} value={city}>{city.charAt(0).toUpperCase() + city.slice(1)}</option>)}
                </select>
              </Field>
              <Field label="Star rating">
                <select className="safrat-input" value={form.starRating} onChange={(event) => update("starRating", event.target.value)} style={input}>
                  <option value="">Not rated</option>
                  {[1, 2, 3, 4, 5].map((rating) => <option key={rating} value={rating}>{rating} star{rating === 1 ? "" : "s"}</option>)}
                </select>
              </Field>
              <Field label="Address"><textarea className="safrat-input" value={form.address} onChange={(event) => update("address", event.target.value)} style={{ ...input, minHeight: 88, padding: "10px 14px" }} /></Field>
            </Section>
            <div style={sectionDivider}>
              <Section title="Stay dates">
                <div style={twoCols}>
                  <Field label="Check-in date"><input className="safrat-input" type="date" value={form.checkInDate} onChange={(event) => update("checkInDate", event.target.value)} style={input} /></Field>
                  <Field label="Check-out date" error={fieldErrors.checkOutDate}><input className="safrat-input" type="date" min={form.checkInDate || undefined} value={form.checkOutDate} onChange={(event) => update("checkOutDate", event.target.value)} style={input} aria-invalid={Boolean(fieldErrors.checkOutDate)} /></Field>
                </div>
              </Section>
            </div>
            {fieldErrors._form && <p role="alert" style={formError}>{fieldErrors._form}</p>}
          </form>
        </div>
        <div style={stickyFooter}><button form="hotel-form" disabled={saving} style={primary}>{saving ? "Adding..." : "Add hotel"}</button></div>
      </aside>
    </div>
  );
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return <section style={{ display: "grid", gap: 16 }}><p style={sectionTitle}>{title}</p>{children}</section>;
}

function Field({ label, required, error, children }: { label: string; required?: boolean; error?: string; children: React.ReactNode }) {
  return <label style={{ display: "grid", gap: 6 }}><span style={fieldLabel}>{label}{required && <span style={{ color: "var(--color-danger-600)", marginLeft: 2, fontWeight: 400 }}>*</span>}</span>{children}{error && <span style={{ fontSize: 11, color: "var(--color-danger-600)" }}>{error}</span>}</label>;
}

const overlay: React.CSSProperties = { position: "fixed", inset: 0, zIndex: 30, display: "flex", justifyContent: "flex-end", background: "rgba(26,20,16,.48)", backdropFilter: "blur(2px)", WebkitBackdropFilter: "blur(2px)" };
const sheet: React.CSSProperties = { width: "min(560px,100%)", height: "100vh", display: "flex", flexDirection: "column", background: "#ffffff", boxShadow: "-6px 0 32px rgba(26,20,16,.12)", borderRadius: "16px 0 0 16px", animation: "sheet-in .22s cubic-bezier(0,0,.2,1)", overflow: "hidden" };
const stickyHeader: React.CSSProperties = { position: "sticky", top: 0, zIndex: 10, background: "#ffffff", borderBottom: "1px solid var(--color-cream-300)", padding: "20px 24px 16px", display: "flex", alignItems: "center", justifyContent: "space-between", gap: 16, flexShrink: 0 };
const closeBtn: React.CSSProperties = { width: 40, height: 40, borderRadius: "50%", border: "1px solid var(--color-cream-400)", background: "transparent", color: "var(--color-warm-400)", display: "flex", alignItems: "center", justifyContent: "center", cursor: "pointer", flexShrink: 0, transition: "background .15s, color .15s" };
const formBody: React.CSSProperties = { flex: 1, overflowY: "auto", padding: "24px", display: "grid", gap: 0 };
const stickyFooter: React.CSSProperties = { position: "sticky", bottom: 0, background: "#ffffff", borderTop: "1px solid var(--color-cream-300)", padding: "16px 24px", flexShrink: 0 };
const eyebrow: React.CSSProperties = { margin: "0 0 6px", color: "var(--color-gold-800)", fontSize: 11, fontWeight: 700, letterSpacing: ".08em" };
const input: React.CSSProperties = { minHeight: 48, width: "100%", border: "1.5px solid var(--color-cream-400)", borderRadius: 10, padding: "0 14px", background: "#ffffff", font: "inherit", color: "var(--color-warm-900)", outline: "none", transition: "border-color .15s, box-shadow .15s" };
const primary: React.CSSProperties = { minHeight: 48, border: 0, borderRadius: 10, background: "var(--color-gold-500)", color: "var(--color-warm-900)", fontWeight: 700, padding: "0 20px", cursor: "pointer", fontFamily: "'Plus Jakarta Sans', sans-serif", fontSize: 14, width: "100%" };
const twoCols: React.CSSProperties = { display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(180px, 1fr))", gap: 12 };
const sectionDivider: React.CSSProperties = { paddingTop: 28 };
const sectionTitle: React.CSSProperties = { margin: 0, fontSize: 11, fontWeight: 700, letterSpacing: ".1em", textTransform: "uppercase", color: "var(--color-warm-400)", paddingBottom: 8, borderBottom: "1px solid var(--color-cream-300)", fontFamily: "'Plus Jakarta Sans', sans-serif" };
const fieldLabel: React.CSSProperties = { fontSize: 13, fontWeight: 600, color: "var(--color-warm-700)", display: "block", marginBottom: 6 };
const formError: React.CSSProperties = { margin: 0, fontSize: 13, color: "var(--color-danger-600)", background: "#fdf0f0", padding: "10px 12px", borderRadius: 8 };
