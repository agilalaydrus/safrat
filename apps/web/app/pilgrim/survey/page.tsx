"use client";

import { useEffect, useState } from "react";
import { IconCheck, IconStar, IconStarFilled } from "@tabler/icons-react";
import { usePilgrimCode } from "@/lib/pilgrim-context";

type SurveyAnswers = { overall: number; accommodation: number; transport: number; guidance: number; comments: string; submittedAt: string };

const CATEGORIES: { key: keyof Pick<SurveyAnswers, "overall" | "accommodation" | "transport" | "guidance">; label: string }[] = [
  { key: "overall", label: "Kepuasan Keseluruhan" },
  { key: "accommodation", label: "Akomodasi & Hotel" },
  { key: "transport", label: "Transportasi" },
  { key: "guidance", label: "Bimbingan Petugas" },
];

function storageKey(code: string): string {
  return `pilgrim-survey:${code}`;
}

function StarRating({ value, onChange }: { value: number; onChange: (value: number) => void }) {
  return (
    <div style={{ display: "flex", gap: 4 }}>
      {[1, 2, 3, 4, 5].map((star) => (
        <button key={star} type="button" onClick={() => onChange(star)} aria-label={`${star} bintang`} style={starButton}>
          {star <= value ? <IconStarFilled size={28} color="var(--color-gold-500)" /> : <IconStar size={28} color="var(--color-cream-500)" />}
        </button>
      ))}
    </div>
  );
}

export default function PilgrimSurveyPage() {
  const code = usePilgrimCode();
  const [answers, setAnswers] = useState<Omit<SurveyAnswers, "submittedAt">>({ overall: 0, accommodation: 0, transport: 0, guidance: 0, comments: "" });
  const [submitted, setSubmitted] = useState<SurveyAnswers | null>(null);

  useEffect(() => {
    if (!code) return;
    const raw = window.localStorage.getItem(storageKey(code));
    if (raw) {
      try { setSubmitted(JSON.parse(raw) as SurveyAnswers); } catch { /* ignore corrupt data */ }
    }
  }, [code]);

  function submit() {
    if (!code || !answers.overall) return;
    const record: SurveyAnswers = { ...answers, submittedAt: new Date().toISOString() };
    window.localStorage.setItem(storageKey(code), JSON.stringify(record));
    setSubmitted(record);
  }

  function editAgain() {
    if (submitted) setAnswers({ overall: submitted.overall, accommodation: submitted.accommodation, transport: submitted.transport, guidance: submitted.guidance, comments: submitted.comments });
    setSubmitted(null);
  }

  if (submitted) {
    return (
      <main style={page}>
        <div style={doneCard}>
          <IconCheck size={40} color="var(--color-emerald-900)" />
          <h1 style={title}>Terima kasih atas ulasan Anda</h1>
          <p style={{ color: "var(--color-warm-500)" }}>Dikirim pada {new Date(submitted.submittedAt).toLocaleString("id-ID")}</p>
          <button onClick={editAgain} style={secondaryButton}>Ubah Ulasan</button>
        </div>
      </main>
    );
  }

  return (
    <main style={page}>
      <p style={eyebrow}>ULASAN PERJALANAN</p>
      <h1 style={title}>Bagaimana Perjalanan Anda?</h1>
      <p style={{ color: "var(--color-warm-500)", margin: "0 0 20px" }}>Ulasan Anda membantu operator meningkatkan layanan di musim mendatang.</p>
      <div style={list}>
        {CATEGORIES.map(({ key, label }) => (
          <div key={key} style={card}>
            <p style={{ margin: "0 0 10px", fontWeight: 700 }}>{label}</p>
            <StarRating value={answers[key]} onChange={(value) => setAnswers((current) => ({ ...current, [key]: value }))} />
          </div>
        ))}
        <div style={card}>
          <p style={{ margin: "0 0 10px", fontWeight: 700 }}>Komentar (opsional)</p>
          <textarea value={answers.comments} onChange={(e) => setAnswers((current) => ({ ...current, comments: e.target.value }))} rows={4} placeholder="Ceritakan pengalaman Anda..." style={textarea} />
        </div>
      </div>
      <button disabled={!answers.overall} onClick={submit} style={primaryButton}>Kirim Ulasan</button>
    </main>
  );
}

const page: React.CSSProperties = { maxWidth: 480, margin: "0 auto", padding: "28px 20px" };
const eyebrow: React.CSSProperties = { color: "var(--color-gold-800)", fontSize: 11, fontWeight: 700, letterSpacing: ".08em", margin: "0 0 6px" };
const title: React.CSSProperties = { fontSize: 26, margin: "0 0 8px" };
const list: React.CSSProperties = { display: "grid", gap: 12, marginBottom: 20 };
const card: React.CSSProperties = { background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 16 };
const starButton: React.CSSProperties = { border: 0, background: "transparent", padding: 0, cursor: "pointer" };
const textarea: React.CSSProperties = { width: "100%", minHeight: 90, border: "1px solid var(--color-cream-500)", borderRadius: 8, padding: "10px 12px", background: "var(--color-cream-200)", color: "var(--color-warm-900)", font: "inherit", resize: "vertical" };
const primaryButton: React.CSSProperties = { width: "100%", minHeight: 48, border: 0, borderRadius: 10, background: "var(--color-emerald-900)", color: "white", fontWeight: 700 };
const secondaryButton: React.CSSProperties = { minHeight: 44, border: "1px solid var(--color-cream-500)", borderRadius: 10, background: "transparent", color: "var(--color-warm-700)", fontWeight: 600, padding: "0 18px", marginTop: 8 };
const doneCard: React.CSSProperties = { textAlign: "center", display: "grid", justifyItems: "center", gap: 8, background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 16, padding: 32, marginTop: 40 };
