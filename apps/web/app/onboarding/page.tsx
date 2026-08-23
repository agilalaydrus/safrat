"use client";

import { Timestamp } from "@bufbuild/protobuf";
import { Code, ConnectError } from "@connectrpc/connect";
import { FormEvent, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { SeasonType } from "@hajj-saas/proto-gen/hajj/v1/season_pb";
import { authClient } from "@/lib/auth-client";
import { operatorClient, seasonClient } from "@/lib/rpc";
import { SEASON_TYPE_OPTIONS } from "@/lib/season-types";

type Values = {
  name: string;
  licenseNumber: string;
  country: string;
  description: string;
  whatsappNumber: string;
  city: string;
  website: string;
  seasonName: string;
  seasonType: SeasonType;
  startDate: string;
  endDate: string;
};

const initialValues: Values = {
  name: "",
  licenseNumber: "",
  country: "ID",
  description: "",
  whatsappNumber: "",
  city: "",
  website: "",
  seasonName: "",
  seasonType: SeasonType.UMRAH_REGULER,
  startDate: "",
  endDate: "",
};

const STEP_LABELS = ["Profil Operator", "Detail Publik", "Musim Pertama"];

export default function OnboardingPage() {
  const router = useRouter();
  const { data: session, isPending } = authClient.useSession();
  const [step, setStep] = useState(1);
  const [values, setValues] = useState(initialValues);
  const [orgCreated, setOrgCreated] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const submittingRef = useRef(false);

  function update<K extends keyof Values>(key: K, value: Values[K]) {
    setValues((current) => ({ ...current, [key]: value }));
  }

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (submittingRef.current) return;
    submittingRef.current = true;
    setError(null);
    if (isPending) {
      setError("Mohon tunggu, sesi Anda sedang dimuat.");
      submittingRef.current = false;
      return;
    }
    if (!session?.user) {
      submittingRef.current = false;
      router.push("/sign-in");
      return;
    }
    setBusy(true);
    try {
      if (step === 1) {
        // Create the org + operator once; re-entering step 1 (via Kembali)
        // must not create a duplicate organization.
        if (!orgCreated) {
          const result = await authClient.organization.create({ name: values.name, slug: organizationSlug(values.name) });
          if (result.error || !result.data) throw new Error(result.error?.message ?? "Gagal membuat organisasi.");
          await authClient.organization.setActive({ organizationId: result.data.id });
          await authClient.getSession();
          await operatorClient.createOperator({
            betterAuthOrgId: result.data.id,
            name: values.name,
            country: values.country.toUpperCase(),
            email: session.user.email,
            licenseNumber: values.licenseNumber,
          });
          setOrgCreated(true);
        }
        setStep(2);
      } else if (step === 2) {
        setStep(3);
      } else {
        await seasonClient.createSeason({
          name: values.seasonName,
          type: values.seasonType,
          startDate: Timestamp.fromDate(new Date(`${values.startDate}T00:00:00.000Z`)),
          endDate: Timestamp.fromDate(new Date(`${values.endDate}T00:00:00.000Z`)),
        });
        await operatorClient.updateMyProfile({
          logoUrl: "",
          description: values.description,
          whatsappNumber: values.whatsappNumber,
          website: values.website,
          address: "",
          city: values.city,
        });
        router.push("/dashboard");
      }
    } catch (caught) {
      const message = ConnectError.from(caught).code === Code.AlreadyExists
        ? "Nama musim sudah digunakan. Ubah musim yang ada atau gunakan nama lain."
        : caught instanceof Error ? caught.message : "Terjadi kesalahan. Silakan coba lagi.";
      setError(message);
    } finally {
      submittingRef.current = false;
      setBusy(false);
    }
  }

  if (isPending) {
    return (
      <main style={pageStyle}>
        <p style={{ color: "var(--color-warm-400)" }}>Memuat sesi...</p>
      </main>
    );
  }

  return (
    <main style={pageStyle}>
      <div style={cardStyle}>
        <Stepper current={step} />
        <h1 style={titleStyle}>{STEP_LABELS[step - 1]}</h1>
        <p style={subtitleStyle}>
          {step === 1 && "Data dasar travel Anda untuk membuat ruang kerja."}
          {step === 2 && "Informasi ini tampil di halaman profil publik Anda untuk calon jamaah."}
          {step === 3 && "Buat musim pertama Anda — bisa diubah kapan saja nanti."}
        </p>

        <form onSubmit={submit} style={{ display: "grid", gap: 16 }}>
          {step === 1 && (
            <>
              <Field label="Nama perusahaan" value={values.name} onChange={(v) => update("name", v)} placeholder="PT. Barokah Tour & Travel" />
              <Field label="Nomor izin PPIU/PIHK (opsional)" value={values.licenseNumber} onChange={(v) => update("licenseNumber", v)} required={false} placeholder="PPIU-1234" />
              <Field label="Negara (ISO-2)" value={values.country} onChange={(v) => update("country", v.toUpperCase())} maxLength={2} placeholder="ID" />
            </>
          )}

          {step === 2 && (
            <>
              <label style={labelStyle}>
                Deskripsi singkat
                <textarea
                  value={values.description}
                  onChange={(e) => update("description", e.target.value)}
                  maxLength={300}
                  rows={3}
                  placeholder="Ceritakan sedikit tentang travel Anda..."
                  style={{ ...inputStyle, resize: "vertical" }}
                />
                <span style={hintStyle}>{values.description.length}/300 karakter</span>
              </label>
              <Field label="Nomor WhatsApp CS" value={values.whatsappNumber} onChange={(v) => update("whatsappNumber", v)} required={false} placeholder="+62 812-xxxx-xxxx" />
              <Field label="Kota kantor" value={values.city} onChange={(v) => update("city", v)} required={false} placeholder="Jakarta" />
              <Field label="Website (opsional)" value={values.website} onChange={(v) => update("website", v)} required={false} placeholder="https://..." />
            </>
          )}

          {step === 3 && (
            <>
              <Field label="Nama musim" value={values.seasonName} onChange={(v) => update("seasonName", v)} placeholder="Umrah Ramadhan 2027" />
              <label style={labelStyle}>
                Jenis musim
                <select value={values.seasonType} onChange={(e) => update("seasonType", Number(e.target.value) as SeasonType)} style={inputStyle}>
                  {SEASON_TYPE_OPTIONS.map((o) => (
                    <option key={o.value} value={o.value}>{o.label}</option>
                  ))}
                </select>
              </label>
              <Field label="Tanggal mulai" value={values.startDate} onChange={(v) => update("startDate", v)} type="date" />
              <Field label="Tanggal selesai" value={values.endDate} onChange={(v) => update("endDate", v)} type="date" />
            </>
          )}

          {error && <p role="alert" style={{ color: "var(--color-danger-600)", fontSize: 14, margin: 0 }}>{error}</p>}

          <div style={{ display: "flex", gap: 12, marginTop: 4 }}>
            {step > 1 && (
              <button type="button" onClick={() => setStep((s) => s - 1)} disabled={busy} style={secondaryBtnStyle}>
                Kembali
              </button>
            )}
            <button type="submit" disabled={busy || isPending} style={primaryBtnStyle}>
              {busy ? "Menyimpan..." : step === 3 ? "Selesai & Buka Dashboard" : "Lanjutkan"}
            </button>
          </div>
        </form>
      </div>
    </main>
  );
}

