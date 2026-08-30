"use client";

import { useEffect, useMemo, useState } from "react";
import { IconFile, IconFiles, IconSearch, IconTrash, IconUpload } from "@tabler/icons-react";
import { Pilgrim, PilgrimDocument } from "@hajj-saas/proto-gen/hajj/v1/pilgrim_pb";
import { pilgrimClient, seasonClient } from "@/lib/rpc";
import { getBearerToken } from "@/lib/transport";
import { ActionCenter, type ActionCenterItem } from "@/components/ui/ActionCenter";

const DOC_TYPES = [
  { value: "PASSPORT", label: "Paspor" },
  { value: "PHOTO", label: "Foto" },
  { value: "VACCINE", label: "Sertifikat Vaksin" },
  { value: "OTHER", label: "Lainnya" },
];
const DOC_LABEL: Record<string, string> = Object.fromEntries(DOC_TYPES.map((d) => [d.value, d.label]));
const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "";
// Passport, photo and meningitis certificate are the three Saudi will not
// process a visa without. OTHER is supporting paper, so it is not counted.
const REQUIRED_DOC_TYPES = [
  { value: "PASSPORT", label: "paspor", consequence: "Tanpa paspor terunggah, pengajuan visa tidak bisa dikirim sama sekali dan jamaah tidak masuk manifes muassasah." },
  { value: "VACCINE", label: "sertifikat vaksin meningitis", consequence: "Saudi menolak masuk tanpa sertifikat meningitis, dan jadwal vaksinasi butuh waktu — ini tidak bisa diurus di pekan terakhir." },
  { value: "PHOTO", label: "pasfoto", consequence: "Pasfoto dipakai untuk visa dan kartu identitas jamaah; tanpa itu berkas tertahan di meja verifikasi." },
];

