"use client";

import { useEffect, useMemo, useState } from "react";
import { IconCheck, IconPlus, IconTrash } from "@tabler/icons-react";
import type { AddonItem, PilgrimAddon } from "@hajj-saas/proto-gen/hajj/v1/addon_pb";
import type { Group } from "@hajj-saas/proto-gen/hajj/v1/group_pb";
import { addonClient, groupClient, pilgrimClient, seasonClient } from "@/lib/rpc";
import AssignAddonDialog from "./AssignAddonDialog";

const rupiah = (n: bigint | number) => new Intl.NumberFormat("id-ID", { style: "currency", currency: "IDR", maximumFractionDigits: 0 }).format(Number(n));

export default function AddonDashboard() {
  const [seasons, setSeasons] = useState<{ id: string; name: string; isActive: boolean }[]>([]);
  const [seasonId, setSeasonId] = useState("");
  const [groups, setGroups] = useState<Group[]>([]);
  const [groupId, setGroupId] = useState("");
  const [items, setItems] = useState<AddonItem[]>([]);
  const [pilgrims, setPilgrims] = useState<{ id: string; fullName: string }[]>([]);
  const [assignments, setAssignments] = useState<PilgrimAddon[]>([]);
  const [notice, setNotice] = useState("");
  const [catalogForm, setCatalogForm] = useState({ name: "", unitPriceIdr: "" });
  const [assignOpen, setAssignOpen] = useState(false);

  useEffect(() => {
    seasonClient.listSeasons({}).then((r) => {
      setSeasons(r.seasons);
      setSeasonId(r.seasons.find((s) => s.isActive)?.id ?? r.seasons[0]?.id ?? "");
    }).catch(() => setNotice("Gagal memuat musim."));
  }, []);

  const refresh = () => {
    if (!seasonId) return;
    Promise.all([
      addonClient.listAddonItems({ seasonId }).then((r) => setItems(r.items)),
      addonClient.listPilgrimAddons({ seasonId, groupId }).then((r) => setAssignments(r.addons)),
      groupClient.listGroups({ seasonId }).then((r) => setGroups(r.groups)),
      pilgrimClient.listPilgrims({ seasonId, limit: 1000 }).then((r) => setPilgrims(r.pilgrims.map((p) => ({ id: p.id, fullName: p.fullName })))),
    ]).catch(() => setNotice("Gagal memuat layanan tambahan."));
  };
  useEffect(refresh, [seasonId, groupId]);

  const activeItems = useMemo(() => items.filter((i) => i.isActive), [items]);

  const addCatalogItem = async () => {
    const price = Number(catalogForm.unitPriceIdr);
    if (!catalogForm.name.trim() || !Number.isFinite(price) || price < 0) { setNotice("Nama dan harga satuan wajib diisi."); return; }
    try {
      await addonClient.createAddonItem({ seasonId, name: catalogForm.name.trim(), unitPriceIdr: BigInt(Math.round(price)) });
      setCatalogForm({ name: "", unitPriceIdr: "" });
      refresh();
    } catch (caught) { setNotice(caught instanceof Error ? caught.message : "Gagal menambah layanan."); }
  };

  const toggleActive = async (item: AddonItem) => {
    try { await addonClient.updateAddonItem({ itemId: item.id, name: item.name, unitPriceIdr: item.unitPriceIdr, isActive: !item.isActive }); refresh(); }
    catch (caught) { setNotice(caught instanceof Error ? caught.message : "Gagal mengubah layanan."); }
  };

  const togglePaid = async (a: PilgrimAddon) => {
    try { await addonClient.setPilgrimAddonPaid({ pilgrimAddonId: a.id, paid: !a.paid }); refresh(); }
    catch (caught) { setNotice(caught instanceof Error ? caught.message : "Gagal mengubah status bayar."); }
  };

  const removeAssignment = async (a: PilgrimAddon) => {
    if (!window.confirm(`Hapus "${a.addonName}" dari ${a.pilgrimName}?`)) return;
    try { await addonClient.removePilgrimAddon({ pilgrimAddonId: a.id }); refresh(); }
    catch (caught) { setNotice(caught instanceof Error ? caught.message : "Gagal menghapus."); }
  };

  const totalAll = assignments.reduce((sum, a) => sum + Number(a.totalIdr), 0);
  const totalUnpaid = assignments.filter((a) => !a.paid).reduce((sum, a) => sum + Number(a.totalIdr), 0);

  return (
    <main style={page}>
      <header>
        <p style={eyebrow}>PER JAMAAH</p>
        <h1 style={{ margin: 0, fontSize: 32 }}>Layanan Tambahan</h1>
        <p style={{ margin: "4px 0 0", color: "var(--color-warm-500)" }}>
          Add-on di luar paket: kursi eksekutif, VIP handling, badal umroh, dan sejenisnya.
        </p>
      </header>
      <div style={{ marginTop: 12, display: "flex", gap: 8, flexWrap: "wrap" }}>
        <select value={seasonId} onChange={(e) => setSeasonId(e.target.value)} style={seasonSelect}>
          {seasons.map((s) => <option key={s.id} value={s.id}>{s.name}{s.isActive ? " (aktif)" : ""}</option>)}
        </select>
        {groups.length > 0 && (
          <select value={groupId} onChange={(e) => setGroupId(e.target.value)} style={seasonSelect}>
            <option value="">Semua grup</option>
            {groups.map((g) => <option key={g.id} value={g.id}>{g.name}</option>)}
          </select>
        )}
      </div>
      {notice && <p style={{ color: "var(--color-danger-600)" }}>{notice}</p>}
      <div className="gold-divider" />

      <section style={card}>
        <h2 style={sectionTitle}>Katalog Layanan</h2>
        {items.length ? (
          <div style={{ display: "grid", gap: 6, marginBottom: 12 }}>
            {items.map((i) => (
              <div key={i.id} style={row}>
                <div style={{ flex: 1, minWidth: 0 }}>
                  <strong style={{ opacity: i.isActive ? 1 : 0.5 }}>{i.name}</strong>
                  <p style={{ margin: "2px 0 0", fontSize: 12, color: "var(--color-warm-500)" }}>{rupiah(i.unitPriceIdr)}{!i.isActive && " · nonaktif"}</p>
                </div>
                <button type="button" onClick={() => void toggleActive(i)} style={ghostBtn}>{i.isActive ? "Nonaktifkan" : "Aktifkan"}</button>
              </div>
            ))}
          </div>
        ) : <p style={{ color: "var(--color-warm-400)", fontSize: 13, marginBottom: 12 }}>Belum ada layanan di katalog.</p>}
        <div style={inlineForm}>
          <input placeholder="Nama layanan (mis. Kursi Eksekutif)" value={catalogForm.name} onChange={(e) => setCatalogForm({ ...catalogForm, name: e.target.value })} style={input} />
          <input type="number" placeholder="Harga satuan (Rp)" value={catalogForm.unitPriceIdr} onChange={(e) => setCatalogForm({ ...catalogForm, unitPriceIdr: e.target.value })} style={input} />
          <button type="button" onClick={() => void addCatalogItem()} style={ghostBtn}><IconPlus size={14} /> Tambah Layanan</button>
        </div>
      </section>

      <section style={card}>
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", flexWrap: "wrap", gap: 8 }}>
          <h2 style={sectionTitle}>Jamaah dengan Layanan Tambahan ({assignments.length})</h2>
          <button type="button" onClick={() => setAssignOpen(true)} style={primaryBtn} disabled={!activeItems.length}><IconPlus size={14} /> Tambahkan</button>
        </div>
        {assignments.length > 0 && (
          <p style={{ margin: "6px 0 0", fontSize: 12, color: "var(--color-warm-500)" }}>
            Total {rupiah(totalAll)} · belum dibayar {rupiah(totalUnpaid)}
          </p>
        )}
        {assignments.length ? (
          <div style={{ display: "grid", gap: 8, marginTop: 12 }}>
            {assignments.map((a) => (
              <div key={a.id} style={row}>
                <div style={{ flex: 1, minWidth: 0 }}>
                  <div style={{ display: "flex", alignItems: "center", gap: 8, flexWrap: "wrap" }}>
                    <strong>{a.pilgrimName}</strong>
                    <span style={miniBadge}>{a.addonName}</span>
                    {a.groupName && <span style={miniBadge}>{a.groupName}</span>}
                    <span style={{ ...statusBadge, color: a.paid ? "var(--color-emerald-800)" : "var(--color-danger-600)" }}>
                      {a.paid ? "Lunas" : "Belum bayar"}
                    </span>
                  </div>
                  <p style={{ margin: "2px 0 0", fontSize: 12, color: "var(--color-warm-500)" }}>
                    {a.quantity}× {rupiah(a.unitPriceIdr)} = {rupiah(a.totalIdr)}{a.notes && ` · ${a.notes}`}
                  </p>
                </div>
                <div style={{ display: "flex", gap: 6, flexWrap: "wrap" }}>
                  <button type="button" onClick={() => void togglePaid(a)} style={ghostBtn}><IconCheck size={14} /> {a.paid ? "Batal Lunas" : "Tandai Lunas"}</button>
                  <button type="button" onClick={() => void removeAssignment(a)} style={iconBtnDanger} aria-label="Hapus"><IconTrash size={14} /></button>
                </div>
              </div>
            ))}
          </div>
        ) : <p style={{ color: "var(--color-warm-400)", fontSize: 13, marginTop: 12 }}>Belum ada jamaah dengan layanan tambahan{groupId ? " di grup ini" : ""}.</p>}
      </section>

      <AssignAddonDialog open={assignOpen} items={activeItems} pilgrims={pilgrims} onClose={() => setAssignOpen(false)} onSaved={refresh} />
    </main>
  );
}

