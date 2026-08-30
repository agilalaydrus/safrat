"use client";

import { useEffect, useState } from "react";
import { IconTrash, IconUsers } from "@tabler/icons-react";
import { KloterScheduleSummary, KloterStaff } from "@hajj-saas/proto-gen/hajj/v1/staff_schedule_pb";
import { OperatorMember } from "@hajj-saas/proto-gen/hajj/v1/group_pb";
import { groupClient, seasonClient, staffScheduleClient } from "@/lib/rpc";
import { RoleGate } from "@/components/auth/RoleGate";

const ROLE_LABEL: Record<string, string> = { COORDINATOR: "Koordinator", MEDICAL: "Medis", GUIDE: "Pemandu", ADMIN_SUPPORT: "Dukungan Admin" };

function countColor(count: number): string {
  if (count === 0) return "var(--color-danger-600)";
  if (count === 1) return "var(--color-gold-800)";
  return "var(--color-emerald-800)";
}

export default function StaffScheduleDashboard() {
  const [seasons, setSeasons] = useState<{ id: string; name: string; isActive: boolean }[]>([]);
  const [seasonId, setSeasonId] = useState("");
  const [summaries, setSummaries] = useState<KloterScheduleSummary[]>([]);
  const [members, setMembers] = useState<OperatorMember[]>([]);
  const [loading, setLoading] = useState(true);
  const [notice, setNotice] = useState("");

  const [drawerKloter, setDrawerKloter] = useState<KloterScheduleSummary>();
  const [drawerStaff, setDrawerStaff] = useState<KloterStaff[]>([]);
  const [assignMemberId, setAssignMemberId] = useState("");
  const [assignRole, setAssignRole] = useState("COORDINATOR");
  const [assignDuties, setAssignDuties] = useState("");
  const [assigning, setAssigning] = useState(false);

  useEffect(() => {
    seasonClient.listSeasons({}).then((response) => {
      setSeasons(response.seasons);
      setSeasonId(response.seasons.find((s) => s.isActive)?.id ?? response.seasons[0]?.id ?? "");
    }).catch(() => setNotice("Gagal memuat data musim."));
    groupClient.listOperatorMembers({}).then((response) => setMembers(response.members)).catch(() => {});
  }, []);

  const refresh = () => {
    if (!seasonId) return;
    setLoading(true);
    staffScheduleClient.listAllStaffSchedule({ seasonId }).then((response) => setSummaries(response.kloters)).catch(() => setNotice("Gagal memuat jadwal tim.")).finally(() => setLoading(false));
  };
  useEffect(refresh, [seasonId]);

  const activeName = seasons.find((s) => s.id === seasonId)?.name ?? "Pilih musim";

  const openDrawer = async (kloter: KloterScheduleSummary) => {
    setDrawerKloter(kloter);
    setAssignMemberId("");
    setAssignDuties("");
    try {
      const response = await staffScheduleClient.listKloterStaff({ kloterId: kloter.kloterId });
      setDrawerStaff(response.staff);
    } catch {
      setNotice("Gagal memuat staf kloter.");
    }
  };

  const assign = async () => {
    if (!drawerKloter || !assignMemberId) return;
    const member = members.find((m) => m.userId === assignMemberId);
    if (!member) return;
    setAssigning(true);
    try {
      await staffScheduleClient.assignStaffToKloter({
        kloterId: drawerKloter.kloterId, staffId: member.userId, staffName: member.name, staffEmail: member.email, role: assignRole, duties: assignDuties,
      });
      const response = await staffScheduleClient.listKloterStaff({ kloterId: drawerKloter.kloterId });
      setDrawerStaff(response.staff);
      setAssignMemberId("");
      setAssignDuties("");
      refresh();
    } catch (error) {
      setNotice(`Gagal menugaskan: ${error instanceof Error ? error.message : "tidak diketahui"}`);
    } finally {
      setAssigning(false);
    }
  };

  const removeStaff = async (staff: KloterStaff) => {
    if (!drawerKloter) return;
    try {
      await staffScheduleClient.removeStaffFromKloter({ kloterId: drawerKloter.kloterId, staffId: staff.staffId });
      setDrawerStaff((current) => current.filter((s) => s.staffId !== staff.staffId));
      refresh();
    } catch (error) {
      setNotice(`Gagal menghapus: ${error instanceof Error ? error.message : "tidak diketahui"}`);
    }
  };

  return <main style={page}>
    <header style={header}>
      <div><p style={eyebrow}>OPERASIONAL / JADWAL TIM</p><h1 style={title}>Jadwal Tim</h1><p style={{ color: "var(--color-warm-500)", margin: 0 }}>{summaries.length} kloter · {summaries.reduce((total, summary) => total + summary.staffCount, 0)} penugasan staf · {activeName}</p></div>
      <select aria-label="Musim" value={seasonId} onChange={(e) => setSeasonId(e.target.value)} style={select}>
        {seasons.length ? seasons.map((s) => <option key={s.id} value={s.id}>{s.name}{s.isActive ? " · Aktif" : ""}</option>) : <option>{activeName}</option>}
      </select>
    </header>
    <div className="gold-divider" />
    {notice && <p role="status" style={{ color: "var(--color-gold-800)" }}>{notice}</p>}

    {loading ? <p style={{ color: "var(--color-warm-500)" }}>Memuat...</p> : <div style={grid}>
      {summaries.map((summary) => <button key={summary.kloterId} onClick={() => openDrawer(summary)} style={kloterCard}>
        <h3 style={{ margin: 0 }}>{summary.kloterName}</h3>
        {summary.departureDate && <p style={{ margin: "4px 0 0", fontSize: 13, color: "var(--color-warm-500)" }}>{summary.departureDate.toDate().toLocaleDateString("id-ID")}</p>}
        <div style={{ display: "flex", alignItems: "center", gap: 6, marginTop: 10 }}>
          <IconUsers size={16} color={countColor(summary.staffCount)} />
          <strong style={{ color: countColor(summary.staffCount) }}>{summary.staffCount} staf</strong>
        </div>
        {summary.staffNames && <p style={{ margin: "6px 0 0", fontSize: 12, color: "var(--color-warm-400)" }}>{summary.staffNames}</p>}
      </button>)}
      {!summaries.length && <p style={{ color: "var(--color-warm-500)" }}>Belum ada jadwal tim karena musim ini belum memiliki kloter. Buat kloter melalui menu Kloter, lalu kembali ke sini untuk menugaskan staf.</p>}
    </div>}

    {drawerKloter && <div role="dialog" aria-modal="true" style={overlay} onClick={() => setDrawerKloter(undefined)}>
      <aside style={drawer} onClick={(e) => e.stopPropagation()}>
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
          <h2 style={{ margin: 0 }}>{drawerKloter.kloterName}</h2>
          <button onClick={() => setDrawerKloter(undefined)} style={ghost}>Tutup</button>
        </div>
        <div className="gold-divider" />
        <h3 style={{ margin: "0 0 10px" }}>Staf Bertugas</h3>
        <div style={{ display: "grid", gap: 8, marginBottom: 20 }}>
          {drawerStaff.map((staff) => <div key={staff.id} style={staffRow}>
            <div><strong>{staff.staffName}</strong><span style={{ display: "block", fontSize: 12, color: "var(--color-warm-400)" }}>{ROLE_LABEL[staff.role] ?? staff.role}{staff.duties ? ` · ${staff.duties}` : ""}</span></div>
            <RoleGate require={["owner", "admin"]}><button onClick={() => removeStaff(staff)} aria-label={`Hapus ${staff.staffName}`} style={deleteBtn}><IconTrash size={14} /></button></RoleGate>
          </div>)}
          {!drawerStaff.length && <p style={{ color: "var(--color-warm-400)", fontSize: 13 }}>Belum ada staf yang ditugaskan ke kloter ini. Pilih anggota dan perannya pada formulir Penugasan Staf di bawah.</p>}
        </div>
        <RoleGate require={["owner", "admin"]}>
          <h3 style={{ margin: "0 0 10px" }}>Tugaskan Staf</h3>
          <div style={{ display: "grid", gap: 10 }}>
            <select value={assignMemberId} onChange={(e) => setAssignMemberId(e.target.value)} style={input}>
              <option value="">Pilih anggota tim</option>
              {members.map((m) => <option key={m.userId} value={m.userId}>{m.name} ({m.email})</option>)}
            </select>
            <select value={assignRole} onChange={(e) => setAssignRole(e.target.value)} style={input}>
              {Object.entries(ROLE_LABEL).map(([value, label]) => <option key={value} value={value}>{label}</option>)}
            </select>
            <input value={assignDuties} onChange={(e) => setAssignDuties(e.target.value)} placeholder="Deskripsi tugas (opsional)" style={input} />
            <button disabled={assigning || !assignMemberId} onClick={assign} style={emerald}>{assigning ? "Menugaskan..." : "Tugaskan"}</button>
          </div>
        </RoleGate>
      </aside>
    </div>}
  </main>;
}

