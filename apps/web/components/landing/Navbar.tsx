"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { IconCompass, IconMenu2, IconMoon, IconSun, IconX } from "@tabler/icons-react";
import { authClient } from "@/lib/auth-client";
import { resolveLandingPath } from "@/lib/post-login";
import { useTheme } from "./ThemeProvider";
import { NAV_LINKS } from "./content";

export default function Navbar() {
  const { theme, toggleTheme } = useTheme();
  const { data: session, isPending } = authClient.useSession();
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [accountPath, setAccountPath] = useState<string>();
  const isAuthenticated = Boolean(session?.user);
  const userId = session?.user?.id;

  useEffect(() => {
    if (!userId) {
      setAccountPath(undefined);
      return;
    }
    let cancelled = false;
    resolveLandingPath()
      .then((path) => { if (!cancelled) setAccountPath(path); })
      // /dashboard has its own authoritative access guard. Keeping it as the
      // failure fallback means a temporary API outage never strands a signed-
      // in visitor on the marketing page.
      .catch(() => { if (!cancelled) setAccountPath("/dashboard"); });
    return () => { cancelled = true; };
  }, [userId]);

  const accountLabel = accountPath === "/onboarding" ? "Lanjutkan pendaftaran" : "Dashboard";

  useEffect(() => {
    if (!drawerOpen) return;
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape") setDrawerOpen(false);
    };
    document.body.style.overflow = "hidden";
    window.addEventListener("keydown", closeOnEscape);
    return () => {
      document.body.style.overflow = "";
      window.removeEventListener("keydown", closeOnEscape);
    };
  }, [drawerOpen]);

  return (
    <>
      <header className="landing-nav">
        <div className="landing-container landing-nav-inner">
          <Link href="/" aria-label="TawafiqHub" className="landing-brand">
            <span className="landing-brand-mark" aria-hidden>
              <IconCompass size={21} stroke={1.9} />
            </span>
            <span className="landing-brand-copy">
              <strong>TawafiqHub</strong>
              <small>Sistem operasional Umrah</small>
            </span>
          </Link>

          <nav className="landing-desktop-nav" aria-label="Navigasi utama">
            {NAV_LINKS.map(([label, href]) => (
              <a key={href} href={href}>{label}</a>
            ))}
          </nav>

          <div className="landing-nav-actions">
            <button
              type="button"
              onClick={toggleTheme}
              aria-label={theme === "light" ? "Aktifkan mode gelap" : "Aktifkan mode terang"}
              className="landing-icon-button"
            >
              {theme === "light" ? <IconMoon size={18} /> : <IconSun size={18} />}
            </button>

            {isAuthenticated ? (
              accountPath ? (
                <Link href={accountPath} className="landing-button landing-button-primary landing-nav-cta">
                  {accountLabel}
                </Link>
              ) : null
            ) : !isPending ? (
              <>
                <Link href="/sign-in" className="landing-login-link">Masuk</Link>
                <Link href="/sign-up" className="landing-button landing-button-primary landing-nav-cta">Mulai gratis</Link>
              </>
            ) : null}

            <button
              type="button"
              onClick={() => setDrawerOpen(true)}
              aria-label="Buka menu"
              className="landing-icon-button landing-menu-button"
            >
              <IconMenu2 size={20} />
            </button>
          </div>
        </div>
      </header>

      {drawerOpen && (
        <div className="landing-drawer-layer">
          <button type="button" aria-label="Tutup menu" className="landing-drawer-scrim" onClick={() => setDrawerOpen(false)} />
          <aside className="landing-drawer" aria-label="Menu seluler" role="dialog" aria-modal="true">
            <div className="landing-drawer-head">
              <span className="landing-brand"><span className="landing-brand-mark"><IconCompass size={20} /></span><strong>TawafiqHub</strong></span>
              <button type="button" aria-label="Tutup" className="landing-icon-button" onClick={() => setDrawerOpen(false)}><IconX size={19} /></button>
            </div>
            <nav>
              {NAV_LINKS.map(([label, href]) => (
                <a key={href} href={href} onClick={() => setDrawerOpen(false)}>{label}</a>
              ))}
            </nav>
            <div className="landing-drawer-actions">
              {isAuthenticated ? (
                accountPath ? <Link href={accountPath} className="landing-button landing-button-primary">{accountLabel}</Link> : null
              ) : (
                <>
                  <Link href="/sign-in" className="landing-button landing-button-secondary">Masuk</Link>
                  <Link href="/sign-up" className="landing-button landing-button-primary">Mulai gratis</Link>
                </>
              )}
            </div>
          </aside>
        </div>
      )}
    </>
  );
}
