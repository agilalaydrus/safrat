"use client";

import { useEffect, useState } from "react";
import { IconPlus, IconTrash } from "@tabler/icons-react";
import { VendorContract, ContractEvent } from "@hajj-saas/proto-gen/hajj/v1/vendor_pb";
import { vendorClient, seasonClient } from "@/lib/rpc";
import { RoleGate } from "@/components/auth/RoleGate";

const VENDOR_TYPE_LABEL: Record<string, string> = { HOTEL: "Hotel", TRANSPORT: "Transportasi", CATERING: "Katering", VISA_AGENT: "Agen Visa", INSURANCE: "Asuransi", OTHER: "Lainnya" };
const STATUS_LABEL: Record<string, string> = { NEGOTIATING: "Negosiasi", CONFIRMED: "Terkonfirmasi", PARTIAL: "Sebagian", CANCELLED: "Dibatalkan" };
const HEALTH_LABEL: Record<string, string> = { ON_TRACK: "Sesuai Jadwal", AT_RISK: "Berisiko", OVERDUE: "Terlambat", PENDING: "Menunggu" };
const HEALTH_COLOR: Record<string, string> = { ON_TRACK: "var(--color-emerald-800)", AT_RISK: "var(--color-gold-800)", OVERDUE: "var(--color-danger-600)", PENDING: "var(--color-warm-400)" };

function formatIDR(value: bigint | number | string): string {
  return new Intl.NumberFormat("id-ID", { style: "currency", currency: "IDR", maximumFractionDigits: 0 }).format(Number(value));
}

