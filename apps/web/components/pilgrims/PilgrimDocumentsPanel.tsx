"use client";

import { useEffect, useState } from "react";
import { IconFile, IconTrash, IconUpload } from "@tabler/icons-react";
import { Timestamp } from "@bufbuild/protobuf";
import { Pilgrim, PilgrimDocument } from "@hajj-saas/proto-gen/hajj/v1/pilgrim_pb";
import { pilgrimClient } from "@/lib/rpc";
import { getBearerToken } from "@/lib/transport";
import PilgrimDocumentChecklist from "./PilgrimDocumentChecklist";
import CameraCaptureButton from "@/components/shared/CameraCaptureButton";

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "";

const PAYMENT_STATUSES = [
  { value: "UNPAID", label: "Belum Bayar", color: "var(--color-danger-600)" },
  { value: "DP", label: "DP", color: "var(--color-gold-800)" },
  { value: "PAID", label: "Lunas", color: "var(--color-emerald-900)" },
];

const KYC_STATUSES = [
  { value: "UNVERIFIED", label: "Belum Diisi", color: "var(--color-warm-400)" },
  { value: "PENDING_REVIEW", label: "Menunggu Verifikasi", color: "var(--color-gold-800)" },
  { value: "VERIFIED", label: "Terverifikasi", color: "var(--color-emerald-900)" },
  { value: "REJECTED", label: "Ditolak", color: "var(--color-danger-600)" },
];

const MARITAL_STATUSES = [
  { value: "", label: "Belum diisi" },
  { value: "SINGLE", label: "Belum Menikah" },
  { value: "MARRIED", label: "Menikah" },
  { value: "DIVORCED", label: "Cerai" },
  { value: "WIDOWED", label: "Janda/Duda" },
];

// Providers commonly used by Indonesian PPIU for umrah/hajj travel
// insurance — standard coverage across these is: santunan meninggal dunia
// akibat kecelakaan, cacat tetap, biaya pengobatan selama di Arab Saudi,
// evakuasi medis & repatriasi jenazah, serta kehilangan/keterlambatan
// bagasi. Offered as suggestions, not a locked enum — an operator may use
// a provider not in this list.
const INSURANCE_PROVIDERS = ["Allianz", "Sompo Insurance", "Jasindo", "AXA Mandiri", "Chubb", "Zurich Asuransi Indonesia", "Asuransi Astra", "Asuransi Sinarmas"];

function toDateInput(ts?: Timestamp): string {
  if (!ts) return "";
  return ts.toDate().toISOString().slice(0, 10);
}

function fromDateInput(value: string): Timestamp | undefined {
  if (!value) return undefined;
  return Timestamp.fromDate(new Date(`${value}T00:00:00Z`));
}

