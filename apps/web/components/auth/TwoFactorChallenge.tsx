"use client";

import { FormEvent, useState } from "react";
import { authClient } from "@/lib/auth-client";

export function TwoFactorChallenge({ onVerified }: { onVerified: () => Promise<void> }) {
  const [mode, setMode] = useState<"totp" | "backup">("totp");
  const [value, setValue] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");

    const code = mode === "totp" ? value.replace(/\D/g, "") : value.trim();
    if (mode === "totp" && code.length !== 6) {
      setError("Masukkan 6 digit dari aplikasi authenticator.");
      return;
    }
    if (mode === "backup" && !/^[A-Za-z0-9]{5}-[A-Za-z0-9]{5}$/.test(code)) {
      setError("Masukkan kode cadangan lengkap, termasuk tanda hubung.");
      return;
    }

    setBusy(true);
    try {
      const result = mode === "totp"
        ? await authClient.twoFactor.verifyTotp({ code })
        : await authClient.twoFactor.verifyBackupCode({ code });
      if (result.error) {
        setError(mode === "totp"
          ? "Kode tidak cocok. Gunakan kode terbaru dari aplikasi Anda."
          : "Kode cadangan tidak valid atau sudah pernah digunakan.");
        return;
      }
      await onVerified();
    } catch {
      setError("Verifikasi tidak dapat diselesaikan. Silakan coba lagi.");
    } finally {
      setBusy(false);
    }
  }

  function switchMode(next: "totp" | "backup") {
    setMode(next);
    setValue("");
    setError("");
  }

  return (
    <form onSubmit={submit} style={{ display: "grid", gap: 14 }}>
      <div style={{ display: "grid", gap: 6 }}>
        <p style={{ fontWeight: 700, fontSize: 15, margin: 0 }}>Verifikasi dua langkah</p>
        <p style={{ fontSize: 13, color: "var(--color-warm-500)", margin: 0 }}>
          {mode === "totp"
            ? "Masukkan 6 digit dari aplikasi authenticator Anda."
            : "Gunakan satu kode cadangan yang Anda simpan saat mengaktifkan 2FA."}
        </p>
      </div>

      <input
        inputMode={mode === "totp" ? "numeric" : "text"}
        autoComplete="one-time-code"
        autoFocus
        maxLength={mode === "totp" ? 6 : 11}
        aria-label={mode === "totp" ? "Kode authenticator" : "Kode cadangan"}
        placeholder={mode === "backup" ? "ABCDE-12345" : undefined}
        value={value}
        onChange={(event) => {
          setValue(mode === "totp" ? event.target.value.replace(/\D/g, "") : event.target.value);
          setError("");
        }}
        style={{
          ...inputStyle,
          letterSpacing: mode === "totp" ? "0.35em" : "0.12em",
          fontSize: mode === "totp" ? 20 : 16,
          textAlign: "center",
        }}
      />

      {error && <p role="alert" style={{ color: "var(--color-danger-600)", fontSize: 13, margin: 0 }}>{error}</p>}
      <button type="submit" disabled={busy} style={submitStyle}>
        {busy ? "Memverifikasi..." : "Verifikasi"}
      </button>
      <button
        type="button"
        disabled={busy}
        onClick={() => switchMode(mode === "totp" ? "backup" : "totp")}
        style={linkButton}
      >
        {mode === "totp" ? "Gunakan kode cadangan" : "Gunakan aplikasi authenticator"}
      </button>
    </form>
  );
}

const inputStyle: React.CSSProperties = {
  display: "block",
  width: "100%",
  padding: "10px 12px",
  border: "1px solid var(--color-cream-500)",
  borderRadius: 8,
  background: "var(--color-cream-200)",
  fontFamily: "'Plus Jakarta Sans',sans-serif",
  outline: "none",
};

const submitStyle: React.CSSProperties = {
  width: "100%",
  height: 44,
  background: "var(--color-gold-500)",
  color: "#fff",
  border: "none",
  borderRadius: 8,
  fontSize: 14,
  fontWeight: 700,
  cursor: "pointer",
  fontFamily: "'Plus Jakarta Sans',sans-serif",
};

const linkButton: React.CSSProperties = {
  border: 0,
  background: "transparent",
  color: "var(--color-emerald-900)",
  fontSize: 13,
  fontWeight: 600,
  cursor: "pointer",
};
