"use client";

import { useEffect, useMemo, useState } from "react";
import { InsuranceClaim, InsuranceClaimExportData } from "@hajj-saas/proto-gen/hajj/v1/insurance_pb";
import { Pilgrim } from "@hajj-saas/proto-gen/hajj/v1/pilgrim_pb";
import { insuranceClient, pilgrimClient, seasonClient } from "@/lib/rpc";
import { RoleGate } from "@/components/auth/RoleGate";

const STATUS_LABEL: Record<string, string> = { FILED: "Diajukan", SUBMITTED: "Terkirim", PROCESSING: "Diproses", SETTLED: "Selesai", REJECTED: "Ditolak" };
const STATUS_COLOR: Record<string, string> = {
  FILED: "var(--color-warm-500)", SUBMITTED: "var(--color-gold-800)", PROCESSING: "var(--color-gold-800)", SETTLED: "var(--color-emerald-900)", REJECTED: "var(--color-danger-600)",
};
const CLAIM_TYPE_LABEL: Record<string, string> = { MEDICAL: "Medis", DEATH: "Meninggal Dunia", FLIGHT: "Penerbangan", BAGGAGE: "Bagasi", OTHER: "Lainnya" };

function formatIDR(value: number): string {
  return new Intl.NumberFormat("id-ID", { style: "currency", currency: "IDR", maximumFractionDigits: 0 }).format(value);
}

