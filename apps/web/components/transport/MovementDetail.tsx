"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { IconDownload, IconPlus } from "@tabler/icons-react";
import { Movement, Vehicle } from "@hajj-saas/proto-gen/hajj/v1/transport_pb";
import { transportClient, pilgrimClient } from "@/lib/rpc";
import { exportCSV } from "@/lib/csv";
import VehicleFormDialog from "./VehicleFormDialog";
import VehicleManifestPanel from "./VehicleManifestPanel";

export default function MovementDetail({ movementId }: { movementId: string }) {
  const [movement, setMovement] = useState<Movement>();
  const [vehicles, setVehicles] = useState<Vehicle[]>([]);
  const [vehicleFormOpen, setVehicleFormOpen] = useState(false);
  const [selectedVehicleId, setSelectedVehicleId] = useState("");
  const [notice, setNotice] = useState("");
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [working, setWorking] = useState(false);
  const router = useRouter();

  const refresh = useCallback(async () => {
    try {
      const [nextMovement, response] = await Promise.all([
        transportClient.getMovement({ movementId }),
        transportClient.listVehicles({ movementId }),
      ]);
      setMovement(nextMovement);
      setVehicles(response.vehicles);
    } catch {
      setNotice("Unable to load this movement.");
    }
  }, [movementId]);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  async function updateStatus(status: string) {
    setWorking(true);
    setNotice("");
    try {
      await transportClient.updateMovementStatus({ movementId, status });
      await refresh();
    } catch (caught) {
      setNotice(messageForStatusError(caught));
    } finally {
      setWorking(false);
    }
  }

  async function deleteMovement() {
    setWorking(true);
    setNotice("");
    try {
      await transportClient.deleteMovement({ movementId });
      router.push("/dashboard/transport");
    } catch (caught) {
      setNotice(caught instanceof Error ? caught.message : "Unable to delete movement.");
      setWorking(false);
    }
  }

  const isScheduled = movement?.status === "scheduled";

  async function exportManifest() {
    setNotice("");
    try {
      const scheduled = movement?.scheduledAt?.toDate().toLocaleString("id-ID", { day: "2-digit", month: "short", year: "numeric", hour: "2-digit", minute: "2-digit" }) ?? "";
      const pilgrimsResponse = movement?.seasonId ? await pilgrimClient.listPilgrims({ seasonId: movement.seasonId, limit: 1000, offset: 0 }) : undefined;
      const pilgrimById = new Map((pilgrimsResponse?.pilgrims ?? []).map((p) => [p.id, p]));
      const headers = ["Movement", "Origin", "Destination", "Scheduled", "Vehicle Plate", "Driver", "Driver Phone", "Seat No.", "Full Name", "Passport No.", "Nationality", "Gender", "Date of Birth", "Phone", "Group Code", "Wheelchair", "Emergency Contact"];
      const rows: string[][] = [];
      for (const vehicle of vehicles) {
        const manifest = await transportClient.getVehicleManifest({ vehicleId: vehicle.id });
        rows.push(...manifest.pilgrims.map((p) => {
          const full = pilgrimById.get(p.id);
          return [movement?.name ?? "", movement?.origin ?? "", movement?.destination ?? "", scheduled, vehicle.plateNumber, vehicle.driverName || "-", vehicle.driverPhone || "-", String(p.seatNumber), p.fullName, p.passportNumber, full?.nationality ?? "", p.gender, full?.dateOfBirth?.toDate().toLocaleDateString("id-ID") ?? "", full?.phone ?? "", full?.groupId || "-", p.requiresWheelchair ? "Yes" : "No", full?.emergencyContact ?? ""];
        }));
      }
      exportCSV(`${movement?.name ?? "movement"}-manifest.csv`, headers, rows);
    } catch (caught) {
      setNotice(caught instanceof Error ? caught.message : "Unable to export manifest.");
    }
  }

  return (
    <main style={page}>
      <Link href="/dashboard/transport" style={backLink}>Back to transport</Link>
      <header style={header}>
        <div>
          <p style={eyebrow}>TRANSPORT / MOVEMENT</p>
          <h1 style={title}>{movement?.name ?? "Movement vehicles"}</h1>
          <p style={{ color: "var(--color-warm-500)" }}>{movement?.origin} → {movement?.destination}</p>
          {movement && <div style={statusRow}>
            <span style={badge(movement.status)}>{movement.status}</span>
            {movement.status === "scheduled" && <>
              <button disabled={working} onClick={() => void updateStatus("departed")} style={emerald}>Mark Departed</button>
              <button disabled={working} onClick={() => void updateStatus("cancelled")} style={dangerOutline}>Cancel</button>
            </>}
            {movement.status === "departed" && <button disabled={working} onClick={() => void updateStatus("arrived")} style={emerald}>Mark Arrived</button>}
          </div>}
        </div>
        <div style={{ display: "flex", gap: 10, alignItems: "flex-start" }}>
          {!!vehicles.length && <button onClick={() => void exportManifest()} style={secondary}><IconDownload size={18} />Export CSV</button>}
          <button disabled={!isScheduled} onClick={() => setVehicleFormOpen(true)} style={emerald}><IconPlus size={18} />Add Vehicle</button>
        </div>
      </header>
      <div className="gold-divider" />
      {notice && <p role="alert" style={noticeStyle}>{notice}</p>}
      <section style={grid} aria-label="Vehicles">
        {vehicles.map((vehicle) => <button key={vehicle.id} type="button" onClick={() => setSelectedVehicleId(vehicle.id)} style={card}>
          <strong>{vehicle.plateNumber}</strong>
          <span style={{ color: "var(--color-warm-500)" }}>{vehicle.driverName || "Driver not set"}</span>
          <span style={{ color: "var(--color-warm-400)", fontSize: 13 }}>{vehicle.assignedCount}/{vehicle.capacity} seats · {vehicle.status}</span>
          <span style={bar}><span style={{ ...fill, width: `${vehicle.capacity ? (vehicle.assignedCount / vehicle.capacity) * 100 : 0}%` }} /></span>
        </button>)}
      </section>
      {isScheduled && <section style={deleteSection}>
        {confirmDelete ? <>
          <p style={{ margin: 0 }}>Delete this movement? This cannot be undone.</p>
          <button disabled={working} onClick={() => void deleteMovement()} style={dangerSolid}>Delete movement</button>
          <button disabled={working} onClick={() => setConfirmDelete(false)} style={secondary}>Keep movement</button>
        </> : <button onClick={() => setConfirmDelete(true)} style={dangerOutline}>Delete movement</button>}
      </section>}
      <VehicleFormDialog open={vehicleFormOpen} movementId={movementId} onClose={() => setVehicleFormOpen(false)} onSaved={() => void refresh()} />
      <VehicleManifestPanel open={Boolean(selectedVehicleId)} vehicleId={selectedVehicleId} seasonId={movement?.seasonId ?? ""} movementStatus={movement?.status ?? ""} onClose={() => setSelectedVehicleId("")} onChanged={() => void refresh()} />
    </main>
  );
}

