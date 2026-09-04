"use client";

import React, { FormEvent, useCallback, useEffect, useRef, useState } from "react";
import { Timestamp } from "@bufbuild/protobuf";
import { IconX } from "@tabler/icons-react";
import { Gender, Pilgrim } from "@hajj-saas/proto-gen/hajj/v1/pilgrim_pb";
import { Group } from "@hajj-saas/proto-gen/hajj/v1/group_pb";
import { Kloter } from "@hajj-saas/proto-gen/hajj/v1/kloter_pb";
import { groupClient, kloterClient, pilgrimClient } from "@/lib/rpc";

type FormValues = {
  fullName: string;
  passportNumber: string;
  nationality: string;
  dateOfBirth: string;
  gender: "" | "MALE" | "FEMALE";
  phone: string;
  email: string;
  emergencyContact: string;
  preferredLang: string;
  medicalNotes: string;
  requiresWheelchair: boolean;
  groupId: string;
  mahramId: string;
  kloterId: string;
};

type Props = {
  open: boolean;
  onClose: () => void;
  seasonId: string;
  pilgrims: Pilgrim[];
  onSaved: (name: string) => void;
  initial?: Pilgrim;
  /** Pre-selects a group when adding a new pilgrim from the Grup page — ignored in edit mode. */
  defaultGroupId?: string;
};

// The API stores ISO 3166-1 alpha-2 country codes; labels remain readable for coordinators.
export const NATIONALITIES = [
  ["AF", "Afghanistan"], ["DZ", "Algeria"], ["AZ", "Azerbaijan"], ["BH", "Bahrain"], ["BD", "Bangladesh"],
  ["BA", "Bosnia and Herzegovina"], ["BN", "Brunei"], ["TD", "Chad"], ["KM", "Comoros"], ["DJ", "Djibouti"],
  ["EG", "Egypt"], ["ER", "Eritrea"], ["ET", "Ethiopia"], ["GM", "Gambia"], ["GN", "Guinea"], ["ID", "Indonesia"],
  ["IR", "Iran"], ["IQ", "Iraq"], ["JO", "Jordan"], ["KZ", "Kazakhstan"], ["KW", "Kuwait"], ["KG", "Kyrgyzstan"],
  ["LB", "Lebanon"], ["LY", "Libya"], ["MY", "Malaysia"], ["MV", "Maldives"], ["ML", "Mali"], ["MR", "Mauritania"],
  ["MA", "Morocco"], ["NE", "Niger"], ["NG", "Nigeria"], ["OM", "Oman"], ["PK", "Pakistan"], ["PS", "Palestine"],
  ["QA", "Qatar"], ["SA", "Saudi Arabia"], ["SN", "Senegal"], ["SL", "Sierra Leone"], ["SO", "Somalia"],
  ["SD", "Sudan"], ["SY", "Syria"], ["TJ", "Tajikistan"], ["TN", "Tunisia"], ["TR", "Turkey"], ["TM", "Turkmenistan"],
  ["AE", "United Arab Emirates"], ["UZ", "Uzbekistan"], ["YE", "Yemen"], ["OT", "Other"],
] as const;

const EMPTY_FORM: FormValues = {
  fullName: "", passportNumber: "", nationality: "", dateOfBirth: "", gender: "",
  phone: "", email: "", emergencyContact: "", preferredLang: "ar", medicalNotes: "",
  requiresWheelchair: false, groupId: "", mahramId: "", kloterId: "",
};

const UUID_PATTERN = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

function valuesFromPilgrim(pilgrim?: Pilgrim): FormValues {
  if (!pilgrim) return { ...EMPTY_FORM };
  return {
    fullName: pilgrim.fullName,
    passportNumber: pilgrim.passportNumber,
    nationality: pilgrim.nationality,
    dateOfBirth: pilgrim.dateOfBirth?.toDate().toISOString().slice(0, 10) ?? "",
    gender: pilgrim.gender === Gender.FEMALE ? "FEMALE" : pilgrim.gender === Gender.MALE ? "MALE" : "",
    phone: pilgrim.phone,
    email: pilgrim.email,
    emergencyContact: pilgrim.emergencyContact,
    preferredLang: pilgrim.preferredLang,
    medicalNotes: pilgrim.medicalNotes,
    requiresWheelchair: pilgrim.requiresWheelchair,
    groupId: pilgrim.groupId,
    mahramId: pilgrim.mahramId,
    kloterId: pilgrim.kloterId,
  };
}

