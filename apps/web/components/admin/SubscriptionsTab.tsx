"use client";

import { Fragment, useCallback, useEffect, useMemo, useState } from "react";
import { IconAlertTriangle, IconBan, IconClockHour4, IconReceipt, IconRefresh } from "@tabler/icons-react";
import type {
  PlatformOperator,
  PreviewSubscriptionPlanChangeResponse,
  SubscriptionBillingCandidate,
  SubscriptionBillingResult,
  SubscriptionInvoiceRow,
} from "@hajj-saas/proto-gen/hajj/v1/platform_pb";
import { platformClient } from "@/lib/rpc";

const rupiah = (n: bigint | number) =>
  new Intl.NumberFormat("id-ID", { style: "currency", currency: "IDR", maximumFractionDigits: 0 }).format(Number(n));

const tanggal = (d: Date) => d.toLocaleDateString("id-ID", { day: "2-digit", month: "short", year: "numeric" });

const STAGE_LABEL: Record<string, string> = {
  H1: "Pengingat 1", H7: "Pengingat 2", H14: "Peringatan akhir", SUSPEND: "Ditangguhkan",
};

function daysOverdue(accessUntil?: Date): number {
  if (!accessUntil) return 0;
  const ms = Date.now() - accessUntil.getTime();
  return ms <= 0 ? 0 : Math.floor(ms / 86_400_000);
}

