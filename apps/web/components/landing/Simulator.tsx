"use client";

import { useEffect, useState } from "react";
import { AlertTriangle, CheckCircle2, RefreshCcw, Siren, XCircle } from "lucide-react";

const MAHRAM_SCENARIOS = [
  { id: "a", label: "Skenario A, Suami dan Istri Sah", genderA: "L", genderB: "P", blocked: false },
  { id: "b", label: "Skenario B, Pria dan Wanita Non Mahram", genderA: "L", genderB: "P", blocked: true },
  { id: "c", label: "Skenario C, Sesama Wanita", genderA: "P", genderB: "P", blocked: false },
] as const;

function MahramSimulator() {
  const [selected, setSelected] = useState<(typeof MAHRAM_SCENARIOS)[number] | null>(null);
  const [shake, setShake] = useState(false);

  function pick(s: (typeof MAHRAM_SCENARIOS)[number]) {
    setSelected(s);
    if (s.blocked) {
      setShake(false);
      window.setTimeout(() => setShake(true), 10);
    }
  }

  return (
    <div className="rounded-2xl border border-slate-200 bg-white p-6 dark:border-slate-700 dark:bg-slate-900">
      <h3 className="mb-1.5 text-lg font-extrabold text-slate-900 dark:text-white">Simulator Algoritma Mahram</h3>
      <p className="mb-5 text-sm text-slate-500 dark:text-slate-400">
        Pilih pasangan jamaahnya, sistem yang menilai boleh atau tidak ditempatkan satu kamar.
      </p>
      <div className="mb-5 flex flex-wrap gap-2">
        {MAHRAM_SCENARIOS.map((s) => (
          <button
            key={s.id}
            type="button"
            onClick={() => pick(s)}
            className={
              selected?.id === s.id
                ? "rounded-lg bg-emerald-600 px-3.5 py-2.5 text-xs font-bold text-white"
                : "rounded-lg border border-slate-300 bg-white px-3.5 py-2.5 text-xs font-bold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-200 dark:hover:bg-slate-800"
            }
          >
            {s.label}
          </button>
        ))}
      </div>
      {selected && (
        <div
          onAnimationEnd={() => setShake(false)}
          className={
            (selected.blocked
              ? "flex items-start gap-3 rounded-xl border border-red-300 bg-red-100 p-4 text-red-800 dark:border-red-800 dark:bg-red-950/60 dark:text-red-300"
              : "flex items-start gap-3 rounded-xl border border-emerald-300 bg-emerald-100 p-4 text-emerald-800 dark:border-emerald-800 dark:bg-emerald-950/60 dark:text-emerald-300") +
            (shake ? " animate-shake" : "")
          }
        >
          {selected.blocked ? <XCircle size={20} className="mt-0.5 shrink-0" /> : <CheckCircle2 size={20} className="mt-0.5 shrink-0" />}
          <div>
            <p className="text-sm font-extrabold">{selected.blocked ? "STATUS: BLOCKED" : "STATUS: VALID"}</p>
            <p className="mt-1 text-xs leading-relaxed">
              {selected.blocked
                ? "Pelanggaran, pria dan wanita bukan mahram tidak bisa dialokasikan dalam 1 kamar."
                : "Kombinasi gender dan hubungan mahramnya sudah sesuai aturan, boleh ditempatkan satu kamar."}
            </p>
          </div>
        </div>
      )}
    </div>
  );
}

const SOS_SECONDS = 10;