export default function VendorsDashboard() {
  const [seasons, setSeasons] = useState<{ id: string; name: string; isActive: boolean }[]>([]);
  const [seasonId, setSeasonId] = useState("");
  const [tab, setTab] = useState<"sla" | "contracts" | "events">("sla");
  const [contracts, setContracts] = useState<VendorContract[]>([]);
  const [slaContracts, setSlaContracts] = useState<VendorContract[]>([]);
  const [loading, setLoading] = useState(true);
  const [notice, setNotice] = useState("");
  const [formOpen, setFormOpen] = useState(false);
  const [form, setForm] = useState({ vendorName: "", vendorType: "HOTEL", contractNumber: "", committedUnits: "", confirmationDeadline: "", ratePerUnit: "", depositAmount: "", contactName: "", contactPhone: "", notes: "" });
  const [saving, setSaving] = useState(false);
  const [selectedContract, setSelectedContract] = useState<VendorContract>();
  const [events, setEvents] = useState<ContractEvent[]>([]);

  useEffect(() => {
    seasonClient.listSeasons({}).then((response) => {
      setSeasons(response.seasons);
      setSeasonId(response.seasons.find((s) => s.isActive)?.id ?? response.seasons[0]?.id ?? "");
    }).catch(() => setNotice("Gagal memuat data musim."));
  }, []);

  const refresh = () => {
    if (!seasonId) return;
    setLoading(true);
    Promise.all([
      vendorClient.listVendorContracts({ seasonId }),
      vendorClient.getVendorSLAStatus({ seasonId }),
    ]).then(([contractsResponse, slaResponse]) => {
      setContracts(contractsResponse.contracts);
      setSlaContracts(slaResponse.contracts);
    }).catch(() => setNotice("Gagal memuat data vendor.")).finally(() => setLoading(false));
  };
  useEffect(refresh, [seasonId]);

  const activeName = seasons.find((s) => s.id === seasonId)?.name ?? "Pilih musim";

  const addContract = async () => {
    if (!form.vendorName.trim()) { setNotice("Nama vendor wajib diisi."); return; }
    setSaving(true);
    setNotice("");
    try {
      await vendorClient.createVendorContract({
        seasonId, vendorName: form.vendorName.trim(), vendorType: form.vendorType, contractNumber: form.contractNumber,
        committedUnits: Number(form.committedUnits) || 0, confirmationDeadline: form.confirmationDeadline,
        ratePerUnitIdr: BigInt(Math.round(Number(form.ratePerUnit)) || 0), depositAmountIdr: BigInt(Math.round(Number(form.depositAmount)) || 0),
        notes: form.notes, contactName: form.contactName, contactPhone: form.contactPhone,
      });
      setForm({ vendorName: "", vendorType: "HOTEL", contractNumber: "", committedUnits: "", confirmationDeadline: "", ratePerUnit: "", depositAmount: "", contactName: "", contactPhone: "", notes: "" });
      setFormOpen(false);
      refresh();
    } catch (error) {
      setNotice(`Gagal menyimpan: ${error instanceof Error ? error.message : "tidak diketahui"}`);
    } finally {
      setSaving(false);
    }
  };

  const updateConfirmedUnits = async (contract: VendorContract, confirmedUnits: number) => {
    try {
      await vendorClient.updateVendorContract({
        id: contract.id, vendorName: contract.vendorName, confirmedUnits, confirmationDeadline: contract.confirmationDeadline,
        status: contract.status, notes: contract.notes, depositPaid: contract.depositPaid, contactName: contract.contactName, contactPhone: contract.contactPhone,
      });
      refresh();
    } catch (error) {
      setNotice(`Gagal: ${error instanceof Error ? error.message : "tidak diketahui"}`);
    }
  };

  const updateStatus = async (contract: VendorContract, status: string) => {
    try {
      await vendorClient.updateVendorContract({
        id: contract.id, vendorName: contract.vendorName, confirmedUnits: contract.confirmedUnits, confirmationDeadline: contract.confirmationDeadline,
        status, notes: contract.notes, depositPaid: contract.depositPaid, contactName: contract.contactName, contactPhone: contract.contactPhone,
      });
      refresh();
    } catch (error) {
      setNotice(`Gagal: ${error instanceof Error ? error.message : "tidak diketahui"}`);
    }
  };

  const toggleDepositPaid = async (contract: VendorContract) => {
    try {
      await vendorClient.updateVendorContract({
        id: contract.id, vendorName: contract.vendorName, confirmedUnits: contract.confirmedUnits, confirmationDeadline: contract.confirmationDeadline,
        status: contract.status, notes: contract.notes, depositPaid: !contract.depositPaid, contactName: contract.contactName, contactPhone: contract.contactPhone,
      });
      refresh();
    } catch (error) {
      setNotice(`Gagal: ${error instanceof Error ? error.message : "tidak diketahui"}`);
    }
  };

  const remove = async (contract: VendorContract) => {
    if (!window.confirm(`Hapus kontrak dengan ${contract.vendorName}?`)) return;
    try {
      await vendorClient.deleteVendorContract({ id: contract.id });
      refresh();
    } catch (error) {
      setNotice(`Gagal menghapus: ${error instanceof Error ? error.message : "tidak diketahui"}`);
    }
  };

  const openEvents = async (contract: VendorContract) => {
    setSelectedContract(contract);
    setTab("events");
    try {
      const response = await vendorClient.listContractEvents({ contractId: contract.id });
      setEvents(response.events);
    } catch {
      setNotice("Gagal memuat riwayat kontrak.");
    }
  };

  return <main style={page}>
    <header style={header}>
      <div><p style={eyebrow}>OPERASIONAL / VENDOR</p><h1 style={title}>Vendor &amp; Kontrak</h1><p style={{ color: "var(--color-warm-500)", margin: 0 }}>{contracts.length} kontrak · {slaContracts.length} dalam pantauan SLA · {activeName}</p></div>
      <div style={{ display: "flex", gap: 10, flexWrap: "wrap" }}>
        <select aria-label="Musim" value={seasonId} onChange={(e) => setSeasonId(e.target.value)} style={select}>
          {seasons.length ? seasons.map((s) => <option key={s.id} value={s.id}>{s.name}{s.isActive ? " · Aktif" : ""}</option>) : <option>{activeName}</option>}
        </select>
        <RoleGate require={["owner", "admin"]}>
          <button onClick={() => setFormOpen((v) => !v)} style={outline}><IconPlus size={18} />Tambah Kontrak</button>
        </RoleGate>
      </div>
    </header>
    <div className="gold-divider" />
    {notice && <p role="status" style={{ color: "var(--color-gold-800)" }}>{notice}</p>}

    <div style={tabBar}>
      <button onClick={() => setTab("sla")} style={tab === "sla" ? tabActive : tabInactive}>Ringkasan SLA</button>
      <button onClick={() => setTab("contracts")} style={tab === "contracts" ? tabActive : tabInactive}>Daftar Kontrak</button>
      <button onClick={() => setTab("events")} style={tab === "events" ? tabActive : tabInactive}>Riwayat Peristiwa</button>
    </div>

    {formOpen && <section style={card}>
      <h2 style={{ margin: "0 0 12px" }}>Kontrak Vendor Baru</h2>
      <div style={formGrid}>
        <label style={field}>Nama Vendor<input value={form.vendorName} onChange={(e) => setForm((f) => ({ ...f, vendorName: e.target.value }))} style={input} /></label>
        <label style={field}>Jenis Vendor
          <select value={form.vendorType} onChange={(e) => setForm((f) => ({ ...f, vendorType: e.target.value }))} style={input}>
            {Object.entries(VENDOR_TYPE_LABEL).map(([value, label]) => <option key={value} value={value}>{label}</option>)}
          </select>
        </label>
        <label style={field}>No. Kontrak<input value={form.contractNumber} onChange={(e) => setForm((f) => ({ ...f, contractNumber: e.target.value }))} style={input} /></label>
        <label style={field}>Unit Terkomitmen<input type="number" min={0} value={form.committedUnits} onChange={(e) => setForm((f) => ({ ...f, committedUnits: e.target.value }))} style={input} /></label>
        <label style={field}>Batas Konfirmasi<input type="date" value={form.confirmationDeadline} onChange={(e) => setForm((f) => ({ ...f, confirmationDeadline: e.target.value }))} style={input} /></label>
        <label style={field}>Harga per Unit (Rp)<input type="number" min={0} value={form.ratePerUnit} onChange={(e) => setForm((f) => ({ ...f, ratePerUnit: e.target.value }))} style={input} /></label>
        <label style={field}>Deposit (Rp)<input type="number" min={0} value={form.depositAmount} onChange={(e) => setForm((f) => ({ ...f, depositAmount: e.target.value }))} style={input} /></label>
        <label style={field}>Kontak<input value={form.contactName} onChange={(e) => setForm((f) => ({ ...f, contactName: e.target.value }))} style={input} /></label>
        <label style={field}>Telepon Kontak<input value={form.contactPhone} onChange={(e) => setForm((f) => ({ ...f, contactPhone: e.target.value }))} style={input} /></label>
      </div>
      <label style={{ ...field, marginTop: 12 }}>Catatan<textarea value={form.notes} onChange={(e) => setForm((f) => ({ ...f, notes: e.target.value }))} rows={2} style={{ ...input, minHeight: 60, resize: "vertical" }} /></label>
      <button disabled={saving} onClick={addContract} style={{ ...emerald, marginTop: 12 }}>{saving ? "Menyimpan..." : "Simpan Kontrak"}</button>
    </section>}

    {loading ? <p style={{ color: "var(--color-warm-500)" }}>Memuat...</p> : <>
      {tab === "sla" && <div style={cardGrid}>
        {slaContracts.map((contract) => <div key={contract.id} style={card}>
          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", gap: 8 }}>
            <h3 style={{ margin: 0 }}>{contract.vendorName}</h3>
            <span style={{ ...healthBadge, background: HEALTH_COLOR[contract.slaHealth] ?? "var(--color-warm-400)" }}>{HEALTH_LABEL[contract.slaHealth] ?? contract.slaHealth}</span>
          </div>
          <p style={{ margin: "4px 0 0", color: "var(--color-warm-500)", fontSize: 13 }}>{VENDOR_TYPE_LABEL[contract.vendorType] ?? contract.vendorType}</p>
          <p style={{ margin: "8px 0 0", fontWeight: 700 }}>{contract.confirmedUnits}/{contract.committedUnits} unit</p>
          {contract.confirmationDeadline && <p style={{ margin: "4px 0 0", fontSize: 13, color: "var(--color-warm-500)" }}>Batas: {contract.confirmationDeadline}</p>}
          <p style={{ margin: "4px 0 0", fontSize: 13, color: contract.depositPaid ? "var(--color-emerald-800)" : "var(--color-danger-600)" }}>{contract.depositPaid ? "Deposit lunas" : "Deposit belum lunas"}</p>
        </div>)}
        {!slaContracts.length && <p style={{ color: "var(--color-warm-500)" }}>Belum ada kontrak yang dapat dipantau pada musim ini. Gunakan Tambah Kontrak di kanan atas untuk mulai mengukur komitmen dan tenggat vendor.</p>}
      </div>}

      {tab === "contracts" && <section style={card}>
        {contracts.length ? <div style={{ overflowX: "auto" }}>
          <table style={table}>
            <thead><tr>{["Vendor", "Jenis", "Unit", "Nilai", "Deposit", "Status", "Aksi"].map((h) => <th key={h} style={th}>{h}</th>)}</tr></thead>
            <tbody>
              {contracts.map((contract) => <tr key={contract.id} style={tr}>
                <td style={td}><strong>{contract.vendorName}</strong>{contract.contractNumber && <span style={{ display: "block", color: "var(--color-warm-400)", fontSize: 12 }}>{contract.contractNumber}</span>}</td>
                <td style={td}>{VENDOR_TYPE_LABEL[contract.vendorType] ?? contract.vendorType}</td>
                <td style={td}>
                  <input type="number" min={0} defaultValue={contract.confirmedUnits} onBlur={(e) => { const value = Number(e.target.value); if (value !== contract.confirmedUnits) updateConfirmedUnits(contract, value); }} style={{ ...input, minHeight: 36, width: 80 }} />
                  <span style={{ color: "var(--color-warm-400)", fontSize: 12 }}> / {contract.committedUnits}</span>
                </td>
                <td style={td}>{formatIDR(contract.totalValueIdr)}</td>
                <td style={td}><button onClick={() => toggleDepositPaid(contract)} style={contract.depositPaid ? depositPaidBtn : depositUnpaidBtn}>{contract.depositPaid ? "Lunas" : "Belum"}</button></td>
                <td style={td}>
                  <select value={contract.status} onChange={(e) => updateStatus(contract, e.target.value)} style={{ ...input, minHeight: 36, fontSize: 13 }}>
                    {Object.entries(STATUS_LABEL).map(([value, label]) => <option key={value} value={value}>{label}</option>)}
                  </select>
                </td>
                <td style={td}>
                  <div style={{ display: "flex", gap: 8 }}>
                    <button onClick={() => openEvents(contract)} style={eventsBtn}>Riwayat</button>
                    <RoleGate require={["owner", "admin"]}><button onClick={() => remove(contract)} aria-label={`Hapus ${contract.vendorName}`} style={deleteBtn}><IconTrash size={14} /></button></RoleGate>
                  </div>
                </td>
              </tr>)}
            </tbody>
          </table>
        </div> : <p style={{ color: "var(--color-warm-500)" }}>Belum ada kontrak vendor pada musim ini. Gunakan Tambah Kontrak di kanan atas untuk mencatat vendor pertama.</p>}
      </section>}

      {tab === "events" && <section style={card}>
        <h2 style={{ margin: "0 0 12px" }}>{selectedContract ? `Riwayat Kontrak ${selectedContract.vendorName}` : "Pilih kontrak dari tab Daftar Kontrak untuk melihat riwayat"}</h2>
        {selectedContract && (events.length ? <div style={{ display: "grid", gap: 10 }}>
          {events.map((event) => <div key={event.id} style={eventRow}>
            <span style={{ fontSize: 11, color: "var(--color-warm-400)", textTransform: "uppercase" }}>{event.eventType}</span>
            <p style={{ margin: "2px 0 0" }}>{event.description}</p>
            <span style={{ fontSize: 12, color: "var(--color-warm-400)" }}>{event.createdAt?.toDate().toLocaleString("id-ID") ?? ""}</span>
          </div>)}
        </div> : <p style={{ color: "var(--color-warm-500)" }}>Belum ada perubahan pada kontrak ini. Ubah unit terkonfirmasi, deposit, atau status di tab Daftar Kontrak agar riwayat tercatat di sini.</p>)}
      </section>}
    </>}
  </main>;
}