export default function PilgrimFormDialog({ open, onClose, seasonId, pilgrims, onSaved, initial, defaultGroupId }: Props) {
  const [saving, setSaving] = useState(false);
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});
  const [form, setForm] = useState<FormValues>(EMPTY_FORM);
  const [mahramPilgrims, setMahramPilgrims] = useState<Pilgrim[]>(pilgrims);
  const [loadingMahrams, setLoadingMahrams] = useState(false);
  const [groups, setGroups] = useState<Group[]>([]);
  const [kloters, setKloters] = useState<Kloter[]>([]);
  const initialFormValues = useRef<FormValues>(EMPTY_FORM);

  useEffect(() => {
    if (!open) return;
    const values = valuesFromPilgrim(initial);
    if (!initial && defaultGroupId) values.groupId = defaultGroupId;
    initialFormValues.current = values;
    setForm(values);
    setFieldErrors({});
  }, [open, initial, defaultGroupId]);

  useEffect(() => {
    if (!open || !seasonId) return;
    let cancelled = false;

    async function loadMahramPilgrims() {
      setLoadingMahrams(true);
      // The dashboard only holds one page of pilgrims; load candidates for the full season.
      setMahramPilgrims(pilgrims);
      try {
        const response = await pilgrimClient.listPilgrims({ seasonId, limit: 100, offset: 0 });
        if (!cancelled) setMahramPilgrims(response.pilgrims);
      } catch {
        // Keep the dashboard's current page available if the supplementary load fails.
      } finally {
        if (!cancelled) setLoadingMahrams(false);
      }
    }

    void loadMahramPilgrims();
    return () => { cancelled = true; };
  }, [open, pilgrims, seasonId]);

  useEffect(() => {
    if (!open || !seasonId) return;
    groupClient.listGroups({ seasonId }).then((response) => setGroups(response.groups)).catch(() => setGroups([]));
    kloterClient.listKloters({ seasonId }).then((response) => setKloters(response.kloters)).catch(() => setKloters([]));
  }, [open, seasonId]);

  const isDirty = JSON.stringify(form) !== JSON.stringify(initialFormValues.current);
  const eligibleMahrams = mahramPilgrims.filter((pilgrim) => {
    if (pilgrim.id === initial?.id) return false;
    if (form.gender === "FEMALE") return pilgrim.gender === Gender.MALE;
    return true;
  });

  const requestClose = useCallback((force = false) => {
    if (!force && saving) return;
    if (!force && isDirty && !window.confirm("Buang perubahan yang belum disimpan?")) return;
    onClose();
  }, [isDirty, onClose, saving]);

  useEffect(() => {
    if (!open) return;
    const handler = (event: KeyboardEvent) => {
      if (event.key === "Escape") requestClose();
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, [open, requestClose]);

  function update(key: keyof FormValues, value: string | boolean) {
    setForm((current) => ({ ...current, [key]: value }) as FormValues);
    setFieldErrors((current) => {
      if (!current[key]) return current;
      const { [key]: _removed, ...remaining } = current;
      return remaining;
    });
  }

  function validate(): Record<string, string> {
    const errs: Record<string, string> = {};
    if (!seasonId) errs._form = "Pilih musim aktif sebelum menambahkan jamaah.";
    if (!form.fullName.trim()) errs.fullName = "Nama lengkap wajib diisi.";
    if (!form.passportNumber.trim()) errs.passportNumber = "Nomor paspor wajib diisi.";
    if (!form.nationality) errs.nationality = "Kewarganegaraan wajib diisi.";
    else if (!/^[A-Z]{2}$/.test(form.nationality)) errs.nationality = "Pilih kewarganegaraan yang valid.";
    if (!form.dateOfBirth) errs.dateOfBirth = "Tanggal lahir wajib diisi.";
    if (!form.gender) errs.gender = "Jenis kelamin wajib diisi.";
    if (form.passportNumber && !/^[A-Z0-9]{5,20}$/.test(form.passportNumber.toUpperCase())) {
      errs.passportNumber = "Nomor paspor harus 5-20 karakter alfanumerik.";
    }
    if (form.dateOfBirth) {
      const dob = new Date(form.dateOfBirth);
      const today = new Date();
      const minAge = new Date(today.getFullYear() - 1, today.getMonth(), today.getDate());
      if (Number.isNaN(dob.getTime()) || dob >= today) errs.dateOfBirth = "Tanggal lahir harus di masa lalu.";
      else if (dob > minAge) errs.dateOfBirth = "Jamaah harus berusia minimal 1 tahun.";
    }
    if (form.phone && !/^\+?[\d\s\-()]{7,20}$/.test(form.phone)) {
      errs.phone = "Masukkan nomor telepon yang valid.";
    }
    if (form.emergencyContact && !/^\+?[\d\s\-()]{7,20}$/.test(form.emergencyContact)) {
      errs.emergencyContact = "Masukkan nomor telepon yang valid.";
    }
    if (form.email && !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(form.email)) {
      errs.email = "Masukkan alamat email yang valid.";
    }
    if (form.mahramId && !UUID_PATTERN.test(form.mahramId)) {
      errs.mahramId = "Pilih mahram yang valid.";
    }
    return errs;
  }

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setFieldErrors({});
    const errs = validate();
    if (Object.keys(errs).length > 0) {
      setFieldErrors(errs);
      window.requestAnimationFrame(() => {
        document.querySelector("[data-field-error]")?.scrollIntoView({ behavior: "smooth", block: "center" });
      });
      return;
    }

    setSaving(true);
    try {
      const payload = {
        seasonId,
        groupId: form.groupId.trim(),
        fullName: form.fullName.trim(),
        passportNumber: form.passportNumber.toUpperCase().trim(),
        nationality: form.nationality,
        dateOfBirth: Timestamp.fromDate(new Date(`${form.dateOfBirth}T00:00:00Z`)),
        gender: form.gender === "FEMALE" ? Gender.FEMALE : Gender.MALE,
        phone: form.phone.trim(),
        email: form.email.trim().toLowerCase(),
        emergencyContact: form.emergencyContact.trim(),
        preferredLang: form.preferredLang,
        medicalNotes: form.medicalNotes.trim(),
        requiresWheelchair: form.requiresWheelchair,
        mahramId: form.mahramId,
        kloterId: form.kloterId,
      };
      if (initial) await pilgrimClient.updatePilgrim({ ...payload, pilgrimId: initial.id });
      else await pilgrimClient.createPilgrim(payload);
      onSaved(payload.fullName);
      requestClose(true);
    } catch (caught) {
      const message = caught instanceof Error ? caught.message : "Gagal menyimpan data jamaah.";
      if (/email/i.test(message) && /already|duplicate|exists/i.test(message)) {
        setFieldErrors({ email: "Email ini sudah terhubung dengan jamaah lain." });
      } else if (/already|duplicate|exists/i.test(message)) {
        setFieldErrors({ passportNumber: "Paspor ini sudah terdaftar untuk musim ini." });
      } else if (/unauthenticated/i.test(message)) {
        setFieldErrors({ _form: "Sesi Anda telah berakhir. Silakan masuk kembali." });
      } else if (/invalid_argument|validation failed/i.test(message)) {
        setFieldErrors({ _form: "Data jamaah belum lengkap atau belum sesuai. Periksa kembali isian Anda." });
      } else {
        setFieldErrors({ _form: "Data jamaah belum dapat disimpan. Silakan coba lagi." });
      }
    } finally {
      setSaving(false);
    }
  }

  if (!open) return null;

  return (
    <div role="dialog" aria-modal="true" aria-label={initial ? "Ubah data jamaah" : "Tambah jamaah"} style={overlay}>
      <aside style={sheet}>
        <div style={stickyHeader}>
          <div>
            <p style={eyebrow}>DATA JAMAAH</p>
            <h2 style={{ margin: 0 }}>{initial ? "Ubah data jamaah" : "Tambah jamaah"}</h2>
          </div>
          <button type="button" className="btn-close-sheet" onClick={() => requestClose()} style={closeBtn} aria-label="Tutup"><IconX size={18} /></button>
        </div>
        <div style={formBody}>
        <form id="pilgrim-form" onSubmit={submit} style={{ display: "grid", gap: 24 }}>
          <Section title="Identitas">
            <Field fieldKey="fullName" label="Nama lengkap" required error={fieldErrors.fullName}>
              <input className="safrat-input" required value={form.fullName} onChange={(event) => update("fullName", event.target.value)} style={input} aria-invalid={Boolean(fieldErrors.fullName)} />
            </Field>
            <Field fieldKey="passportNumber" label="Nomor paspor" required error={fieldErrors.passportNumber}>
              <input className="safrat-input" required value={form.passportNumber} onChange={(event) => update("passportNumber", event.target.value.toUpperCase())} style={input} aria-invalid={Boolean(fieldErrors.passportNumber)} />
            </Field>
            <Field fieldKey="nationality" label="Kewarganegaraan" required error={fieldErrors.nationality}>
              <select className="safrat-input" required value={form.nationality} onChange={(event) => update("nationality", event.target.value)} style={input} aria-invalid={Boolean(fieldErrors.nationality)}>
                <option value="">Pilih kewarganegaraan</option>
                {NATIONALITIES.map(([code, name]) => <option key={code} value={code}>{name}</option>)}
              </select>
            </Field>
            <Field fieldKey="dateOfBirth" label="Tanggal lahir" required error={fieldErrors.dateOfBirth}>
              <input className="safrat-input" required type="date" value={form.dateOfBirth} onChange={(event) => update("dateOfBirth", event.target.value)} style={input} aria-invalid={Boolean(fieldErrors.dateOfBirth)} />
            </Field>
            <Field fieldKey="gender" label="Jenis kelamin" required error={fieldErrors.gender}>
              <div className="safrat-radio-group">
                {(["MALE", "FEMALE"] as const).map((value) => (
                  <label key={value} className={`safrat-radio-label${form.gender === value ? " selected" : ""}`} onClick={() => update("gender", value)}>
                    <input type="radio" name="gender" checked={form.gender === value} onChange={() => update("gender", value)} />
                    <span className="safrat-radio-dot"><span className="safrat-radio-dot-inner" /></span>
                    {value === "MALE" ? "Pria" : "Wanita"}
                  </label>
                ))}
              </div>
            </Field>
          </Section>

          <div style={sectionDivider}>
            <Section title="Kontak">
              <Field fieldKey="phone" label="Telepon" error={fieldErrors.phone}>
                <input className="safrat-input" value={form.phone} onChange={(event) => update("phone", event.target.value)} style={input} aria-invalid={Boolean(fieldErrors.phone)} />
              </Field>
              <Field fieldKey="email" label="Email" hint="Jamaah masuk pakai email ini di halaman Masuk yang sama seperti staf, tanpa perlu link kode. Kosongkan jika belum ada." error={fieldErrors.email}>
                <input className="safrat-input" type="email" value={form.email} onChange={(event) => update("email", event.target.value)} style={input} aria-invalid={Boolean(fieldErrors.email)} />
              </Field>
              <Field fieldKey="emergencyContact" label="Kontak darurat" error={fieldErrors.emergencyContact}>
                <input className="safrat-input" value={form.emergencyContact} onChange={(event) => update("emergencyContact", event.target.value)} style={input} aria-invalid={Boolean(fieldErrors.emergencyContact)} />
              </Field>
            </Section>
          </div>

          <div style={sectionDivider}>
            <Section title="Preferensi">
              <Field fieldKey="preferredLang" label="Bahasa yang disukai">
                <select className="safrat-input" value={form.preferredLang} onChange={(event) => update("preferredLang", event.target.value)} style={input}>
                  <option value="ar">Arab</option><option value="en">Inggris</option><option value="id">Bahasa Indonesia</option>
                </select>
              </Field>
              <Field fieldKey="medicalNotes" label="Catatan medis">
                <textarea className="safrat-input" value={form.medicalNotes} maxLength={500} onChange={(event) => update("medicalNotes", event.target.value)} style={{ ...input, minHeight: 90, padding: "10px 14px" }} />
                <span style={characterCount}>{form.medicalNotes.length}/500</span>
              </Field>
              <div>
                <label style={{ display: "flex", gap: 10, alignItems: "center" }}>
                  <input className="safrat-input" type="checkbox" checked={form.requiresWheelchair} onChange={(event) => update("requiresWheelchair", event.target.checked)} />
                  Membutuhkan kursi roda
                </label>
                {form.requiresWheelchair && form.gender === "FEMALE" && !form.mahramId && (
                  <p style={advisory}>Perhatian: Jamaah pengguna kursi roda umumnya membutuhkan mahram. Pertimbangkan untuk menetapkan satu.</p>
                )}
              </div>
            </Section>
          </div>

          <div style={sectionDivider}>
            <Section title="Grup & Mahram">
              <Field fieldKey="kloterId" label="Kloter Keberangkatan" hint="Biarkan kosong untuk ditentukan nanti dari halaman Kloter.">
                <select className="safrat-input" value={form.kloterId} onChange={(event) => update("kloterId", event.target.value)} style={input}>
                  <option value="">Belum ditentukan</option>
                  {kloters.map((kloter) => <option key={kloter.id} value={kloter.id}>{kloter.code}{kloter.embarkation ? ` · ${kloter.embarkation}` : ""}</option>)}
                </select>
              </Field>
              <Field fieldKey="groupId" label="Grup" hint="Biarkan kosong untuk ditentukan nanti dari halaman Grup.">
                <select className="safrat-input" value={form.groupId} onChange={(event) => update("groupId", event.target.value)} style={input}>
                  <option value="">Belum ditentukan</option>
                  {groups.map((group) => <option key={group.id} value={group.id}>{group.name} ({group.pilgrimCount}/{group.capacity} jamaah)</option>)}
                </select>
              </Field>
              <Field fieldKey="mahramId" label="Mahram" hint="Untuk jamaah wanita: pilih wali/mahram laki-laki" error={fieldErrors.mahramId}>
                <select className="safrat-input" value={form.mahramId} onChange={(event) => update("mahramId", event.target.value)} style={input} aria-invalid={Boolean(fieldErrors.mahramId)}>
                  {loadingMahrams ? <option value="">Memuat jamaah yang memenuhi syarat...</option> : eligibleMahrams.length === 0 ? <option value="">Tidak ada jamaah yang memenuhi syarat</option> : <option value="">Tidak ada mahram dipilih</option>}
                  {eligibleMahrams.map((pilgrim) => <option key={pilgrim.id} value={pilgrim.id}>{pilgrim.fullName} · {pilgrim.passportNumber}</option>)}
                </select>
              </Field>
            </Section>
          </div>

          {fieldErrors._form && <p role="alert" style={formError}>{fieldErrors._form}</p>}
        </form>
        </div>
        <div style={stickyFooter}>
          <button form="pilgrim-form" disabled={saving || !seasonId} style={primary}>{saving ? "Menyimpan…" : initial ? "Simpan perubahan" : "Tambah jamaah"}</button>
        </div>
      </aside>
    </div>
  );
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return <section style={{ display: "grid", gap: 16 }}><p style={sectionTitle}>{title}</p>{children}</section>;
}

