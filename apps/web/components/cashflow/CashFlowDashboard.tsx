"use client";

import { useCallback, useEffect, useMemo, useState, type Dispatch, type SetStateAction } from "react";
import { IconAlertTriangle, IconArrowBackUp, IconCash, IconCheck, IconFileInvoice, IconPlus, IconPrinter, IconReceipt } from "@tabler/icons-react";
import { CashFlowSummary, Installment, InstallmentPayment, InstallmentPaymentMethod, InstallmentPlan, InstallmentPlanDetail, InstallmentScheme, MonthlyProjectionEntry, VendorPayment } from "@hajj-saas/proto-gen/hajj/v1/cashflow_pb";
import type { Pilgrim } from "@hajj-saas/proto-gen/hajj/v1/pilgrim_pb";
import { cashFlowClient, pilgrimClient, seasonClient } from "@/lib/rpc";
import { ActionCenter, type ActionCenterItem } from "@/components/ui/ActionCenter";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { DataTable, type DataTableColumn } from "@/components/ui/DataTable";
import { DetailDrawer } from "@/components/ui/DetailDrawer";
import { EmptyState } from "@/components/ui/EmptyState";
import { PageHero } from "@/components/ui/PageHero";
import { StatCard } from "@/components/ui/StatCard";
import { RoleGate } from "@/components/auth/RoleGate";

const CATEGORY_LABEL: Record<string, string> = { HOTEL: "Hotel", TRANSPORT: "Transportasi", CATERING: "Katering", VISA: "Visa", INSURANCE: "Asuransi", OTHER: "Lainnya" };
const VENDOR_STATUS_LABEL: Record<string, string> = { PENDING: "Menunggu", PAID: "Lunas", OVERDUE: "Terlambat", CANCELLED: "Dibatalkan" };
const PLAN_STATUS_LABEL: Record<string, string> = { UNPAID: "Belum bayar", PARTIAL: "Berjalan", PAID: "Lunas", OVERDUE: "Menunggak" };
const INSTALLMENT_STATUS_LABEL: Record<string, string> = { UPCOMING: "Akan datang", DUE: "Jatuh tempo", PARTIAL: "Sebagian", PAID: "Lunas", OVERDUE: "Menunggak" };
const SCHEME_LABEL: Record<number, string> = {
  [InstallmentScheme.INSTALLMENT_SCHEME_FULL]: "Bayar penuh",
  [InstallmentScheme.INSTALLMENT_SCHEME_DP_50]: "DP 50% dan pelunasan",
  [InstallmentScheme.INSTALLMENT_SCHEME_6X]: "Cicilan 6 kali",
  [InstallmentScheme.INSTALLMENT_SCHEME_12X]: "Cicilan 12 kali",
  [InstallmentScheme.INSTALLMENT_SCHEME_CASH_BONUS]: "Bonus pelunasan tunai",
};

type Tab = "receivables" | "vendors";
type PlanForm = { pilgrimId: string; scheme: InstallmentScheme; grossAmount: string; cashBonus: string; firstDueDate: string; idempotencyKey: string };
type PaymentForm = { installmentId: string; amount: string; method: InstallmentPaymentMethod; reference: string; note: string; idempotencyKey: string };

function nextKey(): string { return globalThis.crypto?.randomUUID?.() ?? `${Date.now()}-${Math.random().toString(16).slice(2)}`; }
function blankPlanForm(): PlanForm { return { pilgrimId: "", scheme: InstallmentScheme.INSTALLMENT_SCHEME_DP_50, grossAmount: "", cashBonus: "", firstDueDate: "", idempotencyKey: nextKey() }; }
function blankPaymentForm(installmentId = ""): PaymentForm { return { installmentId, amount: "", method: InstallmentPaymentMethod.BANK_TRANSFER, reference: "", note: "", idempotencyKey: nextKey() }; }
function parseIDR(value: string): bigint | undefined { const normalized = value.replace(/[^0-9]/g, ""); if (!normalized) return; const parsed = BigInt(normalized); return parsed > 0n ? parsed : undefined; }
function formatIDR(value: bigint | number | string): string { return new Intl.NumberFormat("id-ID", { style: "currency", currency: "IDR", maximumFractionDigits: 0 }).format(Number(value)); }
function toneForStatus(status: string): "success" | "warning" | "danger" | "neutral" { if (status === "PAID") return "success"; if (status === "OVERDUE") return "danger"; if (status === "PARTIAL" || status === "DUE") return "warning"; return "neutral"; }
function errorMessage(error: unknown, fallback: string): string { return error instanceof Error && error.message ? error.message : fallback; }