export default function DocumentsDashboard() {
  const [seasons, setSeasons] = useState<{ id: string; name: string; isActive: boolean }[]>([]);
  const [seasonId, setSeasonId] = useState("");
  const [documents, setDocuments] = useState<PilgrimDocument[]>([]);
  const [pilgrims, setPilgrims] = useState<Pilgrim[]>([]);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState("");
  const [term, setTerm] = useState("");
  const [typeFilter, setTypeFilter] = useState("all");
  const [notice, setNotice] = useState("");
  const [actionError, setActionError] = useState("");

  const [uploadOpen, setUploadOpen] = useState(false);
  const [uploadPilgrimQuery, setUploadPilgrimQuery] = useState("");
  const [uploadPilgrimId, setUploadPilgrimId] = useState("");
  const [uploadDocType, setUploadDocType] = useState("PASSPORT");
  const [uploading, setUploading] = useState(false);

  useEffect(() => {
    seasonClient.listSeasons({}).then((response) => {
      setSeasons(response.seasons);
      setSeasonId(response.seasons.find((s) => s.isActive)?.id ?? response.seasons[0]?.id ?? "");
    }).catch(() => setNotice("Gagal memuat data musim."));
  }, []);

  const refresh = () => {
    if (!seasonId) return;
    setLoading(true);
    setActionError("");
    Promise.all([
      pilgrimClient.listSeasonDocuments({ seasonId }),
      pilgrimClient.listPilgrims({ seasonId, limit: 2000, offset: 0 }),
    ]).then(([docsResponse, pilgrimsResponse]) => {
      setDocuments(docsResponse.documents);
      setPilgrims(pilgrimsResponse.pilgrims);
    }).catch(() => {
      setNotice("Gagal memuat dokumen jamaah.");
      setActionError("Dokumen dan data jamaah gagal dimuat. Coba muat ulang sebelum menyimpulkan bahwa berkas sudah lengkap.");
    }).finally(() => setLoading(false));
  };
  useEffect(refresh, [seasonId]);

  useEffect(() => { const timeout = window.setTimeout(() => setTerm(search), 300); return () => window.clearTimeout(timeout); }, [search]);

  const counts = useMemo(() => {
    const result: Record<string, number> = { PASSPORT: 0, PHOTO: 0, VACCINE: 0, OTHER: 0 };
    for (const doc of documents) if (doc.docType in result) result[doc.docType] = (result[doc.docType] ?? 0) + 1;
    return result;
  }, [documents]);

  const actionItems = useMemo<ActionCenterItem[]>(() => {
    if (!pilgrims.length) return [];
    // Count jamaah missing a type, not files — someone who uploaded three
    // photos and no passport is still blocked.
    const held = new Map<string, Set<string>>();
    for (const doc of documents) {
      const set = held.get(doc.pilgrimId) ?? new Set<string>();
      set.add(doc.docType);
      held.set(doc.pilgrimId, set);
    }
    return REQUIRED_DOC_TYPES.flatMap(({ value, label, consequence }) => {
      const missing = pilgrims.filter((pilgrim) => !held.get(pilgrim.id)?.has(value)).length;
      if (missing === 0) return [];
      return [{
        id: `missing-${value}`,
        title: `${missing} jamaah belum mengunggah ${label}`,
        description: consequence,
        financialImpact: `${missing} jamaah`,
        actionHref: "/dashboard/pilgrims",
        actionLabel: "Lihat jamaah",
        tone: value === "PASSPORT" ? "danger" : "warning",
      } satisfies ActionCenterItem];
    });
  }, [pilgrims, documents]);

  const filtered = useMemo(() => documents.filter((doc) => {
    const matchesType = typeFilter === "all" || doc.docType === typeFilter;
    const matchesSearch = `${doc.pilgrimName} ${doc.passportNumber} ${doc.fileName}`.toLowerCase().includes(term.toLowerCase());
    return matchesType && matchesSearch;
  }), [documents, typeFilter, term]);

  const activeName = seasons.find((s) => s.id === seasonId)?.name ?? "Pilih musim";

  const uploadCandidates = useMemo(() => pilgrims
    .filter((p) => !p.isSubstituted && `${p.fullName} ${p.passportNumber}`.toLowerCase().includes(uploadPilgrimQuery.toLowerCase()))
    .slice(0, 20), [pilgrims, uploadPilgrimQuery]);

  const remove = async (doc: PilgrimDocument) => {
    if (!window.confirm(`Hapus dokumen ${doc.fileName}?`)) return;
    try {
      await pilgrimClient.deletePilgrimDocument({ id: doc.id });
      setNotice("Dokumen dihapus.");
      refresh();
    } catch (error) {
      setNotice(`Gagal menghapus: ${error instanceof Error ? error.message : "tidak diketahui"}`);
    }
  };

  const upload = async (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    if (!file || !uploadPilgrimId) return;
    setUploading(true);
    setNotice("");
    try {
      const token = await getBearerToken();
      if (!token) throw new Error("Sesi tidak ditemukan. Silakan login ulang.");
      const form = new FormData();
      form.append("pilgrim_id", uploadPilgrimId);
      form.append("doc_type", uploadDocType);
      form.append("file", file);
      const response = await fetch(`${API_URL}/upload/document`, { method: "POST", headers: { Authorization: `Bearer ${token}` }, body: form });
      if (!response.ok) throw new Error(await response.text() || "Upload gagal.");
      setNotice("Dokumen berhasil diunggah.");
      setUploadOpen(false);
      setUploadPilgrimId("");
      setUploadPilgrimQuery("");
      refresh();
    } catch (error) {
      setNotice(`Upload gagal: ${error instanceof Error ? error.message : "tidak diketahui"}`);
    } finally {
      setUploading(false);
      event.target.value = "";
    }
  };

  return <main style={page}>
    <header style={header}>
      <div><p style={eyebrow}>OPERASIONAL / DOKUMEN</p><h1 style={title}>Manajemen Dokumen Jamaah</h1><p style={{ color: "var(--color-warm-500)", margin: 0 }}>{`${documents.length} dokumen${activeName ? ` · ${activeName}` : ""}`}</p></div>
      <div style={{ display: "flex", gap: 10, flexWrap: "wrap" }}>
        <select aria-label="Musim" value={seasonId} onChange={(e) => setSeasonId(e.target.value)} style={select}>
          {seasons.length ? seasons.map((s) => <option key={s.id} value={s.id}>{s.name}{s.isActive ? " · Aktif" : ""}</option>) : <option>{activeName}</option>}
        </select>
        <button onClick={() => setUploadOpen(true)} style={uploadOpen ? ghost : emerald}><IconUpload size={18} />Upload Dokumen</button>
      </div>
    </header>
    <div className="gold-divider" />
    {notice && <p role="status" style={{ color: "var(--color-gold-800)" }}>{notice}</p>}

    <ActionCenter
      items={loading ? undefined : actionItems}
      error={actionError}
      subtitle="Dihitung per jamaah, bukan per berkas — satu jenis yang hilang tetap menahan pengajuan visa"
      cleanTitle="Berkas lengkap"
      cleanDescription="Setiap jamaah musim ini sudah mengunggah paspor, sertifikat vaksin, dan pasfoto."
      className="tw-action-center--inline"
    />

    <section style={statGrid}>
      {DOC_TYPES.map((d) => <div key={d.value} style={statCard}><span style={statLabel}>{d.label}</span><strong style={statValue}>{counts[d.value] ?? 0} <span className="tw-stat__unit">berkas</span></strong></div>)}
    </section>

    {uploadOpen && <section style={uploadCard}>
      <h2 style={{ margin: 0 }}>Upload Dokumen Baru</h2>
      <label style={{ display: "grid", gap: 6, color: "var(--color-warm-500)", fontSize: 14 }}>Cari jamaah
        <input value={uploadPilgrimQuery} onChange={(e) => { setUploadPilgrimQuery(e.target.value); setUploadPilgrimId(""); }} placeholder="Cari nama jamaah atau nomor paspor…" style={input} />
      </label>
      {uploadPilgrimQuery && !uploadPilgrimId && <div style={{ display: "grid", gap: 6, maxHeight: 180, overflowY: "auto" }}>
        {uploadCandidates.map((p) => <button key={p.id} onClick={() => { setUploadPilgrimId(p.id); setUploadPilgrimQuery(p.fullName); }} style={candidateButton}><strong>{p.fullName}</strong><span style={{ color: "var(--color-warm-400)", fontSize: 12 }}>{p.passportNumber}</span></button>)}
        {!uploadCandidates.length && <p style={{ margin: 0, color: "var(--color-warm-400)", fontSize: 13 }}>Tidak ada jamaah yang cocok.</p>}
      </div>}
      <div style={{ display: "flex", gap: 8, flexWrap: "wrap", alignItems: "center" }}>
        <select value={uploadDocType} onChange={(e) => setUploadDocType(e.target.value)} style={select}>
          {DOC_TYPES.map((d) => <option key={d.value} value={d.value}>{d.label}</option>)}
        </select>
        <label style={{ ...emerald, cursor: uploadPilgrimId ? "pointer" : "not-allowed", opacity: uploadPilgrimId ? 1 : 0.5 }}>
          <IconUpload size={16} />{uploading ? "Mengunggah..." : "Pilih File"}
          <input type="file" accept=".pdf,.jpg,.jpeg,.png" onChange={upload} style={{ display: "none" }} disabled={!uploadPilgrimId || uploading} />
        </label>
        <button onClick={() => { setUploadOpen(false); setUploadPilgrimId(""); setUploadPilgrimQuery(""); }} style={ghost}>Batal</button>
      </div>
      {!uploadPilgrimId && <p style={{ margin: 0, fontSize: 12, color: "var(--color-warm-400)" }}>Pilih jamaah terlebih dahulu sebelum mengunggah file.</p>}
    </section>}

    <section style={toolbar}>
      <label style={{ position: "relative", flex: "1 1 280px" }}>
        <span style={sr}>Cari dokumen</span>
        <IconSearch size={20} style={{ position: "absolute", insetInlineStart: 14, top: 14, color: "var(--color-warm-400)" }} />
        <input value={search} onChange={(e) => setSearch(e.target.value)} placeholder="Cari nama jamaah, nomor paspor, atau nama file…" style={{ ...select, width: "100%", paddingInlineStart: 44 }} />
      </label>
      <div style={chips}>
        {[["all", "Semua"], ...DOC_TYPES.map((d) => [d.value, d.label])].map(([key, label]) => <button key={key} onClick={() => setTypeFilter(key ?? "all")} style={typeFilter === key ? chipActive : chip}>{label}</button>)}
      </div>
    </section>

    <section style={tableCard}>
      {loading ? <div style={skeleton}>Memuat dokumen…</div> : filtered.length ? <div style={{ overflowX: "auto" }}>
        <table style={table}>
          <thead><tr>{["Jamaah", "No. Paspor", "Jenis", "Nama File", "Diunggah Oleh", "Tanggal", ""].map((h) => <th key={h} style={th}>{h}</th>)}</tr></thead>
          <tbody>
            {filtered.map((doc) => <tr key={doc.id} style={tr}>
              <td style={td}><a href={`/dashboard/pilgrims/${doc.pilgrimId}`} style={{ color: "var(--color-emerald-900)", fontWeight: 700 }}>{doc.pilgrimName || "-"}</a></td>
              <td style={{ ...td, fontFamily: "ui-monospace, monospace", color: "var(--color-warm-500)" }}>{doc.passportNumber || "-"}</td>
              <td style={td}>{DOC_LABEL[doc.docType] ?? doc.docType}</td>
              <td style={td}><span style={{ display: "inline-flex", alignItems: "center", gap: 6 }}><IconFile size={15} style={{ color: "var(--color-emerald-700)" }} />{doc.fileName}</span></td>
              <td style={td}>{doc.uploadedBy === "pilgrim" ? "Jamaah" : "Operator"}</td>
              <td style={{ ...td, color: "var(--color-warm-400)" }}>{doc.createdAt?.toDate().toLocaleDateString("id-ID") ?? "-"}</td>
              <td style={td}>
                <div style={{ display: "flex", gap: 8 }}>
                  <a href={`${API_URL}${doc.fileUrl}`} target="_blank" rel="noreferrer" style={{ fontSize: 12, color: "var(--color-emerald-700)", fontWeight: 600, textDecoration: "none" }}>Buka</a>
                  <button onClick={() => remove(doc)} aria-label={`Hapus ${doc.fileName}`} style={deleteButton}><IconTrash size={15} /></button>
                </div>
              </td>
            </tr>)}
          </tbody>
        </table>
      </div> : <div style={empty}><IconFiles size={48} color="var(--color-warm-400)" /><h2 style={{ margin: 0 }}>Belum ada dokumen</h2><p style={{ color: "var(--color-warm-500)" }}>Belum ada berkas yang diunggah untuk musim atau filter ini. Gunakan Upload Dokumen di kanan atas untuk memilih jamaah dan jenis berkas.</p></div>}
    </section>
  </main>;
}