const page: React.CSSProperties = { maxWidth: 1000, margin: "0 auto", padding: "32px 24px", display: "grid", gap: 16 };
const eyebrow: React.CSSProperties = { margin: "0 0 6px", fontSize: 11, fontWeight: 700, color: "var(--color-gold-800)", letterSpacing: ".08em" };
const seasonSelect: React.CSSProperties = { minHeight: 40, border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 10px", font: "inherit", background: "#fff" };
const card: React.CSSProperties = { background: "var(--color-cream-200)", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 20 };
const sectionTitle: React.CSSProperties = { margin: 0, fontSize: 16 };
const row: React.CSSProperties = { display: "flex", alignItems: "center", gap: 10, padding: "10px 12px", background: "#fff", border: "1px solid var(--color-cream-300)", borderRadius: 8, flexWrap: "wrap" };
const statusBadge: React.CSSProperties = { fontSize: 11, fontWeight: 700 };
const miniBadge: React.CSSProperties = { padding: "2px 6px", borderRadius: 99, background: "var(--color-cream-200)", color: "var(--color-warm-500)", fontSize: 10, fontWeight: 700 };
const inlineForm: React.CSSProperties = { display: "grid", gap: 8, padding: 12, background: "#fff", border: "1px solid var(--color-cream-300)", borderRadius: 8 };
const input: React.CSSProperties = { minHeight: 38, border: "1px solid var(--color-cream-400)", borderRadius: 6, padding: "0 8px", font: "inherit", background: "#fff" };
const ghostBtn: React.CSSProperties = { minHeight: 32, border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 10px", background: "transparent", color: "var(--color-emerald-900)", fontSize: 12, fontWeight: 600, display: "inline-flex", alignItems: "center", gap: 6 };
const primaryBtn: React.CSSProperties = { minHeight: 36, border: 0, borderRadius: 8, padding: "0 14px", background: "var(--color-gold-500)", color: "#fff", fontWeight: 700, display: "inline-flex", alignItems: "center", gap: 6, fontSize: 13 };
const iconBtnDanger: React.CSSProperties = { width: 28, height: 28, border: "1px solid var(--color-cream-400)", borderRadius: 6, background: "#fff", color: "var(--color-danger-600)", display: "grid", placeItems: "center" };
