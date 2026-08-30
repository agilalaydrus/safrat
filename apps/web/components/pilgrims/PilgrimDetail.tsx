"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { IconAward, IconEdit, IconShare, IconUser } from "@tabler/icons-react";
import { Pilgrim } from "@hajj-saas/proto-gen/hajj/v1/pilgrim_pb";
import { pilgrimClient, groupClient } from "@/lib/rpc";
import PilgrimFormDialog from "./PilgrimFormDialog";
import PilgrimDocumentsPanel from "./PilgrimDocumentsPanel";
import ShareLinkDialog from "./ShareLinkDialog";
import { RoleGate } from "@/components/auth/RoleGate";

export default function PilgrimDetail({ id }: { id: string }) {
  const [pilgrim, setPilgrim] = useState<Pilgrim>();
  const [group, setGroup] = useState<{ name: string; leaderName: string }>();
  const [edit, setEdit] = useState(false);
  const [tab, setTab] = useState<"profil" | "dokumen">("profil");
  const [confirmSubstitution, setConfirmSubstitution] = useState(false);
  const [replacementSearch, setReplacementSearch] = useState("");
  const [replacementTerm, setReplacementTerm] = useState("");
  const [replacementPilgrims, setReplacementPilgrims] = useState<Pilgrim[]>([]);
  const [selectedReplacement, setSelectedReplacement] = useState<Pilgrim>();
  const [substitutionReason, setSubstitutionReason] = useState("");
  const [substituting, setSubstituting] = useState(false);
  const [notice, setNotice] = useState("");
  const [shareLink, setShareLink] = useState<{ title: string; url: string }>();

  useEffect(() => {
    pilgrimClient.getPilgrim({ pilgrimId: id }).then(setPilgrim).catch(() => setNotice("Gagal memuat data jamaah."));
  }, [id]);

  useEffect(() => {
    if (!pilgrim?.groupId || !pilgrim?.seasonId) { setGroup(undefined); return; }
    groupClient.listGroups({ seasonId: pilgrim.seasonId })
      .then((response) => setGroup(response.groups.find((g) => g.id === pilgrim.groupId)))
      .catch(() => setGroup(undefined));
  }, [pilgrim?.groupId, pilgrim?.seasonId]);

  useEffect(() => {
    if (!confirmSubstitution || !pilgrim) return;
    pilgrimClient.listPilgrims({ seasonId: pilgrim.seasonId, limit: 100, offset: 0 }).then((response) => setReplacementPilgrims(response.pilgrims)).catch(() => setNotice("Gagal memuat calon pengganti."));
  }, [confirmSubstitution, pilgrim]);

  useEffect(() => { const timeout = window.setTimeout(() => setReplacementTerm(replacementSearch), 300); return () => window.clearTimeout(timeout); }, [replacementSearch]);

  const candidates = useMemo(() => replacementPilgrims.filter((candidate) => candidate.id !== pilgrim?.id && !candidate.isSubstituted && `${candidate.fullName} ${candidate.passportNumber}`.toLowerCase().includes(replacementTerm.toLowerCase())), [pilgrim?.id, replacementPilgrims, replacementTerm]);

  if (!pilgrim) return <main style={page}>{notice || "Memuat data jamaah..."}</main>;

  const substitute = async () => {
    if (!selectedReplacement || !substitutionReason.trim()) return;
    setSubstituting(true);
    try {
      const result = await pilgrimClient.substitutePilgrim({ originalPilgrimId: id, replacementPilgrimId: selectedReplacement.id, reason: substitutionReason.trim() });
      setPilgrim(result.original);
      setConfirmSubstitution(false);
      setSelectedReplacement(undefined);
      setReplacementSearch("");
      setSubstitutionReason("");
      setNotice("Penggantian selesai");
    } catch (error) {
      const message = error instanceof Error ? error.message : "Gagal menandai jamaah sebagai digantikan.";
      setNotice(`Gagal: ${message}`);
    } finally {
      setSubstituting(false);
    }
  };

  return <main style={page}>
    <Link href="/dashboard/pilgrims" style={{ color: "var(--color-gold-800)" }}>Kembali ke daftar jamaah</Link>
    <header style={{ display: "flex", justifyContent: "space-between", gap: 16, flexWrap: "wrap", marginTop: 20 }}>
      <div><p style={eyebrow}>PROFIL JAMAAH</p><h1 style={{ margin: 0, fontSize: 40 }}>{pilgrim.fullName}</h1></div>
      <div style={{ display: "flex", gap: 10, flexWrap: "wrap" }}>
        <button onClick={() => setShareLink({ title: "Bagikan ke Keluarga", url: `${window.location.origin}/track/${pilgrim.appAccessCode}` })} style={ghostBtn}><IconShare size={18} />Bagikan ke Keluarga</button>
        <button onClick={() => setShareLink({ title: "Bagikan Sertifikat", url: `${window.location.origin}/certificate/${pilgrim.appAccessCode}` })} style={ghostBtn}><IconAward size={18} />Bagikan Sertifikat</button>
        <button onClick={() => setEdit(true)} style={emerald}><IconEdit size={18} />Ubah</button>
      </div>
    </header>
    {pilgrim.status === "CANCELLED" && <p style={{ margin: "12px 0 0", padding: "10px 14px", borderRadius: 8, background: "var(--color-danger-100)", color: "var(--color-danger-600)", fontWeight: 700 }}>Jamaah ini telah dibatalkan.</p>}
    <div className="gold-divider" />
    <div style={tabBar}>
      <button onClick={() => setTab("profil")} style={tab === "profil" ? tabActive : tabInactive}>Profil</button>
      <button onClick={() => setTab("dokumen")} style={tab === "dokumen" ? tabActive : tabInactive}>Dokumen &amp; Pembayaran</button>
    </div>
    {tab === "profil" ? <div style={grid}>
      <section style={card}>
        <div style={avatar}><IconUser size={36} /></div>
        <h2>Identitas</h2>
        <Details values={[["Paspor", pilgrim.passportNumber], ["Kewarganegaraan", pilgrim.nationality], ["Jenis kelamin", pilgrim.gender === 2 ? "Wanita" : "Pria"], ["Tanggal lahir", pilgrim.dateOfBirth?.toDate().toLocaleDateString("id-ID") ?? "-"], ["Telepon", pilgrim.phone || "-"], ["Kontak darurat", pilgrim.emergencyContact || "-"], ["Bahasa", pilgrim.preferredLang]]} />
      </section>
      <aside style={card}>
        <h2>Penempatan</h2>
        <Details values={[["Grup", pilgrim.groupId ? (group?.name ?? "Memuat...") : "Belum ditentukan"], ["Muttawwif", pilgrim.groupId ? (group?.leaderName || "Belum ada Muttawwif") : "-"], ["Kamar", "Belum ditentukan"], ["Kendaraan", "Belum ditentukan"]]} />
        <div className="gold-divider" />
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
          <h2 style={{ margin: 0 }}>Penggantian Jamaah</h2>
          <Link href={`/dashboard/pilgrims/substitution?seasonId=${pilgrim.seasonId}`} style={{ fontSize: 12, color: "var(--color-gold-800)" }}>Lihat riwayat</Link>
        </div>
        {pilgrim.isSubstituted ? <p style={{ color: "var(--color-gold-800)" }}>Ditandai sebagai digantikan</p> : pilgrim.status === "CANCELLED" ? <p style={{ color: "var(--color-warm-400)", fontSize: 13 }}>Jamaah yang dibatalkan tidak dapat disubstitusi.</p> : <RoleGate require={["owner", "admin"]} fallback={<p style={{ margin: 0, color: "var(--color-warm-400)", fontSize: 13 }}>Hanya pemilik atau admin yang dapat melakukan substitusi.</p>}>
          {confirmSubstitution ? <div style={confirmation}>
            {selectedReplacement ? <>
              <p style={{ margin: 0 }}>Ganti {pilgrim.fullName} dengan {selectedReplacement.fullName}?</p>
              <p style={{ margin: 0, color: "var(--color-warm-500)", fontSize: 13 }}>{selectedReplacement.passportNumber}</p>
              <label style={{ display: "grid", gap: 6, color: "var(--color-warm-500)", fontSize: 14 }}>Alasan substitusi (wajib)
                <textarea value={substitutionReason} onChange={(event) => setSubstitutionReason(event.target.value)} rows={3} placeholder="Contoh: Jamaah sakit dan tidak dapat berangkat" style={{ ...input, minHeight: 80, resize: "vertical" }} maxLength={500} />
              </label>
              <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
                <button disabled={substituting || !substitutionReason.trim()} onClick={substitute} style={danger}>{substituting ? "Mengganti..." : "Konfirmasi penggantian"}</button>
                <button onClick={() => { setSelectedReplacement(undefined); setSubstitutionReason(""); }} style={disabled}>Pilih yang lain</button>
              </div>
            </> : <><label style={{ display: "grid", gap: 6, color: "var(--color-warm-500)", fontSize: 14 }}>Cari pengganti<input value={replacementSearch} onChange={(event) => setReplacementSearch(event.target.value)} placeholder="Cari nama jamaah atau nomor paspor…" style={input} /></label><div style={{ display: "grid", gap: 8, maxHeight: 240, overflowY: "auto" }}>{candidates.map((candidate) => <button key={candidate.id} onClick={() => setSelectedReplacement(candidate)} style={candidateButton}><strong>{candidate.fullName}</strong><span>{candidate.passportNumber}</span></button>)}{!candidates.length && <p style={{ margin: 0, color: "var(--color-warm-500)" }}>Tidak ada calon pengganti yang memenuhi syarat.</p>}</div><button onClick={() => setConfirmSubstitution(false)} style={disabled}>Batal</button></>}
          </div> : <button onClick={() => setConfirmSubstitution(true)} style={danger}>Tandai sebagai Digantikan</button>}
        </RoleGate>}
        {pilgrim.status !== "CANCELLED" && !pilgrim.isSubstituted && <>
          <div className="gold-divider" />
          <h2 style={{ margin: 0 }}>Pembatalan</h2>
          <RoleGate require={["owner", "admin"]} fallback={<p style={{ margin: 0, color: "var(--color-warm-400)", fontSize: 13 }}>Hanya pemilik atau admin yang dapat membatalkan jamaah.</p>}>
            <Link href={`/dashboard/pilgrims/${id}/cancel`} style={{ ...danger, textDecoration: "none", display: "inline-flex", alignItems: "center", justifyContent: "center" }}>Batalkan Jamaah</Link>
          </RoleGate>
        </>}
      </aside>
    </div> : <PilgrimDocumentsPanel pilgrim={pilgrim} onUpdated={setPilgrim} />}
    {notice && <p role="status">{notice}</p>}
    <PilgrimFormDialog open={edit} onClose={() => setEdit(false)} seasonId={pilgrim.seasonId} pilgrims={[pilgrim]} initial={pilgrim} onSaved={() => { setNotice("Data jamaah diperbarui"); pilgrimClient.getPilgrim({ pilgrimId: id }).then(setPilgrim); }} />
    {shareLink && <ShareLinkDialog title={shareLink.title} url={shareLink.url} onClose={() => setShareLink(undefined)} />}
  </main>;
}

