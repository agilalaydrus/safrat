"use client";
import { FormEvent, useEffect, useState } from "react";
import { IconX } from "@tabler/icons-react";
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
  { value: "OTHER", label: "Lainnya" },
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

type Props = { open: boolean; agent?: Agent; onClose: () => void; onUpdated: (agent: Agent) => void };

export default function AgentKycDialog({ open, agent, onClose, onUpdated }: Props) {
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
  const [verifying, setVerifying] = useState(false);
  const [docs, setDocs] = useState<AgentDocument[]>([]);
  const [docType, setDocType] = useState("KTP");
  const [uploading, setUploading] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    if (!open || !agent) return;
    setNik(agent.nik);
    setNpwp(agent.npwp);
    setAddress(agent.address);
    setDateOfBirth(toDateInput(agent.dateOfBirth));
    setPassportNumber(agent.passportNumber);
    setPassportExpiryDate(toDateInput(agent.passportExpiryDate));
    setBankName(agent.bankName);
    setBankAccountNumber(agent.bankAccountNumber);
    setBankAccountHolder(agent.bankAccountHolder);
    setError("");
    agentClient.listAgentDocuments({ agentId: agent.id }).then((r) => setDocs(r.documents)).catch(() => {});
  }, [open, agent]);

  if (!open || !agent) return null;

  async function submit(e: FormEvent) {
    e.preventDefault();
    if (!agent) return;
    setSaving(true);
    setError("");
    try {
      const result = await agentClient.updateAgentKyc({
        agentId: agent.id, nik, npwp, address, dateOfBirth: fromDateInput(dateOfBirth), passportNumber,
        passportExpiryDate: fromDateInput(passportExpiryDate), bankName, bankAccountNumber, bankAccountHolder,
      });
      onUpdated(result);
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "Gagal menyimpan KYC.");
    } finally {
      setSaving(false);
    }
  }

  async function verify(approve: boolean) {
    if (!agent) return;
    const rejectionReason = approve ? "" : window.prompt("Alasan penolakan KYC?") ?? "";
    if (!approve && !rejectionReason.trim()) return;
    setVerifying(true);
    try {
      const result = await agentClient.verifyAgentKyc({ agentId: agent.id, approve, rejectionReason });
      onUpdated(result);
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "Gagal memverifikasi KYC.");
    } finally {
      setVerifying(false);
    }
  }

  async function upload(file: File) {
    if (!agent) return;
    setUploading(true);
    setError("");
    try {
      const token = await getBearerToken();
      if (!token) throw new Error("Sesi tidak ditemukan. Silakan login ulang.");
      const form = new FormData();
      form.append("agent_id", agent.id);
      form.append("doc_type", docType);
      form.append("file", file);
      const response = await fetch(`${API_URL}/upload/agent-document`, { method: "POST", headers: { Authorization: `Bearer ${token}` }, body: form });
      if (!response.ok) throw new Error((await response.text()) || "Upload gagal.");
      agentClient.listAgentDocuments({ agentId: agent.id }).then((r) => setDocs(r.documents)).catch(() => {});
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

  const statusMeta = KYC_STATUSES[agent.kycStatus] ?? KYC_STATUSES.UNVERIFIED!;

  return (
    <div style={o} onClick={onClose}>
      <form style={s} onClick={(e) => e.stopPropagation()} onSubmit={submit}>
        <div style={h}>
          <div>
            <p style={ey}>KYC</p>
            <h2 style={{ margin: 0, fontSize: 22 }}>{agent.name}</h2>
          </div>
          <button type="button" onClick={onClose} style={x}><IconX size={18} /></button>
        </div>
        <div style={b}>
          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 16 }}>
            <span style={{ ...badge, background: statusMeta.color }}>{statusMeta.label}</span>
            {agent.kycSource === "SELF" && <span style={{ fontSize: 12, color: "var(--color-warm-400)" }}>Diisi sendiri</span>}
          </div>
          {agent.kycStatus === "REJECTED" && agent.kycRejectionReason && <p style={warn}>Alasan penolakan: {agent.kycRejectionReason}</p>}
          {error && <p style={err}>{error}</p>}

          <div style={{ display: "grid", gap: 12 }}>
            <label style={{ display: "grid", gap: 6 }}><span style={lab}>NIK</span><input value={nik} onChange={(e) => setNik(e.target.value)} maxLength={32} style={i} /></label>
            <label style={{ display: "grid", gap: 6 }}><span style={lab}>NPWP</span><input value={npwp} onChange={(e) => setNpwp(e.target.value)} maxLength={32} style={i} /></label>
            <label style={{ display: "grid", gap: 6 }}><span style={lab}>Alamat sesuai KTP</span><textarea value={address} onChange={(e) => setAddress(e.target.value)} rows={2} style={{ ...i, minHeight: 60, resize: "vertical" }} /></label>
            <label style={{ display: "grid", gap: 6 }}><span style={lab}>Tanggal lahir</span><input type="date" value={dateOfBirth} onChange={(e) => setDateOfBirth(e.target.value)} style={i} /></label>
            <label style={{ display: "grid", gap: 6 }}><span style={lab}>Nomor paspor (opsional)</span><input value={passportNumber} onChange={(e) => setPassportNumber(e.target.value)} style={i} /></label>
            <label style={{ display: "grid", gap: 6 }}><span style={lab}>Masa berlaku paspor</span><input type="date" value={passportExpiryDate} onChange={(e) => setPassportExpiryDate(e.target.value)} style={i} /></label>
            <p style={sec}>REKENING (UNTUK PENCAIRAN KOMISI)</p>
            <label style={{ display: "grid", gap: 6 }}><span style={lab}>Nama bank</span><input value={bankName} onChange={(e) => setBankName(e.target.value)} style={i} /></label>
            <label style={{ display: "grid", gap: 6 }}><span style={lab}>Nomor rekening</span><input value={bankAccountNumber} onChange={(e) => setBankAccountNumber(e.target.value)} style={i} /></label>
            <label style={{ display: "grid", gap: 6 }}><span style={lab}>Nama pemilik rekening</span><input value={bankAccountHolder} onChange={(e) => setBankAccountHolder(e.target.value)} style={i} /></label>
            <button type="submit" disabled={saving} style={primary}>{saving ? "Menyimpan..." : "Simpan KYC"}</button>
            {agent.kycStatus === "PENDING_REVIEW" && (
              <div style={{ display: "flex", gap: 8 }}>
                <button type="button" disabled={verifying} onClick={() => verify(true)} style={{ ...primary, flex: 1 }}>Verifikasi</button>
                <button type="button" disabled={verifying} onClick={() => verify(false)} style={{ ...primary, flex: 1, background: "var(--color-danger-600)", color: "#fff" }}>Tolak</button>
              </div>
            )}

            <p style={sec}>DOKUMEN</p>
            <div style={{ display: "flex", gap: 8, flexWrap: "wrap", alignItems: "center" }}>
              <select value={docType} onChange={(e) => setDocType(e.target.value)} style={i}>
                {DOC_TYPES.map((d) => <option key={d.value} value={d.value}>{d.label}</option>)}
              </select>
              <CameraCaptureButton label={uploading ? "..." : "Ambil Foto"} onCapture={(file) => void upload(file)} disabled={uploading} style={cameraLabel} />
              <label style={uploadLabel}>
                {uploading ? "Mengunggah..." : "Upload File"}
                <input type="file" accept=".pdf,.jpg,.jpeg,.png" onChange={uploadFromInput} style={{ display: "none" }} disabled={uploading} />
              </label>
            </div>
            <div style={{ display: "grid", gap: 8 }}>
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
        </div>
      </form>
    </div>
  );
}

