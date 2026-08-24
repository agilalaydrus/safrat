"use client";

import { useState } from "react";
import Link from "next/link";
import { CheckCircle2 } from "lucide-react";
import { PRICING_TIERS, WA_SALES_LINK } from "./content";

export default function Pricing() {
  const [billingCycle, setBillingCycle] = useState<"bulanan" | "tahunan">("tahunan");

  return (
    <section id="harga" className="border-t border-slate-200 bg-slate-50 py-20 dark:border-slate-800 dark:bg-slate-900">
      <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
        <div className="mx-auto mb-12 max-w-3xl text-center">
          <span className="mb-3 inline-block rounded-full bg-emerald-100 px-3 py-1 text-xs font-bold uppercase tracking-wider text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300">
            Investasi &amp; Paket Layanan
          </span>
          <h3 className="text-3xl font-extrabold tracking-tight text-slate-900 sm:text-4xl dark:text-slate-100">
            Pilihan Paket Sesuai Skala Travel Anda
          </h3>
          <p className="mt-3 text-sm text-slate-600 sm:text-base dark:text-slate-300">
            Tanpa biaya tersembunyi. Dapatkan pendampingan migrasi data Excel jamaah secara cuma-cuma.
          </p>

          <div className="mt-6 inline-flex items-center rounded-xl border border-slate-200 bg-white p-1 shadow-sm dark:border-slate-700 dark:bg-slate-950">
            <button
              type="button"
              onClick={() => setBillingCycle("bulanan")}
              className={`rounded-lg px-4 py-2 text-xs font-bold transition-all ${
                billingCycle === "bulanan"
                  ? "bg-slate-900 text-white shadow-sm dark:bg-slate-700"
                  : "text-slate-500 hover:text-slate-900 dark:text-slate-400 dark:hover:text-white"
              }`}
            >
              Bulanan
            </button>
            <button
              type="button"
              onClick={() => setBillingCycle("tahunan")}
              className={`flex items-center gap-1.5 rounded-lg px-4 py-2 text-xs font-bold transition-all ${
                billingCycle === "tahunan"
                  ? "bg-emerald-600 text-white shadow-sm"
                  : "text-slate-500 hover:text-slate-900 dark:text-slate-400 dark:hover:text-white"
              }`}
            >
              <span>Tahunan (Musim)</span>
              <span className="rounded bg-emerald-100 px-1.5 py-0.5 text-[10px] font-bold text-emerald-800 dark:bg-emerald-950 dark:text-emerald-300">
                Hemat 20%
              </span>
            </button>
          </div>
        </div>

        <div className="mx-auto grid max-w-6xl grid-cols-1 gap-8 md:grid-cols-3">
          {PRICING_TIERS.map((tier) => (
            <div
              key={tier.name}
              className={`relative flex flex-col justify-between rounded-3xl p-8 ${
                tier.highlighted
                  ? "border-2 border-emerald-500 bg-white shadow-xl dark:bg-slate-950"
                  : "border border-slate-200 bg-white shadow-sm transition-shadow hover:shadow-md dark:border-slate-800 dark:bg-slate-950"
              }`}
            >
              {tier.highlighted && (
                <div className="absolute -top-3.5 left-1/2 -translate-x-1/2 rounded-full bg-emerald-600 px-4 py-1 text-[11px] font-bold uppercase tracking-wider text-white shadow-md">
                  Paling Banyak Dipilih
                </div>
              )}
              <div>
                <h4 className="text-lg font-bold text-slate-900 dark:text-white">{tier.name}</h4>
                <p className="mt-1 text-xs text-slate-500 dark:text-slate-400">{tier.blurb}</p>
                <div className="mt-6 mb-6">
                  <span className="text-3xl font-black text-slate-900 dark:text-white">{tier.price}</span>
                  <span className="text-xs text-slate-500 dark:text-slate-400"> / {tier.unit}</span>
                </div>
                <ul className="space-y-3 text-xs text-slate-700 dark:text-slate-300">
                  {tier.features.map((feature) => (
                    <li key={feature} className="flex items-center gap-2">
                      <CheckCircle2 className="h-4 w-4 flex-shrink-0 text-emerald-600 dark:text-emerald-400" />
                      <span>{feature}</span>
                    </li>
                  ))}
                </ul>
              </div>
              {tier.price === "Custom" ? (
                <a
                  href={WA_SALES_LINK}
                  target="_blank"
                  rel="noreferrer"
                  className="mt-8 block w-full rounded-xl border border-slate-300 bg-slate-50 py-3 text-center text-xs font-bold text-slate-900 shadow-sm transition-all hover:bg-slate-100 dark:border-slate-700 dark:bg-slate-900 dark:text-white dark:hover:bg-slate-800"
                >
                  {tier.cta}
                </a>
              ) : (
                <Link
                  href="/sign-up"
                  className={`mt-8 block w-full rounded-xl py-3 text-center text-xs font-bold transition-all ${
                    tier.highlighted
                      ? "bg-emerald-600 text-white shadow-md shadow-emerald-600/30 hover:bg-emerald-700"
                      : "border border-slate-300 bg-slate-50 text-slate-900 shadow-sm hover:bg-slate-100 dark:border-slate-700 dark:bg-slate-900 dark:text-white dark:hover:bg-slate-800"
                  }`}
                >
                  {tier.cta}
                </Link>
              )}
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}