export default function CashFlowDashboard() {
  const [tab, setTab] = useState<Tab>("receivables");
  const [seasons, setSeasons] = useState<{ id: string; name: string; isActive: boolean }[]>([]);
  const [seasonId, setSeasonId] = useState("");
  const [pilgrims, setPilgrims] = useState<Pilgrim[]>([]);
  const [summary, setSummary] = useState<CashFlowSummary>();
  const [months, setMonths] = useState<MonthlyProjectionEntry[]>([]);
  const [vendors, setVendors] = useState<VendorPayment[]>([]);
  const [plans, setPlans] = useState<InstallmentPlan[]>([]);
  const [receivableTotal, setReceivableTotal] = useState(0n);
  const [overdueTotal, setOverdueTotal] = useState(0n);
  const [dueNext7, setDueNext7] = useState(0n);
  const [collectionBps, setCollectionBps] = useState(0);
  const [aging, setAging] = useState<readonly bigint[]>([0n, 0n, 0n, 0n, 0n]);
  const [loading, setLoading] = useState(true);
  const [notice, setNotice] = useState("");
  const [search, setSearch] = useState("");
  const [statusFilter, setStatusFilter] = useState("");
  const [planOpen, setPlanOpen] = useState(false);
  const [planForm, setPlanForm] = useState<PlanForm>(blankPlanForm);
  const [selected, setSelected] = useState<InstallmentPlanDetail>();
  const [detailLoading, setDetailLoading] = useState(false);
  const [paymentForm, setPaymentForm] = useState<PaymentForm>(blankPaymentForm);
  const [reversing, setReversing] = useState<InstallmentPayment>();
  const [reversalReason, setReversalReason] = useState("");
  const [reversalKey, setReversalKey] = useState(nextKey);
  const [saving, setSaving] = useState(false);
  const [vendorFormOpen, setVendorFormOpen] = useState(false);
  const [vendorForm, setVendorForm] = useState({ vendorName: "", category: "HOTEL", amount: "", dueDate: "", description: "" });

  useEffect(() => { seasonClient.listSeasons({}).then((response) => { setSeasons(response.seasons); setSeasonId(response.seasons.find((season) => season.isActive)?.id ?? response.seasons[0]?.id ?? ""); }).catch((error) => setNotice(`Data musim gagal dimuat. ${errorMessage(error, "Coba muat ulang halaman.")}`)); }, []);

  const refresh = useCallback(async () => {
    if (!seasonId) return;
    setLoading(true); setNotice("");
    try {
      const [summaryResponse, projectionResponse, vendorResponse, receivableResponse, pilgrimResponse] = await Promise.all([
        cashFlowClient.getCashFlowSummary({ seasonId }), cashFlowClient.getMonthlyProjection({ seasonId }), cashFlowClient.listVendorPayments({ seasonId }),
        cashFlowClient.listInstallmentReceivables({ seasonId, status: statusFilter, search: search.trim(), limit: 200, offset: 0 }),
        pilgrimClient.listPilgrims({ seasonId, limit: 2000, offset: 0 }),
      ]);
      setSummary(summaryResponse); setMonths(projectionResponse.months); setVendors(vendorResponse.payments); setPlans(receivableResponse.plans);
      setReceivableTotal(receivableResponse.totalReceivableIdr); setOverdueTotal(receivableResponse.totalOverdueIdr); setDueNext7(receivableResponse.dueNext7DaysIdr); setCollectionBps(receivableResponse.collectionRateBps); setPilgrims(pilgrimResponse.pilgrims);
      setAging([receivableResponse.agingCurrentIdr, receivableResponse.aging130Idr, receivableResponse.aging3160Idr, receivableResponse.aging6190Idr, receivableResponse.agingOver90Idr]);
    } catch (error) { setNotice(`Data keuangan gagal dimuat. ${errorMessage(error, "Coba lagi.")}`); } finally { setLoading(false); }
  }, [search, seasonId, statusFilter]);

  useEffect(() => { const timeout = window.setTimeout(() => void refresh(), search ? 250 : 0); return () => window.clearTimeout(timeout); }, [refresh, search]);

  const openDetail = useCallback(async (plan: InstallmentPlan) => {
    setDetailLoading(true); setNotice("");
    try { const detail = await cashFlowClient.getPilgrimInstallmentPlan({ pilgrimId: plan.pilgrimId }); setSelected(detail); const firstOpen = detail.installments.find((item) => item.status !== "PAID"); setPaymentForm(blankPaymentForm(firstOpen?.id ?? detail.installments[0]?.id ?? "")); }
    catch (error) { setNotice(`Detail piutang gagal dibuka. ${errorMessage(error, "Coba lagi.")}`); } finally { setDetailLoading(false); }
  }, []);

  const actionItems = useMemo<ActionCenterItem[]>(() => {
    const items: ActionCenterItem[] = [];
    if (overdueTotal > 0n) items.push({ id: "overdue", title: "Tagihan jamaah melewati jatuh tempo", description: "Tunggakan yang dibiarkan mempersempit kas untuk hotel, tiket, dan layanan keberangkatan.", financialImpact: formatIDR(overdueTotal), actionHref: "#buku-piutang", actionLabel: "Buka tunggakan", tone: "danger" });
    if (dueNext7 > 0n) items.push({ id: "due-soon", title: "Cicilan jatuh tempo dalam 7 hari", description: "Hubungi jamaah sebelum jatuh tempo agar penerimaan kas tidak bergeser menjadi tunggakan.", financialImpact: formatIDR(dueNext7), actionHref: "#buku-piutang", actionLabel: "Lihat jadwal", tone: "warning" });
    return items;
  }, [dueNext7, overdueTotal]);

  const columns = useMemo<readonly DataTableColumn<InstallmentPlan>[]>(() => [
    { id: "pilgrim", header: "Jamaah", cell: (plan) => <div className="cashflow-person"><strong>{plan.pilgrimName}</strong><span>{SCHEME_LABEL[plan.scheme] ?? "Rencana pembayaran"}</span></div> },
    { id: "payable", header: "Nilai kontrak", align: "right", cell: (plan) => formatIDR(plan.payableAmountIdr) },
    { id: "paid", header: "Sudah masuk", align: "right", cell: (plan) => formatIDR(plan.paidAmountIdr) },
    { id: "outstanding", header: "Sisa piutang", align: "right", cell: (plan) => <strong>{formatIDR(plan.outstandingAmountIdr)}</strong> },
    { id: "progress", header: "Progres", cell: (plan) => <div className="cashflow-progress" aria-label={`${plan.progressPercent}% tertagih`}><span style={{ width: `${Math.min(100, Math.max(0, plan.progressPercent))}%` }} /><small>{plan.progressPercent}%</small></div> },
    { id: "status", header: "Risiko", cell: (plan) => <Badge tone={toneForStatus(plan.status)}>{PLAN_STATUS_LABEL[plan.status] ?? plan.status}</Badge> },
  ], []);

  const createPlan = async () => {
    const gross = parseIDR(planForm.grossAmount); const bonus = planForm.scheme === InstallmentScheme.INSTALLMENT_SCHEME_CASH_BONUS ? parseIDR(planForm.cashBonus) : 0n;
    if (!planForm.pilgrimId || !gross || !planForm.firstDueDate || bonus === undefined) { setNotice("Jamaah, skema, nilai kontrak, dan tanggal jatuh tempo wajib diisi."); return; }
    setSaving(true); setNotice("");
    try { const detail = await cashFlowClient.createInstallmentPlan({ pilgrimId: planForm.pilgrimId, scheme: planForm.scheme, grossAmountIdr: gross, cashBonusIdr: bonus, firstDueDate: planForm.firstDueDate, idempotencyKey: planForm.idempotencyKey }); setPlanOpen(false); setPlanForm(blankPlanForm()); setSelected(detail); await refresh(); }
    catch (error) { setNotice(`Rencana pembayaran gagal disimpan. ${errorMessage(error, "Periksa data lalu coba lagi.")}`); } finally { setSaving(false); }
  };

  const recordPayment = async () => {
    const amount = parseIDR(paymentForm.amount); if (!paymentForm.installmentId || !amount) { setNotice("Pilih angsuran dan masukkan nominal pembayaran yang valid."); return; }
    setSaving(true); setNotice("");
    try { const response = await cashFlowClient.recordInstallmentPayment({ installmentId: paymentForm.installmentId, amountIdr: amount, method: paymentForm.method, reference: paymentForm.reference.trim(), note: paymentForm.note.trim(), idempotencyKey: paymentForm.idempotencyKey }); setSelected(response.detail); setPaymentForm(blankPaymentForm(response.detail?.installments.find((item) => item.status !== "PAID")?.id ?? "")); await refresh(); }
    catch (error) { setNotice(`Pembayaran gagal dicatat. ${errorMessage(error, "Tidak ada perubahan pada ledger.")}`); } finally { setSaving(false); }
  };

  const reversePayment = async () => {
    if (!reversing || !reversalReason.trim()) { setNotice("Alasan koreksi wajib diisi agar jejak audit lengkap."); return; }
    setSaving(true); setNotice("");
    try { const response = await cashFlowClient.reverseInstallmentPayment({ paymentId: reversing.id, reason: reversalReason.trim(), idempotencyKey: reversalKey }); setSelected(response.detail); setReversing(undefined); setReversalReason(""); setReversalKey(nextKey()); await refresh(); }
    catch (error) { setNotice(`Koreksi gagal dicatat. ${errorMessage(error, "Pembayaran asli tetap utuh.")}`); } finally { setSaving(false); }
  };

  const createVendor = async () => {
    const amount = parseIDR(vendorForm.amount); if (!vendorForm.vendorName.trim() || !amount || !vendorForm.dueDate) { setNotice("Nama vendor, jumlah, dan tanggal jatuh tempo wajib diisi."); return; }
    setSaving(true);
    try { await cashFlowClient.createVendorPayment({ seasonId, vendorName: vendorForm.vendorName.trim(), category: vendorForm.category, amountIdr: amount, dueDate: vendorForm.dueDate, description: vendorForm.description.trim() }); setVendorForm({ vendorName: "", category: "HOTEL", amount: "", dueDate: "", description: "" }); setVendorFormOpen(false); await refresh(); }
    catch (error) { setNotice(`Kewajiban vendor gagal disimpan. ${errorMessage(error, "Coba lagi.")}`); } finally { setSaving(false); }
  };

  const queueReminders = async (planIds: string[], allDueWithin7Days: boolean) => {
    if (!seasonId || saving) return;
    setSaving(true); setNotice("");
    try { const response = await cashFlowClient.queueInstallmentReminders({ seasonId, planIds, allDueWithin7Days, idempotencyKey: nextKey() }); setNotice(`${response.queuedCount} pengingat masuk antrean email${response.skippedCount ? `, ${response.skippedCount} duplikat dilewati` : ""}.`); }
    catch (error) { setNotice(`Pengingat gagal diantrekan. ${errorMessage(error, "Coba lagi.")}`); } finally { setSaving(false); }
  };

  const queueReceipt = async (paymentId: string) => {
    if (saving) return;
    setSaving(true); setNotice("");
    try { const response = await cashFlowClient.queueInstallmentReceipt({ paymentId, idempotencyKey: nextKey() }); setNotice(response.queuedCount ? "Kwitansi masuk antrean email." : "Kwitansi ini sudah ada di antrean."); }
    catch (error) { setNotice(`Kwitansi gagal diantrekan. ${errorMessage(error, "Coba lagi.")}`); } finally { setSaving(false); }
  };

  const activeName = seasons.find((season) => season.id === seasonId)?.name ?? "Pilih musim";
  const openPilgrims = pilgrims.filter((pilgrim) => !plans.some((plan) => plan.pilgrimId === pilgrim.id));
  const reversedPaymentIDs = new Set(selected?.payments.filter((payment) => payment.kind === "REVERSAL").map((payment) => payment.originalPaymentId) ?? []);

  return <main className="cashflow-page">
    <PageHero eyebrow="KEUANGAN" title="Pembayaran & Cicilan" subtitle="Arus kas masuk, verifikasi transaksi, umur piutang, dan penagihan cicilan" actions={<><select className="cashflow-season" aria-label="Musim" value={seasonId} onChange={(event) => setSeasonId(event.target.value)}>{seasons.map((season) => <option key={season.id} value={season.id}>{season.name}{season.isActive ? " (aktif)" : ""}</option>)}</select>{tab === "receivables" ? <RoleGate require={["owner", "admin"]}><Button variant="outline" disabled={saving} onClick={() => void queueReminders([], true)}><IconReceipt size={17} />Kirim pengingat</Button><Button variant="emerald" onClick={() => setPlanOpen(true)}><IconPlus size={17} />Buat rencana bayar</Button></RoleGate> : <RoleGate require={["owner", "admin"]}><Button variant="outline" onClick={() => setVendorFormOpen(true)}><IconPlus size={17} />Tambah kewajiban</Button></RoleGate>}</>} />
    <div className="cashflow-content">
      <nav className="cashflow-tabs" aria-label="Bagian keuangan"><button type="button" data-active={tab === "receivables" || undefined} onClick={() => setTab("receivables")}>Piutang jamaah</button><button type="button" data-active={tab === "vendors" || undefined} onClick={() => setTab("vendors")}>Kewajiban vendor</button></nav>
      {notice && <div className="cashflow-notice" role="alert"><IconAlertTriangle size={18} /><span>{notice}</span><button type="button" onClick={() => setNotice("")}>Tutup</button></div>}
      {tab === "receivables" ? <>
        <section className="cashflow-stats tw-stagger" aria-label="Ringkasan piutang"><StatCard label="Kas masuk periode" value={formatIDR(summary?.totalCollectedIdr ?? 0n)} unit="rupiah" tone="success" /><StatCard label="Piutang berjalan" value={formatIDR(receivableTotal)} unit="rupiah" tone="brand" /><StatCard label="Tunggakan" value={formatIDR(overdueTotal)} unit="rupiah" tone={overdueTotal > 0n ? "danger" : "neutral"} /><StatCard label="Kolektibilitas" value={(collectionBps / 100).toLocaleString("id-ID", { maximumFractionDigits: 2 })} unit="persen" tone={collectionBps >= 8000 ? "success" : collectionBps >= 5000 ? "warning" : "danger"} /></section>
        <ActionCenter items={actionItems} subtitle={`Prioritas penagihan untuk ${activeName}`} cleanTitle="Tidak ada piutang mendesak" cleanDescription="Belum ada tunggakan atau cicilan yang jatuh tempo dalam tujuh hari." className="tw-action-center--inline" />
        <AgingChart values={aging} />
        <section id="buku-piutang" className="cashflow-section"><div className="cashflow-section__heading"><div><h2>Buku piutang per jamaah</h2><p>Sisa tagihan, progres penerimaan, dan status risiko dihitung langsung dari ledger.</p></div></div><DataTable ariaLabel="Buku piutang jamaah" columns={columns} rows={plans} getRowId={(plan) => plan.id} searchValue={search} onSearchChange={setSearch} searchPlaceholder="Cari nama jamaah" filters={<select className="cashflow-filter" aria-label="Filter status piutang" value={statusFilter} onChange={(event) => setStatusFilter(event.target.value)}><option value="">Semua status</option><option value="OVERDUE">Menunggak</option><option value="PARTIAL">Berjalan</option><option value="UNPAID">Belum bayar</option><option value="PAID">Lunas</option></select>} onRowClick={(plan) => void openDetail(plan)} getRowLabel={(plan) => `Buka rencana pembayaran ${plan.pilgrimName}`} loading={loading || detailLoading} emptyState={<EmptyState icon={<IconFileInvoice size={22} />} title={search || statusFilter ? "Tidak ada hasil" : "Belum ada rencana pembayaran"} cause={search || statusFilter ? "Pencarian atau filter tidak menemukan piutang yang cocok." : "Jamaah musim ini belum memiliki jadwal cicilan."} nextStep={search || statusFilter ? "Ubah pencarian atau tampilkan semua status." : "Buat rencana bayar untuk membentuk jadwal dan buku piutang."} actionLabel={search || statusFilter ? "Hapus filter" : "Buat rencana"} onAction={() => { if (search || statusFilter) { setSearch(""); setStatusFilter(""); } else { setPlanOpen(true); } }} />} /></section>
      </> : <VendorPanel summary={summary} months={months} vendors={vendors} loading={loading} onPaid={async (vendor) => { await cashFlowClient.updateVendorPaymentStatus({ id: vendor.id, status: "PAID" }); await refresh(); }} />}
    </div>
    <DetailDrawer open={planOpen} onClose={() => setPlanOpen(false)} title="Buat rencana pembayaran" subtitle="Nilai kontrak dan jadwal akan dibekukan setelah disimpan." footer={<><Button variant="ghost" onClick={() => setPlanOpen(false)}>Batal</Button><Button variant="emerald" disabled={saving} onClick={() => void createPlan()}>{saving ? "Menyimpan..." : "Simpan rencana"}</Button></>}><PlanFormFields form={planForm} setForm={setPlanForm} pilgrims={openPilgrims} /></DetailDrawer>
    <PlanDetailDrawer detail={selected} onClose={() => { setSelected(undefined); setReversing(undefined); }} paymentForm={paymentForm} setPaymentForm={setPaymentForm} onRecord={() => void recordPayment()} saving={saving} reversing={reversing} setReversing={(payment) => { setReversing(payment); setReversalReason(""); setReversalKey(nextKey()); }} reversalReason={reversalReason} setReversalReason={setReversalReason} onReverse={() => void reversePayment()} onReminder={(planId) => void queueReminders([planId], false)} onReceipt={(paymentId) => void queueReceipt(paymentId)} reversedPaymentIDs={reversedPaymentIDs} />
    <DetailDrawer open={vendorFormOpen} onClose={() => setVendorFormOpen(false)} title="Tambah kewajiban vendor" subtitle="Catat komitmen yang akan memengaruhi proyeksi arus kas." footer={<><Button variant="ghost" onClick={() => setVendorFormOpen(false)}>Batal</Button><Button variant="emerald" disabled={saving} onClick={() => void createVendor()}>{saving ? "Menyimpan..." : "Simpan kewajiban"}</Button></>}><div className="cashflow-form"><label>Nama vendor<input value={vendorForm.vendorName} onChange={(event) => setVendorForm((form) => ({ ...form, vendorName: event.target.value }))} /></label><label>Kategori<select value={vendorForm.category} onChange={(event) => setVendorForm((form) => ({ ...form, category: event.target.value }))}>{Object.entries(CATEGORY_LABEL).map(([value, label]) => <option key={value} value={value}>{label}</option>)}</select></label><label>Jumlah (Rp)<input inputMode="numeric" value={vendorForm.amount} onChange={(event) => setVendorForm((form) => ({ ...form, amount: event.target.value }))} /></label><label>Jatuh tempo<input type="date" value={vendorForm.dueDate} onChange={(event) => setVendorForm((form) => ({ ...form, dueDate: event.target.value }))} /></label><label>Deskripsi<textarea rows={3} value={vendorForm.description} onChange={(event) => setVendorForm((form) => ({ ...form, description: event.target.value }))} /></label></div></DetailDrawer>
  </main>;
}

