"use client";

import { FormEvent, useCallback, useEffect, useRef, useState } from "react";
import { funnelClient } from "@/lib/rpc";
import { useSearchParams } from "next/navigation";
import { Timestamp } from "@bufbuild/protobuf";
import { IconCheck } from "@tabler/icons-react";
import { RegistrationFormInfo } from "@hajj-saas/proto-gen/hajj/v1/registration_pb";
import { registrationClient } from "@/lib/rpc";

export default function PublicRegistrationForm({ operatorId, seasonId }: { operatorId: string; seasonId: string }) {
  const searchParams = useSearchParams();
  const referralCode = searchParams.get("ref") ?? "";
  const [formInfo, setFormInfo] = useState<RegistrationFormInfo>();
  const [loadError, setLoadError] = useState("");
  const [form, setForm] = useState({ fullName: "", passportNumber: "", gender: "MALE", phone: "", email: "", nationality: "IDN", address: "", dateOfBirth: "" });
  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);
  // A birthday cannot be in the future. Bounding the picker refuses it where
  // the mistake is made, instead of after a round trip.
  const today = new Date().toISOString().slice(0, 10);
  const errorRef = useRef<HTMLParagraphElement>(null);
  const successRef = useRef<HTMLHeadingElement>(null);
  const [message, setMessage] = useState("");
  // Read once on mount. The parameters live on the page the visitor arrived at,
  // and they have to survive to the submit — the funnel's own visitor token
  // resets at midnight, so a channel that starts a weeks-long decision would
  // otherwise never be credited with finishing it.
  const [attribution, setAttribution] = useState({ utmSource: "", utmCampaign: "" });
  const startedRef = useRef(false);

  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    setAttribution({ utmSource: params.get("utm_source") ?? "", utmCampaign: params.get("utm_campaign") ?? "" });
  }, []);

  // MULAI_ISI fires once, on the first field a person actually touches —
  // opening the page is already counted as KATALOG, and intent is what this
  // step is for. Failures are ignored: a form must never depend on analytics.
  const markStarted = useCallback(() => {
    if (startedRef.current) return;
    startedRef.current = true;
    const params = new URLSearchParams(window.location.search);
    void funnelClient
      .recordEvent({
        operatorId,
        step: "MULAI_ISI",
        path: "/register",
        utmSource: params.get("utm_source") ?? "",
        utmCampaign: params.get("utm_campaign") ?? "",
      })
      .catch(() => undefined);
  }, [operatorId]);

  useEffect(() => {
    registrationClient.getRegistrationForm({ operatorId, seasonId }).then(setFormInfo).catch(() => setLoadError("Tautan pendaftaran ini tidak valid atau sudah tidak aktif."));
  }, [operatorId, seasonId]);

  // Focus follows the error. Without this the form appears to do nothing when
  // submission fails from the bottom of a phone screen.
  useEffect(() => {
    if (error) errorRef.current?.focus();
  }, [error]);

  useEffect(() => {
    if (message) successRef.current?.focus();
  }, [message]);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    if (!form.fullName.trim() || !form.passportNumber.trim()) { setError("Nama lengkap dan nomor paspor wajib diisi."); return; }
    setSubmitting(true);
    try {
      const response = await registrationClient.submitRegistration({
        operatorId, seasonId,
        fullName: form.fullName.trim(),
        passportNumber: form.passportNumber.trim(),
        gender: form.gender,
        phone: form.phone.trim(),
        email: form.email.trim(),
        nationality: form.nationality.trim() || "IDN",
        address: form.address.trim(),
        dateOfBirth: form.dateOfBirth ? Timestamp.fromDate(new Date(`${form.dateOfBirth}T00:00:00Z`)) : undefined,
        referralCode,
        utmSource: attribution.utmSource,
        utmCampaign: attribution.utmCampaign,
      });
      setMessage(response.message);
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "Gagal mengirim pendaftaran Anda.");
    } finally {
      setSubmitting(false);
    }
  }

  if (loadError) {
    return <main style={page}><div style={card}><h1 style={title}>Tautan tidak valid</h1><p style={{ color: "var(--color-warm-500)" }}>{loadError}</p></div></main>;
  }

  if (message) {
    // The whole form is replaced, so the heading is what a screen reader lands
    // on — but only if something moves focus there. Otherwise focus stays on a
    // submit button that no longer exists and the page appears to have done
    // nothing.
    return (
      <main style={page}>
        <div style={card} role="status" aria-live="polite">
          <IconCheck size={40} color="var(--color-emerald-900)" />
          <h1 ref={successRef} tabIndex={-1} style={title}>Pendaftaran terkirim</h1>
          <p style={{ color: "var(--color-warm-500)" }}>{message}</p>
        </div>
      </main>
    );
  }

  return (
    <main style={page}>
      <div style={card}>
        <p style={eyebrow}>PENDAFTARAN JAMAAH</p>
        <h1 style={title}>{formInfo ? `Daftar ${formInfo.seasonName} bersama ${formInfo.operatorName}` : "Formulir Pendaftaran"}</h1>
        {!!formInfo?.availableProducts.length && <p style={{ color: "var(--color-warm-500)", fontSize: 13 }}>Paket tersedia: {formInfo.availableProducts.join(", ")}</p>}
        <form onFocusCapture={markStarted} onSubmit={submit} style={{ display: "grid", gap: 16, marginTop: 16 }}>
          <label style={{ display: "grid", gap: 6 }}><span style={label}>Nama lengkap (sesuai paspor)</span><input required autoComplete="name" autoCapitalize="words" className="safrat-input" value={form.fullName} onChange={(e) => setForm((f) => ({ ...f, fullName: e.target.value }))} style={input} /></label>
          <label style={{ display: "grid", gap: 6 }}><span style={label}>Nomor paspor</span><input required autoCapitalize="characters" autoCorrect="off" spellCheck={false} className="safrat-input" value={form.passportNumber} onChange={(e) => setForm((f) => ({ ...f, passportNumber: e.target.value.toUpperCase() }))} style={input} /></label>
          <label style={{ display: "grid", gap: 6 }}><span style={label}>Jenis kelamin</span><select className="safrat-input" value={form.gender} onChange={(e) => setForm((f) => ({ ...f, gender: e.target.value }))} style={input}><option value="MALE">Pria</option><option value="FEMALE">Wanita</option></select></label>
          <label style={{ display: "grid", gap: 6 }}><span style={label}>Tanggal lahir</span><input type="date" required autoComplete="bday" max={today} className="safrat-input" value={form.dateOfBirth} onChange={(e) => setForm((f) => ({ ...f, dateOfBirth: e.target.value }))} style={input} /></label>
          <label style={{ display: "grid", gap: 6 }}><span style={label}>Telepon</span><input type="tel" inputMode="tel" required autoComplete="tel" placeholder="08xxxxxxxxxx" className="safrat-input" value={form.phone} onChange={(e) => setForm((f) => ({ ...f, phone: e.target.value }))} style={input} /></label>
          <label style={{ display: "grid", gap: 6 }}><span style={label}>Email</span><input type="email" inputMode="email" required autoComplete="email" autoCapitalize="none" autoCorrect="off" spellCheck={false} className="safrat-input" value={form.email} onChange={(e) => setForm((f) => ({ ...f, email: e.target.value }))} style={input} /></label>
          <label style={{ display: "grid", gap: 6 }}><span style={label}>Kewarganegaraan</span><input required autoCapitalize="characters" className="safrat-input" value={form.nationality} onChange={(e) => setForm((f) => ({ ...f, nationality: e.target.value }))} style={input} /></label>
          <label style={{ display: "grid", gap: 6 }}><span style={label}>Alamat</span><textarea autoComplete="street-address" className="safrat-input" rows={3} value={form.address} onChange={(e) => setForm((f) => ({ ...f, address: e.target.value }))} style={{ ...input, minHeight: 80, resize: "vertical" as const }} /></label>
          {error && (
            // role="alert" so a screen reader announces it when it appears, and
            // tabIndex so the focus move below has somewhere to land. On a phone
            // the button sits below a long form; a message rendered silently
            // above it is a message nobody sees.
            <p ref={errorRef} tabIndex={-1} role="alert" style={errStyle}>{error}</p>
          )}
          <button disabled={submitting} style={primary}>{submitting ? "Mengirim..." : "Kirim pendaftaran"}</button>
        </form>
      </div>
    </main>
  );
}

