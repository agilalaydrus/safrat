"use client";

import { useState } from "react";
import { CheckCircle2 } from "lucide-react";
import { FEATURE_MODULES } from "./content";

export default function FeatureMatrix() {
  const [active, setActive] = useState(0);
  const mod = FEATURE_MODULES[active] ?? FEATURE_MODULES[0]!;
  const Icon = mod.icon;

  return (
    <section id="fitur" className="bg-white px-5 py-20 dark:bg-slate-900">
      <div className="mx-auto max-w-6xl">
        <p className="mb-2 text-center text-xs font-bold uppercase tracking-widest text-emerald-700 dark:text-emerald-400">
          Fitur Operasional
        </p>
        <h2 className="mb-3 text-center text-3xl font-extrabold text-slate-900 dark:text-white sm:text-4xl">
          Semua Modul yang Operator Beneran Pakai
        </h2>
        <p className="mx-auto mb-12 max-w-xl text-center text-sm text-slate-500 dark:text-slate-400">
          Bukan daftar fitur generik. Pilih modulnya, lihat sendiri apa yang ada di dalamnya.
        </p>

        <div className="grid grid-cols-1 gap-6 lg:grid-cols-[0.85fr_1.15fr]">
          <div className="grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-1">
            {FEATURE_MODULES.map((m, i) => {
              const ItemIcon = m.icon;
              return (
                <button
                  key={m.title}
                  type="button"
                  onClick={() => setActive(i)}
                  className={
                    active === i
                      ? "flex items-center gap-3 rounded-xl border border-emerald-300 bg-emerald-50 px-4 py-3.5 text-left dark:border-emerald-800 dark:bg-emerald-950/40"
                      : "flex items-center gap-3 rounded-xl border border-slate-200 bg-white px-4 py-3.5 text-left hover:border-slate-300 dark:border-slate-700 dark:bg-slate-900 dark:hover:border-slate-600"
                  }
                >
                  <span
                    className={
                      active === i
                        ? "grid h-9 w-9 shrink-0 place-items-center rounded-lg bg-emerald-600 text-white"
                        : "grid h-9 w-9 shrink-0 place-items-center rounded-lg bg-slate-100 text-slate-500 dark:bg-slate-800 dark:text-slate-400"
                    }
                  >
                    <ItemIcon size={16} />
                  </span>
                  <span className={active === i ? "text-sm font-bold text-emerald-800 dark:text-emerald-300" : "text-sm font-semibold text-slate-700 dark:text-slate-200"}>
                    {m.title}
                  </span>
                </button>
              );
            })}
          </div>

          <div className="rounded-2xl border border-slate-200 bg-slate-50 p-7 dark:border-slate-700 dark:bg-slate-950">
            <div className="mb-4 grid h-12 w-12 place-items-center rounded-xl bg-gradient-to-br from-emerald-500 to-teal-600 text-white">
              <Icon size={22} />
            </div>
            <h3 className="mb-1.5 text-xl font-extrabold text-slate-900 dark:text-white">{mod.title}</h3>
            <p className="mb-5 text-sm text-slate-500 dark:text-slate-400">{mod.tagline}</p>
            <ul className="space-y-3">
              {mod.points.map((point) => (
                <li key={point} className="flex items-start gap-2.5 text-sm text-slate-700 dark:text-slate-200">
                  <CheckCircle2 size={16} className="mt-0.5 shrink-0 text-emerald-600 dark:text-emerald-400" />
                  {point}
                </li>
              ))}
            </ul>
          </div>
        </div>
      </div>
    </section>
  );
}
