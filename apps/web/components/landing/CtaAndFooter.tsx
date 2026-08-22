"use client";

import Link from "next/link";
import { ArrowRight, Compass } from "lucide-react";
import { authClient } from "@/lib/auth-client";
import { FEATURE_MODULES } from "./content";

export default function CtaAndFooter({ onOpenDemo }: { onOpenDemo: () => void }) {
  const { data: session } = authClient.useSession();
  const isAuthenticated = Boolean(session?.user);

  return (
    <>
      <section className="bg-gradient-to-br from-emerald-800 to-emerald-950 px-5 py-20 text-center">
        <p className="mb-3 text-xs font-bold uppercase tracking-widest text-emerald-300">
          {isAuthenticated ? "Selamat datang kembali" : "Siap Musim 1447H atau 1448H Berikutnya?"}
        </p>
        <h2 className="mb-4 text-3xl font-extrabold leading-tight text-white sm:text-4xl">
          Musim Berikutnya, Kelola Lebih Rapi.
        </h2>
        <p className="mx-auto mb-8 max-w-md text-sm text-emerald-100">
          Buat musim pertama Anda dalam hitungan menit, gratis untuk mulai.
        </p>
        <div className="flex flex-wrap items-center justify-center gap-3">
          <Link
            href={isAuthenticated ? "/dashboard" : "/sign-up"}
            className="inline-flex items-center gap-2 rounded-xl bg-white px-6 py-3.5 text-sm font-bold text-emerald-800 hover:bg-emerald-50"
          >
            {isAuthenticated ? "Buka Dashboard" : "Mulai Kelola Musim Anda"}
            <ArrowRight size={16} />
          </Link>
          {!isAuthenticated && (
            <>
              <Link
                href="/sign-in"
                className="inline-flex items-center gap-2 rounded-xl border border-emerald-400/40 px-6 py-3.5 text-sm font-bold text-white hover:bg-emerald-800/40"
              >
                Masuk ke Dashboard
              </Link>
              <button
                type="button"
                onClick={onOpenDemo}
                className="inline-flex items-center gap-2 rounded-xl border border-emerald-400/40 px-6 py-3.5 text-sm font-bold text-white hover:bg-emerald-800/40"
              >
                Jadwalkan Demo & Setup
              </button>
            </>
          )}
        </div>
      </section>

      <footer className="bg-slate-950 px-5 py-14">
        <div className="mx-auto grid max-w-6xl grid-cols-1 gap-10 sm:grid-cols-3">
          <div>
            <Link href="/" className="mb-3 flex items-center gap-2 font-extrabold text-white">
              <span className="grid h-8 w-8 place-items-center rounded-lg bg-gradient-to-br from-emerald-500 to-emerald-700">
                <Compass size={16} />
              </span>
              TawafiqHub
            </Link>
            <p className="text-xs leading-relaxed text-slate-400">
              Sistem operasi terpadu untuk PPIU dan penyelenggara Haji Khusus, dari pendaftaran jamaah sampai jamaah pulang
              dengan selamat.
            </p>
          </div>
          <div>
            <p className="mb-3 text-xs font-bold uppercase tracking-widest text-slate-500">Modul</p>
            <div className="grid gap-2">
              {FEATURE_MODULES.slice(0, 5).map((m) => (
                <span key={m.title} className="text-xs text-slate-400">{m.title}</span>
              ))}
            </div>
          </div>
          <div>
            <p className="mb-3 text-xs font-bold uppercase tracking-widest text-slate-500">Kontak</p>
            <div className="grid gap-2 text-xs text-slate-400">
              <span>Jadwalkan konsultasi lewat tombol demo di atas</span>
              <span>Kantor dan layanan lapangan akan diumumkan menyusul</span>
            </div>
          </div>
        </div>
        <p className="mx-auto mt-10 max-w-6xl border-t border-slate-800 pt-6 text-xs text-slate-500">
          © 2026 Tawafiq Hub. Hak cipta dilindungi.
        </p>
      </footer>
    </>
  );
}