const page: React.CSSProperties = { maxWidth: 1200, margin: "0 auto", padding: "32px 24px" };
const header: React.CSSProperties = { display: "flex", justifyContent: "space-between", gap: 24, alignItems: "flex-start", flexWrap: "wrap" };
const eyebrow: React.CSSProperties = { color: "var(--color-gold-800)", fontSize: 11, fontWeight: 700, letterSpacing: ".08em", margin: "4px 0 8px" };
const title: React.CSSProperties = { fontSize: "clamp(32px,5vw,48px)", fontWeight: 500, margin: 0 };
const select: React.CSSProperties = { minHeight: 48, border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 12px", background: "var(--color-cream-200)", font: "inherit", color: "var(--color-warm-900)" };
const grid: React.CSSProperties = { display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(220px, 1fr))", gap: 16, marginTop: 20 };
const kloterCard: React.CSSProperties = { textAlign: "start", background: "white", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 18 };
const overlay: React.CSSProperties = { position: "fixed", inset: 0, zIndex: 30, display: "flex", justifyContent: "flex-end", background: "rgba(15,23,42,.48)" };
const drawer: React.CSSProperties = { width: "min(440px,100%)", height: "100vh", overflowY: "auto", background: "var(--color-cream-100)", padding: 24 };
const ghost: React.CSSProperties = { minHeight: 40, border: "1px solid var(--color-cream-400)", borderRadius: 8, background: "transparent", color: "var(--color-warm-500)", padding: "0 14px" };
const staffRow: React.CSSProperties = { display: "flex", justifyContent: "space-between", alignItems: "center", gap: 10, border: "1px solid var(--color-cream-300)", borderRadius: 8, padding: "10px 14px", background: "white" };
const deleteBtn: React.CSSProperties = { minHeight: 32, minWidth: 32, border: "1px solid var(--color-danger-600)", borderRadius: 8, background: "transparent", color: "var(--color-danger-600)", display: "grid", placeItems: "center", flexShrink: 0 };
const input: React.CSSProperties = { minHeight: 44, width: "100%", border: "1px solid var(--color-cream-500)", borderRadius: 8, padding: "10px 12px", background: "white", color: "var(--color-warm-900)", font: "inherit" };
const emerald: React.CSSProperties = { minHeight: 44, border: 0, borderRadius: 8, background: "var(--color-emerald-900)", color: "white", fontWeight: 700, padding: "0 16px" };
