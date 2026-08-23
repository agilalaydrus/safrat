import Link from "next/link";
import {
  ArrowRight,
  Building2,
  Bus,
  CheckCircle2,
  Hotel,
  ShieldCheck,
  Sparkles,
  TrendingUp,
} from "lucide-react";
import { HERO_TRUST } from "./content";

export default function Hero() {
  return (
    <section className="relative overflow-hidden bg-gradient-to-b from-white via-slate-50 to-slate-100 pt-16 pb-20 md:pt-24 md:pb-28 dark:from-slate-950 dark:via-slate-950 dark:to-slate-900">
      {/* Soft grid pattern */}
      <div className="pointer-events-none absolute inset-0 bg-[linear-gradient(to_right,#e2e8f030_1px,transparent_1px),linear-gradient(to_bottom,#e2e8f030_1px,transparent_1px)] bg-[size:4rem_4rem] [mask-image:radial-gradient(ellipse_60%_50%_at_50%_0%,#000_70%,transparent_100%)]" />

      <div className="relative z-10 mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
        <div className="mx-auto max-w-3xl text-center">
          <div className="mb-6 inline-flex items-center gap-2 rounded-full border border-emerald-200 bg-emerald-50 px-3.5 py-1.5 text-xs font-bold uppercase tracking-wider text-emerald-800 shadow-sm dark:border-emerald-800 dark:bg-emerald-950 dark:text-emerald-300">
            <Sparkles className="h-3.5 w-3.5 text-emerald-600 dark:text-emerald-400" />
            <span>Platform Manajemen Umrah &amp; Haji Terpadu</span>
          </div>

          <h1 className="text-4xl font-black leading-[1.15] tracking-tight text-slate-900 sm:text-5xl lg:text-6xl dark:text-white">
            Tinggalkan Spreadsheet. Kelola Musim Umrah{" "}
            <span className="text-emerald-600 dark:text-emerald-400">Lebih Tenang &amp; Teratur.</span>
          </h1>

          <p className="mx-auto mt-6 max-w-2xl text-base leading-relaxed text-slate-600 sm:text-lg dark:text-slate-300">
            Sistem operasi all-in-one untuk PPIU &amp; PIHK di Indonesia: Validasi kamar mahram otomatis, manifest bus
            real-time Saudi, dan gateway darurat SOS terpusat.
          </p>

          <div className="mt-8 flex flex-col items-center justify-center gap-4 sm:flex-row">
            <Link
              href="/sign-up"
              className="flex w-full items-center justify-center gap-2 rounded-xl bg-emerald-600 px-8 py-4 text-sm font-bold text-white shadow-lg shadow-emerald-600/30 transition-all hover:scale-[1.02] hover:bg-emerald-700 sm:w-auto"
            >
              <span>Coba Gratis</span>
              <ArrowRight className="h-4 w-4" />
            </Link>
            <a
              href="#cara-kerja"
              className="flex w-full items-center justify-center gap-2 rounded-xl border border-slate-300 bg-white px-7 py-4 text-sm font-bold text-slate-700 shadow-sm transition-all hover:border-slate-400 hover:bg-slate-50 sm:w-auto dark:border-slate-700 dark:bg-slate-900 dark:text-slate-200 dark:hover:bg-slate-800"
            >
              <span>Lihat Cara Kerja</span>
            </a>
          </div>

          <div className="mt-8 flex flex-wrap items-center justify-center gap-6 text-xs font-semibold text-slate-500 dark:text-slate-400">
            {HERO_TRUST.map((item) => (
              <span key={item} className="flex items-center gap-1.5">
                <CheckCircle2 className="h-4 w-4 text-emerald-600 dark:text-emerald-400" /> {item}
              </span>
            ))}
          </div>
        </div>

        {/* Dashboard mockup */}
        <div className="relative mx-auto mt-14 max-w-5xl">
          <div className="overflow-hidden rounded-3xl border border-slate-200 bg-white p-4 shadow-2xl sm:p-6 dark:border-slate-800 dark:bg-slate-900">
            {/* Browser bar */}
            <div className="mb-4 flex items-center justify-between border-b border-slate-100 pb-4 dark:border-slate-800">
              <div className="flex items-center gap-2">
                <span className="h-3 w-3 rounded-full bg-rose-400" />
                <span className="h-3 w-3 rounded-full bg-amber-400" />
                <span className="h-3 w-3 rounded-full bg-emerald-400" />
                <span className="ml-2 font-mono text-xs text-slate-400">app.tawafiqhub.id/dashboard/musim-1447h</span>
              </div>
              <span className="rounded-full border border-emerald-200 bg-emerald-100 px-2.5 py-0.5 text-[11px] font-bold text-emerald-800 dark:border-emerald-800 dark:bg-emerald-950 dark:text-emerald-300">
                Kloter 01 - Syawal 1447H (Aktif)
              </span>
            </div>

            <div className="grid grid-cols-1 gap-4 lg:grid-cols-12">
              {/* Mini stats */}
              <div className="space-y-3 lg:col-span-4">
                <div className="rounded-2xl border border-slate-200 bg-slate-50 p-4 dark:border-slate-800 dark:bg-slate-950">
                  <span className="block text-xs font-medium text-slate-500 dark:text-slate-400">Total Jamaah Terdaftar</span>
                  <div className="mt-1 flex items-baseline justify-between">
                    <span className="text-2xl font-black text-slate-900 dark:text-white">45 Jamaah</span>
                    <span className="rounded bg-emerald-100 px-2 py-0.5 text-xs font-bold text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300">
                      100% Valid
                    </span>
                  </div>
                </div>
                <div className="rounded-2xl border border-slate-200 bg-slate-50 p-4 dark:border-slate-800 dark:bg-slate-950">
                  <span className="block text-xs font-medium text-slate-500 dark:text-slate-400">Hotel Makkah (Pullman Zamzam)</span>
                  <div className="mt-1 flex items-baseline justify-between">
                    <span className="text-lg font-bold text-slate-900 dark:text-white">12 Kamar Terisi</span>
                    <span className="font-mono text-xs text-slate-500 dark:text-slate-400">0 Sengketa</span>
                  </div>
                </div>
                <div className="rounded-2xl border border-slate-200 bg-slate-50 p-4 dark:border-slate-800 dark:bg-slate-950">
                  <span className="block text-xs font-medium text-slate-500 dark:text-slate-400">Armada Bus VIP Saudi</span>
                  <div className="mt-1 flex items-baseline justify-between">
                    <span className="text-lg font-bold text-slate-900 dark:text-white">Bus #01 &amp; #02</span>
                    <span className="rounded bg-teal-100 px-2 py-0.5 text-xs font-bold text-teal-700 dark:bg-teal-950 dark:text-teal-300">
                      Standby JED
                    </span>
                  </div>
                </div>
              </div>

              {/* Live manifest */}
              <div className="rounded-2xl border border-slate-200 bg-slate-50 p-4 lg:col-span-8 dark:border-slate-800 dark:bg-slate-950">
                <div className="mb-3 flex items-center justify-between">
                  <span className="font-mono text-xs font-bold uppercase tracking-wider text-slate-700 dark:text-slate-300">
                    Live Manifest &amp; Alokasi Kamar Sesuai Syariah
                  </span>
                  <span className="text-[11px] font-bold text-emerald-600 dark:text-emerald-400">Auto-Validation: AKTIF</span>
                </div>
                <div className="space-y-2">
                  {[
                    {
                      code: "#101",
                      title: "Kamar Quad (4 Pria) - Safwah Royal Orchid",
                      sub: "H. Ahmad, H. Syamsul, Bpk. Ridwan, Bpk. Fajar",
                      badge: "✓ Sah Mahram",
                      teal: false,
                    },
                    {
                      code: "#102",
                      title: "Kamar Double (Pasutri) - Safwah Royal Orchid",
                      sub: "H. Hendra (Suami) & Hj. Farida (Istri Sah)",
                      badge: "✓ Pasutri Valid",
                      teal: false,
                    },
                    {
                      code: "BUS-1",
                      title: "Bus VIP Saptco #01 (45 Seat)",
                      sub: "Driver: Syeikh Tariq (+966) • Muthowif: Ust. Syauqi",
                      badge: "✓ 45/45 Terplot",
                      teal: true,
                    },
                  ].map((row) => (
                    <div
                      key={row.code}
                      className="flex items-center justify-between rounded-xl border border-slate-200 bg-white p-3 text-xs shadow-sm dark:border-slate-800 dark:bg-slate-900"
                    >
                      <div className="flex items-center gap-3">
                        <span
                          className={`flex h-7 w-7 items-center justify-center rounded-lg font-mono font-bold ${
                            row.teal
                              ? "bg-teal-100 text-teal-800 dark:bg-teal-950 dark:text-teal-300"
                              : "bg-emerald-100 text-emerald-800 dark:bg-emerald-950 dark:text-emerald-300"
                          }`}
                        >
                          {row.code}
                        </span>
                        <div>
                          <p className="font-bold text-slate-900 dark:text-white">{row.title}</p>
                          <p className="text-[11px] text-slate-500 dark:text-slate-400">{row.sub}</p>
                        </div>
                      </div>
                      <span
                        className={`whitespace-nowrap rounded-full px-2.5 py-1 text-[10px] font-bold ${
                          row.teal
                            ? "bg-teal-100 text-teal-800 dark:bg-teal-950 dark:text-teal-300"
                            : "bg-emerald-100 text-emerald-800 dark:bg-emerald-950 dark:text-emerald-300"
                        }`}
                      >
                        {row.badge}
                      </span>
                    </div>
                  ))}
                </div>
              </div>
            </div>
          </div>

          {/* Floating cards */}
          <div className="absolute top-1/2 -left-6 hidden -translate-y-1/2 animate-bounce items-center gap-3 rounded-2xl border border-slate-200 bg-white p-4 shadow-xl [animation-duration:4s] sm:flex dark:border-slate-800 dark:bg-slate-900">
            <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-emerald-100 text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300">
              <ShieldCheck className="h-6 w-6" />
            </div>
            <div>
              <p className="text-xs font-bold text-slate-900 dark:text-white">Zero Rooming Dispute</p>
              <p className="text-[11px] text-slate-500 dark:text-slate-400">100% Bebas Salah Kamar</p>
            </div>
          </div>

          <div className="absolute -right-6 -bottom-4 hidden items-center gap-3 rounded-2xl border border-slate-200 bg-white p-4 shadow-xl sm:flex dark:border-slate-800 dark:bg-slate-900">
            <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-teal-100 text-teal-700 dark:bg-teal-950 dark:text-teal-300">
              <TrendingUp className="h-6 w-6" />
            </div>
            <div>
              <p className="text-xs font-bold text-slate-900 dark:text-white">Hemat ~380 Jam Kerja</p>
              <p className="text-[11px] text-slate-500 dark:text-slate-400">Tanpa Lembur Rekap Excel</p>
            </div>
          </div>
        </div>

        {/* Ecosystem / regulator strip */}
        <div className="mt-20 border-t border-slate-200 pt-10 text-center dark:border-slate-800">
          <p className="mb-6 text-xs font-bold uppercase tracking-wider text-slate-400">
            Mendukung Ekosistem &amp; Standar Regulasi Penyelenggara Umrah Indonesia
          </p>
          <div className="flex flex-wrap items-center justify-center gap-8 opacity-75 sm:gap-14">
            {[
              { icon: Building2, label: "SISKOPATUH KEMENAG" },
              { icon: ShieldCheck, label: "ASOSIASI PPIU & PIHK" },
              { icon: Bus, label: "SAPTCO SAUDI ARABIA" },
              { icon: Hotel, label: "MAKKAH & MADINAH HOTELS" },
            ].map(({ icon: Icon, label }) => (
              <div key={label} className="flex items-center gap-1.5 text-sm font-extrabold text-slate-700 dark:text-slate-300">
                <Icon className="h-4 w-4 text-emerald-600 dark:text-emerald-400" /> {label}
              </div>
            ))}
          </div>
        </div>
      </div>
    </section>
  );
}