// The label was a <span>, so nothing connected it to its control: a screen
// reader announced thirteen unlabelled boxes, and the hint and the validation
// error were never read out with the field they belonged to. This is the
// longest form in the app and the one a jamaah's whole record is typed into.
//
// fieldKey is already unique per field, so it is used to generate the ids —
// no new prop, and nothing to keep in step by hand.
function Field({ fieldKey, label, required, hint, error, children }: { fieldKey: string; label: string; required?: boolean; hint?: string; error?: string; children: React.ReactNode }) {
  const controlId = `field-${fieldKey}`;
  const hintId = hint ? `${controlId}-hint` : undefined;
  const errorId = error ? `${controlId}-error` : undefined;
  // Both, when both exist: a reader should hear the guidance and what went
  // wrong, not one at the expense of the other.
  const describedBy = [hintId, errorId].filter(Boolean).join(" ") || undefined;

  const control = React.isValidElement(children)
    ? React.cloneElement(children as React.ReactElement<{ id?: string; "aria-describedby"?: string }>, {
        id: controlId,
        "aria-describedby": describedBy,
      })
    : children;

  return (
    <div style={{ display: "grid", gap: 6 }}>
      <label htmlFor={controlId} style={fieldLabel}>
        {label}
        {required && <span style={{ color: "var(--color-danger-600)", marginLeft: 2, fontWeight: 400 }}>*</span>}
      </label>
      {control}
      {hint && <span id={hintId} style={{ fontSize: 11, color: "var(--color-warm-400)", marginTop: 2 }}>{hint}</span>}
      {/* role="alert" so a validation failure is announced when it appears,
          rather than only being found by someone who happens to navigate back
          to the field. */}
      {error && <span id={errorId} role="alert" data-field-error={fieldKey} style={{ fontSize: 11, color: "var(--color-danger-600)" }}>{error}</span>}
    </div>
  );
}