const page: React.CSSProperties = { maxWidth: 1400, margin: "0 auto", padding: "32px 24px" };
const header: React.CSSProperties = { display: "flex", justifyContent: "space-between", gap: 24, alignItems: "flex-start", flexWrap: "wrap" };
const eyebrow: React.CSSProperties = { color: "var(--color-gold-800)", fontSize: 11, fontWeight: 700, letterSpacing: ".08em", margin: "4px 0 8px" };
const title: React.CSSProperties = { fontSize: "clamp(32px,5vw,48px)", fontWeight: 500, margin: 0 };
const select: React.CSSProperties = { minHeight: 48, border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 12px", background: "var(--color-cream-200)", font: "inherit", color: "var(--color-warm-900)" };
const emerald: React.CSSProperties = { minHeight: 48, border: 0, borderRadius: 8, background: "var(--color-emerald-900)", color: "white", fontWeight: 700, padding: "0 18px", display: "inline-flex", gap: 8, alignItems: "center" };
const ghost: React.CSSProperties = { minHeight: 48, border: "1px solid var(--color-cream-400)", borderRadius: 8, background: "transparent", color: "var(--color-warm-500)", padding: "0 16px" };
const statGrid: React.CSSProperties = { display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(160px, 1fr))", gap: 12, margin: "20px 0" };
const statCard: React.CSSProperties = { border: "1px solid var(--color-cream-400)", borderRadius: 12, background: "var(--color-cream-200)", padding: "16px 20px", display: "grid", gap: 6 };
const statLabel: React.CSSProperties = { fontSize: 12, color: "var(--color-warm-500)", textTransform: "uppercase", letterSpacing: ".06em" };
const statValue: React.CSSProperties = { fontSize: 28, fontWeight: 700, color: "var(--color-emerald-900)" };
const uploadCard: React.CSSProperties = { display: "grid", gap: 12, background: "white", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 20, marginBottom: 20 };
const input: React.CSSProperties = { minHeight: 44, width: "100%", border: "1px solid var(--color-cream-500)", borderRadius: 8, padding: "10px 12px", background: "var(--color-cream-200)", color: "var(--color-warm-900)", font: "inherit" };
const candidateButton: React.CSSProperties = { minHeight: 44, display: "grid", gap: 2, textAlign: "start", border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "8px 12px", background: "var(--color-cream-100)", color: "var(--color-emerald-900)" };
const toolbar: React.CSSProperties = { display: "flex", gap: 16, alignItems: "center", flexWrap: "wrap", margin: "8px 0 16px" };
const chips: React.CSSProperties = { display: "flex", gap: 8, flexWrap: "wrap" };
const chip: React.CSSProperties = { minHeight: 40, borderWidth: 1, borderStyle: "solid", borderColor: "var(--color-cream-400)", borderRadius: 999, padding: "0 14px", background: "transparent", color: "var(--color-warm-500)" };
const chipActive: React.CSSProperties = { ...chip, background: "var(--color-gold-50)", borderColor: "var(--color-gold-100)", color: "var(--color-gold-600)", boxShadow: "0 0 0 4px color-mix(in srgb, var(--color-gold-500) 12%, transparent)" };
const tableCard: React.CSSProperties = { border: "1px solid var(--color-cream-400)", borderRadius: 12, background: "white", overflow: "hidden", minHeight: 260 };
const table: React.CSSProperties = { width: "100%", borderCollapse: "collapse", minWidth: 800 };
const th: React.CSSProperties = { background: "var(--color-cream-200)", padding: "14px 16px", textAlign: "start", fontSize: 11, textTransform: "uppercase", letterSpacing: ".08em", color: "var(--color-warm-400)" };
const tr: React.CSSProperties = { borderTop: "1px solid var(--color-cream-300)" };
const td: React.CSSProperties = { padding: "14px 16px", fontSize: 14 };
const deleteButton: React.CSSProperties = { border: 0, background: "transparent", color: "var(--color-danger-600)", display: "flex", alignItems: "center" };
const empty: React.CSSProperties = { padding: "64px 24px", textAlign: "center", display: "grid", justifyItems: "center", gap: 12 };
const skeleton: React.CSSProperties = { padding: 32, color: "var(--color-warm-500)" };
const sr: React.CSSProperties = { position: "absolute", width: 1, height: 1, overflow: "hidden", clip: "rect(0 0 0 0)" };
