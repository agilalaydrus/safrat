"use client";

import { useState } from "react";
import { Timestamp } from "@bufbuild/protobuf";
import { Pilgrim } from "@hajj-saas/proto-gen/hajj/v1/pilgrim_pb";
import { pilgrimClient } from "@/lib/rpc";
import DocumentUploader from "./DocumentUploader";

const PAYMENT_STATUSES = [
  { value: "UNPAID", label: "Belum Bayar", color: "var(--color-danger-600)" },
  { value: "DP", label: "DP", color: "var(--color-gold-800)" },
  { value: "PAID", label: "Lunas", color: "var(--color-emerald-900)" },
];

function toDateInput(ts?: Timestamp): string {
  if (!ts) return "";
  return ts.toDate().toISOString().slice(0, 10);
}

function fromDateInput(value: string): Timestamp | undefined {
  if (!value) return undefined;
  return Timestamp.fromDate(new Date(`${value}T00:00:00Z`));
}

function monthsUntil(ts?: Timestamp): number | undefined {
  if (!ts) return undefined;
  const diffMs = ts.toDate().getTime() - Date.now();
  return diffMs / (1000 * 60 * 60 * 24 * 30);
}

export default function PilgrimDocumentsPanel({ pilgrim, onUpdated }: { pilgrim: Pilgrim; onUpdated: (pilgrim: Pilgrim) => void }) {
  const [paymentStatus, setPaymentStatus] = useState(pilgrim.paymentStatus || "UNPAID");
  const [paymentReceiptUrl, setPaymentReceiptUrl] = useState(pilgrim.paymentReceiptUrl);
  const [paymentNotes, setPaymentNotes] = useState(pilgrim.paymentNotes);
  const [savingPayment, setSavingPayment] = useState(false);

  const [documentsPassport, setDocumentsPassport] = useState(pilgrim.documentsPassport);
  const [documentsPhoto, setDocumentsPhoto] = useState(pilgrim.documentsPhoto);
  const [documentsVaccine, setDocumentsVaccine] = useState(pilgrim.documentsVaccine);
  const [passportExpiryDate, setPassportExpiryDate] = useState(toDateInput(pilgrim.passportExpiryDate));
  const [vaccineMeningitisDate, setVaccineMeningitisDate] = useState(toDateInput(pilgrim.vaccineMeningitisDate));
  const [savingDocuments, setSavingDocuments] = useState(false);

  const [emergencyContactName, setEmergencyContactName] = useState(pilgrim.emergencyContactName);
  const [emergencyContactPhone, setEmergencyContactPhone] = useState(pilgrim.emergencyContactPhone);
  const [savingContact, setSavingContact] = useState(false);

  const [checkingIn, setCheckingIn] = useState(false);
  const [notice, setNotice] = useState("");

  const [insuranceProvider, setInsuranceProvider] = useState(pilgrim.insuranceProvider);
  const [insurancePolicyNo, setInsurancePolicyNo] = useState(pilgrim.insurancePolicyNo);
  const [insuranceClass, setInsuranceClass] = useState(pilgrim.insuranceClass || "STANDARD");
  const [bloodType, setBloodType] = useState(pilgrim.bloodType);
  const [chronicConditions, setChronicConditions] = useState(pilgrim.chronicConditions);
  const [currentMedications, setCurrentMedications] = useState(pilgrim.currentMedications);
  const [savingInsurance, setSavingInsurance] = useState(false);

  const monthsToExpiry = monthsUntil(pilgrim.passportExpiryDate);
  const expiryWarning = monthsToExpiry !== undefined && monthsToExpiry < 6;

  const savePayment = async () => {
    setSavingPayment(true);
    try {
      const result = await pilgrimClient.updatePilgrimPayment({ pilgrimId: pilgrim.id, paymentStatus, paymentReceiptUrl, paymentNotes });
      onUpdated(result);
      setNotice("Status pembayaran diperbarui");
    } catch (error) {
      setNotice(`Gagal: ${error instanceof Error ? error.message : "tidak diketahui"}`);
    } finally {
      setSavingPayment(false);
    }
  };

  const saveDocuments = async () => {
    setSavingDocuments(true);
    try {
      const result = await pilgrimClient.updatePilgrimDocuments({
        pilgrimId: pilgrim.id,
        documentsPassport,
        documentsPhoto,
        documentsVaccine,
        passportExpiryDate: fromDateInput(passportExpiryDate),
        vaccineMeningitisDate: fromDateInput(vaccineMeningitisDate),
      });
      onUpdated(result);
      setNotice("Dokumen diperbarui");
    } catch (error) {
      setNotice(`Gagal: ${error instanceof Error ? error.message : "tidak diketahui"}`);
    } finally {
      setSavingDocuments(false);
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

  const toggleCheckIn = async () => {
    setCheckingIn(true);
    try {
      const result = await pilgrimClient.checkInPilgrimHotel({ pilgrimId: pilgrim.id, checkedIn: !pilgrim.hotelCheckedIn });
      onUpdated(result);
      setNotice(result.hotelCheckedIn ? "Jamaah ditandai sudah check-in hotel" : "Status check-in dibatalkan");
    } catch (error) {
      setNotice(`Gagal: ${error instanceof Error ? error.message : "tidak diketahui"}`);
    } finally {
      setCheckingIn(false);
    }
  };

  const saveInsurance = async () => {
    setSavingInsurance(true);
    try {
      const result = await pilgrimClient.updatePilgrimInsurance({
        pilgrimId: pilgrim.id, insuranceProvider, insurancePolicyNo, insuranceClass, bloodType, chronicConditions, currentMedications,
      });
      onUpdated(result);
      setNotice("Data asuransi & medis diperbarui");
    } catch (error) {
      setNotice(`Gagal: ${error instanceof Error ? error.message : "tidak diketahui"}`);
    } finally {
      setSavingInsurance(false);
    }
  };

  const statusMeta = PAYMENT_STATUSES.find((s) => s.value === pilgrim.paymentStatus) ?? PAYMENT_STATUSES[0]!;

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
      <label style={field}>Tautan bukti pembayaran
        <input value={paymentReceiptUrl} onChange={(e) => setPaymentReceiptUrl(e.target.value)} placeholder="https://..." style={input} />
      </label>
      {pilgrim.paymentReceiptUrl && <a href={pilgrim.paymentReceiptUrl} target="_blank" rel="noreferrer" style={{ color: "var(--color-gold-800)", fontSize: 13 }}>Lihat bukti pembayaran saat ini</a>}
      <label style={field}>Catatan pembayaran
        <textarea value={paymentNotes} onChange={(e) => setPaymentNotes(e.target.value)} rows={3} style={{ ...input, minHeight: 80, resize: "vertical" }} />
      </label>
      <button disabled={savingPayment} onClick={savePayment} style={emerald}>{savingPayment ? "Menyimpan..." : "Simpan Pembayaran"}</button>
    </section>

    <section style={card}>
      <h2 style={{ margin: 0 }}>Dokumen</h2>
      {expiryWarning && <p style={warning}>Peringatan: paspor akan kedaluwarsa dalam kurang dari 6 bulan.</p>}
      <label style={checkboxRow}><input type="checkbox" checked={documentsPassport} onChange={(e) => setDocumentsPassport(e.target.checked)} /> Paspor tersedia</label>
      <label style={field}>Tanggal kedaluwarsa paspor
        <input type="date" value={passportExpiryDate} onChange={(e) => setPassportExpiryDate(e.target.value)} style={input} />
      </label>
      <label style={checkboxRow}><input type="checkbox" checked={documentsPhoto} onChange={(e) => setDocumentsPhoto(e.target.checked)} /> Foto tersedia</label>
      <label style={checkboxRow}><input type="checkbox" checked={documentsVaccine} onChange={(e) => setDocumentsVaccine(e.target.checked)} /> Vaksin Meningitis tersedia</label>
      <label style={field}>Tanggal vaksin Meningitis
        <input type="date" value={vaccineMeningitisDate} onChange={(e) => setVaccineMeningitisDate(e.target.value)} style={input} />
      </label>
      <button disabled={savingDocuments} onClick={saveDocuments} style={emerald}>{savingDocuments ? "Menyimpan..." : "Simpan Dokumen"}</button>
    </section>

    <section style={card}>
      <h2 style={{ margin: 0 }}>File Dokumen</h2>
      <DocumentUploader pilgrimId={pilgrim.id} />
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
      <h2 style={{ margin: 0 }}>Check-in Hotel</h2>
      <p style={{ margin: 0, color: "var(--color-warm-500)" }}>{pilgrim.hotelCheckedIn ? "Jamaah sudah check-in di hotel." : "Jamaah belum check-in di hotel."}</p>
      <button disabled={checkingIn} onClick={toggleCheckIn} style={pilgrim.hotelCheckedIn ? disabledBtn : emerald}>
        {checkingIn ? "Memproses..." : pilgrim.hotelCheckedIn ? "Batalkan Check-in" : "Tandai Check-in"}
      </button>
    </section>

    <section style={card}>
      <h2 style={{ margin: 0 }}>Asuransi & Medis</h2>
      <label style={field}>Penyedia asuransi
        <input value={insuranceProvider} onChange={(e) => setInsuranceProvider(e.target.value)} style={input} />
      </label>
      <label style={field}>Nomor polis
        <input value={insurancePolicyNo} onChange={(e) => setInsurancePolicyNo(e.target.value)} style={input} />
      </label>
      <label style={field}>Kelas asuransi
        <select value={insuranceClass} onChange={(e) => setInsuranceClass(e.target.value)} style={input}>
          <option value="STANDARD">Standard</option>
          <option value="PLUS">Plus</option>
          <option value="PREMIUM">Premium</option>
        </select>
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
const disabledBtn: React.CSSProperties = { minHeight: 44, border: "1px solid var(--color-cream-400)", borderRadius: 8, background: "var(--color-cream-300)", color: "var(--color-warm-500)", fontWeight: 700, padding: "0 16px" };
const badge: React.CSSProperties = { color: "white", fontWeight: 700, fontSize: 12, borderRadius: 999, padding: "6px 14px" };
const warning: React.CSSProperties = { margin: 0, color: "var(--color-danger-600)", fontWeight: 600, fontSize: 13 };
const checkboxRow: React.CSSProperties = { display: "flex", alignItems: "center", gap: 8, color: "var(--color-warm-700)", fontSize: 14 };