function PlanFormFields({ form, setForm, pilgrims }: { form: PlanForm; setForm: Dispatch<SetStateAction<PlanForm>>; pilgrims: Pilgrim[] }) {
  return <div className="cashflow-form"><label>Jamaah<select value={form.pilgrimId} onChange={(event) => setForm((value) => ({ ...value, pilgrimId: event.target.value }))}><option value="">Pilih jamaah</option>{pilgrims.map((pilgrim) => <option key={pilgrim.id} value={pilgrim.id}>{pilgrim.fullName}</option>)}</select><small>Hanya jamaah tanpa rencana aktif yang ditampilkan.</small></label><label>Skema pembayaran<select value={form.scheme} onChange={(event) => setForm((value) => ({ ...value, scheme: Number(event.target.value) as InstallmentScheme, cashBonus: "" }))}>{Object.entries(SCHEME_LABEL).map(([value, label]) => <option key={value} value={value}>{label}</option>)}</select></label><label>Nilai kontrak (Rp)<input inputMode="numeric" value={form.grossAmount} onChange={(event) => setForm((value) => ({ ...value, grossAmount: event.target.value }))} placeholder="35000000" /></label>{form.scheme === InstallmentScheme.INSTALLMENT_SCHEME_CASH_BONUS && <label>Bonus pelunasan (Rp)<input inputMode="numeric" value={form.cashBonus} onChange={(event) => setForm((value) => ({ ...value, cashBonus: event.target.value }))} placeholder="500000" /><small>Bonus mengurangi nilai yang harus dibayar.</small></label>}<label>Jatuh tempo pertama<input type="date" value={form.firstDueDate} onChange={(event) => setForm((value) => ({ ...value, firstDueDate: event.target.value }))} /></label><div className="cashflow-ledger-note"><IconReceipt size={20} /><p><strong>Kontrak tidak diedit langsung.</strong> Koreksi pembayaran dibuat sebagai entry pembalik agar nominal asli dan jejak audit tetap utuh.</p></div></div>;
}

