"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { IconBuildingHospital, IconLogout, IconPhone, IconPlane, IconShieldCheck, IconUserCircle, IconUsersGroup } from "@tabler/icons-react";
import { PilgrimAppInfo } from "@hajj-saas/proto-gen/hajj/v1/pilgrim_app_pb";
import { authClient } from "@/lib/auth-client";
import { pilgrimAppClient } from "@/lib/rpc";
import { usePilgrimCode } from "@/lib/pilgrim-context";
import { invalidateMyAccessCache } from "@/lib/access-cache";
import CameraCaptureButton from "@/components/shared/CameraCaptureButton";

const KYC_STATUSES: Record<string, { label: string; color: string }> = {
  UNVERIFIED: { label: "Belum Diisi", color: "var(--color-warm-400)" },
  PENDING_REVIEW: { label: "Menunggu Verifikasi", color: "var(--color-gold-800)" },
  VERIFIED: { label: "Terverifikasi", color: "var(--color-emerald-900)" },
  REJECTED: { label: "Ditolak", color: "var(--color-danger-600)" },
};
const DOC_TYPES = [
  { value: "KTP", label: "KTP" },
  { value: "SELFIE", label: "Foto Selfie" },
];
const MARITAL_STATUSES = [
  { value: "", label: "Belum diisi" },
  { value: "SINGLE", label: "Belum Menikah" },
  { value: "MARRIED", label: "Menikah" },
  { value: "DIVORCED", label: "Cerai" },
  { value: "WIDOWED", label: "Janda/Duda" },
];
const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "";

