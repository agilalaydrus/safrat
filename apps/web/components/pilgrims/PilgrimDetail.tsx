"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { IconCopy, IconEdit, IconUser } from "@tabler/icons-react";
import { Pilgrim } from "@hajj-saas/proto-gen/hajj/v1/pilgrim_pb";
import { pilgrimClient, groupClient } from "@/lib/rpc";
import PilgrimFormDialog from "./PilgrimFormDialog";

export default function PilgrimDetail({ id }: { id: string }) {
  const [pilgrim, setPilgrim] = useState<Pilgrim>();
  const [group, setGroup] = useState<{ name: string; leaderName: string }>();
  const [edit, setEdit] = useState(false);
  const [confirmSubstitution, setConfirmSubstitution] = useState(false);
  const [replacementSearch, setReplacementSearch] = useState("");
  const [replacementTerm, setReplacementTerm] = useState("");
  const [replacementPilgrims, setReplacementPilgrims] = useState<Pilgrim[]>([]);
  const [selectedReplacement, setSelectedReplacement] = useState<Pilgrim>();
  const [substituting, setSubstituting] = useState(false);
  const [regenerating, setRegenerating] = useState(false);
  const [notice, setNotice] = useState("");

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
  }, [confirmSubstitution, pilgrim?.seasonId]);

  useEffect(() => { const timeout = window.setTimeout(() => setReplacementTerm(replacementSearch), 300); return () => window.clearTimeout(timeout); }, [replacementSearch]);

  const candidates = useMemo(() => replacementPilgrims.filter((candidate) => candidate.id !== pilgrim?.id && !candidate.isSubstituted && `${candidate.fullName} ${candidate.passportNumber}`.toLowerCase().includes(replacementTerm.toLowerCase())), [pilgrim?.id, replacementPilgrims, replacementTerm]);

  if (!pilgrim) return <main style={page}>{notice || "Memuat data jamaah..."}</main>;

  const copy = async () => {
    await navigator.clipboard.writeText(pilgrim.appAccessCode);
    setNotice("Kode akses disalin");
  };

  const regenerateCode = async () => {
    if (!window.confirm("Buat ulang kode akses? Kode lama tidak akan berfungsi lagi.")) return;
    setRegenerating(true);
    try {
      const result = await pilgrimClient.regenerateAccessCode({ pilgrimId: id });
      setPilgrim(result);
      setNotice("Kode akses baru dibuat");
    } catch {
      setNotice("Gagal membuat ulang kode akses.");
    } finally {
      setRegenerating(false);
    }
  };

  const substitute = async () => {
    if (!selectedReplacement) return;
    setSubstituting(true);
    try {
      const result = await pilgrimClient.substitutePilgrim({ originalPilgrimId: id, replacementPilgrimId: selectedReplacement.id });
      setPilgrim(result.original);
      setConfirmSubstitution(false);
      setSelectedReplacement(undefined);
      setReplacementSearch("");
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
      <button onClick={() => setEdit(true)} style={emerald}><IconEdit size={18} />Ubah</button>
    </header>
    <div className="gold-divider" />
    <div style={grid}>
      <section style={card}>
        <div style={avatar}><IconUser size={36} /></div>
        <h2>Identitas</h2>
        <Details values={[["Paspor", pilgrim.passportNumber], ["Kewarganegaraan", pilgrim.nationality], ["Jenis kelamin", pilgrim.gender === 2 ? "Wanita" : "Pria"], ["Tanggal lahir", pilgrim.dateOfBirth?.toDate().toLocaleDateString("id-ID") ?? "-"], ["Telepon", pilgrim.phone || "-"], ["Kontak darurat", pilgrim.emergencyContact || "-"], ["Bahasa", pilgrim.preferredLang]]} />
      </section>
      <aside style={card}>
        <h2>Penempatan</h2>
        <Details values={[["Rombongan", pilgrim.groupId ? (group?.name ?? "Memuat...") : "Belum ditentukan"], ["Ketua Rombongan", pilgrim.groupId ? (group?.leaderName || "Belum ada ketua") : "-"], ["Kamar", "Belum ditentukan"], ["Kendaraan", "Belum ditentukan"]]} />
        <div className="gold-divider" />
        <h2>Kode akses</h2>
        <code style={code}>{pilgrim.appAccessCode}</code>
        <div style={{ display: "flex", gap: 8, marginTop: 12, flexWrap: "wrap" }}>
          <button onClick={copy} style={gold}><IconCopy size={18} />Salin</button>
          <button onClick={regenerateCode} disabled={regenerating} style={disabled}>{regenerating ? "Membuat..." : "Buat Ulang"}</button>
        </div>
        <div className="gold-divider" />
        <h2>Penggantian Jamaah</h2>
        {pilgrim.isSubstituted ? <p style={{ color: "var(--color-gold-800)" }}>Ditandai sebagai digantikan</p> : confirmSubstitution ? <div style={confirmation}>
          {selectedReplacement ? <><p style={{ margin: 0 }}>Ganti {pilgrim.fullName} dengan {selectedReplacement.fullName}?</p><p style={{ margin: 0, color: "var(--color-warm-500)", fontSize: 13 }}>{selectedReplacement.passportNumber}</p><div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}><button disabled={substituting} onClick={substitute} style={danger}>{substituting ? "Mengganti..." : "Konfirmasi penggantian"}</button><button onClick={() => setSelectedReplacement(undefined)} style={disabled}>Pilih yang lain</button></div></> : <><label style={{ display: "grid", gap: 6, color: "var(--color-warm-500)", fontSize: 14 }}>Cari pengganti<input value={replacementSearch} onChange={(event) => setReplacementSearch(event.target.value)} placeholder="Cari nama atau paspor" style={input} /></label><div style={{ display: "grid", gap: 8, maxHeight: 240, overflowY: "auto" }}>{candidates.map((candidate) => <button key={candidate.id} onClick={() => setSelectedReplacement(candidate)} style={candidateButton}><strong>{candidate.fullName}</strong><span>{candidate.passportNumber}</span></button>)}{!candidates.length && <p style={{ margin: 0, color: "var(--color-warm-500)" }}>Tidak ada calon pengganti yang memenuhi syarat.</p>}</div><button onClick={() => setConfirmSubstitution(false)} style={disabled}>Batal</button></>}
        </div> : <button onClick={() => setConfirmSubstitution(true)} style={danger}>Tandai sebagai Digantikan</button>}
      </aside>
    </div>
    {notice && <p role="status">{notice}</p>}
    <PilgrimFormDialog open={edit} onClose={() => setEdit(false)} seasonId={pilgrim.seasonId} pilgrims={[pilgrim]} initial={pilgrim} onSaved={() => { setNotice("Data jamaah diperbarui"); pilgrimClient.getPilgrim({ pilgrimId: id }).then(setPilgrim); }} />
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
const gold: React.CSSProperties = { ...emerald, background: "var(--color-gold-500)" };
const disabled: React.CSSProperties = { minHeight: 48, border: "1px solid var(--color-cream-400)", borderRadius: 8, background: "var(--color-cream-300)", color: "var(--color-warm-400)", padding: "0 14px" };
const danger: React.CSSProperties = { minHeight: 48, border: 0, borderRadius: 8, background: "var(--color-danger-600)", color: "white", fontWeight: 700, padding: "0 14px" };
const confirmation: React.CSSProperties = { display: "grid", gap: 12, padding: 12, border: "1px solid var(--color-danger-600)", borderRadius: 8 };
const input: React.CSSProperties = { minHeight: 48, width: "100%", border: "1px solid var(--color-cream-500)", borderRadius: 8, padding: "10px 12px", background: "var(--color-cream-200)", color: "var(--color-warm-900)", font: "inherit" };
const candidateButton: React.CSSProperties = { minHeight: 48, display: "grid", gap: 2, textAlign: "start", border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "10px 12px", background: "white", color: "var(--color-emerald-900)" };
const code: React.CSSProperties = { display: "block", padding: 12, borderRadius: 8, background: "white", fontFamily: "ui-monospace,monospace" };