function Details({ values }: { values: [string, string][] }) {
  return <dl style={{ display: "grid", gap: 12 }}>{values.map(([label, value]) => <div key={label} style={{ display: "grid", gap: 3 }}><dt style={{ fontSize: 12, color: "var(--color-warm-400)", textTransform: "uppercase", letterSpacing: ".08em" }}>{label}</dt><dd style={{ margin: 0 }}>{value}</dd></div>)}</dl>;
}

const page: React.CSSProperties = { maxWidth: 1200, margin: "0 auto", padding: "32px 24px" };
const grid: React.CSSProperties = { display: "grid", gridTemplateColumns: "minmax(0,1.2fr) minmax(320px,.8fr)", gap: 16 };
const card: React.CSSProperties = { background: "var(--color-cream-200)", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 20 };
const avatar: React.CSSProperties = { width: 80, height: 80, borderRadius: "50%", display: "grid", placeItems: "center", background: "var(--color-emerald-50)", color: "var(--color-emerald-900)" };
const eyebrow: React.CSSProperties = { margin: "0 0 6px", fontSize: 11, fontWeight: 700, color: "var(--color-gold-800)", letterSpacing: ".08em" };
const emerald: React.CSSProperties = { minHeight: 48, border: 0, borderRadius: 8, background: "var(--color-emerald-900)", color: "white", fontWeight: 700, padding: "0 18px", display: "inline-flex", gap: 8, alignItems: "center" };
const ghostBtn: React.CSSProperties = { minHeight: 48, border: "1px solid var(--color-emerald-800)", borderRadius: 8, background: "transparent", color: "var(--color-emerald-900)", fontWeight: 600, padding: "0 16px", display: "inline-flex", gap: 8, alignItems: "center" };
const disabled: React.CSSProperties = { minHeight: 48, border: "1px solid var(--color-cream-400)", borderRadius: 8, background: "var(--color-cream-300)", color: "var(--color-warm-400)", padding: "0 14px" };
const danger: React.CSSProperties = { minHeight: 48, border: 0, borderRadius: 8, background: "var(--color-danger-600)", color: "white", fontWeight: 700, padding: "0 14px" };
const confirmation: React.CSSProperties = { display: "grid", gap: 12, padding: 12, border: "1px solid var(--color-danger-600)", borderRadius: 8 };
const input: React.CSSProperties = { minHeight: 48, width: "100%", border: "1px solid var(--color-cream-500)", borderRadius: 8, padding: "10px 12px", background: "var(--color-cream-200)", color: "var(--color-warm-900)", font: "inherit" };
const candidateButton: React.CSSProperties = { minHeight: 48, display: "grid", gap: 2, textAlign: "start", border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "10px 12px", background: "white", color: "var(--color-emerald-900)" };
const tabBar: React.CSSProperties = { display: "flex", gap: 8, margin: "16px 0" };
const tabActive: React.CSSProperties = { minHeight: 44, border: "1px solid var(--color-gold-100)", borderRadius: 12, background: "var(--color-gold-50)", color: "var(--color-gold-600)", boxShadow: "0 0 0 4px color-mix(in srgb, var(--color-gold-500) 12%, transparent)", fontWeight: 700, padding: "0 18px" };
const tabInactive: React.CSSProperties = { minHeight: 44, border: "1px solid var(--color-cream-400)", borderRadius: 8, background: "var(--color-cream-200)", color: "var(--color-warm-700)", fontWeight: 600, padding: "0 18px" };