export default function PilgrimProfilePage() {
  const router = useRouter();
  const code = usePilgrimCode();
  const { data: session } = authClient.useSession();
  const [info, setInfo] = useState<PilgrimAppInfo>();

  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [changing, setChanging] = useState(false);
  const [notice, setNotice] = useState("");
  const [error, setError] = useState("");

  const [nik, setNik] = useState("");
  const [address, setAddress] = useState("");
  const [placeOfBirth, setPlaceOfBirth] = useState("");
  const [maritalStatus, setMaritalStatus] = useState("");
  const [occupation, setOccupation] = useState("");
  const [fatherName, setFatherName] = useState("");
  const [savingKyc, setSavingKyc] = useState(false);
  const [kycNotice, setKycNotice] = useState("");
  const [docType, setDocType] = useState("KTP");
  const [uploading, setUploading] = useState(false);

  useEffect(() => {
    if (!code) return;
    pilgrimAppClient.getMyInfo({ appAccessCode: code }).then((result) => {
      setInfo(result);
      setNik(result.nik); setAddress(result.address);
      setPlaceOfBirth(result.placeOfBirth); setMaritalStatus(result.maritalStatus); setOccupation(result.occupation); setFatherName(result.fatherName);
    }).catch(() => {});
  }, [code]);

  async function submitKyc() {
    if (!code) return;
    setSavingKyc(true);
    setKycNotice("");
    try {
      const result = await pilgrimAppClient.submitMyPilgrimKyc({ appAccessCode: code, nik, address, placeOfBirth, maritalStatus, occupation, fatherName });
      setInfo(result);
      setKycNotice("Data KYC tersimpan — menunggu verifikasi admin.");
    } catch (err) {
      setKycNotice(err instanceof Error ? err.message : "Gagal menyimpan KYC.");
    } finally {
      setSavingKyc(false);
    }
  }

  async function uploadKycDoc(file: File) {
    if (!code) return;
    setUploading(true);
    setKycNotice("");
    try {
      const form = new FormData();
      form.append("app_access_code", code);
      form.append("doc_type", docType);
      form.append("file", file);
      const response = await fetch(`${API_URL}/upload/document/self`, { method: "POST", body: form });
      if (!response.ok) throw new Error((await response.text()) || "Upload gagal.");
      setKycNotice("Dokumen berhasil diunggah.");
    } catch (err) {
      setKycNotice(err instanceof Error ? err.message : "Upload gagal.");
    } finally {
      setUploading(false);
    }
  }

  function uploadKycDocFromInput(event: React.ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0];
    event.target.value = "";
    if (file) void uploadKycDoc(file);
  }

  async function submitChangePassword() {
    setError("");
    setNotice("");
    if (newPassword.length < 8) { setError("Kata sandi baru minimal 8 karakter."); return; }
    setChanging(true);
    try {
      await authClient.changePassword({ currentPassword, newPassword, revokeOtherSessions: true });
      setNotice("Kata sandi berhasil diubah.");
      setCurrentPassword("");
      setNewPassword("");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Gagal mengubah kata sandi.");
    } finally {
      setChanging(false);
    }
  }

  async function signOut() {
    await authClient.signOut();
    invalidateMyAccessCache();
    router.push("/sign-in");
  }

  return (
    <main style={page}>
      <p style={eyebrow}>PROFIL JAMAAH</p>

      <section style={profileCard}>
        <div style={avatar}><IconUserCircle size={40} /></div>
        <h1 style={{ margin: "10px 0 2px", fontSize: 20 }}>{info?.fullName || session?.user?.name || "Jamaah"}</h1>
        <p style={{ margin: 0, opacity: .9, fontSize: 13 }}>{info?.passportNumber}</p>
        <p style={{ margin: "2px 0 0", opacity: .9, fontSize: 13 }}>{session?.user?.email}</p>
        {info?.phone && <p style={{ margin: "2px 0 0", display: "flex", alignItems: "center", gap: 6, opacity: .9, fontSize: 13 }}><IconPhone size={14} />{info.phone}</p>}
      </section>

      <section style={{ marginTop: 16, display: "grid", gap: 8 }}>
        {info?.groupName && <div style={infoRow}><IconUsersGroup size={18} color="var(--color-emerald-800)" /><div><span style={infoLabel}>Grup</span><p style={infoValue}>{info.groupName}</p></div></div>}
        {info && info.hotelStays.length > 1 && (
          <div style={infoRow}>
            <IconBuildingHospital size={18} color="var(--color-emerald-800)" />
            <div>
              <span style={infoLabel}>Hotel &amp; Kamar</span>
              {info.hotelStays.map((stay, i) => (
                <p key={i} style={infoValue}>{stay.hotelName}{stay.roomNumber ? ` · Kamar ${stay.roomNumber}` : ""}</p>
              ))}
            </div>
          </div>
        )}
        {info && info.hotelStays.length <= 1 && info?.hotelName && <div style={infoRow}><IconBuildingHospital size={18} color="var(--color-emerald-800)" /><div><span style={infoLabel}>Hotel &amp; Kamar</span><p style={infoValue}>{info.hotelName}{info.roomNumber ? ` · Kamar ${info.roomNumber}` : ""}</p></div></div>}
        {info?.kloterCode && <div style={infoRow}><IconPlane size={18} color="var(--color-emerald-800)" /><div><span style={infoLabel}>Kloter</span><p style={infoValue}>{info.kloterCode}{info.kloterFlightNumber ? ` · ${info.kloterFlightNumber}` : ""}</p></div></div>}
      </section>

      <section style={{ marginTop: 20 }}>
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 10 }}>
          <h2 style={sectionTitle}>KYC (IDENTITAS)</h2>
          {info?.kycStatus && <span style={{ ...kycBadge, background: (KYC_STATUSES[info.kycStatus] ?? KYC_STATUSES.UNVERIFIED!).color }}>{(KYC_STATUSES[info.kycStatus] ?? KYC_STATUSES.UNVERIFIED!).label}</span>}
        </div>
        <div style={formCard}>
          {info?.kycStatus === "REJECTED" && info.kycRejectionReason && <p style={{ margin: "0 0 12px", fontSize: 13, color: "var(--color-danger-600)" }}>Alasan penolakan: {info.kycRejectionReason}</p>}
          <label style={{ display: "grid", gap: 6 }}>
            <span style={lab}>NIK</span>
            <input value={nik} onChange={(e) => setNik(e.target.value.replace(/\D/g, ""))} inputMode="numeric" autoComplete="off" maxLength={32} style={input} />
          </label>
          <label style={{ display: "grid", gap: 6, marginTop: 12 }}>
            <span style={lab}>Alamat sesuai KTP</span>
            <textarea value={address} onChange={(e) => setAddress(e.target.value)} rows={2} style={{ ...input, minHeight: 60, resize: "vertical" }} />
          </label>
          <label style={{ display: "grid", gap: 6, marginTop: 12 }}>
            <span style={lab}>Tempat lahir</span>
            <input value={placeOfBirth} onChange={(e) => setPlaceOfBirth(e.target.value)} autoCapitalize="words" style={input} />
          </label>
          <label style={{ display: "grid", gap: 6, marginTop: 12 }}>
            <span style={lab}>Status pernikahan</span>
            <select value={maritalStatus} onChange={(e) => setMaritalStatus(e.target.value)} style={input}>
              {MARITAL_STATUSES.map((s) => <option key={s.value} value={s.value}>{s.label}</option>)}
            </select>
          </label>
          <label style={{ display: "grid", gap: 6, marginTop: 12 }}>
            <span style={lab}>Pekerjaan</span>
            <input value={occupation} onChange={(e) => setOccupation(e.target.value)} autoCapitalize="words" style={input} />
          </label>
          <label style={{ display: "grid", gap: 6, marginTop: 12 }}>
            <span style={lab}>Nama ayah kandung</span>
            <input value={fatherName} onChange={(e) => setFatherName(e.target.value)} autoComplete="off" autoCapitalize="words" style={input} />
          </label>
          {kycNotice && <p style={successBox}>{kycNotice}</p>}
          <button onClick={() => void submitKyc()} disabled={savingKyc} style={primary}>{savingKyc ? "Menyimpan..." : "Simpan KYC"}</button>
        </div>
        <div style={{ ...formCard, marginTop: 12 }}>
          <p style={{ margin: "0 0 10px", display: "flex", alignItems: "center", gap: 6, fontWeight: 700, fontSize: 14 }}><IconShieldCheck size={16} />Upload KTP / Selfie</p>
          <div style={{ display: "flex", gap: 8, flexWrap: "wrap", alignItems: "center" }}>
            <select value={docType} onChange={(e) => setDocType(e.target.value)} style={input}>
              {DOC_TYPES.map((d) => <option key={d.value} value={d.value}>{d.label}</option>)}
            </select>
            <CameraCaptureButton label={uploading ? "..." : "Ambil Foto"} onCapture={(file) => void uploadKycDoc(file)} disabled={uploading} style={cameraLabel} />
            <label style={uploadLabel}>
              {uploading ? "Mengunggah..." : "Upload File"}
              <input type="file" accept=".pdf,.jpg,.jpeg,.png" onChange={uploadKycDocFromInput} style={{ display: "none" }} disabled={uploading} />
            </label>
          </div>
        </div>
      </section>

      <section style={{ marginTop: 20 }}>
        <h2 style={sectionTitle}>GANTI KATA SANDI</h2>
        <div style={formCard}>
          <label style={{ display: "grid", gap: 6 }}>
            <span style={lab}>Kata sandi saat ini</span>
            <input type="password" autoComplete="current-password" value={currentPassword} onChange={(e) => setCurrentPassword(e.target.value)} style={input} />
          </label>
          <label style={{ display: "grid", gap: 6, marginTop: 12 }}>
            <span style={lab}>Kata sandi baru</span>
            <input type="password" autoComplete="new-password" value={newPassword} onChange={(e) => setNewPassword(e.target.value)} style={input} />
          </label>
          {error && <p style={errBox}>{error}</p>}
          {notice && <p style={successBox}>{notice}</p>}
          <button onClick={() => void submitChangePassword()} disabled={changing || !currentPassword || !newPassword} style={primary}>
            {changing ? "Menyimpan..." : "Simpan Kata Sandi Baru"}
          </button>
        </div>
      </section>

      <button onClick={() => void signOut()} style={signOutBtn}><IconLogout size={18} />Keluar</button>
    </main>
  );
}

