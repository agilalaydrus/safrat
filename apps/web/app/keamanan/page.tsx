"use client";

import { useEffect, useState } from "react";
import { IconShieldCheck, IconShieldLock, IconCopy, IconCheck } from "@tabler/icons-react";
import { QRCodeSVG } from "qrcode.react";
import { authClient } from "@/lib/auth-client";

// Enrolment lives outside the admin panel on purpose. Platform access requires
// a second factor, so if enrolling could only be reached from behind that gate,
// the first admin could never get in.
export default function SecurityPage() {
  const { data: session, isPending } = authClient.useSession();
  const [password, setPassword] = useState("");
  const [totpUri, setTotpUri] = useState("");
  const [backupCodes, setBackupCodes] = useState<string[]>([]);
  const [code, setCode] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [done, setDone] = useState(false);
  const [copied, setCopied] = useState(false);
  // null while unknown, so the password step is never shown to an account that
  // turns out not to have one.
  const [hasPassword, setHasPassword] = useState<boolean | null>(null);
  const [emailOtp, setEmailOtp] = useState("");
  const [otpSent, setOtpSent] = useState(false);
  const [resendIn, setResendIn] = useState(0);

  const enabled = Boolean((session?.user as { twoFactorEnabled?: boolean } | undefined)?.twoFactorEnabled);

  // An account created through Google has no password at all — Better Auth
  // writes no `credential` row for it. Those accounts prove ownership with a
  // short-lived email OTP instead of being forced to create a local password.
  useEffect(() => {
    if (!session?.user) return;
    let cancelled = false;
    authClient
      .listAccounts()
      .then(({ data }) => {
        if (cancelled) return;
        setHasPassword(Boolean(data?.some((account) => account.providerId === "credential")));
      })
      // Assume a password rather than block: a failed lookup should degrade to
      // the normal flow, where a wrong guess is a visible error the person can
      // act on, not a screen that refuses to proceed.
      .catch(() => { if (!cancelled) setHasPassword(true); });
    return () => { cancelled = true; };
  }, [session?.user]);

  useEffect(() => {
    if (resendIn <= 0) return;
    const timer = window.setInterval(() => setResendIn((current) => Math.max(0, current - 1)), 1000);
    return () => window.clearInterval(timer);
  }, [resendIn]);

  const postEnrollmentOtp = async (action: "request" | "verify", body: Record<string, string> = {}) => {
    const response = await fetch(`/api/auth/two-factor/enrollment-otp/${action}`, {
      method: "POST",
      credentials: "include",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    const payload = await response.json().catch(() => ({})) as { message?: string };
    if (!response.ok) throw new Error(payload.message || "Permintaan tidak dapat diproses.");
  };

  const sendEnrollmentOtp = async () => {
    setBusy(true);
    setError("");
    try {
      await postEnrollmentOtp("request");
      setOtpSent(true);
      setResendIn(60);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Gagal mengirim kode OTP.");
    } finally {
      setBusy(false);
    }
  };

  useEffect(() => { setError(""); }, [password, emailOtp, code]);

  // The plugin needs the account password to enable 2FA — proof that whoever
  // is holding this session is the account owner, not somebody who found an
  // unlocked laptop.
  const start = async () => {
    if (!password) { setError("Masukkan kata sandi akun Anda."); return; }
    setBusy(true);
    try {
      const { data, error: failed } = await authClient.twoFactor.enable({ password });
      if (failed) { setError(failed.message ?? "Kata sandi salah."); return; }
      setTotpUri(data?.totpURI ?? "");
      setBackupCodes(data?.backupCodes ?? []);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Gagal memulai pendaftaran.");
    } finally {
      setBusy(false);
    }
  };

  const verifyEmailOtpAndStart = async () => {
    const otp = emailOtp.replace(/\D/g, "");
    if (otp.length !== 6) { setError("Masukkan 6 digit OTP dari email Anda."); return; }
    setBusy(true);
    setError("");
    try {
      await postEnrollmentOtp("verify", { otp });
      const { data, error: failed } = await authClient.twoFactor.enable({});
      if (failed) { setError(failed.message ?? "Gagal memulai pemasangan authenticator."); return; }
      setTotpUri(data?.totpURI ?? "");
      setBackupCodes(data?.backupCodes ?? []);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Kode OTP tidak dapat diverifikasi.");
    } finally {
      setBusy(false);
    }
  };

  // Enabling alone does not finish it: the code has to be verified, or somebody
  // could lock themselves out with an authenticator that was never set up
  // correctly.
  const confirm = async () => {
    if (code.replace(/\D/g, "").length !== 6) { setError("Masukkan 6 digit dari aplikasi authenticator."); return; }
    setBusy(true);
    try {
      const { error: failed } = await authClient.twoFactor.verifyTotp({ code: code.replace(/\D/g, "") });
      if (failed) { setError(failed.message ?? "Kode tidak cocok. Coba kode terbaru."); return; }
      setDone(true);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Gagal memverifikasi kode.");
    } finally {
      setBusy(false);
    }
  };

  if (isPending) return <main style={page}><p style={muted}>Memuat...</p></main>;

  if (enabled && !totpUri) {
    return (
      <main style={page}>
        <p style={eyebrow}>KEAMANAN AKUN</p>
        <h1 style={title}>Verifikasi Dua Langkah</h1>
        <section style={{ ...card, borderTopColor: "var(--color-emerald-800)" }}>
          <p style={{ display: "flex", gap: 8, alignItems: "center", margin: 0, fontWeight: 700, color: "var(--color-emerald-900)" }}>
            <IconShieldCheck size={20} />Sudah aktif
          </p>
          <p style={{ ...muted, margin: 0 }}>
            Setiap login dengan email dan kata sandi akan meminta kode dari aplikasi authenticator Anda.
          </p>
        </section>
      </main>
    );
  }

  if (done) {
    return (
      <main style={page}>
        <p style={eyebrow}>KEAMANAN AKUN</p>
        <h1 style={title}>Verifikasi Dua Langkah Aktif</h1>
        <section style={{ ...card, borderTopColor: "var(--color-emerald-800)" }}>
          <p style={{ margin: 0, fontWeight: 700, color: "var(--color-emerald-900)" }}>Berhasil diaktifkan.</p>
          <p style={{ ...muted, margin: 0 }}>
            Simpan kode cadangan di bawah sekarang. Ini satu-satunya cara masuk jika ponsel Anda hilang —
            tanpa itu, pemulihan akun hanya bisa lewat akses database.
          </p>
          <BackupCodes codes={backupCodes} copied={copied} setCopied={setCopied} />
        </section>
      </main>
    );
  }

  return (
    <main style={page}>
      <p style={eyebrow}>KEAMANAN AKUN</p>
      <h1 style={title}>Verifikasi Dua Langkah</h1>
      <p style={{ ...muted, margin: "0 0 20px", maxWidth: 560 }}>
        Menambahkan kode dari aplikasi authenticator (Google Authenticator, Authy, atau sejenisnya)
        untuk melindungi akses akun. Akun yang tidak memiliki kata sandi akan dikonfirmasi melalui
        OTP yang dikirim ke email terverifikasi.
      </p>

      {hasPassword === false ? (
        <section style={card}>
          <p style={{ display: "flex", gap: 8, alignItems: "center", margin: 0, fontWeight: 700 }}>
            <IconShieldLock size={20} />Langkah 1 — verifikasi email akun
          </p>
          <p style={{ ...muted, margin: 0 }}>
            Akun ini belum mempunyai kata sandi. Kami akan mengirim OTP 6 digit ke{" "}
            <strong>{session?.user?.email}</strong> untuk memastikan bahwa akun ini milik Anda.
          </p>
          <p style={{ ...muted, margin: 0 }}>
            OTP berlaku selama 5 menit dan hanya dapat dicoba maksimal 5 kali.
          </p>
          {otpSent && (
            <label style={label}>
              Kode OTP dari email
              <input
                inputMode="numeric"
                maxLength={6}
                value={emailOtp}
                onChange={(event) => setEmailOtp(event.target.value.replace(/\D/g, ""))}
                style={{ ...input, letterSpacing: "0.3em", fontSize: 18 }}
                autoComplete="one-time-code"
                autoFocus
              />
            </label>
          )}
          {error && <p style={errorText}>{error}</p>}
          {otpSent ? (
            <div style={{ display: "flex", flexWrap: "wrap", gap: 10, alignItems: "center" }}>
              <button onClick={verifyEmailOtpAndStart} disabled={busy} style={primary}>
                {busy ? "Memverifikasi…" : "Verifikasi dan lanjutkan"}
              </button>
              <button onClick={sendEnrollmentOtp} disabled={busy || resendIn > 0} style={ghost}>
                {resendIn > 0 ? `Kirim ulang (${resendIn}d)` : "Kirim ulang OTP"}
              </button>
            </div>
          ) : (
            <button onClick={sendEnrollmentOtp} disabled={busy} style={primary}>
              {busy ? "Mengirim…" : "Kirim OTP ke email"}
            </button>
          )}
        </section>
      ) : !totpUri ? (
        <section style={card}>
          <p style={{ display: "flex", gap: 8, alignItems: "center", margin: 0, fontWeight: 700 }}>
            <IconShieldLock size={20} />Langkah 1 — konfirmasi kata sandi
          </p>
          <label style={label}>
            Kata sandi akun
            <input type="password" value={password} onChange={(e) => setPassword(e.target.value)}
              style={input} autoComplete="current-password" />
          </label>
          {error && <p style={errorText}>{error}</p>}
          <button onClick={start} style={primary} disabled={busy}>
            {busy ? "Memproses..." : "Lanjutkan"}
          </button>
        </section>
      ) : (
        <section style={card}>
          <p style={{ display: "flex", gap: 8, alignItems: "center", margin: 0, fontWeight: 700 }}>
            <IconShieldLock size={20} />Langkah 2 — daftarkan di aplikasi authenticator
          </p>
          <p style={{ ...muted, margin: 0 }}>
            Buka aplikasi authenticator Anda, pilih tambah akun, lalu pindai QR Code berikut.
          </p>
          <div style={qrBox} aria-label="QR Code untuk Google Authenticator atau Authy">
            <QRCodeSVG value={totpUri} size={200} level="M" />
          </div>
          <details>
            <summary style={{ cursor: "pointer", color: "var(--color-warm-700)", fontSize: 13 }}>Tidak bisa memindai? Tampilkan kode manual</summary>
            <code style={{ ...uriBox, marginTop: 10 }}>{totpUri}</code>
          </details>

          <label style={label}>
            Kode 6 digit dari aplikasi
            <input inputMode="numeric" maxLength={6} value={code} onChange={(e) => setCode(e.target.value)}
              style={{ ...input, letterSpacing: "0.3em", fontSize: 18 }} autoComplete="one-time-code" />
          </label>
          {error && <p style={errorText}>{error}</p>}
          <button onClick={confirm} style={primary} disabled={busy}>
            {busy ? "Memverifikasi..." : "Aktifkan"}
          </button>

          <div style={{ borderTop: "1px solid var(--color-cream-300)", paddingTop: 14, marginTop: 4 }}>
            <p style={{ margin: "0 0 8px", fontWeight: 700, fontSize: 14 }}>Kode cadangan</p>
            <p style={{ ...muted, margin: "0 0 10px" }}>
              Simpan sekarang, sebelum menutup halaman ini. Kode ini tidak ditampilkan lagi.
            </p>
            <BackupCodes codes={backupCodes} copied={copied} setCopied={setCopied} />
          </div>
        </section>
      )}
    </main>
  );
}

function BackupCodes({ codes, copied, setCopied }: { codes: string[]; copied: boolean; setCopied: (v: boolean) => void }) {
  if (codes.length === 0) return null;
  return (
    <div style={{ display: "grid", gap: 10 }}>
      <div style={codeGrid}>
        {codes.map((backup) => <code key={backup} style={codeChip}>{backup}</code>)}
      </div>
      <button
        style={ghost}
        onClick={() => {
          void navigator.clipboard.writeText(codes.join("\n"));
          setCopied(true);
        }}
      >
        {copied ? <><IconCheck size={15} />Tersalin</> : <><IconCopy size={15} />Salin semua</>}
      </button>
    </div>
  );
}

const page: React.CSSProperties = { maxWidth: 640, margin: "0 auto", padding: "32px 24px 80px" };
const eyebrow: React.CSSProperties = { color: "var(--color-gold-800)", fontSize: 11, fontWeight: 700, letterSpacing: ".08em", margin: "4px 0 8px" };
const title: React.CSSProperties = { fontSize: "clamp(28px,5vw,40px)", fontWeight: 500, margin: "0 0 8px" };
const muted: React.CSSProperties = { color: "var(--color-warm-500)", fontSize: 14 };
const card: React.CSSProperties = { display: "grid", gap: 14, background: "#fff", border: "1px solid var(--color-cream-400)", borderTop: "2px solid var(--color-gold-500)", borderRadius: 12, padding: 22 };
const label: React.CSSProperties = { display: "grid", gap: 6, fontSize: 13, color: "var(--color-warm-700)" };
const input: React.CSSProperties = { minHeight: 48, border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 12px" };
const primary: React.CSSProperties = { minHeight: 48, border: 0, borderRadius: 8, background: "var(--color-emerald-800)", color: "#fff", fontWeight: 700, justifySelf: "start", padding: "0 22px" };
const ghost: React.CSSProperties = { minHeight: 40, border: "1px solid var(--color-cream-400)", borderRadius: 8, background: "transparent", color: "var(--color-warm-700)", display: "inline-flex", alignItems: "center", gap: 6, justifySelf: "start", padding: "0 14px", fontSize: 13 };
const errorText: React.CSSProperties = { color: "var(--color-danger-600)", margin: 0, fontSize: 14 };
const uriBox: React.CSSProperties = { display: "block", padding: 12, background: "var(--color-cream-100)", borderRadius: 8, fontSize: 12, overflowWrap: "anywhere", color: "var(--color-warm-700)" };
const qrBox: React.CSSProperties = { width: "fit-content", padding: 14, background: "#fff", border: "1px solid var(--color-cream-300)", borderRadius: 10 };
const codeGrid: React.CSSProperties = { display: "grid", gridTemplateColumns: "repeat(auto-fill,minmax(130px,1fr))", gap: 8 };
const codeChip: React.CSSProperties = { padding: "8px 10px", background: "var(--color-cream-100)", borderRadius: 6, fontSize: 13, textAlign: "center", letterSpacing: ".05em" };
