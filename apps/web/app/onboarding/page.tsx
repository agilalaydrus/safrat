"use client";

import { Timestamp } from "@bufbuild/protobuf";
import { FormEvent, useState } from "react";
import { useRouter } from "next/navigation";
import { SeasonType } from "@hajj-saas/proto-gen/hajj/v1/season_pb";
import { authClient } from "@/lib/auth-client";
import { operatorClient, seasonClient } from "@/lib/rpc";

type FormValues = { name: string; country: string; seasonName: string; seasonType: "HAJJ" | "UMRAH"; startDate: string; endDate: string };
const initialValues: FormValues = { name: "", country: "", seasonName: "", seasonType: "HAJJ", startDate: "", endDate: "" };

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
      setError("Please wait while we load your session.");
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
        if (result.error || !result.data) throw new Error(result.error?.message ?? "Could not create organization.");
        await authClient.organization.setActive({ organizationId: result.data.id });
        await authClient.getSession();
        await operatorClient.createOperator({ betterAuthOrgId: result.data.id, name: values.name, country: values.country.toUpperCase(), email: session.user.email, licenseNumber: "" });
        setStep(2);
      } else {
        await seasonClient.createSeason({ name: values.seasonName, type: values.seasonType === "HAJJ" ? SeasonType.HAJJ : SeasonType.UMRAH, startDate: Timestamp.fromDate(new Date(`${values.startDate}T00:00:00.000Z`)), endDate: Timestamp.fromDate(new Date(`${values.endDate}T00:00:00.000Z`)) });
        router.refresh();
        router.push("/dashboard");
      }
    } catch (caught) { setError(caught instanceof Error ? caught.message : "We could not create your workspace. Please try again."); }
    finally { setIsSubmitting(false); }
  }

  if (isPending) {
    return <main style={{ maxWidth: 760, margin: "0 auto", padding: "10vh 24px" }}><p style={{ color: "var(--color-warm-400)" }}>Loading session...</p></main>;
  }

  return <main style={{ maxWidth: 760, margin: "0 auto", padding: "10vh 24px" }}>
    <p style={{ color: "var(--gold)", fontWeight: 700 }}>STEP {step} OF 2</p>
    <h1 style={{ fontSize: "clamp(2.5rem, 8vw, 4.5rem)", margin: "8px 0 32px" }}>{step === 1 ? "Your operation" : "First season"}</h1>
    <form onSubmit={submit} style={{ display: "grid", gap: 16 }}>
      {step === 1 && <><Field label="Company name" value={values.name} onChange={(value) => update("name", value)} /><Field label="Country (ISO-2, optional)" value={values.country} onChange={(value) => update("country", value)} maxLength={2} required={false} /></>}
      {step === 2 && <><Field label="Season name" value={values.seasonName} onChange={(value) => update("seasonName", value)} placeholder="Hajj 2027" /><label>Journey type<select value={values.seasonType} onChange={(event) => update("seasonType", event.target.value as FormValues["seasonType"])} style={inputStyle}><option value="HAJJ">Hajj</option><option value="UMRAH">Umrah</option></select></label><Field label="Start date" value={values.startDate} onChange={(value) => update("startDate", value)} type="date" /><Field label="End date" value={values.endDate} onChange={(value) => update("endDate", value)} type="date" /></>}
      {error && <p role="alert" style={{ color: "#b91c1c" }}>{error}</p>}
      <button type="submit" disabled={isSubmitting || isPending} style={{ ...inputStyle, border: 0, background: "var(--teal)", color: "white", fontWeight: 700, cursor: "pointer" }}>{isPending ? "Loading..." : isSubmitting ? "Creating workspace..." : step === 1 ? "Continue" : "Create workspace"}</button>
    </form>
  </main>;
}

function Field({ label, value, onChange, type = "text", placeholder, maxLength, required = true }: { label: string; value: string; onChange: (value: string) => void; type?: string; placeholder?: string; maxLength?: number; required?: boolean }) { return <label>{label}<input required={required} value={value} onChange={(event) => onChange(event.target.value)} type={type} placeholder={placeholder} maxLength={maxLength} style={inputStyle} /></label>; }

function organizationSlug(name: string) {
  const base = name.toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/(^-|-$)/g, "") || "operator";
  return `${base}-${crypto.randomUUID().slice(0, 8)}`;
}

const inputStyle = { display: "block", width: "100%", marginTop: 8, padding: 12, font: "inherit", border: "1px solid #8ca49c", borderRadius: 2, background: "#fffdf8" };
