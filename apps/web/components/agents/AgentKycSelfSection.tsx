"use client";
import { useEffect, useState } from "react";
import { Timestamp } from "@bufbuild/protobuf";
import { Agent, AgentDocument } from "@hajj-saas/proto-gen/hajj/v1/agent_pb";
import { agentClient } from "@/lib/rpc";
import { getBearerToken } from "@/lib/transport";
import CameraCaptureButton from "@/components/shared/CameraCaptureButton";

const KYC_STATUSES: Record<string, { label: string; color: string }> = {
  UNVERIFIED: { label: "Belum Diisi", color: "var(--color-warm-400)" },
  PENDING_REVIEW: { label: "Menunggu Verifikasi", color: "var(--color-gold-800)" },
  VERIFIED: { label: "Terverifikasi", color: "var(--color-emerald-900)" },
  REJECTED: { label: "Ditolak", color: "var(--color-danger-600)" },
};

const DOC_TYPES = [
  { value: "KTP", label: "KTP" },
  { value: "PASSPORT", label: "Paspor" },
  { value: "SELFIE", label: "Foto Selfie" },
  { value: "NPWP", label: "NPWP" },
  { value: "BANK_BOOK", label: "Buku Tabungan" },
];

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "";

function toDateInput(ts?: Timestamp): string {
  if (!ts) return "";
  return ts.toDate().toISOString().slice(0, 10);
}
function fromDateInput(value: string): Timestamp | undefined {
  if (!value) return undefined;
  return Timestamp.fromDate(new Date(`${value}T00:00:00Z`));
}

/** Self-service KYC form shared by the Muttawwif and Agent portals — both
 * resolve to the same `agents` row server-side (EnsureAgentForLeader), so
 * the same SubmitMyAgentKyc/GetMyAgentKyc/self-upload endpoints work for
 * either caller. */
