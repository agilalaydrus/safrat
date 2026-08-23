"use client";

import { useState } from "react";
import Link from "next/link";
import { Compass, Menu, Moon, Sun, X } from "lucide-react";
import { authClient } from "@/lib/auth-client";
import { useTheme } from "./ThemeProvider";
import { NAV_LINKS } from "./content";

export default function Navbar() {
  const { theme, toggleTheme } = useTheme();
  const { data: session, isPending } = authClient.useSession();
  const isAuthenticated = Boolean(session?.user);
  const [drawerOpen, setDrawerOpen] = useState(false);

  return (
    <>
    <header className="sticky top-0 z-40 border-b border-slate-200 bg-white/95 shadow-sm backdrop-blur-md dark:border-slate-800 dark:bg-slate-950/95">
      <div className="mx-auto flex h-20 max-w-7xl items-center justify-between px-4 sm:px-6 lg:px-8">
        {/* Brand */}
        <Link href="/" aria-label="TawafiqHub" className="flex items-center gap-3">
          <span className="flex h-10 w-10 items-center justify-center rounded-xl bg-gradient-to-tr from-emerald-600 to-teal-500 text-white shadow-md shadow-emerald-600/20">
            <Compass className="h-6 w-6" />
          </span>
          <span className="leading-tight">
            <span className="block text-xl font-extrabold tracking-tight text-slate-900 dark:text-white">
              Tawafiq<span className="text-emerald-600 dark:text-emerald-400">Hub</span>
            </span>
            <span className="block text-[10px] font-semibold uppercase tracking-wider text-slate-500 dark:text-slate-400">
              Sistem Operasi PPIU &amp; PIHK
            </span>
          </span>
        </Link>

        {/* Desktop nav */}
        <nav className="hidden items-center gap-7 text-sm font-semibold text-slate-600 md:flex dark:text-slate-300">
          {NAV_LINKS.map(([label, href]) => (
            <a key={href} href={href} className="transition-colors hover:text-emerald-600 dark:hover:text-emerald-400">
              {label}
            </a>
          ))}
        </nav>

        {/* Actions */}
        <div className="flex items-center gap-2 sm:gap-3">
          <button
            type="button"
            onClick={toggleTheme}
            aria-label={theme === "light" ? "Ganti ke mode gelap" : "Ganti ke mode terang"}
            className="grid h-10 w-10 place-items-center rounded-xl border border-slate-300 text-slate-500 transition-colors hover:bg-slate-100 dark:border-slate-700 dark:text-slate-300 dark:hover:bg-slate-800"
          >
            {theme === "light" ? <Moon size={17} /> : <Sun size={17} />}
          </button>

          {isAuthenticated ? (
            <Link
              href="/dashboard"
              className="rounded-xl bg-emerald-600 px-5 py-2.5 text-xs font-bold text-white shadow-md shadow-emerald-600/20 transition-all hover:bg-emerald-700 sm:text-sm"
            >
              Dashboard
            </Link>
          ) : (
            !isPending && (
              <>
                <Link
                  href="/sign-in"
                  className="rounded-xl border border-slate-300 bg-white px-4 py-2.5 text-xs font-bold text-slate-700 shadow-sm transition-all hover:border-emerald-500 hover:text-emerald-600 sm:text-sm dark:border-slate-700 dark:bg-slate-900 dark:text-slate-200 dark:hover:border-emerald-500 dark:hover:text-emerald-400"
                >
                  Masuk
                </Link>
                <Link
                  href="/sign-up"
                  className="rounded-xl bg-emerald-600 px-4 py-2.5 text-xs font-bold text-white shadow-md shadow-emerald-600/20 transition-all hover:bg-emerald-700 sm:px-5 sm:text-sm"
                >
                  Daftar
                </Link>
              </>
            )
          )}

          <button
            type="button"
            onClick={() => setDrawerOpen(true)}
            aria-label="Buka menu"
            className="grid h-10 w-10 place-items-center rounded-xl border border-slate-300 text-slate-600 md:hidden dark:border-slate-700 dark:text-slate-300"
          >
            <Menu size={20} />
          </button>
        </div>
      </div>
      </header>

      {/* Mobile drawer — rendered outside <header> so its fixed positioning is
          relative to the viewport, not trapped by the header's backdrop-blur
          (a backdrop-filter ancestor becomes the containing block for fixed
          descendants, which would clip the drawer to the header bar). */}
      {drawerOpen && (
        <div className="fixed inset-0 z-50 md:hidden">
          <button
            type="button"
            aria-label="Tutup menu"
            onClick={() => setDrawerOpen(false)}
            className="absolute inset-0 bg-slate-900/50"
          />
          <div className="absolute inset-y-0 right-0 flex w-72 flex-col gap-6 bg-white p-6 shadow-xl dark:bg-slate-950">
            <div className="flex items-center justify-between">
              <span className="font-extrabold text-slate-900 dark:text-white">Menu</span>
              <button
                type="button"
                onClick={() => setDrawerOpen(false)}
                aria-label="Tutup"
                className="grid h-9 w-9 place-items-center rounded-lg text-slate-500 hover:bg-slate-100 dark:hover:bg-slate-800"
              >
                <X size={18} />
              </button>
            </div>
            <div className="flex flex-col gap-4">
              {NAV_LINKS.map(([label, href]) => (
                <a
                  key={href}
                  href={href}
                  onClick={() => setDrawerOpen(false)}
                  className="text-base font-semibold text-slate-700 dark:text-slate-200"
                >
                  {label}
                </a>
              ))}
            </div>
            <div className="mt-auto flex flex-col gap-3">
              {isAuthenticated ? (
                <Link href="/dashboard" className="rounded-xl bg-emerald-600 px-4 py-3 text-center text-sm font-bold text-white">
                  Dashboard
                </Link>
              ) : (
                <>
                  <Link
                    href="/sign-in"
                    onClick={() => setDrawerOpen(false)}
                    className="rounded-xl border border-slate-300 px-4 py-3 text-center text-sm font-bold text-slate-700 dark:border-slate-700 dark:text-slate-200"
                  >
                    Masuk
                  </Link>
                  <Link
                    href="/sign-up"
                    onClick={() => setDrawerOpen(false)}
                    className="rounded-xl bg-emerald-600 px-4 py-3 text-center text-sm font-bold text-white"
                  >
                    Daftar
                  </Link>
                </>
              )}
            </div>
          </div>
        </div>
      )}
    </>
  );
}