export default function InsuranceDashboard() {
  const [tab, setTab] = useState<"claims" | "export">("claims");
  const [claims, setClaims] = useState<InsuranceClaim[]>([]);
  const [loading, setLoading] = useState(true);
  const [notice, setNotice] = useState("");

  const [seasonId, setSeasonId] = useState("");
  const [pilgrims, setPilgrims] = useState<Pilgrim[]>([]);
  const [search, setSearch] = useState("");
  const [selectedPilgrim, setSelectedPilgrim] = useState<Pilgrim>();
  const [exportData, setExportData] = useState<InsuranceClaimExportData>();
  const [showClaimForm, setShowClaimForm] = useState(false);
  const [claimType, setClaimType] = useState("MEDICAL");
  const [incidentDate, setIncidentDate] = useState("");
  const [description, setDescription] = useState("");
  const [claimAmount, setClaimAmount] = useState("0");
  const [filing, setFiling] = useState(false);

  const refresh = () => {
    setLoading(true);
    insuranceClient.listInsuranceClaims({}).then((response) => setClaims(response.claims)).catch(() => setNotice("Gagal memuat data klaim.")).finally(() => setLoading(false));
  };
  useEffect(refresh, []);

  useEffect(() => {
    seasonClient.listSeasons({}).then((response) => {
      setSeasonId(response.seasons.find((s) => s.isActive)?.id ?? response.seasons[0]?.id ?? "");
    }).catch(() => {});
  }, []);

  useEffect(() => {
    if (!seasonId) return;
    pilgrimClient.listPilgrims({ seasonId, limit: 500, offset: 0 }).then((response) => setPilgrims(response.pilgrims)).catch(() => {});
  }, [seasonId]);

  const filteredPilgrims = useMemo(() => {
    if (!search.trim()) return [];
    const term = search.toLowerCase();
    return pilgrims.filter((p) => p.fullName.toLowerCase().includes(term) || p.passportNumber.toLowerCase().includes(term)).slice(0, 8);
  }, [pilgrims, search]);

  const pickPilgrim = (pilgrim: Pilgrim) => {
    setSelectedPilgrim(pilgrim);
    setSearch(pilgrim.fullName);
    setExportData(undefined);
  };

  const updateStatus = async (claim: InsuranceClaim, status: string) => {
    try {
      const settledAmountIdr = status === "SETTLED" ? BigInt(claim.claimAmountIdr) : BigInt(0);
      await insuranceClient.updateInsuranceClaimStatus({ id: claim.id, status, settledAmountIdr });
      refresh();
    } catch (error) {
      setNotice(`Gagal memperbarui status: ${error instanceof Error ? error.message : "tidak diketahui"}`);
    }
  };

  const fileClaim = async () => {
    if (!selectedPilgrim || !incidentDate || !description.trim()) return;
    setFiling(true);
    try {
      await insuranceClient.createInsuranceClaim({
        pilgrimId: selectedPilgrim.id, claimType, incidentDate, description, claimAmountIdr: BigInt(claimAmount || "0"),
      });
      setShowClaimForm(false);
      setIncidentDate("");
      setDescription("");
      setClaimAmount("0");
      refresh();
      setNotice("Klaim berhasil diajukan.");
    } catch (error) {
      setNotice(`Gagal mengajukan klaim: ${error instanceof Error ? error.message : "tidak diketahui"}`);
    } finally {
      setFiling(false);
    }
  };

  const loadExport = async (claimId: string) => {
    try {
      const data = await insuranceClient.getInsuranceClaimExportData({ id: claimId });
      setExportData(data);
    } catch (error) {
      setNotice(`Gagal memuat data ekspor: ${error instanceof Error ? error.message : "tidak diketahui"}`);
    }
  };

  return <main style={page}>
    <header>
      <p style={eyebrow}>PERLINDUNGAN / ASURANSI</p>
      <h1 style={title}>Klaim Asuransi</h1>
      <p style={{ color: "var(--color-warm-500)", margin: 0 }}>{claims.length} klaim · {claims.filter((claim) => !["SETTLED", "REJECTED"].includes(claim.status)).length} belum selesai</p>
    </header>
    <div className="gold-divider" />
    {notice && <p role="status" style={{ color: "var(--color-gold-800)" }}>{notice}</p>}

    <div style={tabRow}>
      <button onClick={() => setTab("claims")} style={tab === "claims" ? tabActive : tabInactive}>Klaim Aktif</button>
      <button onClick={() => setTab("export")} style={tab === "export" ? tabActive : tabInactive}>Ekspor Data Klaim</button>
    </div>

    {tab === "claims" && <div style={{ marginTop: 20 }}>
      <RoleGate require={["owner", "admin"]}>
        <button onClick={() => setShowClaimForm((v) => !v)} style={{ ...emerald, marginBottom: 16 }}>{showClaimForm ? "Batal" : "+ Ajukan Klaim Baru"}</button>
      </RoleGate>
      {showClaimForm && <section style={{ ...card, marginBottom: 20 }}>
        <label style={field}>Cari jamaah
          <input value={search} onChange={(e) => { setSearch(e.target.value); setSelectedPilgrim(undefined); }} placeholder="Cari nama jamaah atau nomor paspor…" style={input} />
        </label>
        {!selectedPilgrim && filteredPilgrims.length > 0 && <div style={{ display: "grid", gap: 4 }}>
          {filteredPilgrims.map((p) => <button key={p.id} onClick={() => pickPilgrim(p)} style={pickerRow}>{p.fullName} · {p.passportNumber}</button>)}
        </div>}
        {selectedPilgrim && <>
          <label style={field}>Jenis klaim
            <select value={claimType} onChange={(e) => setClaimType(e.target.value)} style={input}>
              {Object.entries(CLAIM_TYPE_LABEL).map(([value, label]) => <option key={value} value={value}>{label}</option>)}
            </select>
          </label>
          <label style={field}>Tanggal kejadian
            <input type="date" value={incidentDate} onChange={(e) => setIncidentDate(e.target.value)} style={input} />
          </label>
          <label style={field}>Deskripsi
            <textarea value={description} onChange={(e) => setDescription(e.target.value)} rows={3} style={{ ...input, minHeight: 80, resize: "vertical" }} />
          </label>
          <label style={field}>Estimasi nilai klaim (IDR)
            <input type="number" min={0} value={claimAmount} onChange={(e) => setClaimAmount(e.target.value)} style={input} />
          </label>
          <button disabled={filing} onClick={fileClaim} style={emerald}>{filing ? "Mengajukan..." : "Ajukan Klaim"}</button>
        </>}
      </section>}

      {loading ? <p style={{ color: "var(--color-warm-500)" }}>Memuat...</p> : <div style={{ overflowX: "auto" }}>
        <table style={table}>
          <thead><tr>
            <th style={th}>Jamaah</th><th style={th}>Jenis</th><th style={th}>Tgl. Kejadian</th><th style={th}>Nilai Klaim</th><th style={th}>Status</th><th style={th}></th>
          </tr></thead>
          <tbody>
            {claims.map((claim) => <tr key={claim.id}>
              <td style={td}>{claim.pilgrimName}</td>
              <td style={td}>{CLAIM_TYPE_LABEL[claim.claimType] ?? claim.claimType}</td>
              <td style={td}>{claim.incidentDate}</td>
              <td style={td}>{formatIDR(Number(claim.claimAmountIdr))}</td>
              <td style={td}>
                <RoleGate require={["owner", "admin"]} fallback={<span style={{ ...statusBadge, background: STATUS_COLOR[claim.status] }}>{STATUS_LABEL[claim.status] ?? claim.status}</span>}>
                  <select value={claim.status} onChange={(e) => updateStatus(claim, e.target.value)} style={{ ...input, minHeight: 36, padding: "4px 8px" }}>
                    {Object.entries(STATUS_LABEL).map(([value, label]) => <option key={value} value={value}>{label}</option>)}
                  </select>
                </RoleGate>
              </td>
              <td style={td}><button onClick={() => { setTab("export"); loadExport(claim.id); }} style={ghost}>Ekspor</button></td>
            </tr>)}
          </tbody>
        </table>
        {!claims.length && <p style={{ color: "var(--color-warm-500)", marginTop: 16 }}>Belum ada klaim karena belum ada kejadian yang diajukan. Owner atau admin dapat memakai Ajukan Klaim Baru di atas saat dokumen kejadian sudah siap.</p>}
      </div>}
    </div>}

    {tab === "export" && <div style={{ marginTop: 20 }} className="print-area">
      {!exportData && <p style={{ color: "var(--color-warm-500)" }}>Pilih klaim dari tab &quot;Klaim Aktif&quot; dan klik &quot;Ekspor&quot; untuk menampilkan data lengkap di sini.</p>}
      {exportData && <section style={card}>
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
          <h2 style={{ margin: 0 }}>Data Ekspor Klaim</h2>
          <button onClick={() => window.print()} style={emerald}>Cetak / Simpan PDF</button>
        </div>
        <dl style={dl}>
          <Row label="Nama Lengkap" value={exportData.fullName} />
          <Row label="Nomor Paspor" value={exportData.passportNumber} />
          <Row label="Jenis Kelamin" value={exportData.gender} />
          <Row label="Kewarganegaraan" value={exportData.nationality} />
          <Row label="Telepon" value={exportData.phone} />
          <Row label="Kontak Darurat" value={`${exportData.emergencyContactName} (${exportData.emergencyContactPhone})`} />
          <Row label="Golongan Darah" value={exportData.bloodType} />
          <Row label="Kondisi Medis Kronis" value={exportData.chronicConditions} />
          <Row label="Obat Rutin" value={exportData.currentMedications} />
          <Row label="Penyedia Asuransi" value={exportData.insuranceProvider} />
          <Row label="Nomor Polis" value={exportData.insurancePolicyNo} />
          <Row label="Kelas Asuransi" value={exportData.insuranceClass} />
          <Row label="Musim" value={exportData.seasonName} />
          <Row label="Operator" value={exportData.operatorName} />
          <Row label="Lisensi Operator" value={exportData.operatorLicenseNumber} />
          <Row label="Telepon Operator" value={exportData.operatorPhone} />
        </dl>
        {exportData.claim && <>
          <div className="gold-divider" />
          <h3 style={{ margin: "0 0 10px" }}>Detail Klaim</h3>
          <dl style={dl}>
            <Row label="Jenis Klaim" value={CLAIM_TYPE_LABEL[exportData.claim.claimType] ?? exportData.claim.claimType} />
            <Row label="Tanggal Kejadian" value={exportData.claim.incidentDate} />
            <Row label="Deskripsi" value={exportData.claim.description} />
            <Row label="Nilai Klaim" value={formatIDR(Number(exportData.claim.claimAmountIdr))} />
            <Row label="Status" value={STATUS_LABEL[exportData.claim.status] ?? exportData.claim.status} />
          </dl>
        </>}
      </section>}
    </div>}
  </main>;
}