export default function AgentKycSelfSection() {
  const [agent, setAgent] = useState<Agent>();
  const [nik, setNik] = useState("");
  const [npwp, setNpwp] = useState("");
  const [address, setAddress] = useState("");
  const [dateOfBirth, setDateOfBirth] = useState("");
  const [passportNumber, setPassportNumber] = useState("");
  const [passportExpiryDate, setPassportExpiryDate] = useState("");
  const [bankName, setBankName] = useState("");
  const [bankAccountNumber, setBankAccountNumber] = useState("");
  const [bankAccountHolder, setBankAccountHolder] = useState("");
  const [saving, setSaving] = useState(false);
  const [docs, setDocs] = useState<AgentDocument[]>([]);
  const [docType, setDocType] = useState("KTP");
  const [uploading, setUploading] = useState(false);
  const [notice, setNotice] = useState("");
  const [error, setError] = useState("");

  const load = () => {
    agentClient.getMyAgentKyc({}).then((a) => {
      setAgent(a);
      setNik(a.nik); setNpwp(a.npwp); setAddress(a.address);
      setDateOfBirth(toDateInput(a.dateOfBirth)); setPassportNumber(a.passportNumber); setPassportExpiryDate(toDateInput(a.passportExpiryDate));
      setBankName(a.bankName); setBankAccountNumber(a.bankAccountNumber); setBankAccountHolder(a.bankAccountHolder);
    }).catch(() => {});
    agentClient.listMyAgentDocuments({}).then((r) => setDocs(r.documents)).catch(() => {});
  };
  useEffect(load, []);

  async function submit() {
    setSaving(true);
    setNotice("");
    setError("");
    try {
      const result = await agentClient.submitMyAgentKyc({
        nik, npwp, address, dateOfBirth: fromDateInput(dateOfBirth), passportNumber,
        passportExpiryDate: fromDateInput(passportExpiryDate), bankName, bankAccountNumber, bankAccountHolder,
      });
      setAgent(result);
      setNotice("Data KYC tersimpan — menunggu verifikasi admin.");
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "Gagal menyimpan KYC.");
    } finally {
      setSaving(false);
    }
  }

  async function upload(file: File) {
    setUploading(true);
    setError("");
    try {
      const token = await getBearerToken();
      if (!token) throw new Error("Sesi tidak ditemukan. Silakan login ulang.");
      const form = new FormData();
      form.append("doc_type", docType);
      form.append("file", file);
      const response = await fetch(`${API_URL}/upload/agent-document/self`, { method: "POST", headers: { Authorization: `Bearer ${token}` }, body: form });
      if (!response.ok) throw new Error((await response.text()) || "Upload gagal.");
      agentClient.listMyAgentDocuments({}).then((r) => setDocs(r.documents)).catch(() => {});
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "Upload gagal.");
    } finally {
      setUploading(false);
    }
  }

  function uploadFromInput(event: React.ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0];
    event.target.value = "";
    if (file) void upload(file);
  }

  if (!agent) return null;
  const statusMeta = KYC_STATUSES[agent.kycStatus] ?? KYC_STATUSES.UNVERIFIED!;

  return (
    <section style={{ marginTop: 20 }}>
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 10 }}>
        <h2 style={sectionTitle}>KYC (IDENTITAS)</h2>
        <span style={{ ...badge, background: statusMeta.color }}>{statusMeta.label}</span>
      </div>
      <div style={formCard}>
        {agent.kycStatus === "REJECTED" && agent.kycRejectionReason && <p style={warn}>Alasan penolakan: {agent.kycRejectionReason}</p>}
        <label style={{ display: "grid", gap: 6 }}><span style={lab}>NIK</span><input value={nik} onChange={(e) => setNik(e.target.value)} maxLength={32} style={input} /></label>
        <label style={{ display: "grid", gap: 6, marginTop: 12 }}><span style={lab}>NPWP (opsional)</span><input value={npwp} onChange={(e) => setNpwp(e.target.value)} maxLength={32} style={input} /></label>
        <label style={{ display: "grid", gap: 6, marginTop: 12 }}><span style={lab}>Alamat sesuai KTP</span><textarea value={address} onChange={(e) => setAddress(e.target.value)} rows={2} style={{ ...input, minHeight: 60, resize: "vertical" }} /></label>
        <label style={{ display: "grid", gap: 6, marginTop: 12 }}><span style={lab}>Tanggal lahir</span><input type="date" value={dateOfBirth} onChange={(e) => setDateOfBirth(e.target.value)} style={input} /></label>
        <label style={{ display: "grid", gap: 6, marginTop: 12 }}><span style={lab}>Nomor paspor (opsional)</span><input value={passportNumber} onChange={(e) => setPassportNumber(e.target.value)} style={input} /></label>
        <label style={{ display: "grid", gap: 6, marginTop: 12 }}><span style={lab}>Masa berlaku paspor</span><input type="date" value={passportExpiryDate} onChange={(e) => setPassportExpiryDate(e.target.value)} style={input} /></label>
        <p style={{ ...sectionTitle, marginTop: 16 }}>REKENING (UNTUK PENCAIRAN KOMISI)</p>
        <label style={{ display: "grid", gap: 6 }}><span style={lab}>Nama bank</span><input value={bankName} onChange={(e) => setBankName(e.target.value)} style={input} /></label>
        <label style={{ display: "grid", gap: 6, marginTop: 12 }}><span style={lab}>Nomor rekening</span><input value={bankAccountNumber} onChange={(e) => setBankAccountNumber(e.target.value)} style={input} /></label>
        <label style={{ display: "grid", gap: 6, marginTop: 12 }}><span style={lab}>Nama pemilik rekening</span><input value={bankAccountHolder} onChange={(e) => setBankAccountHolder(e.target.value)} style={input} /></label>
        {error && <p style={errBox}>{error}</p>}
        {notice && <p style={successBox}>{notice}</p>}
        <button onClick={() => void submit()} disabled={saving} style={primary}>{saving ? "Menyimpan..." : "Simpan KYC"}</button>
      </div>

      <div style={{ ...formCard, marginTop: 12 }}>
        <p style={{ margin: "0 0 10px", fontWeight: 700, fontSize: 14 }}>Dokumen</p>
        <div style={{ display: "flex", gap: 8, flexWrap: "wrap", alignItems: "center" }}>
          <select value={docType} onChange={(e) => setDocType(e.target.value)} style={input}>
            {DOC_TYPES.map((d) => <option key={d.value} value={d.value}>{d.label}</option>)}
          </select>
          <CameraCaptureButton label={uploading ? "..." : "Ambil Foto"} onCapture={(file) => void upload(file)} disabled={uploading} style={cameraLabel} />
          <label style={uploadLabel}>
            {uploading ? "Mengunggah..." : "Upload File"}
            <input type="file" accept=".pdf,.jpg,.jpeg,.png" onChange={uploadFromInput} style={{ display: "none" }} disabled={uploading} />
          </label>
        </div>
        <div style={{ display: "grid", gap: 8, marginTop: 10 }}>
          {!docs.length && <p style={{ margin: 0, fontSize: 13, color: "var(--color-warm-400)" }}>Belum ada dokumen diunggah.</p>}
          {docs.map((doc) => (
            <div key={doc.id} style={docRow}>
              <div style={{ flex: 1, minWidth: 0 }}>
                <p style={{ margin: 0, fontSize: 13, fontWeight: 600, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{doc.fileName}</p>
                <p style={{ margin: 0, fontSize: 11, color: "var(--color-warm-400)" }}>{DOC_TYPES.find((d) => d.value === doc.docType)?.label ?? doc.docType}</p>
              </div>
              <a href={`${API_URL}${doc.fileUrl}`} target="_blank" rel="noreferrer" style={{ fontSize: 12, color: "var(--color-emerald-700)", fontWeight: 600 }}>Buka</a>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}

const sectionTitle: React.CSSProperties = { fontSize: 13, fontWeight: 700, letterSpacing: ".08em", color: "var(--color-warm-400)", margin: 0 };
const formCard: React.CSSProperties = { padding: 18, background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 12 };
const lab: React.CSSProperties = { fontSize: 13, fontWeight: 600, color: "var(--color-warm-700)" };
const input: React.CSSProperties = { minHeight: 46, width: "100%", border: "1.5px solid var(--color-cream-400)", borderRadius: 10, padding: "0 14px", background: "#fff", font: "inherit" };
const errBox: React.CSSProperties = { margin: "12px 0 0", padding: 10, borderRadius: 8, background: "#fdf0f0", color: "var(--color-danger-600)", fontSize: 13 };
const successBox: React.CSSProperties = { margin: "12px 0 0", padding: 10, borderRadius: 8, background: "var(--color-emerald-50)", color: "var(--color-emerald-900)", fontSize: 13 };
const warn: React.CSSProperties = { margin: "0 0 12px", fontSize: 13, color: "var(--color-danger-600)" };
const primary: React.CSSProperties = { width: "100%", minHeight: 48, marginTop: 16, border: 0, borderRadius: 10, background: "var(--color-gold-500)", color: "var(--color-warm-900)", fontWeight: 700 };
const badge: React.CSSProperties = { color: "white", fontWeight: 700, fontSize: 12, borderRadius: 999, padding: "6px 14px" };
const uploadLabel: React.CSSProperties = { display: "inline-flex", alignItems: "center", gap: 6, minHeight: 44, padding: "0 14px", background: "var(--color-emerald-900)", color: "white", borderRadius: 10, fontSize: 13, fontWeight: 600, cursor: "pointer" };
const cameraLabel: React.CSSProperties = { display: "inline-flex", alignItems: "center", gap: 6, minHeight: 44, padding: "0 14px", background: "var(--color-gold-500)", color: "var(--color-warm-900)", borderRadius: 10, fontSize: 13, fontWeight: 600, cursor: "pointer" };
const docRow: React.CSSProperties = { display: "flex", alignItems: "center", gap: 10, padding: "10px 14px", background: "var(--color-cream-100)", border: "1px solid var(--color-cream-400)", borderRadius: 8 };
