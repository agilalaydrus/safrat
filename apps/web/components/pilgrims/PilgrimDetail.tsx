"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { IconCopy, IconEdit, IconUser } from "@tabler/icons-react";
import { Pilgrim } from "@hajj-saas/proto-gen/hajj/v1/pilgrim_pb";
import { pilgrimClient } from "@/lib/rpc";
import PilgrimFormDialog from "./PilgrimFormDialog";

export default function PilgrimDetail({ id }: { id: string }) {
  const [pilgrim, setPilgrim] = useState<Pilgrim>();
  const [edit, setEdit] = useState(false);
  const [confirmSubstitution, setConfirmSubstitution] = useState(false);
  const [replacementSearch, setReplacementSearch] = useState("");
  const [replacementTerm, setReplacementTerm] = useState("");
  const [replacementPilgrims, setReplacementPilgrims] = useState<Pilgrim[]>([]);
  const [selectedReplacement, setSelectedReplacement] = useState<Pilgrim>();
  const [substituting, setSubstituting] = useState(false);
  const [notice, setNotice] = useState("");

  useEffect(() => {
    pilgrimClient.getPilgrim({ pilgrimId: id }).then(setPilgrim).catch(() => setNotice("Unable to load pilgrim."));
  }, [id]);

  useEffect(() => {
    if (!confirmSubstitution || !pilgrim) return;
    pilgrimClient.listPilgrims({ seasonId: pilgrim.seasonId, limit: 100, offset: 0 }).then((response) => setReplacementPilgrims(response.pilgrims)).catch(() => setNotice("Unable to load replacement pilgrims."));
  }, [confirmSubstitution, pilgrim?.seasonId]);

  useEffect(() => { const timeout = window.setTimeout(() => setReplacementTerm(replacementSearch), 300); return () => window.clearTimeout(timeout); }, [replacementSearch]);

  const candidates = useMemo(() => replacementPilgrims.filter((candidate) => candidate.id !== pilgrim?.id && !candidate.isSubstituted && `${candidate.fullName} ${candidate.passportNumber}`.toLowerCase().includes(replacementTerm.toLowerCase())), [pilgrim?.id, replacementPilgrims, replacementTerm]);

  if (!pilgrim) return <main style={page}>{notice || "Loading pilgrim..."}</main>;

  const copy = async () => {
    await navigator.clipboard.writeText(pilgrim.appAccessCode);
    setNotice("Access code copied");
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
      setNotice("Substitution complete");
    } catch (error) {
      const message = error instanceof Error ? error.message : "Unable to mark pilgrim as substituted.";
      setNotice(`Failed: ${message}`);
    } finally {
      setSubstituting(false);
    }
  };

  return <main style={page}>
    <Link href="/dashboard/pilgrims" style={{ color: "var(--color-gold-800)" }}>Back to pilgrims</Link>
    <header style={{ display: "flex", justifyContent: "space-between", gap: 16, flexWrap: "wrap", marginTop: 20 }}>
      <div><p style={eyebrow}>PILGRIM PROFILE</p><h1 style={{ margin: 0, fontSize: 40 }}>{pilgrim.fullName}</h1></div>
      <button onClick={() => setEdit(true)} style={emerald}><IconEdit size={18} />Edit</button>
    </header>
    <div className="gold-divider" />
    <div style={grid}>
      <section style={card}>
        <div style={avatar}><IconUser size={36} /></div>
        <h2>Identity</h2>
        <Details values={[["Passport", pilgrim.passportNumber], ["Nationality", pilgrim.nationality], ["Gender", pilgrim.gender === 2 ? "Female" : "Male"], ["Date of birth", pilgrim.dateOfBirth?.toDate().toLocaleDateString() ?? "-"], ["Phone", pilgrim.phone || "-"], ["Emergency contact", pilgrim.emergencyContact || "-"], ["Language", pilgrim.preferredLang]]} />
      </section>
      <aside style={card}>
        <h2>Allocations</h2>
        <Details values={[["Group", pilgrim.groupId ? "Assigned group" : "Unassigned"], ["Room", "Not assigned"], ["Vehicle", "Not assigned"]]} />
        <div className="gold-divider" />
        <h2>Access code</h2>
        <code style={code}>{pilgrim.appAccessCode}</code>
        <div style={{ display: "flex", gap: 8, marginTop: 12, flexWrap: "wrap" }}>
          <button onClick={copy} style={gold}><IconCopy size={18} />Copy</button>
          {/* TODO: wire when API exposes regenerateAccessCode RPC */}
          <button disabled style={disabled}>Regenerate</button>
        </div>
        <div className="gold-divider" />
        <h2>Substitution</h2>
        {pilgrim.isSubstituted ? <p style={{ color: "var(--color-gold-800)" }}>Marked as substituted</p> : confirmSubstitution ? <div style={confirmation}>
          {selectedReplacement ? <><p style={{ margin: 0 }}>Replace {pilgrim.fullName} with {selectedReplacement.fullName}?</p><p style={{ margin: 0, color: "var(--color-warm-500)", fontSize: 13 }}>{selectedReplacement.passportNumber}</p><div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}><button disabled={substituting} onClick={substitute} style={danger}>{substituting ? "Replacing..." : "Confirm substitution"}</button><button onClick={() => setSelectedReplacement(undefined)} style={disabled}>Choose another</button></div></> : <><label style={{ display: "grid", gap: 6, color: "var(--color-warm-500)", fontSize: 14 }}>Find a replacement<input value={replacementSearch} onChange={(event) => setReplacementSearch(event.target.value)} placeholder="Search name or passport" style={input} /></label><div style={{ display: "grid", gap: 8, maxHeight: 240, overflowY: "auto" }}>{candidates.map((candidate) => <button key={candidate.id} onClick={() => setSelectedReplacement(candidate)} style={candidateButton}><strong>{candidate.fullName}</strong><span>{candidate.passportNumber}</span></button>)}{!candidates.length && <p style={{ margin: 0, color: "var(--color-warm-500)" }}>No eligible replacement pilgrims found.</p>}</div><button onClick={() => setConfirmSubstitution(false)} style={disabled}>Cancel</button></>}
        </div> : <button onClick={() => setConfirmSubstitution(true)} style={danger}>Mark as Substituted</button>}
      </aside>
    </div>
    {notice && <p role="status">{notice}</p>}
    <PilgrimFormDialog open={edit} onClose={() => setEdit(false)} seasonId={pilgrim.seasonId} pilgrims={[pilgrim]} initial={pilgrim} onSaved={() => { setNotice("Pilgrim updated"); pilgrimClient.getPilgrim({ pilgrimId: id }).then(setPilgrim); }} />
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
