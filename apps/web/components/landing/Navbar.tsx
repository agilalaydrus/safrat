"use client";

import { useState } from "react";
import Link from "next/link";
import { Compass, Menu, Moon, Sun, X } from "lucide-react";
import { authClient } from "@/lib/auth-client";
import { useTheme } from "./ThemeProvider";
import { NAV_LINKS } from "./content";

export default function Navbar({ onOpenDemo }: { onOpenDemo: () => void }) {
  const { theme, toggleTheme } = useTheme();
  const { data: session, isPending } = authClient.useSession();
  const isAuthenticated = Boolean(session?.user);
  const [drawerOpen, setDrawerOpen] = useState(false);

  return (
    <nav className="sticky top-0 z-40 border-b border-slate-200 bg-slate-50/90 backdrop-blur dark:border-slate-800 dark:bg-slate-950/90">
      <div className="mx-auto flex max-w-7xl items-center justify-between gap-4 px-5 py-3">
        <Link href="/" aria-label="TawafiqHub" className="flex items-center gap-2.5 font-extrabold text-slate-900 dark:text-white">
          <span className="grid h-9 w-9 place-items-center rounded-xl bg-gradient-to-br from-emerald-500 to-emerald-700 text-white shadow-sm">
            <Compass size={19} strokeWidth={2.3} />
          </span>
          <span className="text-lg tracking-tight">TawafiqHub</span>
        </Link>

        <div className="hidden items-center gap-6 lg:flex">
          {NAV_LINKS.map(([label, href]) => (
            <a key={href} href={href} className="text-sm font-semibold text-slate-600 hover:text-emerald-700 dark:text-slate-300 dark:hover:text-emerald-400">
              {label}
            </a>
          ))}
        </div>

        <div className="hidden items-center gap-3 lg:flex">
          <span className="rounded-full border border-emerald-300 bg-emerald-100 px-3 py-1 text-xs font-bold text-emerald-800 dark:border-emerald-800 dark:bg-emerald-950 dark:text-emerald-300">
            Musim 1447H Ready
          </span>
          <button
            type="button"
            onClick={toggleTheme}
            aria-label={theme === "light" ? "Ganti ke mode gelap" : "Ganti ke mode terang"}
            className="grid h-10 w-10 place-items-center rounded-lg border border-slate-200 text-slate-500 hover:bg-slate-100 dark:border-slate-700 dark:text-slate-300 dark:hover:bg-slate-800"
          >
            {theme === "light" ? <Moon size={17} /> : <Sun size={17} />}
          </button>
          {isAuthenticated ? (
            <Link href="/dashboard" className="rounded-lg bg-emerald-600 px-4 py-2.5 text-sm font-bold text-white hover:bg-emerald-700">
              Dashboard
            </Link>
          ) : (
            !isPending && (
              <button type="button" onClick={onOpenDemo} className="rounded-lg bg-emerald-600 px-4 py-2.5 text-sm font-bold text-white hover:bg-emerald-700">
                Jadwalkan Demo
              </button>
            )
          )}
        </div>

        <button
          type="button"
          onClick={() => setDrawerOpen(true)}
          aria-label="Buka menu"
          className="grid h-10 w-10 place-items-center rounded-lg border border-slate-200 text-slate-600 lg:hidden dark:border-slate-700 dark:text-slate-300"
        >
          <Menu size={20} />
        </button>
      </div>

      {drawerOpen && (
        <div className="fixed inset-0 z-50 lg:hidden">
          <button
            type="button"
            aria-label="Tutup menu"
            onClick={() => setDrawerOpen(false)}
            className="absolute inset-0 bg-slate-900/50"
          />
          <div className="absolute inset-y-0 right-0 flex w-72 flex-col gap-6 bg-white p-6 shadow-xl dark:bg-slate-950">
            <div className="flex items-center justify-between">
              <span className="font-extrabold text-slate-900 dark:text-white">Menu</span>
              <button type="button" onClick={() => setDrawerOpen(false)} aria-label="Tutup" className="grid h-9 w-9 place-items-center rounded-lg text-slate-500 hover:bg-slate-100 dark:hover:bg-slate-800">
                <X size={18} />
              </button>
            </div>
            <div className="flex flex-col gap-4">
              {NAV_LINKS.map(([label, href]) => (
                <a key={href} href={href} onClick={() => setDrawerOpen(false)} className="text-base font-semibold text-slate-700 dark:text-slate-200">
                  {label}
                </a>
              ))}
            </div>
            <button
              type="button"
              onClick={toggleTheme}
              className="flex items-center gap-2 rounded-lg border border-slate-200 px-3 py-2.5 text-sm font-semibold text-slate-600 dark:border-slate-700 dark:text-slate-300"
            >
              {theme === "light" ? <Moon size={16} /> : <Sun size={16} />}
              {theme === "light" ? "Mode gelap" : "Mode terang"}
            </button>
            <div className="mt-auto flex flex-col gap-3">
              {isAuthenticated ? (
                <Link href="/dashboard" className="rounded-lg bg-emerald-600 px-4 py-3 text-center text-sm font-bold text-white">
                  Dashboard
                </Link>
              ) : (
                <button
                  type="button"
                  onClick={() => {
                    setDrawerOpen(false);
                    onOpenDemo();
                  }}
                  className="rounded-lg bg-emerald-600 px-4 py-3 text-center text-sm font-bold text-white"
                >
                  Jadwalkan Demo
                </button>
              )}
            </div>
          </div>
        </div>
      )}
    </nav>
  );
}
