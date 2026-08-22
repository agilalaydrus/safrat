"use client";

import { useMemo, useState } from "react";
import { Calculator } from "lucide-react";

const ADMIN_MINUTES_PER_PILGRIM = 9;
const ADMIN_HOURS_PER_KLOTER = 2;
const HOURLY_ADMIN_COST = 50000;

export default function RoiCalculator() {
  const [pilgrims, setPilgrims] = useState(500);
  const [kloters, setKloters] = useState(6);

  const { hoursSaved, costSaved } = useMemo(() => {
    const hours = (pilgrims * ADMIN_MINUTES_PER_PILGRIM) / 60 + kloters * ADMIN_HOURS_PER_KLOTER;
    return { hoursSaved: Math.round(hours), costSaved: Math.round(hours * HOURLY_ADMIN_COST) };
  }, [pilgrims, kloters]);

  return (
    <section id="roi" className="bg-white px-5 py-20 dark:bg-slate-900">
      <div className="mx-auto max-w-3xl">
        <p className="mb-2 text-center text-xs font-bold uppercase tracking-widest text-emerald-700 dark:text-emerald-400">
          Kalkulator ROI
        </p>
        <h2 className="mb-3 text-center text-3xl font-extrabold text-slate-900 dark:text-white sm:text-4xl">
          Estimasi Penghematan Musim Anda
        </h2>
        <p className="mx-auto mb-10 max-w-lg text-center text-sm text-slate-500 dark:text-slate-400">
          Angka di bawah ini estimasi berdasarkan asumsi yang bisa Anda lihat sendiri, bukan janji pasti.
        </p>

        <div className="rounded-2xl border border-slate-200 bg-slate-50 p-7 dark:border-slate-700 dark:bg-slate-950">
          <div className="mb-8 grid grid-cols-1 gap-7 sm:grid-cols-2">
            <label className="block">
              <span className="mb-2 flex items-center justify-between text-sm font-bold text-slate-700 dark:text-slate-200">
                Target Jamaah per Musim
                <span className="font-mono text-emerald-700 dark:text-emerald-400">{pilgrims.toLocaleString("id-ID")}</span>
              </span>
              <input
                type="range"
                min={100}
                max={5000}
                step={50}
                value={pilgrims}
                onChange={(e) => setPilgrims(Number(e.target.value))}
                className="w-full accent-emerald-600"
              />
            </label>
            <label className="block">
              <span className="mb-2 flex items-center justify-between text-sm font-bold text-slate-700 dark:text-slate-200">
                Jumlah Keberangkatan / Kloter
                <span className="font-mono text-emerald-700 dark:text-emerald-400">{kloters}</span>
              </span>
              <input
                type="range"
                min={2}
                max={50}
                step={1}
                value={kloters}
                onChange={(e) => setKloters(Number(e.target.value))}
                className="w-full accent-emerald-600"
              />
            </label>
          </div>

          <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
            <div className="rounded-xl border border-emerald-300 bg-emerald-100 p-5 text-center dark:border-emerald-800 dark:bg-emerald-950/60">
              <p className="text-2xl font-extrabold text-slate-900 dark:text-white">~{hoursSaved.toLocaleString("id-ID")} Jam</p>
              <p className="mt-1 text-xs text-slate-600 dark:text-slate-300">Waktu admin dihemat per musim</p>
            </div>
            <div className="rounded-xl border border-emerald-300 bg-emerald-100 p-5 text-center dark:border-emerald-800 dark:bg-emerald-950/60">
              <p className="text-2xl font-extrabold text-emerald-700 dark:text-emerald-300">100% 0 Kasus</p>
              <p className="mt-1 text-xs text-slate-600 dark:text-slate-300">Sengketa kamar dicegah, ini jaminan sistem, bukan estimasi</p>
            </div>
            <div className="rounded-xl border border-emerald-300 bg-emerald-100 p-5 text-center dark:border-emerald-800 dark:bg-emerald-950/60">
              <p className="text-2xl font-extrabold text-slate-900 dark:text-white">~Rp {costSaved.toLocaleString("id-ID")}</p>
              <p className="mt-1 text-xs text-slate-600 dark:text-slate-300">Efisiensi biaya operasional per musim</p>
            </div>
          </div>

          <p className="mt-6 flex items-start gap-2 text-xs leading-relaxed text-slate-400">
            <Calculator size={14} className="mt-0.5 shrink-0" />
            Asumsi, {ADMIN_MINUTES_PER_PILGRIM} menit rekap manual per jamaah, {ADMIN_HOURS_PER_KLOTER} jam koordinasi per kloter, biaya
            waktu admin Rp{HOURLY_ADMIN_COST.toLocaleString("id-ID")} per jam. Sesuaikan sendiri dengan kondisi operasional Anda.
          </p>
        </div>
      </div>
    </section>
  );
}
