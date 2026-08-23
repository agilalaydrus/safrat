import { Check, FileSpreadsheet, Sparkles, X } from "lucide-react";
import { PROBLEM_ITEMS, SOLUTION_ITEMS } from "./content";

export default function ProblemSolution() {
  return (
    <section id="solusi" className="border-y border-slate-200 bg-white py-20 dark:border-slate-800 dark:bg-slate-950">
      <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
        <div className="mx-auto mb-16 max-w-3xl text-center">
          <span className="mb-3 inline-block rounded-full bg-emerald-50 px-3 py-1 text-xs font-bold uppercase tracking-wider text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300">
            Mengapa Travel Perlu Beralih?
          </span>
          <h3 className="text-3xl font-extrabold tracking-tight text-slate-900 sm:text-4xl dark:text-white">
            Bandingkan Cara Lama vs <span className="text-emerald-600 dark:text-emerald-400">Tawafiq Hub</span>
          </h3>
          <p className="mt-3 text-sm text-slate-600 sm:text-base dark:text-slate-300">
            Kekacauan manifest sering kali baru meledak saat jamaah sudah tiba di bandara Saudi.
          </p>
        </div>

        <div className="mx-auto grid max-w-5xl grid-cols-1 gap-8 lg:grid-cols-2">
          {/* Cara Lama */}
          <div className="rounded-3xl border border-rose-200 bg-rose-50/50 p-8 dark:border-rose-900/60 dark:bg-rose-950/20">
            <div className="mb-6 flex items-center gap-3">
              <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-rose-100 text-rose-700 dark:bg-rose-950 dark:text-rose-300">
                <FileSpreadsheet className="h-5 w-5" />
              </div>
              <div>
                <h4 className="text-lg font-bold text-slate-900 dark:text-white">Cara Lama (Spreadsheet &amp; WA)</h4>
                <p className="text-xs text-slate-500 dark:text-slate-400">Rawan human error &amp; menguras tenaga staf</p>
              </div>
            </div>
            <ul className="space-y-4 text-sm text-slate-700 dark:text-slate-300">
              {PROBLEM_ITEMS.map((item) => (
                <li key={item.bold} className="flex items-start gap-3">
                  <X className="mt-0.5 h-5 w-5 flex-shrink-0 text-rose-600" />
                  <span>
                    <strong>{item.bold}</strong> {item.rest}
                  </span>
                </li>
              ))}
            </ul>
          </div>

          {/* Dengan Tawafiq Hub */}
          <div className="rounded-3xl border border-emerald-200 bg-emerald-50/50 p-8 shadow-lg shadow-emerald-600/5 dark:border-emerald-900/60 dark:bg-emerald-950/20">
            <div className="mb-6 flex items-center gap-3">
              <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-emerald-600 text-white">
                <Sparkles className="h-5 w-5" />
              </div>
              <div>
                <h4 className="text-lg font-bold text-slate-900 dark:text-white">Dengan Tawafiq Hub</h4>
                <p className="text-xs font-semibold text-emerald-800 dark:text-emerald-300">
                  Otomatis, akurat &amp; tersinkronisasi 24/7
                </p>
              </div>
            </div>
            <ul className="space-y-4 text-sm text-slate-700 dark:text-slate-300">
              {SOLUTION_ITEMS.map((item) => (
                <li key={item.bold} className="flex items-start gap-3">
                  <Check className="mt-0.5 h-5 w-5 flex-shrink-0 text-emerald-600" />
                  <span>
                    <strong>{item.bold}</strong> {item.rest}
                  </span>
                </li>
              ))}
            </ul>
          </div>
        </div>
      </div>
    </section>
  );
}
