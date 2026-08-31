"use client";

import { FormEvent, useCallback, useEffect, useMemo, useRef, useState, type CSSProperties } from "react";
import { ConnectError } from "@connectrpc/connect";
import { IconBuildingCommunity, IconPlus } from "@tabler/icons-react";
import type { BranchPerformance } from "@hajj-saas/proto-gen/hajj/v1/branch_pb";
import { branchClient, seasonClient } from "@/lib/rpc";
import { ActionCenter, type ActionCenterItem } from "@/components/ui/ActionCenter";
import { Badge } from "@/components/ui/Badge";
import { DataTable, type DataTableColumn } from "@/components/ui/DataTable";
import { DetailDrawer } from "@/components/ui/DetailDrawer";
import { EmptyState } from "@/components/ui/EmptyState";
import { PageHeader } from "@/components/ui/PageHeader";
import { ProgressBar } from "@/components/ui/ProgressBar";
import { StatCard } from "@/components/ui/StatCard";

const compactRupiah = (value: bigint | number) => new Intl.NumberFormat("id-ID", { notation: "compact", maximumFractionDigits: 1 }).format(Number(value));
const pct = (value: number) => `${Math.max(0, value).toLocaleString("id-ID", { maximumFractionDigits: 1 })}%`;

export default function BranchPage() {
  const [rows, setRows] = useState<BranchPerformance[]>();
  const [seasonId, setSeasonId] = useState("");
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [search, setSearch] = useState("");
  const [selected, setSelected] = useState<BranchPerformance | null>(null);
  const [creating, setCreating] = useState(false);
  const [saving, setSaving] = useState(false);
  const branchNameRef = useRef<HTMLInputElement>(null);

  const load = useCallback(async () => {
    setError("");
    try {
      const seasons = await seasonClient.listSeasons({});
      const season = seasons.seasons.find((item) => item.isActive) ?? seasons.seasons[0];
      if (!season) {
        setRows([]);
        setError("Buat musim aktif terlebih dahulu agar kinerja cabang dapat dihitung.");
        return;
      }
      setSeasonId(season.id);
      const report = await branchClient.getBranchPerformance({ seasonId: season.id });
      setRows(report.branches);
    } catch (caught) {
      setRows([]);
      setError(ConnectError.from(caught).rawMessage || "Kinerja cabang belum dapat dimuat.");
    }
  }, []);

  useEffect(() => { void load(); }, [load]);

  const filtered = useMemo(() => {
    const query = search.trim().toLocaleLowerCase("id-ID");
    if (!query) return rows ?? [];
    return (rows ?? []).filter(({ branch }) => [branch?.name, branch?.city, branch?.headName, branch?.headEmail].some((value) => value?.toLocaleLowerCase("id-ID").includes(query)));
  }, [rows, search]);

  const summary = useMemo(() => {
    const data = rows ?? [];
    const revenue = data.reduce((total, row) => total + Number(row.revenueIdr), 0);
    const targetRevenue = data.reduce((total, row) => total + Number(row.branch?.targetRevenueIdr ?? 0), 0);
    return {
      revenue,
      revenueAchievement: targetRevenue > 0 ? revenue / targetRevenue * 100 : 0,
      pilgrims: data.reduce((total, row) => total + row.pilgrimCount, 0),
      agents: data.reduce((total, row) => total + row.agentCount, 0),
      belowTarget: data.filter((row) => row.score < 100).length,
    };
  }, [rows]);

  const actions = useMemo<ActionCenterItem[] | undefined>(() => {
    if (!rows) return undefined;
    const items: ActionCenterItem[] = [];
    for (const row of rows) {
      const branch = row.branch;
      if (!branch) continue;
      if (!branch.headUserId) items.push({ id: `head-${branch.id}`, title: `${branch.name} belum memiliki kepala cabang`, description: "Tetapkan penanggung jawab agar eskalasi operasional memiliki pemilik yang jelas.", financialImpact: "PIC kosong", actionHref: "#daftar-cabang", actionLabel: "Buka daftar", tone: "warning" });
      if (row.score < 100) items.push({ id: `target-${branch.id}`, title: `${branch.name} masih di bawah target`, description: `Capaian omzet ${pct(row.revenueAchievementPct)} dan jamaah ${pct(row.pilgrimAchievementPct)}.`, financialImpact: `${pct(row.score)} skor`, actionHref: "#daftar-cabang", actionLabel: "Bandingkan", tone: row.score < 60 ? "danger" : "warning" });
      if (row.documentsReadyPct < 80 && row.pilgrimCount > 0) items.push({ id: `docs-${branch.id}`, title: `Dokumen ${branch.name} perlu perhatian`, description: `${pct(row.documentsReadyPct)} jamaah siap dokumen; tindak lanjuti sebelum tenggat keberangkatan.`, financialImpact: `${pct(100 - row.documentsReadyPct)} belum siap`, actionHref: "/dashboard/documents", actionLabel: "Buka dokumen", tone: "warning" });
    }
    return items.sort((a, b) => (a.tone === "danger" ? -1 : 0) - (b.tone === "danger" ? -1 : 0)).slice(0, 5);
  }, [rows]);

  const columns = useMemo<readonly DataTableColumn<BranchPerformance>[]>(() => [
    { id: "branch", header: "Cabang", cell: (row) => <div className="branch-name-cell"><strong>{row.branch?.name}</strong><span>{row.branch?.city || "Kota belum diisi"}</span></div> },
    { id: "head", header: "Kepala", cell: (row) => row.branch?.headName ? <div className="branch-name-cell"><strong>{row.branch.headName}</strong><span>{row.branch.headEmail}</span></div> : <Badge tone="warning">Belum ditetapkan</Badge> },
    { id: "target", header: "Target vs realisasi", cell: (row) => <ProgressBar label="Omzet" value={row.revenueAchievementPct} max={100} valueLabel={pct(row.revenueAchievementPct)} tone={row.revenueAchievementPct >= 100 ? "success" : row.revenueAchievementPct < 60 ? "danger" : "warning"} /> },
    { id: "pilgrims", header: "Jamaah", align: "right", cell: (row) => row.pilgrimCount.toLocaleString("id-ID") },
    { id: "agents", header: "Agen", align: "right", cell: (row) => row.agentCount.toLocaleString("id-ID") },
    { id: "score", header: "Skor", align: "right", cell: (row) => <Badge tone={row.score >= 100 ? "success" : row.score < 60 ? "danger" : "warning"}>{row.score.toFixed(1)}</Badge> },
    { id: "collection", header: "Kolektibilitas", align: "right", cell: (row) => pct(row.collectionPct) },
  ], []);

  async function createBranch(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); setSaving(true); setError("");
    const form = new FormData(event.currentTarget);
    try {
      await branchClient.createBranch({ name: String(form.get("name") ?? "").trim(), city: String(form.get("city") ?? "").trim(), targetPilgrims: Number(form.get("targetPilgrims") ?? 0), targetRevenueIdr: BigInt(String(form.get("targetRevenueIdr") || "0")), phone: String(form.get("phone") ?? "").trim(), bankName: String(form.get("bankName") ?? "").trim(), accountNumber: String(form.get("accountNumber") ?? "").trim(), accountHolder: String(form.get("accountHolder") ?? "").trim() });
      setCreating(false); setNotice("Cabang berhasil ditambahkan."); await load();
    } catch (caught) { setError(ConnectError.from(caught).rawMessage || "Cabang belum berhasil dibuat."); }
    finally { setSaving(false); }
  }

  const ranking = [...(rows ?? [])].sort((a, b) => b.score - a.score);
  const maxScore = Math.max(...ranking.map((row) => row.score), 100);

  return <div>
    <PageHeader eyebrow="Jaringan Cabang" title="Cabang" subtitle={rows ? `${rows.length} cabang · ${summary.agents} agen aktif · ${summary.pilgrims.toLocaleString("id-ID")} jamaah terealisasi` : "Menghitung kinerja jaringan cabang…"} primaryAction={<button className="tw-btn tw-btn--emerald tw-btn--md" type="button" onClick={() => setCreating(true)}><IconPlus size={16} aria-hidden />Tambah cabang</button>} />
    <div className="dashboard-home branch-dashboard">
      {notice && <p className="dashboard-success-banner" role="status">{notice}</p>}
      {error && <p className="dashboard-error-banner" role="alert">{error}</p>}
      <div className="dashboard-stats-grid tw-stagger"><StatCard label="Omzet jaringan" value={rows ? compactRupiah(summary.revenue) : "–"} unit="rupiah" tone="brand" /><StatCard label="Capaian target" value={rows ? summary.revenueAchievement.toFixed(1) : "–"} unit="persen" tone={summary.revenueAchievement >= 100 ? "success" : "warning"} /><StatCard label="Jamaah terealisasi" value={rows ? summary.pilgrims : "–"} unit="jamaah" tone="info" /><StatCard label="Cabang di bawah target" value={rows ? summary.belowTarget : "–"} unit="cabang" tone={summary.belowTarget ? "warning" : "success"} /></div>
      <div className="branch-insight-grid"><section className="tw-card tw-card--large branch-ranking" aria-labelledby="branch-ranking-title"><header><div><h2 id="branch-ranking-title">Papan Peringkat</h2><p>Skor gabungan: 70% capaian omzet · 30% capaian jamaah</p></div><Badge tone="brand">{ranking.length} cabang</Badge></header><ol>{ranking.map((row, index) => <li key={row.branch?.id}><span className="branch-rank-number">{index + 1}</span><div><strong>{row.branch?.name}</strong><span className="branch-ranking-track"><i style={{ "--branch-score-width": `${Math.min(row.score / maxScore * 100, 100)}%` } as CSSProperties} /></span></div><b>{row.score.toFixed(1)}</b></li>)}</ol>{!ranking.length && <p className="branch-ranking-empty">Peringkat muncul setelah cabang memiliki target dan realisasi.</p>}</section><ActionCenter items={actions} title="Pusat Aksi Cabang" subtitle="Rekomendasi otomatis dari target, penanggung jawab, dan kesiapan dokumen" cleanTitle="Jaringan cabang terkendali" cleanDescription="Seluruh cabang memiliki kepala, memenuhi target, dan kesiapan dokumennya sehat." error={error || undefined} /></div>
      <div id="daftar-cabang"><DataTable ariaLabel="Daftar kinerja cabang" columns={columns} rows={filtered} getRowId={(row) => row.branch?.id ?? ""} searchValue={search} onSearchChange={setSearch} searchPlaceholder="Cari nama cabang, kota, kepala, atau email…" onRowClick={setSelected} getRowLabel={(row) => `Buka detail ${row.branch?.name}`} loading={rows === undefined} emptyState={<EmptyState title="Belum ada cabang yang dapat ditampilkan" cause={search ? "Tidak ada cabang yang cocok dengan pencarian." : "Jaringan cabang belum dibuat untuk operator ini."} nextStep={search ? "Hapus kata pencarian untuk melihat seluruh cabang." : "Tambahkan cabang pertama beserta kota dan targetnya."} actionHref={search ? "#daftar-cabang" : "/dashboard/langganan"} actionLabel={search ? "Lihat semua" : "Periksa paket"} icon={<IconBuildingCommunity size={22} />} />} /></div>
    </div>
    <DetailDrawer open={Boolean(selected)} onClose={() => setSelected(null)} title={selected?.branch?.name ?? "Detail cabang"} subtitle={selected?.branch?.city || "Kota belum diisi"}>{selected && <div className="branch-detail-stack"><div className="branch-detail-stats"><StatCard label="Omzet" value={compactRupiah(selected.revenueIdr)} unit="rupiah" tone="brand" sparkline={selected.trend.map((point) => Number(point.revenueIdr))} sparklineLabel={`Tren omzet ${selected.branch?.name}`} /><StatCard label="Skor kinerja" value={selected.score.toFixed(1)} unit="skor" tone={selected.score >= 100 ? "success" : "warning"} /></div><section className="branch-detail-section"><h3>Pencapaian target</h3><ProgressBar label="Omzet" value={selected.revenueAchievementPct} max={100} valueLabel={pct(selected.revenueAchievementPct)} tone={selected.revenueAchievementPct >= 100 ? "success" : "warning"} /><ProgressBar label="Jamaah" value={selected.pilgrimAchievementPct} max={100} valueLabel={`${selected.pilgrimCount} / ${selected.branch?.targetPilgrims || "—"}`} tone={selected.pilgrimAchievementPct >= 100 ? "success" : "warning"} /><ProgressBar label="Kesiapan dokumen" value={selected.documentsReadyPct} max={100} valueLabel={pct(selected.documentsReadyPct)} tone={selected.documentsReadyPct >= 80 ? "success" : "danger"} /></section><section className="branch-detail-section"><h3>Penanggung jawab & rekening</h3><dl><div><dt>Kepala cabang</dt><dd>{selected.branch?.headName || "Belum ditetapkan"}</dd></div><div><dt>Email</dt><dd>{selected.branch?.headEmail || "—"}</dd></div><div><dt>Rekening</dt><dd>{selected.branch?.bankName ? `${selected.branch.bankName} · ${selected.branch.accountNumber}` : "Belum diisi"}</dd></div><div><dt>Atas nama</dt><dd>{selected.branch?.accountHolder || "—"}</dd></div></dl></section></div>}</DetailDrawer>
    <DetailDrawer open={creating} onClose={() => setCreating(false)} title="Tambah cabang" subtitle="Isi identitas, target, dan rekening operasional cabang." initialFocusRef={branchNameRef}><form className="branch-create-form" onSubmit={createBranch}><label>Nama cabang<input ref={branchNameRef} name="name" required maxLength={120} /></label><label>Kota<input name="city" maxLength={120} /></label><div className="branch-form-grid"><label>Target jamaah<input name="targetPilgrims" type="number" min="0" defaultValue="0" /></label><label>Target omzet (Rp)<input name="targetRevenueIdr" type="number" min="0" defaultValue="0" /></label></div><label>Nomor telepon<input name="phone" maxLength={40} /></label><div className="branch-form-grid"><label>Bank<input name="bankName" maxLength={120} /></label><label>Nomor rekening<input name="accountNumber" maxLength={80} /></label></div><label>Nama pemilik rekening<input name="accountHolder" maxLength={120} /></label><button className="tw-btn tw-btn--emerald tw-btn--md" disabled={saving || !seasonId}>{saving ? "Menyimpan…" : "Simpan cabang"}</button></form></DetailDrawer>
  </div>;
}
