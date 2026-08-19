"use client";

import { useEffect, useMemo, useState } from "react";
import { IconCheck, IconClipboardList, IconLink, IconX } from "@tabler/icons-react";
import { Gender } from "@hajj-saas/proto-gen/hajj/v1/pilgrim_pb";
import { PilgrimRegistration } from "@hajj-saas/proto-gen/hajj/v1/registration_pb";
import { operatorClient, pilgrimClient, registrationClient, seasonClient } from "@/lib/rpc";

const STATUS_LABEL: Record<string, string> = { PENDING: "Menunggu", APPROVED: "Disetujui", REJECTED: "Ditolak" };

function genderFromString(value: string): Gender {
  return value === "FEMALE" ? Gender.FEMALE : Gender.MALE;
}

export default function RegistrationsDashboard() {
  const [seasonId, setSeasonId] = useState("");
  const [seasons, setSeasons] = useState<{ id: string; name: string; isActive: boolean }[]>([]);
  const [operatorId, setOperatorId] = useState("");
  const [registrations, setRegistrations] = useState<PilgrimRegistration[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState("PENDING");
  const [workingId, setWorkingId] = useState("");
  const [rejectingId, setRejectingId] = useState("");
  const [rejectNotes, setRejectNotes] = useState("");
  const [notice, setNotice] = useState("");
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    operatorClient.getMyOperator({}).then((response) => setOperatorId(response.id)).catch(() => {});
    seasonClient.listSeasons({}).then((response) => {
      setSeasons(response.seasons);
      setSeasonId(response.seasons.find((s) => s.isActive)?.id ?? response.seasons[0]?.id ?? "");
    }).catch(() => setNotice("Gagal memuat data musim."));
  }, []);

  const refresh = () => {
    if (!seasonId) return;
    setLoading(true);
    registrationClient.listRegistrations({ seasonId }).then((response) => setRegistrations(response.registrations)).catch(() => setNotice("Gagal memuat pendaftaran.")).finally(() => setLoading(false));
  };
  useEffect(refresh, [seasonId]);

  const filtered = useMemo(() => registrations.filter((r) => r.status === filter), [registrations, filter]);
  const activeName = seasons.find((s) => s.id === seasonId)?.name ?? "Pilih musim";
  const publicLink = operatorId && seasonId ? `${typeof window !== "undefined" ? window.location.origin : ""}/register/${operatorId}/${seasonId}` : "";

  const copyLink = async () => {
    if (!publicLink) return;
    await navigator.clipboard.writeText(publicLink);
    setCopied(true);
    window.setTimeout(() => setCopied(false), 2000);
  };

  const approve = async (registration: PilgrimRegistration) => {
    setWorkingId(registration.id);
    setNotice("");
    try {
      await pilgrimClient.createPilgrim({
        seasonId: registration.seasonId,
        fullName: registration.fullName,
        passportNumber: registration.passportNumber,
        nationality: registration.nationality || "IDN",
        dateOfBirth: registration.dateOfBirth,
        gender: genderFromString(registration.gender),
        phone: registration.phone,
        email: registration.email,
        agentId: registration.referredByAgentId,
      });
      await registrationClient.approveRegistration({ registrationId: registration.id, notes: "" });
      setNotice(`${registration.fullName} disetujui dan ditambahkan sebagai jamaah.`);
      refresh();
    } catch (error) {
      setNotice(`Gagal menyetujui: ${error instanceof Error ? error.message : "tidak diketahui"}`);
    } finally {
      setWorkingId("");
    }
  };

  const reject = async (registration: PilgrimRegistration) => {
    if (!rejectNotes.trim()) { setNotice("Alasan penolakan wajib diisi."); return; }
    setWorkingId(registration.id);
    setNotice("");
    try {
      await registrationClient.rejectRegistration({ registrationId: registration.id, notes: rejectNotes.trim() });
      setNotice(`${registration.fullName} ditolak.`);
      setRejectingId("");
      setRejectNotes("");
      refresh();
    } catch (error) {
      setNotice(`Gagal menolak: ${error instanceof Error ? error.message : "tidak diketahui"}`);
    } finally {
      setWorkingId("");
    }
  };

  return <main style={page}>
    <header style={header}>
      <div><p style={eyebrow}>OPERASIONAL / PENDAFTARAN</p><h1 style={title}>Pendaftaran Jamaah</h1><p style={{ color: "var(--color-warm-500)", margin: 0 }}>{`${registrations.length} pendaftaran${activeName ? ` · ${activeName}` : ""}`}</p></div>
      <select aria-label="Musim" value={seasonId} onChange={(e) => setSeasonId(e.target.value)} style={select}>
        {seasons.length ? seasons.map((s) => <option key={s.id} value={s.id}>{s.name}{s.isActive ? " · Aktif" : ""}</option>) : <option>{activeName}</option>}
      </select>
    </header>
    <div className="gold-divider" />
    {notice && <p role="status" style={{ color: "var(--color-gold-800)" }}>{notice}</p>}
    {publicLink && <section style={linkCard}>
      <IconLink size={18} />
      <span style={{ overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", flex: 1 }}>{publicLink}</span>
      <button onClick={copyLink} style={copyButton}>{copied ? "Tersalin!" : "Salin tautan"}</button>
    </section>}
    <div style={chips}>{["PENDING", "APPROVED", "REJECTED"].map((status) => <button key={status} onClick={() => setFilter(status)} style={filter === status ? chipActive : chip}>{STATUS_LABEL[status]}</button>)}</div>
    <section style={{ marginTop: 16 }}>
      {loading ? <p style={{ color: "var(--color-warm-500)" }}>Memuat...</p> : filtered.length ? <div style={{ display: "grid", gap: 12 }}>
        {filtered.map((registration) => <article key={registration.id} style={card}>
          <div style={{ display: "flex", justifyContent: "space-between", gap: 12, flexWrap: "wrap" }}>
            <div>
              <h3 style={{ margin: "0 0 4px" }}>{registration.fullName}</h3>
              <p style={{ margin: 0, color: "var(--color-warm-500)", fontSize: 13 }}>{registration.passportNumber} · {registration.gender === "FEMALE" ? "Wanita" : "Pria"} · {registration.phone || "-"} · {registration.email || "-"}</p>
              {registration.notes && <p style={{ margin: "6px 0 0", fontSize: 13, color: "var(--color-warm-500)" }}>Catatan: {registration.notes}</p>}
              {registration.referredByAgentName && <p style={{ margin: "6px 0 0", fontSize: 13, color: "var(--color-gold-800)", fontWeight: 600 }}>Referral: {registration.referredByAgentName}</p>}
            </div>
            {registration.status === "PENDING" && <div style={{ display: "flex", gap: 8 }}>
              <button disabled={workingId === registration.id} onClick={() => approve(registration)} style={approveBtn}><IconCheck size={16} />Setujui</button>
              <button disabled={workingId === registration.id} onClick={() => setRejectingId(registration.id === rejectingId ? "" : registration.id)} style={rejectBtn}><IconX size={16} />Tolak</button>
            </div>}
          </div>
          {rejectingId === registration.id && <div style={{ display: "grid", gap: 8, marginTop: 12 }}>
            <textarea value={rejectNotes} onChange={(e) => setRejectNotes(e.target.value)} placeholder="Alasan penolakan" rows={2} style={input} />
            <button disabled={workingId === registration.id} onClick={() => reject(registration)} style={rejectBtn}>Konfirmasi Penolakan</button>
          </div>}
        </article>)}
      </div> : <div style={empty}><IconClipboardList size={48} color="var(--color-warm-400)" /><p style={{ color: "var(--color-warm-500)" }}>Tidak ada pendaftaran dengan status ini.</p></div>}
    </section>
  </main>;
}

const page: React.CSSProperties = { maxWidth: 1000, margin: "0 auto", padding: "32px 24px" };
const header: React.CSSProperties = { display: "flex", justifyContent: "space-between", gap: 24, alignItems: "flex-start", flexWrap: "wrap" };
const eyebrow: React.CSSProperties = { color: "var(--color-gold-800)", fontSize: 11, fontWeight: 700, letterSpacing: ".08em", margin: "4px 0 8px" };
const title: React.CSSProperties = { fontSize: "clamp(32px,5vw,48px)", fontWeight: 500, margin: 0 };
const select: React.CSSProperties = { minHeight: 48, border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 12px", background: "var(--color-cream-200)", font: "inherit", color: "var(--color-warm-900)" };
const linkCard: React.CSSProperties = { display: "flex", alignItems: "center", gap: 10, background: "var(--color-cream-200)", border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "10px 14px", margin: "16px 0", color: "var(--color-warm-700)", fontSize: 13 };
const copyButton: React.CSSProperties = { minHeight: 36, border: 0, borderRadius: 8, background: "var(--color-emerald-900)", color: "white", fontWeight: 700, padding: "0 14px", flexShrink: 0 };
const chips: React.CSSProperties = { display: "flex", gap: 8, flexWrap: "wrap" };
const chip: React.CSSProperties = { minHeight: 40, borderWidth: 1, borderStyle: "solid", borderColor: "var(--color-cream-400)", borderRadius: 999, padding: "0 14px", background: "transparent", color: "var(--color-warm-500)" };
const chipActive: React.CSSProperties = { ...chip, background: "var(--color-emerald-900)", borderColor: "var(--color-emerald-900)", color: "var(--color-cream-100)" };
const card: React.CSSProperties = { border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 16, background: "white" };
const approveBtn: React.CSSProperties = { minHeight: 40, border: 0, borderRadius: 8, background: "var(--color-emerald-900)", color: "white", fontWeight: 700, padding: "0 14px", display: "inline-flex", gap: 6, alignItems: "center" };
const rejectBtn: React.CSSProperties = { minHeight: 40, border: "1px solid var(--color-danger-600)", borderRadius: 8, background: "transparent", color: "var(--color-danger-600)", fontWeight: 700, padding: "0 14px", display: "inline-flex", gap: 6, alignItems: "center" };
const input: React.CSSProperties = { minHeight: 44, width: "100%", border: "1px solid var(--color-cream-500)", borderRadius: 8, padding: "10px 12px", background: "var(--color-cream-200)", color: "var(--color-warm-900)", font: "inherit" };
const empty: React.CSSProperties = { padding: "48px 24px", textAlign: "center", display: "grid", justifyItems: "center", gap: 12 };