const page: React.CSSProperties = { maxWidth: 1200, margin: "0 auto", padding: "32px 24px" };
const header: React.CSSProperties = { display: "flex", justifyContent: "space-between", gap: 24, alignItems: "flex-start", flexWrap: "wrap" };
const eyebrow: React.CSSProperties = { color: "var(--color-gold-800)", fontSize: 11, fontWeight: 700, letterSpacing: ".08em", margin: "4px 0 8px" };
const title: React.CSSProperties = { fontSize: "clamp(32px,5vw,48px)", fontWeight: 500, margin: 0 };
const select: React.CSSProperties = { minHeight: 48, border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 12px", background: "var(--color-cream-200)", font: "inherit", color: "var(--color-warm-900)" };
const emerald: React.CSSProperties = { minHeight: 48, border: 0, borderRadius: 8, background: "var(--color-emerald-900)", color: "white", fontWeight: 700, padding: "0 18px", display: "inline-flex", gap: 8, alignItems: "center" };
const outline: React.CSSProperties = { ...emerald, border: "1px solid var(--color-emerald-700)", borderRadius: 12, background: "transparent", color: "var(--color-emerald-900)" };
const tabBar: React.CSSProperties = { display: "flex", gap: 8, margin: "16px 0 20px" };
const tabActive: React.CSSProperties = { minHeight: 44, border: 0, borderRadius: 8, background: "var(--color-emerald-900)", color: "white", fontWeight: 700, padding: "0 18px" };
const tabInactive: React.CSSProperties = { minHeight: 44, border: "1px solid var(--color-cream-400)", borderRadius: 8, background: "var(--color-cream-200)", color: "var(--color-warm-700)", fontWeight: 600, padding: "0 18px" };
const card: React.CSSProperties = { background: "white", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 20, marginBottom: 20 };
const cardGrid: React.CSSProperties = { display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(240px, 1fr))", gap: 16 };
const formGrid: React.CSSProperties = { display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(200px, 1fr))", gap: 12 };
const field: React.CSSProperties = { display: "grid", gap: 6, color: "var(--color-warm-500)", fontSize: 14 };
const input: React.CSSProperties = { minHeight: 44, width: "100%", border: "1px solid var(--color-cream-500)", borderRadius: 8, padding: "10px 12px", background: "var(--color-cream-200)", color: "var(--color-warm-900)", font: "inherit" };
const healthBadge: React.CSSProperties = { color: "white", fontWeight: 700, fontSize: 11, borderRadius: 999, padding: "4px 10px", flexShrink: 0 };
const table: React.CSSProperties = { width: "100%", borderCollapse: "collapse", minWidth: 800 };
const th: React.CSSProperties = { background: "var(--color-cream-200)", padding: "12px 16px", textAlign: "start", fontSize: 11, textTransform: "uppercase", letterSpacing: ".08em", color: "var(--color-warm-400)" };
const tr: React.CSSProperties = { borderTop: "1px solid var(--color-cream-300)" };
const td: React.CSSProperties = { padding: "12px 16px", fontSize: 14 };
const deleteBtn: React.CSSProperties = { minHeight: 32, minWidth: 32, border: "1px solid var(--color-danger-600)", borderRadius: 8, background: "transparent", color: "var(--color-danger-600)", display: "grid", placeItems: "center" };
const eventsBtn: React.CSSProperties = { minHeight: 32, border: "1px solid var(--color-cream-400)", borderRadius: 8, background: "transparent", color: "var(--color-warm-700)", fontSize: 12, padding: "0 10px" };
const depositPaidBtn: React.CSSProperties = { minHeight: 32, border: "1px solid var(--color-emerald-800)", borderRadius: 8, background: "var(--color-emerald-50)", color: "var(--color-emerald-900)", fontSize: 12, padding: "0 10px", fontWeight: 700 };
const depositUnpaidBtn: React.CSSProperties = { minHeight: 32, border: "1px solid var(--color-danger-600)", borderRadius: 8, background: "transparent", color: "var(--color-danger-600)", fontSize: 12, padding: "0 10px", fontWeight: 700 };
const eventRow: React.CSSProperties = { border: "1px solid var(--color-cream-300)", borderRadius: 8, padding: "10px 14px", background: "var(--color-cream-100)" };
