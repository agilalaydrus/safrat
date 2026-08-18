"use client";

import { Timestamp } from "@bufbuild/protobuf";
import { FormEvent, useState } from "react";
import { useRouter } from "next/navigation";
import { SeasonType } from "@hajj-saas/proto-gen/hajj/v1/season_pb";
import { authClient } from "@/lib/auth-client";
import { operatorClient, seasonClient } from "@/lib/rpc";
import { SEASON_TYPE_OPTIONS } from "@/lib/season-types";

type FormValues = { name: string; country: string; seasonName: string; seasonType: SeasonType; startDate: string; endDate: string };
const initialValues: FormValues = { name: "", country: "", seasonName: "", seasonType: SeasonType.UMRAH_REGULER, startDate: "", endDate: "" };

export default function OnboardingPage() {
  const router = useRouter();
  const { data: session, isPending } = authClient.useSession();
  const [step, setStep] = useState(1);
  const [values, setValues] = useState(initialValues);
  const [error, setError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);

  function update<K extends keyof FormValues>(key: K, value: FormValues[K]) { setValues((current) => ({ ...current, [key]: value })); }
  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError(null);
    if (isPending) {
      setError("Mohon tunggu, sesi Anda sedang dimuat.");
      return;
    }
    if (!session?.user) {
      router.push("/sign-in");
      return;
    }
    setIsSubmitting(true);
    try {
      if (step === 1) {
        const result = await authClient.organization.create({ name: values.name, slug: organizationSlug(values.name) });
        if (result.error || !result.data) throw new Error(result.error?.message ?? "Gagal membuat organisasi.");
        await authClient.organization.setActive({ organizationId: result.data.id });
        await authClient.getSession();
        await operatorClient.createOperator({ betterAuthOrgId: result.data.id, name: values.name, country: values.country.toUpperCase(), email: session.user.email, licenseNumber: "" });
        setStep(2);
      } else {
        await seasonClient.createSeason({ name: values.seasonName, type: values.seasonType, startDate: Timestamp.fromDate(new Date(`${values.startDate}T00:00:00.000Z`)), endDate: Timestamp.fromDate(new Date(`${values.endDate}T00:00:00.000Z`)) });
        router.push("/dashboard");
      }
    } catch (caught) { setError(caught instanceof Error ? caught.message : "Gagal membuat ruang kerja Anda. Silakan coba lagi."); }
    finally { setIsSubmitting(false); }
  }

  if (isPending) {
    return <main style={{ maxWidth: 760, margin: "0 auto", padding: "10vh 24px" }}><p style={{ color: "var(--color-warm-400)" }}>Memuat sesi...</p></main>;
  }

  return <main style={{ maxWidth: 760, margin: "0 auto", padding: "10vh 24px" }}>
    <p style={{ color: "var(--gold)", fontWeight: 700 }}>LANGKAH {step} DARI 2</p>
    <h1 style={{ fontSize: "clamp(2.5rem, 8vw, 4.5rem)", margin: "8px 0 32px" }}>{step === 1 ? "Data operator Anda" : "Musim pertama"}</h1>
    <form onSubmit={submit} style={{ display: "grid", gap: 16 }}>
      {step === 1 && <><Field label="Nama perusahaan" value={values.name} onChange={(value) => update("name", value)} /><Field label="Negara (ISO-2, opsional)" value={values.country} onChange={(value) => update("country", value)} maxLength={2} required={false} /></>}
      {step === 2 && <><Field label="Nama musim" value={values.seasonName} onChange={(value) => update("seasonName", value)} placeholder="Umrah Ramadhan 2027" /><label>Jenis musim<select value={values.seasonType} onChange={(event) => update("seasonType", Number(event.target.value) as SeasonType)} style={inputStyle}>{SEASON_TYPE_OPTIONS.map((o) => <option key={o.value} value={o.value}>{o.label}</option>)}</select></label><Field label="Tanggal mulai" value={values.startDate} onChange={(value) => update("startDate", value)} type="date" /><Field label="Tanggal selesai" value={values.endDate} onChange={(value) => update("endDate", value)} type="date" /></>}
      {error && <p role="alert" style={{ color: "#b91c1c" }}>{error}</p>}
      <button type="submit" disabled={isSubmitting || isPending} style={{ ...inputStyle, border: 0, background: "var(--teal)", color: "white", fontWeight: 700, cursor: "pointer" }}>{isPending ? "Memuat..." : isSubmitting ? "Membuat ruang kerja..." : step === 1 ? "Lanjutkan" : "Buat ruang kerja"}</button>
    </form>
  </main>;
}

function Field({ label, value, onChange, type = "text", placeholder, maxLength, required = true }: { label: string; value: string; onChange: (value: string) => void; type?: string; placeholder?: string; maxLength?: number; required?: boolean }) { return <label>{label}<input required={required} value={value} onChange={(event) => onChange(event.target.value)} type={type} placeholder={placeholder} maxLength={maxLength} style={inputStyle} /></label>; }

function organizationSlug(name: string) {
  const base = name.toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/(^-|-$)/g, "") || "operator";
  return `${base}-${crypto.randomUUID().slice(0, 8)}`;
}

const inputStyle = { display: "block", width: "100%", marginTop: 8, padding: 12, font: "inherit", border: "1px solid #8ca49c", borderRadius: 2, background: "#fffdf8" };