function Row({ label, value }: { label: string; value: string }) {
  return <div style={{ display: "grid", gridTemplateColumns: "180px 1fr", gap: 8, padding: "6px 0", borderBottom: "1px solid var(--color-cream-300)" }}>
    <dt style={{ color: "var(--color-warm-500)", fontSize: 13 }}>{label}</dt><dd style={{ margin: 0 }}>{value || "-"}</dd>
  </div>;
}

const page: React.CSSProperties = { maxWidth: 1100, margin: "0 auto", padding: "32px 24px" };
const eyebrow: React.CSSProperties = { color: "var(--color-gold-800)", fontSize: 11, fontWeight: 700, letterSpacing: ".08em", margin: "4px 0 8px" };
const title: React.CSSProperties = { fontSize: "clamp(32px,5vw,48px)", fontWeight: 500, margin: 0 };
const tabRow: React.CSSProperties = { display: "flex", gap: 8, marginTop: 20 };
const tabActive: React.CSSProperties = { minHeight: 44, border: "1px solid var(--color-gold-100)", borderRadius: 12, background: "var(--color-gold-50)", color: "var(--color-gold-600)", boxShadow: "0 0 0 4px color-mix(in srgb, var(--color-gold-500) 12%, transparent)", fontWeight: 700, padding: "0 18px" };
const tabInactive: React.CSSProperties = { minHeight: 44, border: "1px solid var(--color-cream-400)", borderRadius: 8, background: "transparent", color: "var(--color-warm-500)", padding: "0 18px" };
const card: React.CSSProperties = { background: "white", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 20, display: "grid", gap: 12 };
const field: React.CSSProperties = { display: "grid", gap: 6, color: "var(--color-warm-500)", fontSize: 14 };
const input: React.CSSProperties = { minHeight: 44, width: "100%", border: "1px solid var(--color-cream-500)", borderRadius: 8, padding: "10px 12px", background: "white", color: "var(--color-warm-900)", font: "inherit" };
const emerald: React.CSSProperties = { minHeight: 44, border: 0, borderRadius: 8, background: "var(--color-emerald-900)", color: "white", fontWeight: 700, padding: "0 16px" };
const ghost: React.CSSProperties = { minHeight: 34, border: "1px solid var(--color-cream-400)", borderRadius: 8, background: "transparent", color: "var(--color-warm-700)", padding: "0 12px" };
const pickerRow: React.CSSProperties = { textAlign: "start", minHeight: 40, border: "1px solid var(--color-cream-300)", borderRadius: 8, background: "var(--color-cream-100)", padding: "0 12px" };
const table: React.CSSProperties = { width: "100%", borderCollapse: "collapse", minWidth: 720 };
const th: React.CSSProperties = { textAlign: "start", fontSize: 12, color: "var(--color-warm-500)", padding: "8px 10px", borderBottom: "1px solid var(--color-cream-400)" };
const td: React.CSSProperties = { padding: "10px", borderBottom: "1px solid var(--color-cream-300)", fontSize: 14 };
const statusBadge: React.CSSProperties = { color: "white", fontWeight: 700, fontSize: 12, borderRadius: 999, padding: "4px 12px" };
const dl: React.CSSProperties = { margin: 0 };
