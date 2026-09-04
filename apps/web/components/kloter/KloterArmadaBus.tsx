"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { IconBus, IconPlus, IconUsers } from "@tabler/icons-react";
import type { ItinerarySegment } from "@hajj-saas/proto-gen/hajj/v1/kloter_pb";
import type { Vehicle } from "@hajj-saas/proto-gen/hajj/v1/transport_pb";
import { kloterClient, transportClient } from "@/lib/rpc";
import VehicleFormDialog from "@/components/transport/VehicleFormDialog";
import VehicleManifestPanel from "@/components/transport/VehicleManifestPanel";

export default function KloterArmadaBus({ kloterId, seasonId }: { kloterId: string; seasonId: string }) {
  const [segments, setSegments] = useState<ItinerarySegment[]>([]);
  const [vehiclesByMovement, setVehiclesByMovement] = useState<Record<string, Vehicle[]>>({});
  const [loading, setLoading] = useState(true);
  const [failure, setFailure] = useState("");
  const [addVehicleFor, setAddVehicleFor] = useState("");
  const [manifestVehicleId, setManifestVehicleId] = useState("");

  const busSegments = useMemo(() => segments.filter((s) => s.segmentType === "TRANSPORT" && s.movementMode === "BUS"), [segments]);

  const load = useCallback(() => {
    setLoading(true);
    kloterClient.listKloterItinerary({ kloterId })
      .then(async (r) => {
        setSegments(r.segments);
        const busMovements = r.segments.filter((s) => s.segmentType === "TRANSPORT" && s.movementMode === "BUS");
        const entries = await Promise.all(busMovements.map(async (s) => {
          const res = await transportClient.listVehicles({ movementId: s.movementId }).catch(() => ({ vehicles: [] }));
          return [s.movementId, res.vehicles] as const;
        }));
        setVehiclesByMovement(Object.fromEntries(entries));
      })
      .catch(() => setFailure("Gagal memuat Armada Bus."))
      .finally(() => setLoading(false));
  }, [kloterId]);

  useEffect(() => { load(); }, [load]);

  if (loading) return null;

  return (
    <section style={card}>
      <h2 style={sectionTitle}>Armada Bus</h2>
      <p style={{ margin: "0 0 12px", fontSize: 13, color: "var(--color-warm-500)" }}>
        Assign jamaah ke bus, tervalidasi terhadap segmen Bus di tab Rangkaian.
      </p>
      {failure && <p style={{ color: "var(--color-danger-600)", fontSize: 13 }}>{failure}</p>}

      {busSegments.length ? (
        <div style={{ display: "grid", gap: 12 }}>
          {busSegments.map((seg) => {
            const vehicles = vehiclesByMovement[seg.movementId] ?? [];
            return (
              <div key={seg.id} style={movementBlock}>
                <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", flexWrap: "wrap", gap: 8 }}>
                  <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
                    <IconBus size={16} />
                    <strong>{seg.movementName}</strong>
                  </div>
                  <button type="button" onClick={() => setAddVehicleFor(seg.movementId)} style={ghostBtn}><IconPlus size={14} /> Tambah Bus</button>
                </div>
                {vehicles.length ? (
                  <div style={{ display: "grid", gap: 6, marginTop: 8 }}>
                    {vehicles.map((v) => (
                      <button key={v.id} type="button" onClick={() => setManifestVehicleId(v.id)} style={vehicleRow}>
                        <span><strong>{v.plateNumber}</strong> · {v.driverName || "Sopir belum ditentukan"}</span>
                        <span style={{ display: "inline-flex", alignItems: "center", gap: 4, color: "var(--color-warm-500)", fontSize: 12 }}>
                          <IconUsers size={13} />{v.assignedCount}/{v.capacity}
                        </span>
                      </button>
                    ))}
                  </div>
                ) : <p style={{ margin: "8px 0 0", fontSize: 13, color: "var(--color-warm-400)" }}>Belum ada bus untuk segmen ini.</p>}
              </div>
            );
          })}
        </div>
      ) : (
        <p style={{ color: "var(--color-danger-600)", fontSize: 13 }}>
          Belum ada segmen Bus di Rangkaian Perjalanan — Tambahkan segmen Bus di tab Rangkaian sebelum mengelola armada.
        </p>
      )}

      <VehicleFormDialog open={!!addVehicleFor} movementId={addVehicleFor} mode="BUS" onClose={() => setAddVehicleFor("")} onSaved={load} />
      <VehicleManifestPanel
        open={!!manifestVehicleId}
        vehicleId={manifestVehicleId}
        seasonId={seasonId}
        movementStatus="scheduled"
        mode="BUS"
        onClose={() => setManifestVehicleId("")}
        onChanged={load}
      />
    </section>
  );
}

const card: React.CSSProperties = { background: "var(--color-cream-200)", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 20, marginTop: 16 };
const sectionTitle: React.CSSProperties = { margin: "0 0 4px", fontSize: 16 };
const movementBlock: React.CSSProperties = { background: "#fff", border: "1px solid var(--color-cream-300)", borderRadius: 8, padding: 12 };
const vehicleRow: React.CSSProperties = { display: "flex", justifyContent: "space-between", alignItems: "center", width: "100%", textAlign: "start", padding: "8px 10px", background: "var(--color-cream-100)", border: "1px solid var(--color-cream-300)", borderRadius: 6, fontSize: 13, color: "inherit" };
const ghostBtn: React.CSSProperties = { minHeight: 32, border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 10px", background: "transparent", color: "var(--color-emerald-900)", fontSize: 12, fontWeight: 600, display: "inline-flex", alignItems: "center", gap: 6 };