const o: React.CSSProperties = { position: "fixed", inset: 0, zIndex: 30, display: "flex", justifyContent: "flex-end", background: "rgba(15,23,42,.48)", backdropFilter: "blur(2px)" };
const s: React.CSSProperties = { width: "min(560px,100%)", height: "100vh", display: "flex", flexDirection: "column", background: "#fff", borderRadius: "16px 0 0 16px", overflow: "hidden" };
const h: React.CSSProperties = { display: "flex", justifyContent: "space-between", alignItems: "center", padding: "20px 24px 16px", borderBottom: "1px solid var(--color-cream-300)" };
const b: React.CSSProperties = { flex: 1, overflowY: "auto", padding: 24 };
const x: React.CSSProperties = { width: 40, height: 40, borderRadius: "50%", border: "1px solid var(--color-cream-400)", background: "transparent", color: "var(--color-warm-400)" };
const ey: React.CSSProperties = { margin: 0, color: "var(--color-gold-800)", fontSize: 11, fontWeight: 700, letterSpacing: ".1em" };
const sec: React.CSSProperties = { margin: "8px 0 0", fontSize: 11, fontWeight: 700, letterSpacing: ".1em", color: "var(--color-warm-400)", paddingBottom: 8, borderBottom: "1px solid var(--color-cream-300)" };
const i: React.CSSProperties = { minHeight: 44, width: "100%", border: "1.5px solid var(--color-cream-400)", borderRadius: 10, padding: "0 14px", background: "#fff", font: "inherit" };
const lab: React.CSSProperties = { fontSize: 13, fontWeight: 600, color: "var(--color-warm-700)" };
const primary: React.CSSProperties = { minHeight: 44, border: 0, borderRadius: 10, background: "var(--color-emerald-900)", color: "#fff", fontWeight: 700 };
const err: React.CSSProperties = { margin: "0 0 12px", padding: 10, borderRadius: 8, background: "#ffe4e6", color: "var(--color-danger-600)" };
const warn: React.CSSProperties = { margin: "0 0 12px", fontSize: 13, color: "var(--color-danger-600)" };
const badge: React.CSSProperties = { color: "white", fontWeight: 700, fontSize: 12, borderRadius: 999, padding: "6px 14px" };
const uploadLabel: React.CSSProperties = { display: "inline-flex", alignItems: "center", gap: 6, minHeight: 44, padding: "0 14px", background: "var(--color-emerald-900)", color: "white", borderRadius: 10, fontSize: 13, fontWeight: 600, cursor: "pointer" };
const cameraLabel: React.CSSProperties = { display: "inline-flex", alignItems: "center", gap: 6, minHeight: 44, padding: "0 14px", background: "var(--color-gold-500)", color: "#fff", borderRadius: 10, fontSize: 13, fontWeight: 600, cursor: "pointer" };
const docRow: React.CSSProperties = { display: "flex", alignItems: "center", gap: 10, padding: "10px 14px", background: "var(--color-cream-100)", border: "1px solid var(--color-cream-400)", borderRadius: 8 };