function PlanDetailDrawer({ detail, onClose, paymentForm, setPaymentForm, onRecord, saving, reversing, setReversing, reversalReason, setReversalReason, onReverse, onReminder, onReceipt, reversedPaymentIDs }: { detail?: InstallmentPlanDetail; onClose: () => void; paymentForm: PaymentForm; setPaymentForm: Dispatch<SetStateAction<PaymentForm>>; onRecord: () => void; saving: boolean; reversing?: InstallmentPayment; setReversing: (payment?: InstallmentPayment) => void; reversalReason: string; setReversalReason: (value: string) => void; onReverse: () => void; onReminder: (planId: string) => void; onReceipt: (paymentId: string) => void; reversedPaymentIDs: Set<string> }) {
  const plan = detail?.plan; const chosen = detail?.installments.find((item) => item.id === paymentForm.installmentId);
  return <DetailDrawer open={Boolean(detail)} onClose={onClose} title={plan?.pilgrimName ?? "Detail pembayaran"} subtitle={plan ? `${SCHEME_LABEL[plan.scheme] ?? "Rencana pembayaran"} | ${formatIDR(plan.payableAmountIdr)}` : undefined} footer={<>{plan && plan.status !== "PAID" && <Button variant="outline" disabled={saving} onClick={() => onReminder(plan.id)}>Kirim pengingat</Button>}<Button variant="ghost" onClick={onClose}>Tutup</Button></>}>{detail && <div className="cashflow-detail"><section className="cashflow-detail__summary"><div><span>Sudah masuk</span><strong>{formatIDR(detail.plan?.paidAmountIdr ?? 0n)}</strong></div><div><span>Sisa piutang</span><strong>{formatIDR(detail.plan?.outstandingAmountIdr ?? 0n)}</strong></div><Badge tone={toneForStatus(detail.plan?.status ?? "")}>{PLAN_STATUS_LABEL[detail.plan?.status ?? ""] ?? detail.plan?.status}</Badge></section><section><h3>Jadwal angsuran</h3><div className="cashflow-schedule">{detail.installments.map((item) => <InstallmentRow key={item.id} item={item} selected={paymentForm.installmentId === item.id} onSelect={() => setPaymentForm(blankPaymentForm(item.id))} />)}</div></section>{detail.plan?.status !== "PAID" && <RoleGate require={["owner", "admin"]}><PaymentFields detail={detail} form={paymentForm} setForm={setPaymentForm} chosen={chosen} saving={saving} onRecord={onRecord} /></RoleGate>}<section><div className="cashflow-detail__heading"><h3>Riwayat ledger</h3>{detail.payments.length > 0 && <Button variant="outline" size="sm" onClick={() => window.print()}><IconPrinter size={15} />Cetak</Button>}</div>{detail.payments.length ? <div className="cashflow-ledger">{detail.payments.map((payment) => <article key={payment.id} data-kind={payment.kind}><div><strong>{payment.kind === "REVERSAL" ? "Koreksi" : payment.receiptNumber}</strong><span>{payment.createdAt?.toDate().toLocaleString("id-ID")}</span></div><div className="cashflow-ledger__amount"><strong>{formatIDR(payment.amountIdr)}</strong>{payment.kind === "PAYMENT" && <RoleGate require={["owner", "admin"]}><span className="cashflow-ledger__actions"><button type="button" disabled={saving} onClick={() => onReceipt(payment.id)}>Kirim kwitansi</button>{!reversedPaymentIDs.has(payment.id) && <button type="button" onClick={() => setReversing(payment)}>Koreksi</button>}</span></RoleGate>}</div></article>)}</div> : <EmptyState icon={<IconCash size={22} />} title="Belum ada pembayaran" cause="Jadwal sudah terbentuk, tetapi belum ada dana yang diverifikasi." nextStep="Pilih angsuran di atas saat pembayaran diterima." />}</section>{reversing && <section className="cashflow-reversal"><h3>Koreksi {reversing.receiptNumber}</h3><p>Pembayaran asli tidak dihapus. Sistem akan menambahkan entry pembalik sebesar {formatIDR(reversing.amountIdr)}.</p><label>Alasan koreksi<textarea rows={3} value={reversalReason} onChange={(event) => setReversalReason(event.target.value)} /></label><div><Button variant="ghost" onClick={() => setReversing(undefined)}>Batal</Button><Button variant="danger" disabled={saving} onClick={onReverse}><IconArrowBackUp size={16} />{saving ? "Mencatat..." : "Catat koreksi"}</Button></div></section>}</div>}</DetailDrawer>;
}