export default function PilgrimDocumentsPanel({ pilgrim, onUpdated }: { pilgrim: Pilgrim; onUpdated: (pilgrim: Pilgrim) => void }) {
  const [paymentStatus, setPaymentStatus] = useState(pilgrim.paymentStatus || "UNPAID");
  const [paymentNotes, setPaymentNotes] = useState(pilgrim.paymentNotes);
  const [savingPayment, setSavingPayment] = useState(false);
  const [receipts, setReceipts] = useState<PilgrimDocument[]>([]);
  const [uploadingReceipt, setUploadingReceipt] = useState(false);
  const [receiptError, setReceiptError] = useState("");

  const refreshReceipts = () => pilgrimClient.listPilgrimDocuments({ pilgrimId: pilgrim.id }).then((r) => setReceipts(r.documents.filter((d) => d.docType === "PAYMENT_RECEIPT"))).catch(() => setReceiptError("Gagal memuat bukti pembayaran."));
  useEffect(() => { void refreshReceipts(); }, [pilgrim.id]);

  const uploadReceipt = async (file: File) => {
    setUploadingReceipt(true);
    setReceiptError("");
    try {
      const token = await getBearerToken();
      if (!token) throw new Error("Sesi tidak ditemukan. Silakan login ulang.");
      const form = new FormData();
      form.append("pilgrim_id", pilgrim.id);
      form.append("doc_type", "PAYMENT_RECEIPT");
      form.append("file", file);
      const response = await fetch(`${API_URL}/upload/document`, { method: "POST", headers: { Authorization: `Bearer ${token}` }, body: form });
      if (!response.ok) throw new Error((await response.text()) || "Upload gagal.");
      refreshReceipts();
    } catch (caught) {
      setReceiptError(caught instanceof Error ? caught.message : "Upload gagal.");
    } finally {
      setUploadingReceipt(false);
    }
  };
  const uploadReceiptFromInput = (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    event.target.value = "";
    if (file) void uploadReceipt(file);
  };
  const removeReceipt = async (id: string) => {
    try {
      await pilgrimClient.deletePilgrimDocument({ id });
      refreshReceipts();
    } catch (caught) {
      setReceiptError(caught instanceof Error ? caught.message : "Gagal menghapus bukti pembayaran.");
    }
  };

  const [emergencyContactName, setEmergencyContactName] = useState(pilgrim.emergencyContactName);
  const [emergencyContactPhone, setEmergencyContactPhone] = useState(pilgrim.emergencyContactPhone);
  const [savingContact, setSavingContact] = useState(false);

  const [notice, setNotice] = useState("");

  const [nik, setNik] = useState(pilgrim.nik);
  const [address, setAddress] = useState(pilgrim.address);
  const [placeOfBirth, setPlaceOfBirth] = useState(pilgrim.placeOfBirth);
  const [maritalStatus, setMaritalStatus] = useState(pilgrim.maritalStatus);
  const [occupation, setOccupation] = useState(pilgrim.occupation);
  const [fatherName, setFatherName] = useState(pilgrim.fatherName);
  const [savingKyc, setSavingKyc] = useState(false);
  const [verifyingKyc, setVerifyingKyc] = useState(false);

  const [insuranceProvider, setInsuranceProvider] = useState(pilgrim.insuranceProvider);
  const [insurancePolicyNo, setInsurancePolicyNo] = useState(pilgrim.insurancePolicyNo);
  const [insuranceClass, setInsuranceClass] = useState(pilgrim.insuranceClass || "STANDARD");
  const [insuranceStartDate, setInsuranceStartDate] = useState(toDateInput(pilgrim.insuranceStartDate));
  const [insuranceEndDate, setInsuranceEndDate] = useState(toDateInput(pilgrim.insuranceEndDate));
  const [insuranceBeneficiaryName, setInsuranceBeneficiaryName] = useState(pilgrim.insuranceBeneficiaryName);
  const [insuranceBeneficiaryRelation, setInsuranceBeneficiaryRelation] = useState(pilgrim.insuranceBeneficiaryRelation);
  const [bloodType, setBloodType] = useState(pilgrim.bloodType);
  const [chronicConditions, setChronicConditions] = useState(pilgrim.chronicConditions);
  const [currentMedications, setCurrentMedications] = useState(pilgrim.currentMedications);
  const [savingInsurance, setSavingInsurance] = useState(false);

  const savePayment = async () => {
    setSavingPayment(true);
    try {
      const result = await pilgrimClient.updatePilgrimPayment({ pilgrimId: pilgrim.id, paymentStatus, paymentNotes });
      onUpdated(result);
      setNotice("Status pembayaran diperbarui");
    } catch (error) {
      setNotice(`Gagal: ${error instanceof Error ? error.message : "tidak diketahui"}`);
    } finally {
      setSavingPayment(false);
    }
  };

  const saveContact = async () => {
    setSavingContact(true);
    try {
      const result = await pilgrimClient.updatePilgrimEmergencyContact({ pilgrimId: pilgrim.id, emergencyContactName, emergencyContactPhone });
      onUpdated(result);
      setNotice("Kontak darurat diperbarui");
    } catch (error) {
      setNotice(`Gagal: ${error instanceof Error ? error.message : "tidak diketahui"}`);
    } finally {
      setSavingContact(false);
    }
  };

  const saveInsurance = async () => {
    setSavingInsurance(true);
    try {
      const result = await pilgrimClient.updatePilgrimInsurance({
        pilgrimId: pilgrim.id, insuranceProvider, insurancePolicyNo, insuranceClass, bloodType, chronicConditions, currentMedications,
        insuranceStartDate: fromDateInput(insuranceStartDate), insuranceEndDate: fromDateInput(insuranceEndDate),
        insuranceBeneficiaryName, insuranceBeneficiaryRelation,
      });
      onUpdated(result);
      setNotice("Data asuransi & medis diperbarui");
    } catch (error) {
      setNotice(`Gagal: ${error instanceof Error ? error.message : "tidak diketahui"}`);
    } finally {
      setSavingInsurance(false);
    }
  };

  const saveKyc = async () => {
    setSavingKyc(true);
    try {
      const result = await pilgrimClient.updatePilgrimKyc({ pilgrimId: pilgrim.id, nik, address, placeOfBirth, maritalStatus, occupation, fatherName });
      onUpdated(result);
      setNotice("Data KYC diperbarui — menunggu verifikasi admin");
    } catch (error) {
      setNotice(`Gagal: ${error instanceof Error ? error.message : "tidak diketahui"}`);
    } finally {
      setSavingKyc(false);
    }
  };

  const verifyKyc = async (approve: boolean) => {
    const rejectionReason = approve ? "" : window.prompt("Alasan penolakan KYC?") ?? "";
    if (!approve && !rejectionReason.trim()) return;
    setVerifyingKyc(true);
    try {
      const result = await pilgrimClient.verifyPilgrimKyc({ pilgrimId: pilgrim.id, approve, rejectionReason });
      onUpdated(result);
      setNotice(approve ? "KYC diverifikasi" : "KYC ditolak");
    } catch (error) {
      setNotice(`Gagal: ${error instanceof Error ? error.message : "tidak diketahui"}`);
    } finally {
      setVerifyingKyc(false);
    }
  };

  const statusMeta = PAYMENT_STATUSES.find((s) => s.value === pilgrim.paymentStatus) ?? PAYMENT_STATUSES[0]!;
  const kycMeta = KYC_STATUSES.find((s) => s.value === pilgrim.kycStatus) ?? KYC_STATUSES[0]!;

  return <div style={grid}>
    <section style={card}>
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", gap: 8, flexWrap: "wrap" }}>
        <h2 style={{ margin: 0 }}>Pembayaran</h2>
        <span style={{ ...badge, background: statusMeta.color }}>{statusMeta.label}</span>
      </div>
      <label style={field}>Status pembayaran
        <select value={paymentStatus} onChange={(e) => setPaymentStatus(e.target.value)} style={input}>
          {PAYMENT_STATUSES.map((s) => <option key={s.value} value={s.value}>{s.label}</option>)}
        </select>
      </label>
      <label style={field}>Catatan pembayaran
        <textarea value={paymentNotes} onChange={(e) => setPaymentNotes(e.target.value)} rows={3} style={{ ...input, minHeight: 80, resize: "vertical" }} />
      </label>
      <button disabled={savingPayment} onClick={savePayment} style={emerald}>{savingPayment ? "Menyimpan..." : "Simpan Pembayaran"}</button>

      <div style={{ marginTop: 4 }}>
        <p style={{ margin: "0 0 8px", fontWeight: 700, fontSize: 14 }}>Bukti Pembayaran</p>
        <p style={{ margin: "0 0 8px", fontSize: 12, color: "var(--color-warm-400)" }}>Foto/scan bukti transfer — boleh lebih dari satu jika dibayar bertahap (DP lalu pelunasan).</p>
        {receiptError && <p style={warning}>{receiptError}</p>}
        <div style={{ display: "flex", gap: 6, flexWrap: "wrap" }}>
          <CameraCaptureButton label={uploadingReceipt ? "..." : "Ambil Foto"} onCapture={(file) => void uploadReceipt(file)} disabled={uploadingReceipt} style={cameraLabel} />
          <label style={uploadLabel}>
            <IconUpload size={14} />{uploadingReceipt ? "..." : "Upload"}
            <input type="file" accept=".pdf,.jpg,.jpeg,.png" onChange={uploadReceiptFromInput} style={{ display: "none" }} disabled={uploadingReceipt} />
          </label>
        </div>
        {!receipts.length && <p style={{ margin: "8px 0 0", fontSize: 12, color: "var(--color-warm-400)" }}>Belum ada bukti pembayaran diunggah.</p>}
        <div style={{ display: "grid", gap: 6, marginTop: 8 }}>
          {receipts.map((doc) => (
            <div key={doc.id} style={fileRow}>
              <IconFile size={15} style={{ color: "var(--color-emerald-700)", flexShrink: 0 }} />
              <span style={{ flex: 1, minWidth: 0, fontSize: 12, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{doc.fileName}</span>
              <a href={`${API_URL}${doc.fileUrl}`} target="_blank" rel="noreferrer" style={{ fontSize: 11, color: "var(--color-emerald-700)", fontWeight: 600 }}>Buka</a>
              <button onClick={() => removeReceipt(doc.id)} aria-label={`Hapus ${doc.fileName}`} style={deleteButton}><IconTrash size={13} /></button>
            </div>
          ))}
        </div>
      </div>
    </section>

    <PilgrimDocumentChecklist pilgrim={pilgrim} onUpdated={onUpdated} />

    <section style={card}>
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", gap: 8, flexWrap: "wrap" }}>
        <h2 style={{ margin: 0 }}>KYC (Kelengkapan Data Jamaah)</h2>
        <span style={{ ...badge, background: kycMeta.color }}>{kycMeta.label}</span>
      </div>
      <p style={{ margin: 0, fontSize: 12, color: "var(--color-warm-400)" }}>Data identitas lengkap yang diminta pada manifes/visa Arab Saudi — bukan sekadar NIK dan alamat.</p>
      {pilgrim.kycSource === "SELF" && <p style={{ margin: 0, fontSize: 12, color: "var(--color-warm-400)" }}>Diisi sendiri oleh jamaah</p>}
      {pilgrim.kycStatus === "REJECTED" && pilgrim.kycRejectionReason && <p style={warning}>Alasan penolakan: {pilgrim.kycRejectionReason}</p>}
      <label style={field}>NIK
        <input value={nik} onChange={(e) => setNik(e.target.value)} maxLength={32} style={input} />
      </label>
      <label style={field}>Alamat sesuai KTP
        <textarea value={address} onChange={(e) => setAddress(e.target.value)} rows={2} style={{ ...input, minHeight: 60, resize: "vertical" }} />
      </label>
      <label style={field}>Tempat lahir
        <input value={placeOfBirth} onChange={(e) => setPlaceOfBirth(e.target.value)} style={input} />
      </label>
      <label style={field}>Status pernikahan
        <select value={maritalStatus} onChange={(e) => setMaritalStatus(e.target.value)} style={input}>
          {MARITAL_STATUSES.map((s) => <option key={s.value} value={s.value}>{s.label}</option>)}
        </select>
      </label>
      <label style={field}>Pekerjaan
        <input value={occupation} onChange={(e) => setOccupation(e.target.value)} style={input} />
      </label>
      <label style={field}>Nama ayah kandung
        <input value={fatherName} onChange={(e) => setFatherName(e.target.value)} style={input} />
      </label>
      <button disabled={savingKyc} onClick={saveKyc} style={emerald}>{savingKyc ? "Menyimpan..." : "Simpan KYC"}</button>
      {pilgrim.kycStatus === "PENDING_REVIEW" && <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
        <button disabled={verifyingKyc} onClick={() => verifyKyc(true)} style={emerald}>Verifikasi</button>
        <button disabled={verifyingKyc} onClick={() => verifyKyc(false)} style={danger}>Tolak</button>
      </div>}
    </section>

    <section style={card}>
      <h2 style={{ margin: 0 }}>Kontak Darurat</h2>
      <label style={field}>Nama
        <input value={emergencyContactName} onChange={(e) => setEmergencyContactName(e.target.value)} style={input} />
      </label>
      <label style={field}>Telepon
        <input value={emergencyContactPhone} onChange={(e) => setEmergencyContactPhone(e.target.value)} style={input} />
      </label>
      <button disabled={savingContact} onClick={saveContact} style={emerald}>{savingContact ? "Menyimpan..." : "Simpan Kontak Darurat"}</button>
    </section>

    <section style={card}>
      <h2 style={{ margin: 0 }}>Asuransi Perjalanan &amp; Medis</h2>
      <p style={{ margin: 0, fontSize: 12, color: "var(--color-warm-400)" }}>
        Polis umrah/haji standar (Allianz, Sompo, Jasindo, AXA Mandiri, Chubb, dll) umumnya mencakup: santunan meninggal dunia akibat kecelakaan,
        cacat tetap, biaya pengobatan selama di Arab Saudi, evakuasi medis &amp; repatriasi jenazah, serta kehilangan/keterlambatan bagasi.
      </p>
      <label style={field}>Penyedia asuransi
        <input value={insuranceProvider} onChange={(e) => setInsuranceProvider(e.target.value)} list="insurance-providers" placeholder="Pilih atau ketik nama penyedia" style={input} />
        <datalist id="insurance-providers">{INSURANCE_PROVIDERS.map((p) => <option key={p} value={p} />)}</datalist>
      </label>
      <label style={field}>Nomor polis / sertifikat
        <input value={insurancePolicyNo} onChange={(e) => setInsurancePolicyNo(e.target.value)} style={input} />
      </label>
      <label style={field}>Kelas asuransi
        <select value={insuranceClass} onChange={(e) => setInsuranceClass(e.target.value)} style={input}>
          <option value="STANDARD">Standard</option>
          <option value="PLUS">Plus</option>
          <option value="PREMIUM">Premium</option>
        </select>
      </label>
      <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 10 }}>
        <label style={field}>Mulai berlaku
          <input type="date" value={insuranceStartDate} onChange={(e) => setInsuranceStartDate(e.target.value)} style={input} />
        </label>
        <label style={field}>Berakhir
          <input type="date" value={insuranceEndDate} onChange={(e) => setInsuranceEndDate(e.target.value)} style={input} />
        </label>
      </div>
      <p style={{ margin: 0, fontSize: 11, color: "var(--color-warm-400)" }}>Pastikan masa berlaku polis mencakup seluruh durasi perjalanan — klaim di luar periode ini akan ditolak penyedia.</p>
      <label style={field}>Nama ahli waris (penerima santunan)
        <input value={insuranceBeneficiaryName} onChange={(e) => setInsuranceBeneficiaryName(e.target.value)} style={input} />
      </label>
      <label style={field}>Hubungan dengan jamaah
        <input value={insuranceBeneficiaryRelation} onChange={(e) => setInsuranceBeneficiaryRelation(e.target.value)} placeholder="Suami / Istri / Anak / Orang tua" style={input} />
      </label>
      <label style={field}>Golongan darah
        <input value={bloodType} onChange={(e) => setBloodType(e.target.value)} placeholder="A / B / AB / O" style={input} />
      </label>
      <label style={field}>Kondisi medis kronis
        <textarea value={chronicConditions} onChange={(e) => setChronicConditions(e.target.value)} rows={2} style={{ ...input, minHeight: 60, resize: "vertical" }} />
      </label>
      <label style={field}>Obat rutin yang dikonsumsi
        <textarea value={currentMedications} onChange={(e) => setCurrentMedications(e.target.value)} rows={2} style={{ ...input, minHeight: 60, resize: "vertical" }} />
      </label>
      <button disabled={savingInsurance} onClick={saveInsurance} style={emerald}>{savingInsurance ? "Menyimpan..." : "Simpan Asuransi & Medis"}</button>
    </section>

    {notice && <p role="status" style={{ gridColumn: "1 / -1" }}>{notice}</p>}
  </div>;
}

