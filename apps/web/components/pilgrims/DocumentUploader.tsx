"use client";

import { useEffect, useState } from "react";
import { IconFile, IconTrash, IconUpload } from "@tabler/icons-react";
import { PilgrimDocument } from "@hajj-saas/proto-gen/hajj/v1/pilgrim_pb";
import { pilgrimClient } from "@/lib/rpc";
import { getBearerToken } from "@/lib/transport";

const DOC_TYPES = [
  { value: "PASSPORT", label: "Paspor" },
  { value: "PHOTO", label: "Foto" },
  { value: "VACCINE", label: "Sertifikat Vaksin" },
  { value: "OTHER", label: "Lainnya" },
];

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "";

export default function DocumentUploader({ pilgrimId }: { pilgrimId: string }) {
  const [docs, setDocs] = useState<PilgrimDocument[]>([]);
  const [docType, setDocType] = useState("PASSPORT");
  const [uploading, setUploading] = useState(false);
  const [error, setError] = useState("");

  const refresh = () => {
    pilgrimClient.listPilgrimDocuments({ pilgrimId }).then((response) => setDocs(response.documents)).catch(() => setError("Gagal memuat dokumen."));
  };
  useEffect(refresh, [pilgrimId]);

  const upload = async (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    if (!file) return;
    setUploading(true);
    setError("");
    try {
      const token = await getBearerToken();
      if (!token) throw new Error("Sesi tidak ditemukan. Silakan login ulang.");
      const form = new FormData();
      form.append("pilgrim_id", pilgrimId);
      form.append("doc_type", docType);
      form.append("file", file);
      const response = await fetch(`${API_URL}/upload/document`, {
        method: "POST",
        headers: { Authorization: `Bearer ${token}` },
        body: form,
      });
      if (!response.ok) throw new Error(await response.text() || "Upload gagal.");
      refresh();
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "Upload gagal.");
    } finally {
      setUploading(false);
      event.target.value = "";
    }
  };

  const remove = async (id: string) => {
    try {
      await pilgrimClient.deletePilgrimDocument({ id });
      refresh();
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "Gagal menghapus dokumen.");
    }
  };

  return <div>
    <div style={{ display: "flex", gap: 8, marginBottom: 12, flexWrap: "wrap", alignItems: "center" }}>
      <select value={docType} onChange={(e) => setDocType(e.target.value)} style={select}>
        {DOC_TYPES.map((d) => <option key={d.value} value={d.value}>{d.label}</option>)}
      </select>
      <label style={uploadLabel}>
        <IconUpload size={15} />{uploading ? "Mengunggah..." : "Upload File"}
        <input type="file" accept=".pdf,.jpg,.jpeg,.png" onChange={upload} style={{ display: "none" }} disabled={uploading} />
      </label>
    </div>
    {error && <p style={{ color: "var(--color-danger-600)", fontSize: 12, marginBottom: 8 }}>{error}</p>}
    <div style={{ display: "grid", gap: 8 }}>
      {!docs.length && <p style={{ color: "var(--color-warm-400)", fontSize: 13, margin: 0 }}>Belum ada dokumen diunggah.</p>}
      {docs.map((doc) => <div key={doc.id} style={row}>
        <IconFile size={18} style={{ color: "var(--color-emerald-700)", flexShrink: 0 }} />
        <div style={{ flex: 1, minWidth: 0 }}>
          <p style={{ margin: 0, fontSize: 13, fontWeight: 600, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{doc.fileName}</p>
          <p style={{ margin: 0, fontSize: 11, color: "var(--color-warm-400)" }}>{DOC_TYPES.find((d) => d.value === doc.docType)?.label ?? doc.docType}</p>
        </div>
        <a href={`${API_URL}${doc.fileUrl}`} target="_blank" rel="noreferrer" style={{ fontSize: 12, color: "var(--color-emerald-700)", fontWeight: 600, textDecoration: "none" }}>Buka</a>
        <button onClick={() => remove(doc.id)} aria-label={`Hapus ${doc.fileName}`} style={deleteButton}><IconTrash size={15} /></button>
      </div>)}
    </div>
  </div>;
}

const select: React.CSSProperties = { minHeight: 40, padding: "0 12px", borderRadius: 8, border: "1px solid var(--color-cream-500)", background: "var(--color-cream-200)", fontSize: 13, font: "inherit" };
const uploadLabel: React.CSSProperties = { display: "inline-flex", alignItems: "center", gap: 6, minHeight: 40, padding: "0 14px", background: "var(--color-emerald-900)", color: "white", borderRadius: 8, fontSize: 13, fontWeight: 600, cursor: "pointer" };
const row: React.CSSProperties = { display: "flex", alignItems: "center", gap: 10, padding: "10px 14px", background: "var(--color-cream-100)", border: "1px solid var(--color-cream-400)", borderRadius: 8 };
const deleteButton: React.CSSProperties = { border: 0, background: "transparent", color: "var(--color-danger-600)", display: "flex", alignItems: "center", flexShrink: 0 };