const page: React.CSSProperties = { maxWidth: 480, margin: "0 auto", padding: "20px 20px 24px" };
const eyebrow: React.CSSProperties = { color: "var(--color-gold-800)", fontSize: 11, fontWeight: 700, letterSpacing: ".08em", margin: "0 0 12px" };
const profileCard: React.CSSProperties = { padding: 22, borderRadius: 16, background: "linear-gradient(135deg,var(--color-emerald-900),var(--color-emerald-800))", color: "#fff" };
const avatar: React.CSSProperties = { width: 64, height: 64, borderRadius: "50%", background: "rgba(255,255,255,.15)", display: "grid", placeItems: "center" };
const sectionTitle: React.CSSProperties = { fontSize: 13, fontWeight: 700, letterSpacing: ".08em", color: "var(--color-warm-400)", margin: "0 0 10px" };
const infoRow: React.CSSProperties = { display: "flex", alignItems: "flex-start", gap: 12, padding: "12px 14px", background: "#fff", border: "1px solid var(--color-cream-300)", borderRadius: 10 };
const infoLabel: React.CSSProperties = { display: "block", fontSize: 11, color: "var(--color-warm-400)", letterSpacing: ".04em" };
const infoValue: React.CSSProperties = { margin: "2px 0 0", fontSize: 14, fontWeight: 600 };
const formCard: React.CSSProperties = { padding: 18, background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 12 };
const lab: React.CSSProperties = { fontSize: 13, fontWeight: 600, color: "var(--color-warm-700)" };
const input: React.CSSProperties = { minHeight: 46, width: "100%", border: "1.5px solid var(--color-cream-400)", borderRadius: 10, padding: "0 14px", background: "#fff", font: "inherit" };
const errBox: React.CSSProperties = { margin: "12px 0 0", padding: 10, borderRadius: 8, background: "#ffe4e6", color: "var(--color-danger-600)", fontSize: 13 };
const successBox: React.CSSProperties = { margin: "12px 0 0", padding: 10, borderRadius: 8, background: "var(--color-emerald-50)", color: "var(--color-emerald-900)", fontSize: 13 };
const primary: React.CSSProperties = { width: "100%", minHeight: 48, marginTop: 16, border: 0, borderRadius: 10, background: "var(--color-gold-500)", color: "#fff", fontWeight: 700 };
const signOutBtn: React.CSSProperties = { width: "100%", minHeight: 48, marginTop: 20, border: "1px solid var(--color-danger-600)", borderRadius: 10, background: "transparent", color: "var(--color-danger-600)", fontWeight: 700, display: "inline-flex", alignItems: "center", justifyContent: "center", gap: 8 };
const kycBadge: React.CSSProperties = { color: "white", fontWeight: 700, fontSize: 12, borderRadius: 999, padding: "6px 14px" };
const uploadLabel: React.CSSProperties = { display: "inline-flex", alignItems: "center", gap: 6, minHeight: 44, padding: "0 14px", background: "var(--color-emerald-900)", color: "white", borderRadius: 10, fontSize: 13, fontWeight: 600, cursor: "pointer" };
const cameraLabel: React.CSSProperties = { display: "inline-flex", alignItems: "center", gap: 6, minHeight: 44, padding: "0 14px", background: "var(--color-gold-500)", color: "#fff", borderRadius: 10, fontSize: 13, fontWeight: 600, cursor: "pointer" };