const grid: React.CSSProperties = { display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(320px, 1fr))", gap: 16 };
const card: React.CSSProperties = { background: "var(--color-cream-200)", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 20, display: "grid", gap: 12, alignContent: "start" };
const field: React.CSSProperties = { display: "grid", gap: 6, color: "var(--color-warm-500)", fontSize: 14 };
const input: React.CSSProperties = { minHeight: 44, width: "100%", border: "1px solid var(--color-cream-500)", borderRadius: 8, padding: "10px 12px", background: "white", color: "var(--color-warm-900)", font: "inherit" };
const emerald: React.CSSProperties = { minHeight: 44, border: 0, borderRadius: 8, background: "var(--color-emerald-900)", color: "white", fontWeight: 700, padding: "0 16px" };
const danger: React.CSSProperties = { minHeight: 44, border: 0, borderRadius: 8, background: "var(--color-danger-600)", color: "white", fontWeight: 700, padding: "0 16px" };
const badge: React.CSSProperties = { color: "white", fontWeight: 700, fontSize: 12, borderRadius: 999, padding: "6px 14px" };
const warning: React.CSSProperties = { margin: 0, color: "var(--color-danger-600)", fontWeight: 600, fontSize: 13 };
const uploadLabel: React.CSSProperties = { display: "inline-flex", alignItems: "center", gap: 4, minHeight: 32, padding: "0 10px", background: "var(--color-emerald-900)", color: "white", borderRadius: 8, fontSize: 12, fontWeight: 600, cursor: "pointer", whiteSpace: "nowrap" };
const cameraLabel: React.CSSProperties = { display: "inline-flex", alignItems: "center", gap: 4, minHeight: 32, padding: "0 10px", background: "var(--color-gold-500)", color: "var(--color-warm-900)", borderRadius: 8, fontSize: 12, fontWeight: 600, cursor: "pointer", whiteSpace: "nowrap" };
const fileRow: React.CSSProperties = { display: "flex", alignItems: "center", gap: 8, padding: "6px 10px", background: "var(--color-cream-100)", border: "1px solid var(--color-cream-400)", borderRadius: 8 };
const deleteButton: React.CSSProperties = { border: 0, background: "transparent", color: "var(--color-danger-600)", display: "flex", alignItems: "center", flexShrink: 0 };