function PaymentFields({ detail, form, setForm, chosen, saving, onRecord }: { detail: InstallmentPlanDetail; form: PaymentForm; setForm: Dispatch<SetStateAction<PaymentForm>>; chosen?: Installment; saving: boolean; onRecord: () => void }) {
  return <section className="cashflow-payment-form"><h3>Verifikasi pembayaran manual</h3><div className="cashflow-form"><label>Angsuran<select value={form.installmentId} onChange={(event) => setForm(blankPaymentForm(event.target.value))}>{detail.installments.filter((item) => item.status !== "PAID").map((item) => <option key={item.id} value={item.id}>{item.label} | sisa {formatIDR(item.outstandingAmountIdr)}</option>)}</select></label><label>Nominal diterima (Rp)<input inputMode="numeric" value={form.amount} onChange={(event) => setForm((value) => ({ ...value, amount: event.target.value }))} placeholder={chosen ? String(chosen.outstandingAmountIdr) : ""} /></label><label>Kanal pembayaran<select value={form.method} onChange={(event) => setForm((value) => ({ ...value, method: Number(event.target.value) as InstallmentPaymentMethod }))}><option value={InstallmentPaymentMethod.BANK_TRANSFER}>Transfer bank</option><option value={InstallmentPaymentMethod.CASH}>Tunai</option><option value={InstallmentPaymentMethod.XENDIT}>Xendit</option></select></label><label>Referensi transaksi<input value={form.reference} onChange={(event) => setForm((value) => ({ ...value, reference: event.target.value }))} placeholder="Nomor mutasi atau bukti transfer" /></label><label>Catatan<input value={form.note} onChange={(event) => setForm((value) => ({ ...value, note: event.target.value }))} /></label><Button variant="emerald" disabled={saving} onClick={onRecord}><IconCheck size={16} />{saving ? "Mencatat..." : "Verifikasi pembayaran"}</Button></div></section>;
}

