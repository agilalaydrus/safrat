"use client";

import { useEffect, useState } from "react";
import { IconFile, IconTrash, IconUpload } from "@tabler/icons-react";
import { Timestamp } from "@bufbuild/protobuf";
import { Pilgrim, PilgrimDocument } from "@hajj-saas/proto-gen/hajj/v1/pilgrim_pb";
import { pilgrimClient } from "@/lib/rpc";
import { getBearerToken } from "@/lib/transport";
import CameraCaptureButton from "@/components/shared/CameraCaptureButton";

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "";

// The full PPIU + Saudi-entry document checklist — every umrah/hajj
// pilgrim needs all of these, so this is a fixed set of upload slots, not
// a "pick one type" dropdown a staffer has to remember to cycle through.
type BoolField = "documentsPassport" | "documentsPhoto" | "documentsKtp" | "documentsKk" | "documentsVaccine" | "documentsMahramProof" | "documentsVisa";
type DateField = "passportExpiryDate" | "vaccineMeningitisDate" | "visaExpiryDate";

interface RequiredDoc {
  type: string;
  label: string;
  hint: string;
  boolField: BoolField;
  hasDate?: DateField;
  dateLabel?: string;
  hasNumber?: boolean;
  // Documents that are reliably verifiable from a single page/shot — these
  // get a direct "Ambil Foto" camera-capture button alongside the regular
  // file picker. Multi-page documents (KK can list a large family, buku
  // nikah can be several pages) are left to the file picker only, since a
  // phone camera capture is one photo at a time.
  singlePage?: boolean;
}

const REQUIRED_DOCS: RequiredDoc[] = [
  { type: "PASSPORT", label: "Paspor", hint: "Masa berlaku minimal 6 bulan sejak tanggal keberangkatan", boolField: "documentsPassport", hasDate: "passportExpiryDate", dateLabel: "Tanggal kedaluwarsa paspor", singlePage: true },
  { type: "PHOTO", label: "Pas Foto", hint: "Latar belakang putih, wajah tampak 80%", boolField: "documentsPhoto", singlePage: true },
  { type: "KTP", label: "KTP", hint: "Kartu Tanda Penduduk", boolField: "documentsKtp", singlePage: true },
  { type: "KK", label: "Kartu Keluarga", hint: "Untuk verifikasi data keluarga", boolField: "documentsKk" },
  { type: "VACCINE", label: "Sertifikat Vaksin Meningitis", hint: "Wajib untuk visa masuk Arab Saudi (buku kuning ICV)", boolField: "documentsVaccine", hasDate: "vaccineMeningitisDate", dateLabel: "Tanggal vaksin", singlePage: true },
  { type: "MAHRAM_PROOF", label: "Akta Nikah / Kelahiran (Bukti Mahram)", hint: "Wajib jika bepergian bersama mahram, atau untuk jamaah wanita di bawah 45 tahun", boolField: "documentsMahramProof" },
  { type: "VISA", label: "Visa Umrah/Haji", hint: "Diterbitkan lewat Nusuk/eHajj — dokumen terpisah dari paspor", boolField: "documentsVisa", hasDate: "visaExpiryDate", dateLabel: "Tanggal berlaku visa", hasNumber: true, singlePage: true },
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
  return (ts.toDate().getTime() - Date.now()) / (1000 * 60 * 60 * 24 * 30);
}