const page: React.CSSProperties = { minHeight: "100vh", display: "grid", placeItems: "center", padding: 24, background: "var(--color-cream-100)" };
const card: React.CSSProperties = { width: "min(520px,100%)", background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 16, padding: 32, textAlign: "start", display: "grid", gap: 6 };
const eyebrow: React.CSSProperties = { margin: 0, color: "var(--color-gold-800)", fontSize: 11, fontWeight: 700, letterSpacing: ".1em" };
const title: React.CSSProperties = { margin: "4px 0 8px", fontSize: 26, fontFamily: "'Playfair Display', serif", color: "var(--color-emerald-900)" };
const label: React.CSSProperties = { fontSize: 13, fontWeight: 600, color: "var(--color-warm-700)" };
const input: React.CSSProperties = { minHeight: 48, width: "100%", border: "1.5px solid var(--color-cream-400)", borderRadius: 10, padding: "0 14px", background: "#fff", font: "inherit" };
const primary: React.CSSProperties = { minHeight: 48, border: 0, borderRadius: 10, background: "var(--color-gold-500)", color: "#fff", fontWeight: 700 };
const errStyle: React.CSSProperties = { margin: 0, padding: 10, borderRadius: 8, background: "var(--color-danger-100)", color: "var(--color-danger-600)" };