function InstallmentRow({ item, selected, onSelect }: { item: Installment; selected: boolean; onSelect: () => void }) { return <button type="button" className="cashflow-installment" data-selected={selected || undefined} onClick={onSelect}><span><strong>{item.label}</strong><small>Jatuh tempo {new Date(`${item.dueDate}T00:00:00`).toLocaleDateString("id-ID", { day: "numeric", month: "short", year: "numeric" })}</small></span><span><strong>{formatIDR(item.outstandingAmountIdr)}</strong><Badge tone={toneForStatus(item.status)}>{INSTALLMENT_STATUS_LABEL[item.status] ?? item.status}</Badge></span></button>; }

function AgingChart({ values }: { values: readonly bigint[] }) {
  const labels = ["Belum jatuh tempo", "1-30 hari", "31-60 hari", "61-90 hari", "> 90 hari"];
  const max = Math.max(1, ...values.map(Number));
  return <section className="tw-card tw-card--large cashflow-aging" aria-labelledby="aging-title"><div><h2 id="aging-title">Umur piutang</h2><p>Nominal tersisa dikelompokkan menurut lama keterlambatan.</p></div><div className="cashflow-aging__plot">{values.map((value, index) => <article key={labels[index]}><span>{formatIDR(value)}</span><div><i style={{ width: `${Number(value) / max * 100}%` }} /></div><small>{labels[index]}</small></article>)}</div></section>;
}

