"use client";

import { useEffect, useMemo, useState } from "react";
import { IconGenderFemale, IconGenderMale, IconSearch, IconUsersGroup, IconUserMinus, IconUserPlus } from "@tabler/icons-react";
import { Gender, Pilgrim } from "@hajj-saas/proto-gen/hajj/v1/pilgrim_pb";
import { VehicleManifest } from "@hajj-saas/proto-gen/hajj/v1/transport_pb";
import { pilgrimClient, transportClient } from "@/lib/rpc";

type Props = { vehicleId: string; seasonId: string; movementStatus: string; open: boolean; onClose: () => void; onChanged: () => void };

export default function VehicleManifestPanel({ vehicleId, seasonId, movementStatus, open, onClose, onChanged }: Props) {
  const [manifest, setManifest] = useState<VehicleManifest>();
  const [pilgrims, setPilgrims] = useState<Pilgrim[]>([]);
  const [query, setQuery] = useState("");
  const [term, setTerm] = useState("");
  const [seatNumbers, setSeatNumbers] = useState<Record<string, string>>({});
  const [notice, setNotice] = useState("");
  const [workingId, setWorkingId] = useState("");

  const refresh = async () => {
    try { setManifest(await transportClient.getVehicleManifest({ vehicleId })); }
    catch { setNotice("Unable to load this vehicle manifest."); }
  };

  useEffect(() => {
    if (!open || !vehicleId) return;
    setNotice(""); setQuery(""); setTerm(""); setSeatNumbers({});
    void refresh();
    if (seasonId) pilgrimClient.listPilgrims({ seasonId, limit: 20, offset: 0 }).then((response) => setPilgrims(response.pilgrims)).catch(() => setNotice("Unable to load pilgrims."));
  }, [open, vehicleId, seasonId]);
  useEffect(() => { const timeout = window.setTimeout(() => setTerm(query), 300); return () => window.clearTimeout(timeout); }, [query]);

  const occupants = manifest?.pilgrims ?? [];
  const vehicle = manifest?.vehicle;
	const full = vehicle ? occupants.length >= vehicle.capacity : false;
  const canAssign = vehicle?.status === "scheduled" && movementStatus === "scheduled";
  const candidates = useMemo(() => pilgrims.filter((pilgrim) => !pilgrim.isSubstituted && !occupants.some((occupant) => occupant.id === pilgrim.id) && `${pilgrim.fullName} ${pilgrim.passportNumber}`.toLowerCase().includes(term.toLowerCase())), [occupants, pilgrims, term]);
  const groupsWithCandidates = useMemo(() => {
    const byGroup: Record<string, typeof candidates> = {};
    for (const pilgrim of candidates) if (pilgrim.groupId) (byGroup[pilgrim.groupId] ??= []).push(pilgrim);
    return Object.entries(byGroup).filter(([, members]) => members.length > 1);
  }, [candidates]);
  if (!open) return null;

  async function assignGroup(members: typeof candidates) {
    setNotice("");
    const remainingCapacity = (vehicle?.capacity ?? 0) - occupants.length;
    const toAssign = members.slice(0, Math.max(remainingCapacity, 0));
    if (!toAssign.length) { setNotice("Vehicle is full"); return; }
    setWorkingId("group");
    try {
      let nextSeat = occupants.reduce((max, p) => Math.max(max, p.seatNumber), 0) + 1;
      for (const pilgrim of toAssign) { await transportClient.assignSeat({ vehicleId, pilgrimId: pilgrim.id, seatNumber: nextSeat }); nextSeat += 1; }
      await refresh();
      onChanged();
      if (toAssign.length < members.length) setNotice(`Vehicle filled — assigned ${toAssign.length} of ${members.length} in this group.`);
    } catch (caught) {
      setNotice(caught instanceof Error ? caught.message : "Unable to assign this group.");
    } finally {
      setWorkingId("");
    }
  }

  async function updateStatus(status: string) {
    setWorkingId("status"); setNotice("");
    try { await transportClient.updateVehicleStatus({ vehicleId, status }); await refresh(); onChanged(); }
    catch (caught) { setNotice(caught instanceof Error ? caught.message : "Unable to update vehicle status."); }
    finally { setWorkingId(""); }
  }
  async function remove(pilgrimId: string) {
    setWorkingId(pilgrimId); setNotice("");
    try { await transportClient.unassignSeat({ vehicleId, pilgrimId }); await refresh(); onChanged(); }
    catch (caught) { setNotice(caught instanceof Error ? caught.message : "Unable to remove pilgrim."); }
    finally { setWorkingId(""); }
  }
  async function assign(pilgrimId: string) {
    setWorkingId(pilgrimId); setNotice("");
    const seatNumber = Number(seatNumbers[pilgrimId]) || 0;
    try { await transportClient.assignSeat({ vehicleId, pilgrimId, seatNumber }); await refresh(); onChanged(); }
    catch (caught) {
      const message = caught instanceof Error ? caught.message : "Unable to assign pilgrim.";
      if (/resource_exhausted|full|capacity/i.test(message)) setNotice("Vehicle is full");
      else if (/failed_precondition|seat .*taken/i.test(message)) setNotice(seatNumber ? `Seat ${seatNumber} is already taken` : message);
      else setNotice(message);
    } finally { setWorkingId(""); }
  }

  return <div role="dialog" aria-modal="true" aria-label="Vehicle manifest" style={overlay}><aside style={sheet}>
    <header style={header}><div><p style={eyebrow}>VEHICLE MANIFEST</p><h2 style={{ margin: 0 }}>{vehicle?.plateNumber ?? "Vehicle"}</h2><p style={{ margin: "6px 0 0", color: "var(--color-warm-500)" }}>{vehicle?.driverName || "Driver not set"} · {occupants.length}/{vehicle?.capacity ?? 0} assigned</p></div><button onClick={onClose} style={secondary}>Close</button></header>
    <div className="gold-divider" />
    {vehicle && <div style={statusRow}><span style={badge(vehicle.status)}>{vehicle.status}</span>{vehicle.status === "scheduled" && <><button disabled={workingId === "status"} onClick={() => void updateStatus("departed")} style={emerald}>Mark Departed</button><button disabled={workingId === "status"} onClick={() => void updateStatus("cancelled")} style={dangerOutline}>Cancel</button></>}{vehicle.status === "departed" && <button disabled={workingId === "status"} onClick={() => void updateStatus("arrived")} style={emerald}>Mark Arrived</button>}</div>}
    <section style={section}><h3 style={sectionTitle}>Assigned pilgrims</h3>{occupants.length ? occupants.map((pilgrim) => <div key={pilgrim.id} style={person}><div><strong>{pilgrim.fullName}</strong><span style={meta}>{genderIcon(pilgrim.gender)} {pilgrim.passportNumber} · Seat {pilgrim.seatNumber || "-"}</span></div><button disabled={workingId === pilgrim.id} onClick={() => void remove(pilgrim.id)} style={dangerOutline}><IconUserMinus size={17} />Remove</button></div>) : <p style={emptyText}>No pilgrims assigned yet.</p>}</section>
    {!full && canAssign && !!groupsWithCandidates.length && <section style={section}><h3 style={sectionTitle}>Assign whole group</h3>{groupsWithCandidates.map(([groupId, members]) => <div key={groupId} style={person}><div><strong><IconUsersGroup size={16} style={{ verticalAlign: "-3px", marginRight: 4 }} />Group {groupId.slice(0, 8)}</strong><span style={meta}>{members.length} unassigned pilgrims</span></div><button disabled={workingId === "group"} onClick={() => void assignGroup(members)} style={emerald}><IconUserPlus size={17} />Assign group</button></div>)}</section>}
    {!full && canAssign && <section style={section}><h3 style={sectionTitle}>Assign pilgrim</h3><label style={{ position: "relative" }}><span style={sr}>Search unassigned pilgrims</span><IconSearch size={18} style={{ position: "absolute", insetInlineStart: 14, top: 15, color: "var(--color-warm-400)" }} /><input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Search name or passport" style={{ ...input, paddingInlineStart: 42 }} /></label>{candidates.map((pilgrim) => <div key={pilgrim.id} style={person}><div><strong>{pilgrim.fullName}</strong><span style={meta}>{genderIcon(pilgrim.gender)} {pilgrim.passportNumber}</span></div><div style={{ display: "flex", gap: 8, alignItems: "center" }}><input aria-label={`Seat number for ${pilgrim.fullName}`} type="number" min="1" max={vehicle?.capacity} value={seatNumbers[pilgrim.id] ?? ""} onChange={(event) => setSeatNumbers((current) => ({ ...current, [pilgrim.id]: event.target.value }))} placeholder="Seat" style={seatInput} /><button disabled={workingId === pilgrim.id} onClick={() => void assign(pilgrim.id)} style={emerald}><IconUserPlus size={17} />Assign</button></div></div>)}{!candidates.length && <p style={emptyText}>No unassigned pilgrims match this search.</p>}</section>}
    {full && canAssign && <p role="status" style={fullNotice}>Vehicle is full</p>}
    {!canAssign && vehicle && vehicle.status !== "scheduled" && <p role="status" style={fullNotice}>This vehicle is {vehicle.status} — assignments are locked.</p>}
    {!canAssign && vehicle?.status === "scheduled" && movementStatus !== "scheduled" && <p role="status" style={fullNotice}>This movement is {movementStatus} — assignments are locked.</p>}
    {notice && <p role="alert" style={noticeStyle}>{notice}</p>}
  </aside></div>;
}

function genderIcon(gender: Gender | string) { return gender === Gender.FEMALE || gender === "FEMALE" ? <IconGenderFemale size={15} /> : <IconGenderMale size={15} />; }
const overlay: React.CSSProperties = { position: "fixed", inset: 0, zIndex: 30, display: "flex", justifyContent: "flex-end", background: "rgba(26,20,16,.52)" };
const sheet: React.CSSProperties = { width: "min(600px,100%)", minHeight: "100%", overflowY: "auto", background: "var(--color-cream-100)", padding: 24, boxShadow: "0 8px 24px rgba(26,20,16,.12)", animation: "sheet-in .2s ease-out" };
const header: React.CSSProperties = { display: "flex", justifyContent: "space-between", gap: 16, alignItems: "flex-start" };
const eyebrow: React.CSSProperties = { margin: "0 0 6px", color: "var(--color-gold-800)", fontSize: 11, fontWeight: 700, letterSpacing: ".08em" };
const section: React.CSSProperties = { display: "grid", gap: 12, marginTop: 24 };
const sectionTitle: React.CSSProperties = { margin: 0, fontSize: 20, fontFamily: "'Playfair Display', serif", color: "var(--color-emerald-900)" };
const statusRow: React.CSSProperties = { display: "flex", alignItems: "center", flexWrap: "wrap", gap: 8, marginTop: 16 };
const button: React.CSSProperties = { minHeight: 48, borderRadius: 8, padding: "0 12px", fontWeight: 700, display: "inline-flex", alignItems: "center", justifyContent: "center", gap: 6 };
const emerald: React.CSSProperties = { ...button, border: 0, background: "var(--color-emerald-900)", color: "var(--color-cream-100)" };
const secondary: React.CSSProperties = { ...button, border: "1px solid var(--color-cream-500)", background: "transparent", color: "var(--color-warm-500)" };
const dangerOutline: React.CSSProperties = { ...button, border: "1px solid var(--color-danger-600)", background: "transparent", color: "var(--color-danger-600)" };
const person: React.CSSProperties = { display: "flex", justifyContent: "space-between", alignItems: "center", gap: 12, padding: 12, border: "1px solid var(--color-cream-300)", borderRadius: 8, background: "var(white)" };
const meta: React.CSSProperties = { display: "flex", alignItems: "center", gap: 4, marginTop: 3, color: "var(--color-warm-400)", fontFamily: "ui-monospace, monospace", fontSize: 12 };
const input: React.CSSProperties = { minHeight: 48, width: "100%", border: "1px solid var(--color-cream-500)", borderRadius: 8, padding: "10px 12px", background: "var(--color-cream-200)", color: "var(--color-warm-900)", font: "inherit" };
const seatInput: React.CSSProperties = { minHeight: 48, width: 74, border: "1px solid var(--color-cream-500)", borderRadius: 8, padding: "0 8px", background: "var(--color-cream-200)", color: "var(--color-warm-900)", font: "inherit" };
const fullNotice: React.CSSProperties = { marginTop: 24, padding: 12, borderRadius: 8, background: "var(--color-gold-50)", color: "var(--color-gold-800)" };
const noticeStyle: React.CSSProperties = { marginTop: 20, color: "var(--color-danger-600)", fontWeight: 600 };
const emptyText: React.CSSProperties = { margin: 0, color: "var(--color-warm-400)" };
const sr: React.CSSProperties = { position: "absolute", width: 1, height: 1, overflow: "hidden", clip: "rect(0 0 0 0)" };
function badge(status: string): React.CSSProperties { return { padding: "5px 10px", borderRadius: 99, background: status === "arrived" ? "var(--color-emerald-50)" : status === "departed" ? "var(--color-cream-200)" : status === "cancelled" ? "var(--color-danger-100)" : "var(--color-cream-300)", color: "var(--color-warm-500)", fontSize: 12, fontWeight: 700, textTransform: "capitalize" }; }