function SosSimulator() {
  const [phase, setPhase] = useState<"idle" | "counting" | "escalated" | "handled">("idle");
  const [secondsLeft, setSecondsLeft] = useState(SOS_SECONDS);

  useEffect(() => {
    if (phase !== "counting") return;
    const timer = window.setInterval(() => {
      setSecondsLeft((prev) => {
        if (prev <= 1) {
          window.clearInterval(timer);
          setPhase("escalated");
          return 0;
        }
        return prev - 1;
      });
    }, 1000);
    return () => window.clearInterval(timer);
  }, [phase]);

  function send() {
    setPhase("counting");
    setSecondsLeft(SOS_SECONDS);
  }

  function reset() {
    setPhase("idle");
    setSecondsLeft(SOS_SECONDS);
  }

  const pct = Math.round(((SOS_SECONDS - secondsLeft) / SOS_SECONDS) * 100);

  return (
    <div className="rounded-2xl border border-slate-200 bg-white p-6 dark:border-slate-700 dark:bg-slate-900">
      <h3 className="mb-1.5 text-lg font-extrabold text-slate-900 dark:text-white">Simulator Protokol SOS 10 Menit</h3>
      <p className="mb-5 text-sm text-slate-500 dark:text-slate-400">
        Simulasi dipercepat, {SOS_SECONDS} detik di sini mewakili 10 menit asli. Aturan eskalasinya sama persis dengan sistem.
      </p>

      {phase === "idle" && (
        <button
          type="button"
          onClick={send}
          className="inline-flex items-center gap-2 rounded-lg bg-red-600 px-4 py-3 text-sm font-bold text-white hover:bg-red-700"
        >
          <Siren size={16} />
          Kirim Sinyal SOS Jamaah
        </button>
      )}

      {phase === "counting" && (
        <div>
          <div className="mb-2 h-2 overflow-hidden rounded-full bg-amber-200 dark:bg-amber-900">
            <div className="h-full bg-amber-500 transition-all duration-1000" style={{ width: `${pct}%` }} />
          </div>
          <p className="mb-4 text-xs text-slate-500 dark:text-slate-400">
            Menunggu Muthowif Lapangan, {secondsLeft} detik lagi sebelum eskalasi otomatis
          </p>
          <button
            type="button"
            onClick={() => setPhase("handled")}
            className="rounded-lg bg-emerald-600 px-4 py-2.5 text-xs font-bold text-white hover:bg-emerald-700"
          >
            Simulasikan Muthowif Tangani
          </button>
        </div>
      )}

      {phase === "escalated" && (
        <div className="flex items-start gap-3 rounded-xl border border-red-300 bg-red-100 p-4 text-red-800 dark:border-red-800 dark:bg-red-950/60 dark:text-red-300">
          <AlertTriangle size={20} className="mt-0.5 shrink-0" />
          <div>
            <p className="text-sm font-extrabold">Dieskalasi ke Level 2, Direksi PPIU</p>
            <p className="mt-1 text-xs leading-relaxed">
              10 menit terlewati tanpa respons Muthowif, semua koordinator operator sekarang dapat notifikasi.
            </p>
            <button type="button" onClick={reset} className="mt-3 inline-flex items-center gap-1.5 text-xs font-bold underline">
              <RefreshCcw size={12} />
              Ulangi simulasi
            </button>
          </div>
        </div>
      )}

      {phase === "handled" && (
        <div className="flex items-start gap-3 rounded-xl border border-emerald-300 bg-emerald-100 p-4 text-emerald-800 dark:border-emerald-800 dark:bg-emerald-950/60 dark:text-emerald-300">
          <CheckCircle2 size={20} className="mt-0.5 shrink-0" />
          <div>
            <p className="text-sm font-extrabold">Ditangani Muthowif Lapangan</p>
            <p className="mt-1 text-xs leading-relaxed">Kasus selesai sebelum sempat dieskalasi ke Level 2.</p>
            <button type="button" onClick={reset} className="mt-3 inline-flex items-center gap-1.5 text-xs font-bold underline">
              <RefreshCcw size={12} />
              Ulangi simulasi
            </button>
          </div>
        </div>
      )}
    </div>
  );
}

const TABS = ["mahram", "sos"] as const;

export default function Simulator() {
  const [tab, setTab] = useState<(typeof TABS)[number]>("mahram");

  return (
    <section id="simulasi" className="bg-slate-100 px-5 py-20 dark:bg-slate-950">
      <div className="mx-auto max-w-3xl">
        <p className="mb-2 text-center text-xs font-bold uppercase tracking-widest text-emerald-700 dark:text-emerald-400">
          Simulasi Live
        </p>
        <h2 className="mb-3 text-center text-3xl font-extrabold text-slate-900 dark:text-white sm:text-4xl">
          Coba Logikanya Sendiri
        </h2>
        <p className="mx-auto mb-8 max-w-lg text-center text-sm text-slate-500 dark:text-slate-400">
          Dua aturan yang paling sering jadi sumber masalah operasional. Coba langsung di sini.
        </p>

        <div className="mb-6 flex justify-center gap-2">
          <button
            type="button"
            onClick={() => setTab("mahram")}
            className={
              tab === "mahram"
                ? "rounded-lg bg-emerald-600 px-4 py-2 text-xs font-bold text-white"
                : "rounded-lg border border-slate-300 bg-white px-4 py-2 text-xs font-bold text-slate-600 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-300"
            }
          >
            Simulator Mahram
          </button>
          <button
            type="button"
            onClick={() => setTab("sos")}
            className={
              tab === "sos"
                ? "rounded-lg bg-emerald-600 px-4 py-2 text-xs font-bold text-white"
                : "rounded-lg border border-slate-300 bg-white px-4 py-2 text-xs font-bold text-slate-600 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-300"
            }
          >
            Simulator SOS
          </button>
        </div>

        {tab === "mahram" ? <MahramSimulator /> : <SosSimulator />}
      </div>
    </section>
  );
}