export default function SubscriptionsTab() {
  const [operators, setOperators] = useState<PlatformOperator[]>([]);
  const [invoices, setInvoices] = useState<SubscriptionInvoiceRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [notice, setNotice] = useState("");
  const [voiding, setVoiding] = useState("");
  const [reason, setReason] = useState("");
  const [billingPreview, setBillingPreview] = useState<SubscriptionBillingCandidate[] | null>(null);
  const [billingResults, setBillingResults] = useState<SubscriptionBillingResult[] | null>(null);
  const [billingBusy, setBillingBusy] = useState(false);
  const [defaultGraceDays, setDefaultGraceDays] = useState(0);
  const [dunningDays, setDunningDays] = useState<number[]>([]);
  const [suspendAfterDays, setSuspendAfterDays] = useState(0);
  const [trialDays, setTrialDays] = useState(10);
  const [trialDraft, setTrialDraft] = useState("10");
  const [trialReason, setTrialReason] = useState("");
  const [trialConfirmation, setTrialConfirmation] = useState("");
  const [trialBusy, setTrialBusy] = useState(false);
  const [graceScope, setGraceScope] = useState("GLOBAL");
  const [graceDays, setGraceDays] = useState("0");
  const [graceReason, setGraceReason] = useState("");
  const [graceConfirmation, setGraceConfirmation] = useState("");
  const [useDefaultGrace, setUseDefaultGrace] = useState(false);
  const [graceBusy, setGraceBusy] = useState(false);
  const [planOperatorId, setPlanOperatorId] = useState("");
  const [newPlan, setNewPlan] = useState("GROWTH");
  const [planPreview, setPlanPreview] = useState<PreviewSubscriptionPlanChangeResponse | null>(null);
  const [planReason, setPlanReason] = useState("");
  const [planConfirmation, setPlanConfirmation] = useState("");
  const [planBusy, setPlanBusy] = useState(false);
  const [extendOperatorId, setExtendOperatorId] = useState("");
  const [extendDays, setExtendDays] = useState("7");
  const [extendReason, setExtendReason] = useState("");
  const [extendConfirmation, setExtendConfirmation] = useState("");
  const [extendBusy, setExtendBusy] = useState(false);
  const [cancelOperatorId, setCancelOperatorId] = useState("");
  const [cancelReason, setCancelReason] = useState("");
  const [cancelConfirmation, setCancelConfirmation] = useState("");
  const [cancelBusy, setCancelBusy] = useState(false);

  const refresh = useCallback(() => {
    setLoading(true);
    Promise.all([
      platformClient.listOperators({}),
      platformClient.listSubscriptionInvoices({ limit: 100 }),
      platformClient.getSubscriptionBillingSettings({}),
    ])
      .then(([operatorResponse, invoiceResponse, settingsResponse]) => {
        setOperators(operatorResponse.operators);
        setInvoices(invoiceResponse.invoices);
        setDefaultGraceDays(settingsResponse.defaultGracePeriodDays);
        setDunningDays(settingsResponse.dunningDays);
        setSuspendAfterDays(settingsResponse.suspendAfterDays);
        setTrialDays(settingsResponse.trialDays);
        setTrialDraft(String(settingsResponse.trialDays));
      })
      .catch(() => setNotice("Gagal memuat data langganan."))
      .finally(() => setLoading(false));
  }, []);
  useEffect(refresh, [refresh]);

  const lapsed = useMemo(
    () => operators
      .filter((o) => o.suspendedAt || (o.effectiveAccessUntil && o.effectiveAccessUntil.toDate() < new Date()))
      .sort((a, b) => (a.effectiveAccessUntil?.toDate().getTime() ?? 0) - (b.effectiveAccessUntil?.toDate().getTime() ?? 0)),
    [operators],
  );
  const suspended = useMemo(() => operators.filter((o) => o.suspendedAt).length, [operators]);
  const outstanding = useMemo(
    () => lapsed.reduce((sum, o) => sum + Number(o.outstandingIdr), 0),
    [lapsed],
  );

  async function voidInvoice(invoiceId: string) {
    try {
      await platformClient.voidSubscriptionInvoice({ invoiceId, reason: reason.trim() });
      setVoiding("");
      setReason("");
      setNotice("Invoice dibatalkan. Barisnya tetap tersimpan sebagai catatan.");
      refresh();
    } catch {
      setNotice("Gagal membatalkan. Invoice yang sudah dibayar tidak bisa dibatalkan.");
    }
  }

  async function previewBilling() {
    setBillingBusy(true);
    setBillingResults(null);
    try {
      const response = await platformClient.previewSubscriptionBilling({});
      setBillingPreview(response.candidates);
      setNotice(response.candidates.length === 0 ? "Tidak ada periode langganan yang perlu ditagih sekarang." : "");
    } catch {
      setNotice("Gagal menyiapkan pratinjau tagihan. Tidak ada invoice yang diterbitkan.");
    } finally {
      setBillingBusy(false);
    }
  }

  async function issueBilling() {
    if (!billingPreview?.length) return;
    setBillingBusy(true);
    try {
      const response = await platformClient.issueSubscriptionBilling({
        targets: billingPreview.map((candidate) => ({
          operatorId: candidate.operatorId,
          plan: candidate.plan,
          periodStart: candidate.periodStart,
          expectedBaseAmountIdr: candidate.baseAmountIdr,
        })),
      });
      setBillingResults(response.results);
      setNotice(
        response.failedCount > 0
          ? `${response.issuedCount} invoice terbit; ${response.failedCount} gagal dan perlu ditinjau.`
          : `${response.issuedCount} invoice berhasil diterbitkan tanpa kegagalan.`,
      );
      refresh();
    } catch {
      setNotice("Siklus tagihan tidak dapat dijalankan. Tidak ada hasil yang disembunyikan; coba muat ulang pratinjau.");
    } finally {
      setBillingBusy(false);
    }
  }

  const selectedGraceOperator = operators.find((operator) => operator.id === graceScope);

  async function saveTrialDays() {
    const days = Number(trialDraft);
    if (!Number.isInteger(days) || days < 1 || days > 90 || !trialReason.trim() || trialConfirmation !== "TRIAL") return;
    setTrialBusy(true);
    try {
      const response = await platformClient.setTrialDays({
        trialDays: days,
        reason: trialReason.trim(),
        confirmation: trialConfirmation,
        idempotencyKey: crypto.randomUUID(),
      });
      setTrialDays(response.trialDays);
      setTrialDraft(String(response.trialDays));
      setTrialReason("");
      setTrialConfirmation("");
      setNotice(`Masa trial untuk pendaftaran baru menjadi ${response.trialDays} hari.`);
    } catch {
      setNotice("Masa trial gagal disimpan. Muat ulang kebijakan lalu coba kembali.");
    } finally {
      setTrialBusy(false);
    }
  }

  async function saveGracePeriod() {
    const parsedDays = Number(graceDays);
    if ((!useDefaultGrace && (!Number.isInteger(parsedDays) || parsedDays < 0 || parsedDays > 90)) || !graceReason.trim() || !graceConfirmation.trim()) return;
    setGraceBusy(true);
    try {
      await platformClient.setSubscriptionGracePeriod({
        operatorId: graceScope === "GLOBAL" ? "" : graceScope,
        gracePeriodDays: useDefaultGrace ? undefined : parsedDays,
        usePlatformDefault: graceScope !== "GLOBAL" && useDefaultGrace,
        reason: graceReason.trim(),
        confirmation: graceConfirmation.trim(),
        idempotencyKey: crypto.randomUUID(),
      });
      setNotice(graceScope === "GLOBAL" ? `Grace period global menjadi ${parsedDays} hari.` : useDefaultGrace ? `${selectedGraceOperator?.name ?? "Travel"} kembali mengikuti default global.` : `Grace period ${selectedGraceOperator?.name ?? "travel"} menjadi ${parsedDays} hari.`);
      setGraceReason("");
      setGraceConfirmation("");
      refresh();
    } catch {
      setNotice("Grace period gagal disimpan. Pastikan konfirmasi sama persis dan muat ulang jika kebijakan berubah.");
    } finally {
      setGraceBusy(false);
    }
  }

  const selectedPlanOperator = operators.find((operator) => operator.id === planOperatorId);

  async function previewPlanChange() {
    if (!planOperatorId || !newPlan || selectedPlanOperator?.plan === newPlan) return;
    setPlanBusy(true);
    try {
      const response = await platformClient.previewSubscriptionPlanChange({ operatorId: planOperatorId, newPlan });
      setPlanPreview(response);
      setNotice("");
    } catch {
      setPlanPreview(null);
      setNotice("Prorata tidak dapat dihitung. Pastikan langganan masih aktif, tidak ditangguhkan, dan paketnya berbeda.");
    } finally {
      setPlanBusy(false);
    }
  }

  async function applyPlanChange() {
    if (!planPreview || !planReason.trim() || !planConfirmation.trim()) return;
    setPlanBusy(true);
    try {
      const response = await platformClient.applySubscriptionPlanChange({
        operatorId: planPreview.operatorId,
        newPlan: planPreview.newPlan,
        expectedAdjustmentIdr: planPreview.adjustmentIdr,
        reason: planReason.trim(),
        confirmation: planConfirmation.trim(),
        idempotencyKey: crypto.randomUUID(),
      });
      setNotice(
        response.status === "PENDING_PAYMENT"
          ? `Upgrade menunggu pembayaran invoice ${rupiah(response.invoiceAmountIdr)}. Paket belum berubah.`
          : `Downgrade diterapkan. Kredit tenant sekarang ${rupiah(response.creditBalanceIdr)}.`,
      );
      setPlanPreview(null);
      setPlanReason("");
      setPlanConfirmation("");
      refresh();
    } catch {
      setNotice("Perubahan paket tidak diterapkan. Muat ulang pratinjau; nominal atau kondisi langganan mungkin berubah.");
    } finally {
      setPlanBusy(false);
    }
  }

  const selectedExtendOperator = operators.find((operator) => operator.id === extendOperatorId);
  const trialOperators = useMemo(() => operators.filter((o) => o.subscriptionStatus === "TRIALING"), [operators]);

  async function extendTrial() {
    const days = Number(extendDays);
    if (!extendOperatorId || !Number.isInteger(days) || days < 1 || days > 90 || !extendReason.trim() || !extendConfirmation.trim()) return;
    setExtendBusy(true);
    try {
      const response = await platformClient.extendTrial({
        operatorId: extendOperatorId, additionalDays: days,
        reason: extendReason.trim(), confirmation: extendConfirmation.trim(),
        idempotencyKey: crypto.randomUUID(),
      });
      setNotice(`Trial ${selectedExtendOperator?.name ?? "travel"} diperpanjang sampai ${tanggal(response.accessUntil?.toDate() ?? new Date())}.`);
      setExtendOperatorId(""); setExtendReason(""); setExtendConfirmation("");
      refresh();
    } catch {
      setNotice("Trial gagal diperpanjang. Pastikan langganan masih berstatus trial dan konfirmasi nama sudah sama persis.");
    } finally {
      setExtendBusy(false);
    }
  }

  const selectedCancelOperator = operators.find((operator) => operator.id === cancelOperatorId);
  const cancellableOperators = useMemo(() => operators.filter((o) => !o.cancelledAt), [operators]);

  async function cancelSubscription() {
    if (!cancelOperatorId || !cancelReason.trim() || !cancelConfirmation.trim()) return;
    setCancelBusy(true);
    try {
      const response = await platformClient.cancelSubscription({
        operatorId: cancelOperatorId, reason: cancelReason.trim(), confirmation: cancelConfirmation.trim(),
        idempotencyKey: crypto.randomUUID(),
      });
      setNotice(`Langganan ${selectedCancelOperator?.name ?? "travel"} dibatalkan. Akses tetap berjalan sampai ${tanggal(response.accessUntil?.toDate() ?? new Date())} — sisa periode yang sudah dibayar.`);
      setCancelOperatorId(""); setCancelReason(""); setCancelConfirmation("");
      refresh();
    } catch {
      setNotice("Pembatalan gagal. Langganan mungkin sudah dibatalkan sebelumnya, atau konfirmasi nama belum sama persis.");
    } finally {
      setCancelBusy(false);
    }
  }

  if (loading) return <p style={muted}>Memuat data langganan…</p>;

  return (
    <section style={{ display: "grid", gap: 20 }}>
      <div>
        <h2 style={heading}>Langganan</h2>
        <p style={muted}>
          {operators.length} travel · {lapsed.length} lewat jatuh tempo · {suspended} ditangguhkan
          {outstanding > 0 ? ` · ${rupiah(outstanding)} belum tertagih` : ""}
        </p>
      </div>

      {notice && <p role="status" style={noticeBox}>{notice}</p>}

      {lapsed.length > 0 && (
        <p style={warnBox}>
          <IconAlertTriangle size={16} />
          {rupiah(outstanding)} belum tertagih dari {lapsed.length} travel. Pengingat berjalan otomatis;
          penangguhan menutup akses tanpa menghapus data apa pun.
        </p>
      )}

      <details className="admin-policy-card tw-card">
        <summary>
          <span className="admin-policy-card__summary-icon" aria-hidden="true"><IconClockHour4 size={18} /></span>
          <span><strong>Masa trial tenant baru</strong><small>{trialDays} hari, hanya untuk langganan yang dibuat setelah perubahan</small></span>
          <span className="tw-badge tw-badge--neutral">Kebijakan akuisisi</span>
        </summary>
        <div className="admin-policy-card__body">
          <div className="admin-policy-card__callout">
            <strong>Trial yang sedang berjalan tidak dipotong.</strong>
            <span>Nilai ini dibaca satu kali ketika langganan dibuat. Perubahan hanya berlaku untuk tenant berikutnya.</span>
          </div>
          <div className="admin-policy-card__form">
            <label>
              <span>Masa trial (hari)</span>
              <input type="number" min={1} max={90} value={trialDraft} onChange={(event) => setTrialDraft(event.target.value)} />
              <small>Minimal 1 hari, maksimal 90 hari.</small>
            </label>
            <label>
              <span>Alasan perubahan</span>
              <input value={trialReason} maxLength={500} onChange={(event) => setTrialReason(event.target.value)} placeholder="Contoh: evaluasi hasil program onboarding" />
            </label>
            <label>
              <span>Ketik TRIAL untuk mengonfirmasi</span>
              <input value={trialConfirmation} autoComplete="off" onChange={(event) => setTrialConfirmation(event.target.value.toUpperCase())} />
            </label>
          </div>
          <div className="admin-policy-card__footer">
            <span>Perubahan dicatat di audit log dan aman terhadap submit ulang.</span>
            <button className="tw-btn tw-btn--emerald tw-btn--md" onClick={saveTrialDays} disabled={trialBusy || !Number.isInteger(Number(trialDraft)) || Number(trialDraft) < 1 || Number(trialDraft) > 90 || !trialReason.trim() || trialConfirmation !== "TRIAL"}>
              {trialBusy ? "Menyimpan..." : "Simpan masa trial"}
            </button>
          </div>
        </div>
      </details>

      <details className="admin-grace-settings tw-card">
        <summary>
          <span><strong>Grace period akses</strong><small>Default {defaultGraceDays} hari · pengingat H+{dunningDays.join(", H+")} · suspend H+{suspendAfterDays}</small></span>
          <span className="tw-badge tw-badge--neutral">Kebijakan billing</span>
        </summary>
        <div className="admin-grace-settings__body">
          <p>Grace memperpanjang batas akses efektif tanpa mengubah tanggal akses yang sudah dibayar. Penangguhan manual tetap menutup akses.</p>
          <div className="admin-grace-settings__grid">
            <label>
              <span>Cakupan</span>
              <select value={graceScope} onChange={(event) => { setGraceScope(event.target.value); setUseDefaultGrace(false); setGraceConfirmation(""); }}>
                <option value="GLOBAL">Default seluruh travel</option>
                {operators.map((operator) => <option key={operator.id} value={operator.id}>{operator.name} · efektif {operator.gracePeriodDays} hari</option>)}
              </select>
            </label>
            <label>
              <span>Grace period (hari)</span>
              <input type="number" min={0} max={90} value={graceDays} disabled={useDefaultGrace} onChange={(event) => setGraceDays(event.target.value)} />
            </label>
            <label className="admin-grace-settings__wide">
              <span>Alasan perubahan</span>
              <input value={graceReason} maxLength={500} onChange={(event) => setGraceReason(event.target.value)} placeholder="Contoh: waktu kliring transfer kontrak enterprise" />
            </label>
            {graceScope !== "GLOBAL" && (
              <label className="admin-grace-settings__check">
                <input type="checkbox" checked={useDefaultGrace} onChange={(event) => setUseDefaultGrace(event.target.checked)} />
                <span>Hapus override dan ikuti default global ({defaultGraceDays} hari)</span>
              </label>
            )}
            <label className="admin-grace-settings__wide">
              <span>Konfirmasi dengan mengetik {graceScope === "GLOBAL" ? "GLOBAL" : selectedGraceOperator?.name ?? "nama travel"}</span>
              <input value={graceConfirmation} onChange={(event) => setGraceConfirmation(event.target.value)} />
            </label>
          </div>
          <div className="admin-grace-settings__footer">
            <span>Perubahan dicatat di audit log dan aman terhadap submit ulang.</span>
            <button className="tw-btn tw-btn--outline tw-btn--md" disabled={graceBusy || !graceReason.trim() || !graceConfirmation.trim() || (!useDefaultGrace && (!Number.isInteger(Number(graceDays)) || Number(graceDays) < 0 || Number(graceDays) > 90))} onClick={saveGracePeriod}>
              {graceBusy ? "Menyimpan…" : "Simpan grace period"}
            </button>
          </div>
        </div>
      </details>

      <details className="admin-plan-change tw-card">
        <summary>
          <span><strong>Ubah paket dengan prorata</strong><small>Upgrade aktif setelah dibayar · downgrade menjadi kredit tenant</small></span>
          <span className="tw-badge tw-badge--neutral">Ledger append-only</span>
        </summary>
        <div className="admin-plan-change__body">
          <div className="admin-plan-change__selectors">
            <label><span>Travel</span><select value={planOperatorId} onChange={(event) => { setPlanOperatorId(event.target.value); setPlanPreview(null); setPlanConfirmation(""); }}><option value="">Pilih travel</option>{operators.map((operator) => <option key={operator.id} value={operator.id}>{operator.name} · {operator.plan}{operator.creditBalanceIdr > 0n ? ` · kredit ${rupiah(operator.creditBalanceIdr)}` : ""}</option>)}</select></label>
            <label><span>Paket baru</span><select value={newPlan} onChange={(event) => { setNewPlan(event.target.value); setPlanPreview(null); }}>{["STARTER", "GROWTH", "PRO"].map((plan) => <option key={plan} value={plan} disabled={selectedPlanOperator?.plan === plan}>{plan}{selectedPlanOperator?.plan === plan ? " (saat ini)" : ""}</option>)}</select></label>
            <button className="tw-btn tw-btn--outline tw-btn--md" onClick={previewPlanChange} disabled={planBusy || !planOperatorId || selectedPlanOperator?.plan === newPlan}>{planBusy ? "Menghitung…" : "Hitung prorata"}</button>
          </div>
          {planPreview && (
            <div className="admin-plan-change__preview">
              <div className="admin-plan-change__summary">
                <span><small>Perubahan</small><strong>{planPreview.currentPlan} → {planPreview.newPlan}</strong></span>
                <span><small>Sisa periode</small><strong>{planPreview.remainingDays} hari</strong></span>
                <span><small>{planPreview.adjustmentIdr > 0n ? "Perlu dibayar" : "Kredit baru"}</small><strong>{rupiah(planPreview.adjustmentIdr > 0n ? planPreview.adjustmentIdr : -planPreview.adjustmentIdr)}</strong></span>
              </div>
              <p>{planPreview.adjustmentIdr > 0n ? "Paket baru belum aktif sampai invoice prorata lunas; pembayaran tidak menambah masa akses." : `Paket baru berlaku langsung. Kredit ditambahkan ke saldo ${rupiah(planPreview.currentCreditBalanceIdr)} yang sudah ada.`}</p>
              <div className="admin-plan-change__form">
                <label><span>Alasan perubahan</span><input value={planReason} maxLength={500} onChange={(event) => setPlanReason(event.target.value)} /></label>
                <label><span>Konfirmasi dengan mengetik {planPreview.operatorName}</span><input value={planConfirmation} onChange={(event) => setPlanConfirmation(event.target.value)} /></label>
              </div>
              <div className="admin-plan-change__footer">
                <button className="tw-btn tw-btn--ghost tw-btn--sm" onClick={() => setPlanPreview(null)} disabled={planBusy}>Batal</button>
                <button className="tw-btn tw-btn--outline tw-btn--md" onClick={applyPlanChange} disabled={planBusy || !planReason.trim() || planConfirmation.trim().toLocaleLowerCase("id-ID") !== planPreview.operatorName.trim().toLocaleLowerCase("id-ID")}>{planBusy ? "Menerapkan…" : planPreview.adjustmentIdr > 0n ? "Terbitkan invoice upgrade" : "Terapkan downgrade & kredit"}</button>
              </div>
            </div>
          )}
        </div>
      </details>

      <details className="admin-plan-change tw-card">
        <summary>
          <span><strong>Perpanjang trial per tenant</strong><small>Prospek yang masih serius mengevaluasi tidak terkunci kalender</small></span>
          <span className="tw-badge tw-badge--neutral">Hanya trial</span>
        </summary>
        <div className="admin-plan-change__body">
          <div className="admin-plan-change__selectors">
            <label><span>Travel</span>
              <select value={extendOperatorId} onChange={(event) => { setExtendOperatorId(event.target.value); setExtendConfirmation(""); }}>
                <option value="">Pilih travel yang masih trial</option>
                {trialOperators.map((operator) => (
                  <option key={operator.id} value={operator.id}>
                    {operator.name} · trial sampai {operator.accessUntil ? tanggal(operator.accessUntil.toDate()) : "—"}
                  </option>
                ))}
              </select>
              {trialOperators.length === 0 && <small>Tidak ada travel yang masih berstatus trial saat ini.</small>}
            </label>
            <label><span>Tambah (hari)</span><input type="number" min={1} max={90} value={extendDays} onChange={(event) => setExtendDays(event.target.value)} /></label>
          </div>
          <div className="admin-plan-change__form">
            <label><span>Alasan perpanjangan</span><input value={extendReason} maxLength={500} onChange={(event) => setExtendReason(event.target.value)} placeholder="Contoh: masih menunggu persetujuan internal calon pelanggan" /></label>
            <label><span>Konfirmasi dengan mengetik {selectedExtendOperator?.name ?? "nama travel"}</span><input value={extendConfirmation} onChange={(event) => setExtendConfirmation(event.target.value)} /></label>
          </div>
          <div className="admin-plan-change__footer">
            <span>Hanya menambah hari — trial yang sudah lewat waktunya tetap bisa diperpanjang dari tanggal aslinya.</span>
            <button className="tw-btn tw-btn--outline tw-btn--md" onClick={() => void extendTrial()}
              disabled={extendBusy || !extendOperatorId || !extendReason.trim() || !extendConfirmation.trim()}>
              {extendBusy ? "Menyimpan…" : "Perpanjang trial"}
            </button>
          </div>
        </div>
      </details>

      <details className="admin-plan-change tw-card">
        <summary>
          <span><strong>Batalkan langganan</strong><small>Bukan penghapusan — akses tetap berjalan sampai periode yang sudah dibayar habis</small></span>
          <span className="tw-badge tw-badge--neutral">Tidak bisa ditarik kembali</span>
        </summary>
        <div className="admin-plan-change__body">
          <div className="admin-plan-change__selectors">
            <label><span>Travel</span>
              <select value={cancelOperatorId} onChange={(event) => { setCancelOperatorId(event.target.value); setCancelConfirmation(""); }}>
                <option value="">Pilih travel</option>
                {cancellableOperators.map((operator) => (
                  <option key={operator.id} value={operator.id}>
                    {operator.name} · akses sampai {operator.accessUntil ? tanggal(operator.accessUntil.toDate()) : "—"}
                  </option>
                ))}
              </select>
            </label>
          </div>
          <div className="admin-plan-change__form">
            <label><span>Alasan pembatalan</span><input value={cancelReason} maxLength={500} onChange={(event) => setCancelReason(event.target.value)} placeholder="Contoh: pindah ke penyedia lain" /></label>
            <label><span>Konfirmasi dengan mengetik {selectedCancelOperator?.name ?? "nama travel"}</span><input value={cancelConfirmation} onChange={(event) => setCancelConfirmation(event.target.value)} /></label>
          </div>
          <div className="admin-plan-change__footer">
            <span>cancelled_at tercatat; access_until tidak disentuh — sisa periode yang sudah dibayar tetap haknya.</span>
            <button className="tw-btn tw-btn--ghost tw-btn--md" onClick={() => void cancelSubscription()}
              disabled={cancelBusy || !cancelOperatorId || !cancelReason.trim() || !cancelConfirmation.trim()}>
              <IconBan size={15} />{cancelBusy ? "Membatalkan…" : "Batalkan langganan"}
            </button>
          </div>
        </div>
      </details>

      <div className="admin-billing-cycle tw-card" aria-labelledby="billing-cycle-title">
        <div className="admin-billing-cycle__header">
          <div>
            <h3 id="billing-cycle-title">Siklus tagihan massal</h3>
            <p style={muted}>Tinjau travel, periode, dan nominal sebelum satu invoice pun diterbitkan.</p>
          </div>
          {billingPreview === null ? (
            <button className="tw-btn tw-btn--emerald tw-btn--md" onClick={previewBilling} disabled={billingBusy}>
              <IconReceipt size={17} />{billingBusy ? "Menyiapkan…" : "Tinjau siklus"}
            </button>
          ) : (
            <button className="tw-btn tw-btn--ghost tw-btn--sm" onClick={previewBilling} disabled={billingBusy}>
              <IconRefresh size={15} />Muat ulang pratinjau
            </button>
          )}
        </div>

        {billingPreview !== null && !billingResults && (
          billingPreview.length === 0 ? (
            <div style={emptyBox}>
              <p style={{ margin: 0, fontWeight: 700 }}>Tidak ada yang perlu diterbitkan</p>
              <p style={{ ...muted, marginTop: 6 }}>Travel dengan invoice tertunda atau periode yang sudah ditagih tidak dimasukkan lagi.</p>
            </div>
          ) : (
            <div className="admin-billing-cycle__body">
              <div className="tw-data-table admin-billing-cycle__table">
                <div className="tw-data-table__scroller">
                  <table>
                    <caption style={caption}>Pratinjau saja · belum ada perubahan data</caption>
                    <thead><tr>{["Travel", "Paket", "Periode", "Jatuh tempo", "Nominal"].map((h) => <th key={h}>{h}</th>)}</tr></thead>
                    <tbody>
                      {billingPreview.map((candidate) => (
                        <tr key={`${candidate.operatorId}-${candidate.periodStart?.toDate().toISOString()}`}>
                          <td><strong>{candidate.operatorName}</strong></td>
                          <td>{candidate.plan}</td>
                          <td>{candidate.periodStart && candidate.periodEnd ? `${tanggal(candidate.periodStart.toDate())}–${tanggal(candidate.periodEnd.toDate())}` : "—"}</td>
                          <td>{candidate.dueAt ? tanggal(candidate.dueAt.toDate()) : "—"}</td>
                          <td data-align="right">{rupiah(candidate.baseAmountIdr)}</td>
                        </tr>
                      ))}
                    </tbody>
                    <tfoot><tr><td colSpan={4}><strong>Total {billingPreview.length} invoice</strong></td><td data-align="right"><strong>{rupiah(billingPreview.reduce((sum, item) => sum + Number(item.baseAmountIdr), 0))}</strong></td></tr></tfoot>
                  </table>
                </div>
              </div>
              <div className="admin-billing-cycle__actions">
                <p>Setiap travel diproses terpisah. Kegagalan satu nominal tidak membatalkan invoice lain.</p>
                <button className="tw-btn tw-btn--emerald tw-btn--md" onClick={issueBilling} disabled={billingBusy}>
                  <IconReceipt size={17} />{billingBusy ? "Menerbitkan…" : `Terbitkan ${billingPreview.length} invoice`}
                </button>
              </div>
            </div>
          )
        )}

        {billingResults && (
          <div className="admin-billing-cycle__body">
            <div className="tw-data-table admin-billing-cycle__table">
              <div className="tw-data-table__scroller">
                <table>
                  <caption style={caption}>Hasil siklus · setiap baris dilaporkan</caption>
                  <thead><tr>{["Travel", "Hasil", "Nominal", "Keterangan"].map((h) => <th key={h}>{h}</th>)}</tr></thead>
                  <tbody>{billingResults.map((result, index) => (
                    <tr key={`${result.operatorId}-${index}`}>
                      <td><strong>{result.operatorName || result.operatorId}</strong></td>
                      <td>
                        <span className={`tw-badge ${result.errorCode ? "tw-badge--danger" : result.alreadyIssued ? "tw-badge--neutral" : "tw-badge--success"}`}>
                          {result.errorCode ? "Gagal" : result.alreadyIssued ? "Sudah ada" : "Terbit"}
                        </span>
                      </td>
                      <td>{result.amountIdr > 0n ? rupiah(result.amountIdr) : "—"}</td>
                      <td>{result.message}{result.errorCode ? ` (${result.errorCode})` : ""}</td>
                    </tr>
                  ))}</tbody>
                </table>
              </div>
            </div>
            <div className="admin-billing-cycle__actions">
              <p>Hasil ini aman dimuat ulang: periode yang sudah terbit tidak akan ditagih dua kali.</p>
              <button className="tw-btn tw-btn--outline tw-btn--sm" onClick={() => { setBillingPreview(null); setBillingResults(null); }}>Selesai</button>
            </div>
          </div>
        )}
      </div>

      <div>
        <h3 style={{ margin: "0 0 4px", fontSize: 15 }}>Yang lewat jatuh tempo</h3>
        {lapsed.length === 0 ? (
          <div style={emptyBox}>
            <p style={{ margin: 0, fontWeight: 700 }}>Semua langganan lancar</p>
            <p style={{ ...muted, marginTop: 6 }}>
              Tidak ada travel yang aksesnya sudah lewat. Pengingat hanya berjalan saat ada yang telat.
            </p>
          </div>
        ) : (
          <table style={table}>
            <caption style={caption}>Lintas seluruh travel · yang paling lama telat di atas</caption>
            <thead>
              <tr>{["Travel", "Paket", "Akses efektif habis", "Telat", "Tahap pengingat", "Belum dibayar", "Status"].map((h) => <th key={h} style={th}>{h}</th>)}</tr>
            </thead>
            <tbody>
              {lapsed.map((operator) => {
                const late = daysOverdue(operator.effectiveAccessUntil?.toDate());
                return (
                  <tr key={operator.id} style={operator.suspendedAt ? { ...tr, background: "var(--color-danger-100)" } : tr}>
                    <td style={{ ...td, fontWeight: 700 }}>{operator.name}</td>
                    <td style={td}>{operator.plan || "—"}</td>
                    <td style={td}>
                      {operator.effectiveAccessUntil ? tanggal(operator.effectiveAccessUntil.toDate()) : "—"}
                      {operator.gracePeriodDays > 0 && <small className="admin-grace-note">Dibayar s.d. {operator.accessUntil ? tanggal(operator.accessUntil.toDate()) : "—"} · grace {operator.gracePeriodDays} hari</small>}
                    </td>
                    <td style={td}>{late} hari</td>
                    <td style={td}>
                      {operator.dunningStage
                        ? STAGE_LABEL[operator.dunningStage] ?? operator.dunningStage
                        : <span style={{ color: "var(--color-warm-400)" }}>belum dikirim</span>}
                    </td>
                    <td style={td}>{operator.outstandingIdr > 0n ? rupiah(operator.outstandingIdr) : "—"}</td>
                    <td style={td}>
                      {operator.suspendedAt
                        ? <span style={{ color: "var(--color-danger-600)", fontWeight: 700 }}>Ditangguhkan</span>
                        : operator.subscriptionStatus || "—"}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        )}
      </div>

      <div>
        <h3 style={{ margin: "0 0 4px", fontSize: 15 }}>Riwayat tagihan</h3>
        <p style={muted}>
          Invoice yang dibatalkan tetap ditampilkan — ia bagian dari catatan, bukan kesalahan yang disembunyikan.
        </p>
        {invoices.length === 0 ? (
          <div style={emptyBox}>
            <p style={{ margin: 0, fontWeight: 700 }}>Belum ada tagihan</p>
            <p style={{ ...muted, marginTop: 6 }}>
              Tagihan perpanjangan terbit otomatis tujuh hari sebelum akses habis.
            </p>
          </div>
        ) : (
          <table style={table}>
            <caption style={caption}>Lintas seluruh travel · 100 terakhir</caption>
            <thead>
              <tr>{["Travel", "Paket", "Nominal", "Kanal", "Jatuh tempo", "Status", ""].map((h) => <th key={h} style={th}>{h}</th>)}</tr>
            </thead>
            <tbody>
              {invoices.map((invoice) => (
                <Fragment key={invoice.id}>
                  <tr style={tr}>
                    <td style={{ ...td, fontWeight: 700 }}>{invoice.operatorName}</td>
                    <td style={td}>{invoice.plan}</td>
                    <td style={td}>{rupiah(invoice.amountIdr)}</td>
                    <td style={td}>{invoice.channel === "BANK_TRANSFER" ? "Transfer" : "Gateway"}</td>
                    <td style={td}>{invoice.dueAt ? tanggal(invoice.dueAt.toDate()) : "—"}</td>
                    <td style={td}>
                      {invoice.voidedAt
                        ? <span style={{ color: "var(--color-warm-400)" }}>Dibatalkan</span>
                        : invoice.status === "PAID"
                          ? <span style={{ color: "var(--color-emerald-800)", fontWeight: 700 }}>Lunas</span>
                          : invoice.status}
                    </td>
                    <td style={td}>
                      {invoice.status === "PENDING" && (
                        voiding === invoice.id ? (
                          <div style={{ display: "grid", gap: 6, minWidth: 220 }}>
                            <input
                              value={reason}
                              onChange={(e) => setReason(e.target.value)}
                              style={input}
                              placeholder="Alasan pembatalan"
                              aria-label={`Alasan membatalkan invoice ${invoice.operatorName}`}
                              autoFocus
                            />
                            <div style={{ display: "flex", gap: 6 }}>
                              <button
                                onClick={() => voidInvoice(invoice.id)}
                                disabled={!reason.trim()}
                                style={{ ...danger, opacity: reason.trim() ? 1 : 0.5 }}
                              >
                                Batalkan
                              </button>
                              <button onClick={() => { setVoiding(""); setReason(""); }} style={ghost}>Tutup</button>
                            </div>
                          </div>
                        ) : (
                          <button onClick={() => { setVoiding(invoice.id); setReason(""); }} style={ghost}>
                            <IconBan size={15} />Batalkan
                          </button>
                        )
                      )}
                    </td>
                  </tr>
                  {invoice.voidedAt && invoice.voidedReason && (
                    <tr style={tr}>
                      <td colSpan={7} style={{ ...td, color: "var(--color-warm-400)", fontSize: 12 }}>
                        Dibatalkan {tanggal(invoice.voidedAt.toDate())} — {invoice.voidedReason}
                      </td>
                    </tr>
                  )}
                </Fragment>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </section>
  );
}

const muted: React.CSSProperties = { color: "var(--color-warm-500)", fontSize: 14, margin: 0 };
const heading: React.CSSProperties = { margin: "0 0 4px", fontSize: 18 };
const input: React.CSSProperties = { minHeight: 38, border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 10px", fontFamily: "inherit", fontSize: 13 };
const ghost: React.CSSProperties = { minHeight: 38, border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 14px", background: "transparent", color: "var(--color-warm-700)", fontSize: 13, display: "inline-flex", alignItems: "center", gap: 6 };
const danger: React.CSSProperties = { minHeight: 38, border: 0, borderRadius: 8, padding: "0 14px", background: "var(--color-danger-600)", color: "#fff", fontWeight: 700, fontSize: 13 };
const table: React.CSSProperties = { width: "100%", borderCollapse: "collapse", background: "#fff" };
const caption: React.CSSProperties = { captionSide: "top", textAlign: "left", padding: "0 0 8px", fontSize: 11, color: "var(--color-warm-400)", letterSpacing: "0.06em", textTransform: "uppercase" };
const th: React.CSSProperties = { textAlign: "left", padding: 12, fontSize: 11, color: "var(--color-warm-400)", borderBottom: "1px solid var(--color-cream-300)", whiteSpace: "nowrap" };
const tr: React.CSSProperties = { borderBottom: "1px solid var(--color-cream-300)" };
const td: React.CSSProperties = { padding: 12, color: "var(--color-warm-700)", verticalAlign: "top", fontSize: 13 };
const noticeBox: React.CSSProperties = { margin: 0, padding: "10px 14px", borderRadius: 8, background: "var(--color-emerald-50)", color: "var(--color-emerald-800)", fontSize: 13 };
const warnBox: React.CSSProperties = { display: "flex", alignItems: "center", gap: 8, margin: 0, padding: "12px 16px", background: "var(--color-warning-50)", border: "1px solid var(--color-warning-200)", borderRadius: 8, color: "var(--color-warning-700)", fontSize: 13, fontWeight: 600 };
const emptyBox: React.CSSProperties = { padding: "20px 18px", border: "1px dashed var(--color-cream-400)", borderRadius: 10, background: "#fff", marginTop: 10 };