export default function PilgrimDocumentChecklist({ pilgrim, onUpdated }: { pilgrim: Pilgrim; onUpdated: (pilgrim: Pilgrim) => void }) {
  const [docs, setDocs] = useState<PilgrimDocument[]>([]);
  const [checked, setChecked] = useState<Record<BoolField, boolean>>({
    documentsPassport: pilgrim.documentsPassport, documentsPhoto: pilgrim.documentsPhoto, documentsVaccine: pilgrim.documentsVaccine,
    documentsKtp: pilgrim.documentsKtp, documentsKk: pilgrim.documentsKk, documentsMahramProof: pilgrim.documentsMahramProof,
    documentsVisa: pilgrim.documentsVisa,
  });
  const [passportExpiryDate, setPassportExpiryDate] = useState(toDateInput(pilgrim.passportExpiryDate));
  const [vaccineMeningitisDate, setVaccineMeningitisDate] = useState(toDateInput(pilgrim.vaccineMeningitisDate));
  const [visaExpiryDate, setVisaExpiryDate] = useState(toDateInput(pilgrim.visaExpiryDate));
  const [visaNumber, setVisaNumber] = useState(pilgrim.visaNumber);
  const [saving, setSaving] = useState(false);
  const [uploadingType, setUploadingType] = useState("");
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");

  const refresh = () => pilgrimClient.listPilgrimDocuments({ pilgrimId: pilgrim.id }).then((r) => setDocs(r.documents)).catch(() => setError("Gagal memuat dokumen."));
  useEffect(() => { void refresh(); }, [pilgrim.id]);

  const monthsToExpiry = monthsUntil(pilgrim.passportExpiryDate);
  const expiryWarning = monthsToExpiry !== undefined && monthsToExpiry < 6;
  const completeCount = REQUIRED_DOCS.filter((d) => checked[d.boolField]).length;

  const save = async () => {
    setSaving(true);
    setNotice("");
    try {
      const result = await pilgrimClient.updatePilgrimDocuments({
        pilgrimId: pilgrim.id,
        documentsPassport: checked.documentsPassport ?? false,
        documentsPhoto: checked.documentsPhoto ?? false,
        documentsVaccine: checked.documentsVaccine ?? false,
        documentsKtp: checked.documentsKtp ?? false,
        documentsKk: checked.documentsKk ?? false,
        documentsMahramProof: checked.documentsMahramProof ?? false,
        documentsVisa: checked.documentsVisa ?? false,
        passportExpiryDate: fromDateInput(passportExpiryDate),
        vaccineMeningitisDate: fromDateInput(vaccineMeningitisDate),
        visaExpiryDate: fromDateInput(visaExpiryDate),
        visaNumber,
      });
      onUpdated(result);
      setNotice("Checklist dokumen disimpan.");
    } catch (err) {
      setNotice(`Gagal: ${err instanceof Error ? err.message : "tidak diketahui"}`);
    } finally {
      setSaving(false);
    }
  };

  const upload = async (docType: string, file: File) => {
    setUploadingType(docType);
    setError("");
    try {
      const token = await getBearerToken();
      if (!token) throw new Error("Sesi tidak ditemukan. Silakan login ulang.");
      const form = new FormData();
      form.append("pilgrim_id", pilgrim.id);
      form.append("doc_type", docType);
      form.append("file", file);
      const response = await fetch(`${API_URL}/upload/document`, { method: "POST", headers: { Authorization: `Bearer ${token}` }, body: form });
      if (!response.ok) throw new Error((await response.text()) || "Upload gagal.");
      refresh();
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "Upload gagal.");
    } finally {
      setUploadingType("");
    }
  };

  const uploadFromInput = (docType: string, event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    event.target.value = "";
    if (file) void upload(docType, file);
  };

  const remove = async (id: string) => {
    try {
      await pilgrimClient.deletePilgrimDocument({ id });
      refresh();
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "Gagal menghapus dokumen.");
    }
  };

  // PAYMENT_RECEIPT has its own uploader in the Pembayaran card — filtered
  // out here so a receipt doesn't also show up as an unlabeled "other" doc.
  const otherDocs = docs.filter((d) => d.docType !== "PAYMENT_RECEIPT" && !REQUIRED_DOCS.some((r) => r.type === d.docType));

  return (
    <section style={card}>
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", gap: 8, flexWrap: "wrap" }}>
        <h2 style={{ margin: 0 }}>Dokumen (Standar PPIU &amp; Arab Saudi)</h2>
        <span style={{ ...badge, background: completeCount === REQUIRED_DOCS.length ? "var(--color-emerald-900)" : "var(--color-gold-800)" }}>{completeCount}/{REQUIRED_DOCS.length} lengkap</span>
      </div>
      {expiryWarning && <p style={warning}>Peringatan: paspor akan kedaluwarsa dalam kurang dari 6 bulan.</p>}
      {error && <p style={warning}>{error}</p>}

      <div style={{ display: "grid", gap: 10 }}>
        {REQUIRED_DOCS.map((d) => {
          const filesForType = docs.filter((doc) => doc.docType === d.type);
          return (
            <div key={d.type} style={docCard}>
              <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start", gap: 10 }}>
                <label style={{ display: "flex", alignItems: "flex-start", gap: 8, flex: 1 }}>
                  <input type="checkbox" checked={checked[d.boolField] ?? false} onChange={(e) => setChecked((c) => ({ ...c, [d.boolField]: e.target.checked }))} style={{ marginTop: 3 }} />
                  <span>
                    <strong style={{ fontSize: 14 }}>{d.label}</strong>
                    <p style={{ margin: "2px 0 0", fontSize: 12, color: "var(--color-warm-400)" }}>{d.hint}</p>
                  </span>
                </label>
                <div style={{ display: "flex", gap: 6, flexWrap: "wrap" }}>
                  {d.singlePage && (
                    <CameraCaptureButton label={uploadingType === d.type ? "..." : "Ambil Foto"} onCapture={(file) => void upload(d.type, file)} disabled={uploadingType === d.type} style={cameraLabel} />
                  )}
                  <label style={uploadLabel}>
                    <IconUpload size={14} />{uploadingType === d.type ? "..." : "Upload"}
                    <input type="file" accept=".pdf,.jpg,.jpeg,.png" onChange={(e) => uploadFromInput(d.type, e)} style={{ display: "none" }} disabled={uploadingType === d.type} />
                  </label>
                </div>
              </div>
              {d.hasNumber && (
                <label style={{ ...field, marginTop: 8 }}>Nomor visa
                  <input value={visaNumber} onChange={(e) => setVisaNumber(e.target.value)} style={input} />
                </label>
              )}
              {d.hasDate && (
                <label style={{ ...field, marginTop: 8 }}>{d.dateLabel}
                  <input
                    type="date"
                    value={d.hasDate === "passportExpiryDate" ? passportExpiryDate : d.hasDate === "vaccineMeningitisDate" ? vaccineMeningitisDate : visaExpiryDate}
                    onChange={(e) => {
                      if (d.hasDate === "passportExpiryDate") setPassportExpiryDate(e.target.value);
                      else if (d.hasDate === "vaccineMeningitisDate") setVaccineMeningitisDate(e.target.value);
                      else setVisaExpiryDate(e.target.value);
                    }}
                    style={input}
                  />
                </label>
              )}
              {filesForType.length > 0 && (
                <div style={{ display: "grid", gap: 6, marginTop: 8 }}>
                  {filesForType.map((doc) => (
                    <div key={doc.id} style={fileRow}>
                      <IconFile size={15} style={{ color: "var(--color-emerald-700)", flexShrink: 0 }} />
                      <span style={{ flex: 1, minWidth: 0, fontSize: 12, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{doc.fileName}</span>
                      <a href={`${API_URL}${doc.fileUrl}`} target="_blank" rel="noreferrer" style={{ fontSize: 11, color: "var(--color-emerald-700)", fontWeight: 600 }}>Buka</a>
                      <button onClick={() => remove(doc.id)} aria-label={`Hapus ${doc.fileName}`} style={deleteButton}><IconTrash size={13} /></button>
                    </div>
                  ))}
                </div>
              )}
            </div>
          );
        })}
      </div>

      <button disabled={saving} onClick={save} style={emerald}>{saving ? "Menyimpan..." : "Simpan Checklist Dokumen"}</button>
      {notice && <p role="status" style={{ margin: 0, fontSize: 13, color: "var(--color-warm-500)" }}>{notice}</p>}

      <div style={{ marginTop: 8 }}>
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
          <h3 style={{ margin: 0, fontSize: 14 }}>Dokumen Lainnya</h3>
          <label style={uploadLabel}>
            <IconUpload size={14} />{uploadingType === "OTHER" ? "..." : "Upload"}
            <input type="file" accept=".pdf,.jpg,.jpeg,.png" onChange={(e) => uploadFromInput("OTHER", e)} style={{ display: "none" }} disabled={uploadingType === "OTHER"} />
          </label>
        </div>
        {!otherDocs.length && <p style={{ margin: "6px 0 0", fontSize: 12, color: "var(--color-warm-400)" }}>Tidak ada dokumen tambahan.</p>}
        {otherDocs.map((doc) => (
          <div key={doc.id} style={fileRow}>
            <IconFile size={15} style={{ color: "var(--color-emerald-700)", flexShrink: 0 }} />
            <span style={{ flex: 1, minWidth: 0, fontSize: 12, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{doc.fileName}</span>
            <a href={`${API_URL}${doc.fileUrl}`} target="_blank" rel="noreferrer" style={{ fontSize: 11, color: "var(--color-emerald-700)", fontWeight: 600 }}>Buka</a>
            <button onClick={() => remove(doc.id)} aria-label={`Hapus ${doc.fileName}`} style={deleteButton}><IconTrash size={13} /></button>
          </div>
        ))}
      </div>
    </section>
  );
}

const card: React.CSSProperties = { background: "var(--color-cream-200)", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 20, display: "grid", gap: 12, alignContent: "start" };
const docCard: React.CSSProperties = { border: "1px solid var(--color-cream-400)", borderRadius: 10, padding: 12, background: "#fff" };
const field: React.CSSProperties = { display: "grid", gap: 6, color: "var(--color-warm-500)", fontSize: 13 };
const input: React.CSSProperties = { minHeight: 40, width: "100%", border: "1px solid var(--color-cream-500)", borderRadius: 8, padding: "8px 12px", background: "white", color: "var(--color-warm-900)", font: "inherit" };
const emerald: React.CSSProperties = { minHeight: 44, border: 0, borderRadius: 8, background: "var(--color-emerald-900)", color: "white", fontWeight: 700, padding: "0 16px" };
const badge: React.CSSProperties = { color: "white", fontWeight: 700, fontSize: 12, borderRadius: 999, padding: "6px 14px" };
const warning: React.CSSProperties = { margin: 0, color: "var(--color-danger-600)", fontWeight: 600, fontSize: 13 };
const uploadLabel: React.CSSProperties = { display: "inline-flex", alignItems: "center", gap: 4, minHeight: 32, padding: "0 10px", background: "var(--color-emerald-900)", color: "white", borderRadius: 8, fontSize: 12, fontWeight: 600, cursor: "pointer", whiteSpace: "nowrap" };
const cameraLabel: React.CSSProperties = { display: "inline-flex", alignItems: "center", gap: 4, minHeight: 32, padding: "0 10px", background: "var(--color-gold-500)", color: "var(--color-warm-900)", borderRadius: 8, fontSize: 12, fontWeight: 600, cursor: "pointer", whiteSpace: "nowrap" };
const fileRow: React.CSSProperties = { display: "flex", alignItems: "center", gap: 8, padding: "6px 10px", background: "var(--color-cream-100)", border: "1px solid var(--color-cream-400)", borderRadius: 8 };
const deleteButton: React.CSSProperties = { border: 0, background: "transparent", color: "var(--color-danger-600)", display: "flex", alignItems: "center", flexShrink: 0 };