function Stepper({ current }: { current: number }) {
  return (
    <div style={{ display: "flex", alignItems: "center", justifyContent: "center", gap: 0, marginBottom: 28 }}>
      {[1, 2, 3].map((n, index) => {
        const done = n < current;
        const active = n === current;
        return (
          <div key={n} style={{ display: "flex", alignItems: "center" }}>
            <span
              style={{
                width: 34,
                height: 34,
                borderRadius: "50%",
                display: "grid",
                placeItems: "center",
                fontWeight: 700,
                fontSize: 14,
                background: active || done ? "var(--color-emerald-800)" : "var(--color-cream-200)",
                color: active || done ? "white" : "var(--color-warm-400)",
                border: active ? "3px solid var(--color-emerald-200)" : "none",
                transition: "all .2s",
              }}
            >
              {done ? "✓" : n}
            </span>
            {index < 2 && (
              <span style={{ width: 48, height: 2, background: n < current ? "var(--color-emerald-800)" : "var(--color-cream-300)" }} />
            )}
          </div>
        );
      })}
    </div>
  );
}

function Field({
  label,
  value,
  onChange,
  type = "text",
  placeholder,
  maxLength,
  required = true,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  type?: string;
  placeholder?: string;
  maxLength?: number;
  required?: boolean;
}) {
  return (
    <label style={labelStyle}>
      {label}
      <input
        required={required}
        value={value}
        onChange={(event) => onChange(event.target.value)}
        type={type}
        placeholder={placeholder}
        maxLength={maxLength}
        style={inputStyle}
      />
    </label>
  );
}

function organizationSlug(name: string) {
  const base = name.toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/(^-|-$)/g, "") || "operator";
  return `${base}-${crypto.randomUUID().slice(0, 8)}`;
}

const pageStyle: React.CSSProperties = { minHeight: "100vh", display: "grid", placeItems: "center", padding: "6vh 20px", background: "var(--color-cream-100)" };
const cardStyle: React.CSSProperties = { width: "100%", maxWidth: 560, background: "white", borderRadius: 20, border: "1px solid var(--color-cream-300)", boxShadow: "0 10px 40px rgba(15,23,42,.06)", padding: "36px 32px" };
const titleStyle: React.CSSProperties = { fontSize: "1.6rem", fontWeight: 700, margin: "0 0 6px", textAlign: "center", color: "var(--color-warm-900)" };
const subtitleStyle: React.CSSProperties = { fontSize: 14, color: "var(--color-warm-400)", textAlign: "center", margin: "0 0 28px" };
const labelStyle: React.CSSProperties = { display: "grid", gap: 8, fontSize: 14, fontWeight: 600, color: "var(--color-warm-700)" };
const hintStyle: React.CSSProperties = { fontSize: 12, fontWeight: 400, color: "var(--color-warm-400)" };
const inputStyle: React.CSSProperties = { display: "block", width: "100%", padding: "11px 13px", font: "inherit", fontWeight: 400, border: "1.5px solid var(--color-cream-400)", borderRadius: 10, background: "white", color: "var(--color-warm-900)" };
const primaryBtnStyle: React.CSSProperties = { flex: 1, padding: "13px 20px", border: 0, borderRadius: 10, background: "var(--color-emerald-800)", color: "white", fontWeight: 700, fontSize: 14, cursor: "pointer" };
const secondaryBtnStyle: React.CSSProperties = { padding: "13px 20px", borderRadius: 10, border: "1.5px solid var(--color-cream-400)", background: "white", color: "var(--color-warm-700)", fontWeight: 600, fontSize: 14, cursor: "pointer" };
