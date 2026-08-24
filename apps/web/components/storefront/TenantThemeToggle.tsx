"use client";

import { IconMoonStars, IconSunHigh } from "@tabler/icons-react";
import { useTheme } from "@/components/landing/ThemeProvider";

export default function TenantThemeToggle() {
  const { theme, toggleTheme } = useTheme();

  return (
    <button
      type="button"
      onClick={toggleTheme}
      aria-label={theme === "light" ? "Aktifkan mode gelap" : "Aktifkan mode terang"}
      className="tenant-icon-button"
    >
      {theme === "light" ? <IconMoonStars size={19} stroke={1.8} /> : <IconSunHigh size={19} stroke={1.8} />}
    </button>
  );
}