function VendorPanel({ summary, months, vendors, loading, onPaid }: { summary?: CashFlowSummary; months: MonthlyProjectionEntry[]; vendors: VendorPayment[]; loading: boolean; onPaid: (vendor: VendorPayment) => Promise<void> }) {
  const max = Math.max(1, ...months.map((month) => Number(month.vendorObligationsIdr)));
  return <div className="cashflow-vendors"><section className="cashflow-stats tw-stagger" aria-label="Ringkasan kewajiban vendor"><StatCard label="Total komitmen" value={formatIDR(summary?.totalCommittedIdr ?? 0n)} unit="rupiah" tone="brand" /><StatCard label="Sudah dibayar" value={formatIDR(summary?.totalPaidOutIdr ?? 0n)} unit="rupiah" tone="success" /><StatCard label="Belum dibayar" value={formatIDR(summary?.totalOutstandingIdr ?? 0n)} unit="rupiah" tone="warning" /><StatCard label="Lewat jatuh tempo" value={formatIDR(summary?.totalOverdueIdr ?? 0n)} unit="rupiah" tone={Number(summary?.totalOverdueIdr ?? 0n) > 0 ? "danger" : "neutral"} /></section>{months.length > 0 && <section className="tw-card tw-card--large cashflow-chart"><div><h2>Proyeksi kewajiban bulanan</h2><p>Tinggi batang menunjukkan komitmen vendor per bulan.</p></div><div className="cashflow-bars">{months.map((month) => <div key={month.month}><span>{formatIDR(month.vendorObligationsIdr)}</span><i style={{ height: `${Math.max(4, Number(month.vendorObligationsIdr) / max * 100)}%` }} /><small>{month.month}</small></div>)}</div></section>}<section className="tw-card tw-card--large cashflow-vendor-table"><div className="cashflow-section__heading"><div><h2>Kewajiban vendor</h2><p>Tagihan operasional yang memengaruhi proyeksi kas musim berjalan.</p></div></div>{loading ? <p className="cashflow-table-state">Memuat kewajiban...</p> : vendors.length ? <div className="tw-data-table__scroller"><table><thead><tr><th>Vendor</th><th>Kategori</th><th>Jatuh tempo</th><th>Status</th><th data-align="right">Jumlah</th><th>Aksi</th></tr></thead><tbody>{vendors.map((vendor) => <tr key={vendor.id}><td><strong>{vendor.vendorName}</strong>{vendor.description && <small>{vendor.description}</small>}</td><td>{CATEGORY_LABEL[vendor.category] ?? vendor.category}</td><td>{vendor.dueDate}</td><td><Badge tone={vendor.status === "PAID" ? "success" : vendor.status === "OVERDUE" ? "danger" : "warning"}>{VENDOR_STATUS_LABEL[vendor.status] ?? vendor.status}</Badge></td><td data-align="right"><strong>{formatIDR(vendor.amountIdr)}</strong></td><td>{vendor.status !== "PAID" && vendor.status !== "CANCELLED" && <RoleGate require={["owner", "admin"]}><Button variant="outline" size="sm" onClick={() => void onPaid(vendor)}>Tandai dibayar</Button></RoleGate>}</td></tr>)}</tbody></table></div> : <EmptyState icon={<IconFileInvoice size={22} />} title="Belum ada kewajiban vendor" cause="Komitmen hotel, transportasi, dan layanan lain belum dicatat." nextStep="Tambahkan kewajiban pertama dari tombol di bagian atas." />}</section></div>;
}