function messageForStatusError(caught: unknown) {
  const message = caught instanceof Error ? caught.message : "Unable to update status.";
  return /failed_precondition|cannot transition/i.test(message) ? message : message;
}

const page: React.CSSProperties = { maxWidth: 1200, margin: "0 auto", padding: "32px 24px" };
const backLink: React.CSSProperties = { color: "var(--color-emerald-900)", fontWeight: 700, textDecoration: "none" };
const header: React.CSSProperties = { display: "flex", justifyContent: "space-between", gap: 24, flexWrap: "wrap", marginTop: 20 };
const eyebrow: React.CSSProperties = { color: "var(--color-gold-800)", fontSize: 11, fontWeight: 700, letterSpacing: ".08em" };
const title: React.CSSProperties = { margin: 0, fontSize: "clamp(32px,5vw,48px)", fontFamily: "'Playfair Display', serif", color: "var(--color-emerald-900)" };
const statusRow: React.CSSProperties = { display: "flex", alignItems: "center", flexWrap: "wrap", gap: 8, marginTop: 14 };
const button: React.CSSProperties = { minHeight: 48, borderRadius: 8, padding: "0 16px", fontWeight: 700 };
const emerald: React.CSSProperties = { ...button, border: 0, background: "var(--color-emerald-900)", color: "var(--color-cream-100)" };
const secondary: React.CSSProperties = { ...button, border: "1px solid var(--color-cream-500)", background: "transparent", color: "var(--color-warm-500)" };
const dangerOutline: React.CSSProperties = { ...button, border: "1px solid var(--color-danger-600)", background: "transparent", color: "var(--color-danger-600)" };
const dangerSolid: React.CSSProperties = { ...button, border: 0, background: "var(--color-danger-600)", color: "white" };
const grid: React.CSSProperties = { display: "grid", gridTemplateColumns: "repeat(auto-fit,minmax(240px,1fr))", gap: 16 };
const card: React.CSSProperties = { display: "grid", gap: 10, minHeight: 170, textAlign: "start", background: "white", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 20, color: "var(--color-warm-900)" };
const bar: React.CSSProperties = { display: "block", height: 8, borderRadius: 8, overflow: "hidden", background: "var(--color-emerald-200)" };
const fill: React.CSSProperties = { display: "block", height: "100%", background: "var(--color-emerald-900)" };
const deleteSection: React.CSSProperties = { display: "flex", alignItems: "center", flexWrap: "wrap", gap: 12, marginTop: 40, paddingTop: 24, borderTop: "1px solid var(--color-cream-400)" };
const noticeStyle: React.CSSProperties = { margin: "20px 0", color: "var(--color-danger-600)", fontWeight: 600 };
function badge(status: string): React.CSSProperties { return { padding: "5px 10px", borderRadius: 99, background: status === "arrived" ? "var(--color-emerald-50)" : status === "departed" ? "var(--color-cream-200)" : status === "cancelled" ? "var(--color-danger-100)" : "var(--color-cream-300)", color: "var(--color-warm-500)", fontSize: 12, fontWeight: 700, textTransform: "capitalize" }; }
