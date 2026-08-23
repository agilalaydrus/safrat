"use client";

import { useState } from "react";
import { Clock, ShieldCheck } from "lucide-react";

export default function RoiCalculator({ onOpenDemo }: { onOpenDemo: () => void }) {
  const [jamaahCount, setJamaahCount] = useState(350);
  const [cloterCount, setCloterCount] = useState(6);

  const hoursSaved = Math.round(jamaahCount * 0.9 + cloterCount * 12);
  const estimatedSavings = Math.round((hoursSaved * 75000) / 1000000);

  return (
    <section id="kalkulator" className="border-t border-slate-200 bg-white py-20 dark:border-slate-800 dark:bg-slate-950">
      <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
        <div className="mx-auto mb-12 max-w-3xl text-center">
          <span className="mb-3 inline-block rounded-full bg-emerald-50 px-3 py-1 text-xs font-bold uppercase tracking-wider text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300">
            Kalkulator Efisiensi PPIU
          </span>
          <h3 className="text-3xl font-extrabold tracking-tight text-slate-900 sm:text-4xl dark:text-white">
            Hitung Waktu &amp; Biaya yang <span className="text-emerald-600 dark:text-emerald-400">Dihemat Travel Anda</span>
          </h3>
          <p className="mt-3 text-sm text-slate-600 sm:text-base dark:text-slate-300">
            Berapa banyak jam lembur staf admin dan risiko salah kamar yang bisa Anda hilangkan di musim depan?
          </p>
        </div>

        <div className="mx-auto grid max-w-5xl grid-cols-1 items-center gap-8 rounded-3xl border border-slate-200 bg-slate-50 p-6 shadow-sm sm:p-10 lg:grid-cols-12 dark:border-slate-800 dark:bg-slate-900">
          {/* Sliders */}
          <div className="space-y-6 lg:col-span-6">
            <div className="space-y-3">
              <div className="flex items-center justify-between">
                <label className="text-sm font-semibold text-slate-800 dark:text-slate-200">Target Jumlah Jamaah per Musim:</label>
                <span className="rounded-lg border border-emerald-300 bg-emerald-100 px-3 py-1 font-mono text-base font-bold text-emerald-800 dark:border-emerald-800 dark:bg-emerald-950 dark:text-emerald-300">
                  {jamaahCount.toLocaleString("id-ID")} Jamaah
                </span>
              </div>
              <input
                type="range"
                min={50}
                max={3000}
                step={50}
                value={jamaahCount}
                onChange={(e) => setJamaahCount(Number(e.target.value))}
                className="h-2 w-full cursor-pointer appearance-none rounded-lg bg-slate-200 accent-emerald-600 dark:bg-slate-700"
              />
              <div className="flex justify-between font-mono text-[10px] text-slate-500 dark:text-slate-400">
                <span>50 Jamaah (Mini)</span>
                <span>3.000 Jamaah (Enterprise)</span>
              </div>
            </div>

            <div className="space-y-3 border-t border-slate-200 pt-4 dark:border-slate-700">
              <div className="flex items-center justify-between">
                <label className="text-sm font-semibold text-slate-800 dark:text-slate-200">Jumlah Keberangkatan / Kloter:</label>
                <span className="rounded-lg border border-teal-300 bg-teal-100 px-3 py-1 font-mono text-base font-bold text-teal-800 dark:border-teal-800 dark:bg-teal-950 dark:text-teal-300">
                  {cloterCount} Rombongan Kloter
                </span>
              </div>
              <input
                type="range"
                min={1}
                max={40}
                step={1}
                value={cloterCount}
                onChange={(e) => setCloterCount(Number(e.target.value))}
                className="h-2 w-full cursor-pointer appearance-none rounded-lg bg-slate-200 accent-teal-600 dark:bg-slate-700"
              />
              <div className="flex justify-between font-mono text-[10px] text-slate-500 dark:text-slate-400">
                <span>1 Kloter</span>
                <span>40 Kloter</span>
              </div>
            </div>
          </div>

          {/* Results */}
          <div className="space-y-4 rounded-2xl border border-emerald-300 bg-white p-6 shadow-md sm:p-8 lg:col-span-6 dark:border-emerald-900/60 dark:bg-slate-950">
            <span className="font-mono text-xs font-bold uppercase tracking-wider text-emerald-700 dark:text-emerald-400">
              Estimasi Penghematan Musim Depan
            </span>
            <div className="grid grid-cols-2 gap-3">
              <div className="rounded-xl border border-slate-200 bg-slate-50 p-4 dark:border-slate-800 dark:bg-slate-900">
                <div className="mb-1 flex items-center gap-2 text-xs text-slate-500 dark:text-slate-400">
                  <Clock className="h-4 w-4 text-emerald-600 dark:text-emerald-400" />
                  <span>Waktu Admin Dihemat</span>
                </div>
                <div className="text-2xl font-extrabold text-slate-900 dark:text-white">~{hoursSaved} Jam</div>
                <span className="mt-1 block text-[11px] text-emerald-700 dark:text-emerald-400">Bebas lembur rekap Excel H-1</span>
              </div>
              <div className="rounded-xl border border-slate-200 bg-slate-50 p-4 dark:border-slate-800 dark:bg-slate-900">
                <div className="mb-1 flex items-center gap-2 text-xs text-slate-500 dark:text-slate-400">
                  <ShieldCheck className="h-4 w-4 text-teal-600 dark:text-teal-400" />
                  <span>Sengketa Kamar</span>
                </div>
                <div className="text-2xl font-extrabold text-teal-700 dark:text-teal-400">0 Kasus</div>
                <span className="mt-1 block text-[11px] text-teal-700 dark:text-teal-400">100% Terverifikasi Syariah</span>
              </div>
            </div>
            <div className="flex items-center justify-between rounded-xl border border-emerald-200 bg-emerald-50 p-4 text-xs text-slate-800 dark:border-emerald-900/60 dark:bg-emerald-950/30 dark:text-slate-200">
              <div>
                <span className="block text-[11px] text-slate-600 dark:text-slate-400">Efisiensi Biaya Operasional &amp; SDM:</span>
                <span className="text-lg font-bold text-emerald-800 dark:text-emerald-300">
                  Hemat hingga ~Rp {estimatedSavings} Juta / Musim
                </span>
              </div>
              <button
                type="button"
                onClick={onOpenDemo}
                className="rounded-lg bg-emerald-600 px-3.5 py-2 text-xs font-bold text-white shadow-sm transition-colors hover:bg-emerald-700"
              >
                Klaim Penghematan
              </button>
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}
