"use client";

import Link from "next/link";
import { ArrowRight, ShieldCheck, Sparkles } from "lucide-react";
import { authClient } from "@/lib/auth-client";
import { TRUST_POINTS } from "./content";

export default function Hero() {
  const { data: session, isPending } = authClient.useSession();
  const isAuthenticated = Boolean(session?.user);

  return (
    <section className="relative overflow-hidden bg-slate-50 px-5 py-20 dark:bg-slate-950 sm:py-28">
      <div className="mx-auto max-w-4xl text-center">
        <span className="mx-auto mb-6 inline-flex items-center gap-2 rounded-full border border-emerald-300 bg-emerald-100 px-4 py-1.5 text-xs font-bold tracking-wide text-emerald-800 dark:border-emerald-800 dark:bg-emerald-950 dark:text-emerald-300">
          <Sparkles size={13} />
          SISTEM OPERASI PPIU & HAJI KHUSUS #1 DI INDONESIA
        </span>

        <h1 className="text-4xl font-extrabold leading-[1.15] tracking-tight text-slate-900 sm:text-5xl md:text-6xl dark:text-white">
          Tinggalkan Kerepotan Spreadsheet.{" "}
          <span className="bg-gradient-to-r from-emerald-600 to-teal-600 bg-clip-text text-transparent">
            Kelola Operasional Umrah & Haji dengan Tenang.
          </span>
        </h1>

        <p className="mx-auto mt-6 max-w-2xl text-base leading-relaxed text-slate-600 sm:text-lg dark:text-slate-300">
          Dari pendaftaran jamaah, manifes kamar hotel yang bebas sengketa mahram, armada bus di Saudi, sampai gateway
          SOS darurat 10 menit. Semuanya jadi satu sistem yang benar benar dipakai tim Anda tiap hari, bukan cuma
          waktu demo saja.
        </p>

        <div className="mt-9 flex flex-wrap items-center justify-center gap-3">
          {isAuthenticated ? (
            <Link
              href="/dashboard"
              className="inline-flex items-center gap-2 rounded-xl bg-gradient-to-r from-emerald-600 to-emerald-700 px-7 py-3.5 text-sm font-bold text-white shadow-lg shadow-emerald-600/25 hover:from-emerald-500 hover:to-emerald-600"
            >
              Buka Dashboard <ArrowRight size={17} />
            </Link>
          ) : (
            !isPending && (
              <>
                <a
                  href="#preview"
                  className="inline-flex items-center gap-2 rounded-xl bg-gradient-to-r from-emerald-600 to-emerald-700 px-7 py-3.5 text-sm font-bold text-white shadow-lg shadow-emerald-600/25 hover:from-emerald-500 hover:to-emerald-600"
                >
                  Coba Demo Dashboard Interaktif <ArrowRight size={17} />
                </a>
                <a
                  href="#roi"
                  className="inline-flex items-center gap-2 rounded-xl border border-slate-300 bg-white px-7 py-3.5 text-sm font-bold text-slate-800 hover:bg-slate-100 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-100 dark:hover:bg-slate-800"
                >
                  Hitung Efisiensi Biaya
                </a>
              </>
            )
          )}
        </div>

        <div className="mx-auto mt-16 grid max-w-5xl grid-cols-1 gap-4 text-left sm:grid-cols-2 lg:grid-cols-4">
          {TRUST_POINTS.map((point) => (
            <div
              key={point.title}
              className="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-slate-900"
            >
              <ShieldCheck size={20} className="mb-3 text-emerald-600 dark:text-emerald-400" />
              <p className="mb-1.5 text-sm font-bold text-slate-900 dark:text-white">{point.title}</p>
              <p className="text-xs leading-relaxed text-slate-500 dark:text-slate-400">{point.desc}</p>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}