const overlay: React.CSSProperties = { position: "fixed", inset: 0, zIndex: 20, display: "flex", justifyContent: "flex-end", background: "rgba(15,23,42,.48)", backdropFilter: "blur(2px)", WebkitBackdropFilter: "blur(2px)" };
const sheet: React.CSSProperties = { width: "min(560px,100%)", height: "100vh", display: "flex", flexDirection: "column", background: "#ffffff", boxShadow: "-6px 0 32px rgba(15,23,42,.12)", borderRadius: "16px 0 0 16px", animation: "sheet-in .22s cubic-bezier(0,0,.2,1)", overflow: "hidden" };
const stickyHeader: React.CSSProperties = { position: "sticky", top: 0, zIndex: 10, background: "#ffffff", borderBottom: "1px solid var(--color-cream-300)", padding: "20px 24px 16px", display: "flex", alignItems: "center", justifyContent: "space-between", gap: 16, flexShrink: 0 };
const closeBtn: React.CSSProperties = { width: 40, height: 40, borderRadius: "50%", border: "1px solid var(--color-cream-400)", background: "transparent", color: "var(--color-warm-400)", display: "flex", alignItems: "center", justifyContent: "center", cursor: "pointer", flexShrink: 0, transition: "background .15s, color .15s" };
const formBody: React.CSSProperties = { flex: 1, overflowY: "auto", padding: "24px", display: "grid", gap: 0 };
const stickyFooter: React.CSSProperties = { position: "sticky", bottom: 0, background: "#ffffff", borderTop: "1px solid var(--color-cream-300)", padding: "16px 24px", flexShrink: 0 };
const eyebrow: React.CSSProperties = { margin: "0 0 6px", color: "var(--color-gold-800)", fontSize: 11, fontWeight: 700, letterSpacing: ".08em" };
const input: React.CSSProperties = { minHeight: 48, width: "100%", border: "1.5px solid var(--color-cream-400)", borderRadius: 10, padding: "0 14px", background: "#ffffff", font: "inherit", color: "var(--color-warm-900)", outline: "none", transition: "border-color .15s, box-shadow .15s" };
const primary: React.CSSProperties = { minHeight: 48, border: 0, borderRadius: 10, background: "var(--color-gold-500)", color: "#fff", fontWeight: 700, padding: "0 20px", cursor: "pointer", fontFamily: "'Plus Jakarta Sans', sans-serif", fontSize: 14, width: "100%" };
const sectionDivider: React.CSSProperties = { paddingTop: 28 };
const sectionTitle: React.CSSProperties = { margin: 0, fontSize: 11, fontWeight: 700, letterSpacing: ".1em", textTransform: "uppercase", color: "var(--color-warm-400)", paddingBottom: 8, borderBottom: "1px solid var(--color-cream-300)", fontFamily: "'Plus Jakarta Sans', sans-serif" };
const fieldLabel: React.CSSProperties = { fontSize: 13, fontWeight: 600, color: "var(--color-warm-700)", display: "block", marginBottom: 6 };
const characterCount: React.CSSProperties = { fontSize: 11, color: "var(--color-warm-400)", textAlign: "right", display: "block" };
const advisory: React.CSSProperties = { fontSize: 11, color: "var(--color-gold-800)", marginTop: 4, background: "var(--color-gold-50)", padding: "8px 10px", borderRadius: 6 };
const formError: React.CSSProperties = { margin: 0, fontSize: 13, color: "var(--color-danger-600)", background: "var(--color-danger-100)", padding: "10px 12px", borderRadius: 8 };
