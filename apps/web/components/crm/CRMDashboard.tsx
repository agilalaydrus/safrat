"use client";

import { Timestamp } from "@bufbuild/protobuf";
import {
  CRMActivityKind,
  CRMLeadSource,
  CRMLeadStage,
  type CRMAssignee,
  type CRMDashboard as CRMDashboardData,
  type CRMLead,
  type CRMLeadDetail,
} from "@hajj-saas/proto-gen/hajj/v1/crm_pb";
import type { Product } from "@hajj-saas/proto-gen/hajj/v1/product_pb";
import type { Season } from "@hajj-saas/proto-gen/hajj/v1/season_pb";
import {
  IconAlertCircle,
  IconArrowRight,
  IconBrandWhatsapp,
  IconChartFunnel,
  IconClock,
  IconMail,
  IconMessageCircle,
  IconNotes,
  IconPhone,
  IconPlus,
  IconSearch,
  IconUser,
  IconUsers,
} from "@tabler/icons-react";
import { useCallback, useEffect, useMemo, useState } from "react";
import { crmClient, productClient, seasonClient } from "@/lib/rpc";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { DetailDrawer } from "@/components/ui/DetailDrawer";
import { EmptyState } from "@/components/ui/EmptyState";
import { PageHero } from "@/components/ui/PageHero";
import { StatCard } from "@/components/ui/StatCard";

const STAGES = [
  CRMLeadStage.CRM_LEAD_STAGE_NEW,
  CRMLeadStage.CRM_LEAD_STAGE_CONTACT,
  CRMLeadStage.CRM_LEAD_STAGE_OFFER,
  CRMLeadStage.CRM_LEAD_STAGE_HOT,
  CRMLeadStage.CRM_LEAD_STAGE_CLOSING,
] as const;

const STAGE_LABEL: Record<number, string> = {
  [CRMLeadStage.CRM_LEAD_STAGE_NEW]: "Baru",
  [CRMLeadStage.CRM_LEAD_STAGE_CONTACT]: "Kontak",
  [CRMLeadStage.CRM_LEAD_STAGE_OFFER]: "Penawaran",
  [CRMLeadStage.CRM_LEAD_STAGE_HOT]: "Hot",
  [CRMLeadStage.CRM_LEAD_STAGE_CLOSING]: "Closing",
  [CRMLeadStage.CRM_LEAD_STAGE_CANCELLED]: "Batal",
};
const SOURCE_LABEL: Record<number, string> = {
  [CRMLeadSource.CRM_LEAD_SOURCE_WEBSITE]: "Website",
  [CRMLeadSource.CRM_LEAD_SOURCE_INSTAGRAM]: "Instagram",
  [CRMLeadSource.CRM_LEAD_SOURCE_WHATSAPP]: "WhatsApp",
  [CRMLeadSource.CRM_LEAD_SOURCE_WALK_IN]: "Walk-in",
  [CRMLeadSource.CRM_LEAD_SOURCE_REFERRAL]: "Referral",
  [CRMLeadSource.CRM_LEAD_SOURCE_ALUMNI]: "Alumni",
  [CRMLeadSource.CRM_LEAD_SOURCE_OTHER]: "Lainnya",
};
const ACTIVITY_LABEL: Record<number, string> = {
  [CRMActivityKind.CRM_ACTIVITY_KIND_CREATED]: "Lead dibuat",
  [CRMActivityKind.CRM_ACTIVITY_KIND_STAGE_CHANGED]: "Tahap dipindahkan",
  [CRMActivityKind.CRM_ACTIVITY_KIND_CONTACT]: "Kontak",
  [CRMActivityKind.CRM_ACTIVITY_KIND_NOTE]: "Catatan",
  [CRMActivityKind.CRM_ACTIVITY_KIND_OFFER_SENT]: "Penawaran dikirim",
  [CRMActivityKind.CRM_ACTIVITY_KIND_PROFILE_UPDATED]: "Profil diperbarui",
};

type LeadForm = {
  fullName: string; phone: string; email: string; source: CRMLeadSource; campaign: string;
  seasonId: string; productId: string; assigneeUserId: string; pax: string;
  estimatedValue: string; nextAction: string; nextFollowUp: string; note: string;
};

const blankForm = (): LeadForm => ({ fullName: "", phone: "", email: "", source: CRMLeadSource.CRM_LEAD_SOURCE_WHATSAPP,
  campaign: "", seasonId: "", productId: "", assigneeUserId: "", pax: "1", estimatedValue: "",
  nextAction: "", nextFollowUp: "", note: "" });
const nextKey = () => crypto.randomUUID();
const formatIDR = (value: bigint) => new Intl.NumberFormat("id-ID", { style: "currency", currency: "IDR", maximumFractionDigits: 0 }).format(value);
const formatDateTime = (date?: Date) => date ? new Intl.DateTimeFormat("id-ID", { dateStyle: "medium", timeStyle: "short" }).format(date) : "Belum dijadwalkan";
const dateInput = (date?: Date) => date ? new Date(date.getTime() - date.getTimezoneOffset() * 60000).toISOString().slice(0, 16) : "";
const errorMessage = (error: unknown, fallback: string) => error instanceof Error && error.message ? error.message : fallback;
const isOverdue = (lead: CRMLead) => Boolean(lead.nextFollowUpAt && lead.nextFollowUpAt.toDate() < new Date() && ![CRMLeadStage.CRM_LEAD_STAGE_CLOSING, CRMLeadStage.CRM_LEAD_STAGE_CANCELLED].includes(lead.stage));

export default function CRMDashboard() {
  const [leads, setLeads] = useState<CRMLead[]>([]);
  const [summary, setSummary] = useState<CRMDashboardData>();
  const [assignees, setAssignees] = useState<CRMAssignee[]>([]);
  const [seasons, setSeasons] = useState<Season[]>([]);
  const [products, setProducts] = useState<Product[]>([]);
  const [search, setSearch] = useState("");
  const [source, setSource] = useState(CRMLeadSource.CRM_LEAD_SOURCE_UNSPECIFIED);
  const [showCancelled, setShowCancelled] = useState(false);
  const [detail, setDetail] = useState<CRMLeadDetail>();
  const [drawer, setDrawer] = useState<"create" | "detail" | "edit" | undefined>();
  const [form, setForm] = useState<LeadForm>(blankForm);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [notice, setNotice] = useState("");

  const refresh = useCallback(async () => {
    setLoading(true);
    try {
      const [board, dashboard] = await Promise.all([
        crmClient.listLeads({ source, search: search.trim(), limit: 500 }),
        crmClient.getDashboard({}),
      ]);
      setLeads(board.leads);
      setSummary(dashboard);
    } catch (error) {
      setNotice(`CRM gagal dimuat. ${errorMessage(error, "Coba lagi.")}`);
    } finally { setLoading(false); }
  }, [search, source]);

  useEffect(() => { const timer = window.setTimeout(() => void refresh(), 220); return () => window.clearTimeout(timer); }, [refresh]);
  useEffect(() => {
    Promise.all([crmClient.listAssignees({}), seasonClient.listSeasons({})]).then(([people, periods]) => {
      setAssignees(people.assignees); setSeasons(periods.seasons);
    }).catch(() => setNotice("Daftar PIC atau musim belum dapat dimuat."));
  }, []);
  useEffect(() => {
    if (!form.seasonId) { setProducts([]); return; }
    productClient.listProducts({ seasonId: form.seasonId }).then((value) => setProducts(value.products.filter((item) => item.isActive && item.category === "TRAVEL_PACKAGE"))).catch(() => setProducts([]));
  }, [form.seasonId]);

  const stageCounts = useMemo(() => new Map(summary?.stages.map((item) => [item.stage, item.leadCount]) ?? []), [summary]);
  const cancelled = leads.filter((lead) => lead.stage === CRMLeadStage.CRM_LEAD_STAGE_CANCELLED);

  function startCreate() { setForm(blankForm()); setDrawer("create"); setDetail(undefined); }
  async function openLead(lead: CRMLead) {
    setDrawer("detail"); setDetail(undefined);
    try { setDetail(await crmClient.getLead({ leadId: lead.id })); }
    catch (error) { setNotice(`Detail lead gagal dimuat. ${errorMessage(error, "Coba lagi.")}`); }
  }
  function startEdit() {
    const lead = detail?.lead; if (!lead) return;
    setForm({ fullName: lead.fullName, phone: lead.phone, email: lead.email, source: lead.source, campaign: lead.campaign,
      seasonId: lead.seasonId, productId: lead.productId, assigneeUserId: lead.assigneeUserId, pax: String(lead.pax),
      estimatedValue: String(lead.estimatedValueIdr), nextAction: lead.nextAction,
      nextFollowUp: dateInput(lead.nextFollowUpAt?.toDate()), note: "" });
    setDrawer("edit");
  }
  async function reloadDetail(id: string) { const value = await crmClient.getLead({ leadId: id }); setDetail(value); await refresh(); }

  async function saveLead() {
    const pax = Number(form.pax), estimatedValueIdr = BigInt(form.estimatedValue.replace(/\D/g, "") || "0");
    if (!form.fullName.trim() || (!form.phone.trim() && !form.email.trim())) { setNotice("Nama dan minimal satu kontak wajib diisi."); return; }
    setSaving(true);
    try {
      const common = { fullName: form.fullName.trim(), phone: form.phone.trim(), email: form.email.trim(), source: form.source,
        campaign: form.campaign.trim(), seasonId: form.seasonId, productId: form.productId,
        assigneeUserId: form.assigneeUserId, pax, estimatedValueIdr, nextAction: form.nextAction.trim(),
        nextFollowUpAt: form.nextFollowUp ? Timestamp.fromDate(new Date(form.nextFollowUp)) : undefined };
      if (drawer === "edit" && detail?.lead) {
        await crmClient.updateLead({ leadId: detail.lead.id, ...common, reason: "Profil lead diperbarui dari dashboard", idempotencyKey: nextKey() });
        await reloadDetail(detail.lead.id); setDrawer("detail"); setNotice("Profil lead berhasil diperbarui.");
      } else {
        const response = await crmClient.createLead({ ...common, note: form.note.trim(), idempotencyKey: nextKey() });
        await refresh(); setDrawer(undefined); setNotice(response.created ? "Lead baru masuk ke pipeline." : "Permintaan ini sudah pernah diproses.");
      }
    } catch (error) { setNotice(`Lead gagal disimpan. ${errorMessage(error, "Periksa data lalu coba lagi.")}`); }
    finally { setSaving(false); }
  }

  return <div className="crm-page">
    <PageHero eyebrow="Pertumbuhan / CRM" title="Pipeline Lead" subtitle={`${summary?.activeCount ?? 0n} prospek aktif dari ${summary?.sourceCount ?? 0n} kanal pemasaran · diperbarui ${summary?.updatedAt ? formatDateTime(summary.updatedAt.toDate()) : "—"}`} actions={<Button variant="emerald" onClick={startCreate}><IconPlus size={16} />Tambah lead</Button>} />
    <div className="crm-content">
      {notice && <div className="crm-notice" role="status"><IconAlertCircle size={18} /><span>{notice}</span><button type="button" onClick={() => setNotice("")}>Tutup</button></div>}
      <section className="crm-stats" aria-label="Ringkasan CRM">
        <StatCard label="Aktif dalam pipeline" value={String(summary?.activeCount ?? 0n)} unit="lead" tone="brand" />
        <StatCard label="Nilai pipeline" value={formatIDR(summary?.pipelineValueIdr ?? 0n)} unit="estimasi" tone="warning" />
        <StatCard label="Konversi bulan ini" value={`${Number(summary?.monthlyConversionBps ?? 0) / 100}%`} unit="closing / lead baru" tone="success" />
        <StatCard label="Follow-up telat" value={String(summary?.overdueFollowUpCount ?? 0n)} unit="butuh tindakan" tone={(summary?.overdueFollowUpCount ?? 0n) > 0n ? "danger" : "neutral"} />
      </section>

      {(summary?.attentionLeads.length ?? 0) > 0 && <section className="tw-card crm-attention" aria-labelledby="crm-attention-title">
        <div><span className="crm-attention__icon"><IconClock size={19} /></span><div><h2 id="crm-attention-title">{summary?.attentionLeads.length} lead butuh perhatian tim hari ini</h2><p>Follow-up melewati jadwal. Dahulukan prospek terlama agar pipeline tidak mendingin.</p></div></div>
        <div className="crm-attention__list">{summary?.attentionLeads.map((lead) => <button type="button" key={lead.id} onClick={() => void openLead(lead)}><span><strong>{lead.fullName}</strong><small>{lead.nextAction || "Belum ada langkah berikutnya"}</small></span><span>{formatDateTime(lead.nextFollowUpAt?.toDate())}<IconArrowRight size={14} /></span></button>)}</div>
      </section>}

      <section className="crm-insights" aria-label="Analitik pipeline">
        <article className="tw-card crm-funnel"><header><div><h2>Corong konversi</h2><p>Jumlah prospek per tahap</p></div><IconChartFunnel size={20} /></header><div>{STAGES.map((stage, index) => { const count = stageCounts.get(stage) ?? 0n; const max = Math.max(1, ...Array.from(stageCounts.values(), Number)); return <div key={stage}><span>{STAGE_LABEL[stage]}</span><i style={{ width: `${Math.max(7, Number(count) / max * 100)}%` }} /><strong>{String(count)}</strong>{index < STAGES.length - 1 && <small>→</small>}</div>; })}</div></article>
        <article className="tw-card crm-sources"><header><div><h2>Sumber lead</h2><p>Klik kanal untuk memfilter papan</p></div></header><div>{summary?.sources.length ? summary.sources.map((item) => <button type="button" key={item.source} data-active={source === item.source || undefined} onClick={() => setSource(source === item.source ? CRMLeadSource.CRM_LEAD_SOURCE_UNSPECIFIED : item.source)}><span>{SOURCE_LABEL[item.source]}</span><strong>{String(item.leadCount)}</strong><small>{formatIDR(item.valueIdr)}</small></button>) : <p className="crm-muted">Belum ada data kanal.</p>}</div></article>
        <article className="tw-card crm-leaderboard"><header><div><h2>Papan peringkat PIC</h2><p>Closing bulan berjalan</p></div></header><div>{summary?.assignees.length ? summary.assignees.slice(0, 5).map((item, index) => <div key={item.userId || "unassigned"}><span>{index + 1}</span><p><strong>{item.name}</strong><small>{String(item.activeCount)} aktif</small></p><b>{String(item.closingCount)} closing</b></div>) : <p className="crm-muted">Belum ada PIC yang ditugaskan.</p>}</div></article>
      </section>

      <section className="crm-board-section" aria-labelledby="crm-board-title">
        <div className="crm-board-heading"><div><h2 id="crm-board-title">Papan pipeline</h2><p>Pindahkan tahap dari detail lead agar setiap perubahan memiliki alasan dan jejak audit.</p></div><div className="crm-filters"><label><IconSearch size={15} /><span className="sr-only">Cari lead</span><input value={search} onChange={(event) => setSearch(event.target.value)} placeholder="Cari nama, telepon, email" /></label><select aria-label="Filter sumber" value={source} onChange={(event) => setSource(Number(event.target.value) as CRMLeadSource)}><option value={CRMLeadSource.CRM_LEAD_SOURCE_UNSPECIFIED}>Semua sumber</option>{Object.entries(SOURCE_LABEL).map(([value, label]) => <option key={value} value={value}>{label}</option>)}</select><button type="button" data-active={showCancelled || undefined} onClick={() => setShowCancelled((value) => !value)}>Batal ({cancelled.length})</button></div></div>
        {loading && leads.length === 0 ? <div className="tw-card crm-loading">Memuat pipeline…</div> : leads.length === 0 ? <EmptyState icon={<IconChartFunnel size={22} />} title="Pipeline masih kosong" cause="Belum ada prospek yang dicatat untuk filter ini." nextStep="Tambahkan lead pertama dari WhatsApp, website, referral, atau walk-in." actionLabel="Tambah lead" onAction={startCreate} /> : <div className="crm-board">{(showCancelled ? [CRMLeadStage.CRM_LEAD_STAGE_CANCELLED] : STAGES).map((stage) => <PipelineColumn key={stage} stage={stage} leads={leads.filter((lead) => lead.stage === stage)} onOpen={openLead} />)}</div>}
      </section>
    </div>

    <DetailDrawer open={drawer === "create" || drawer === "edit"} onClose={() => setDrawer(detail ? "detail" : undefined)} title={drawer === "edit" ? "Edit lead" : "Tambah lead"} subtitle="Data kontak dan nilai peluang penjualan" footer={<><Button variant="ghost" onClick={() => setDrawer(detail ? "detail" : undefined)}>Batal</Button><Button variant="emerald" disabled={saving} onClick={() => void saveLead()}>{saving ? "Menyimpan…" : "Simpan lead"}</Button></>}><LeadFields form={form} setForm={setForm} assignees={assignees} seasons={seasons} products={products} editing={drawer === "edit"} /></DetailDrawer>
    <LeadDetailDrawer detail={detail} open={drawer === "detail"} saving={saving} onClose={() => setDrawer(undefined)} onEdit={startEdit} onSaving={setSaving} onNotice={setNotice} onReload={reloadDetail} />
  </div>;
}

function PipelineColumn({ stage, leads, onOpen }: { stage: CRMLeadStage; leads: CRMLead[]; onOpen: (lead: CRMLead) => void }) {
  const value = leads.reduce((sum, lead) => sum + lead.estimatedValueIdr, 0n);
  return <section className="crm-column" data-stage={stage}><header><div><span>{STAGE_LABEL[stage]}</span><b>{leads.length}</b></div><small>{formatIDR(value)}</small></header><div>{leads.length ? leads.map((lead) => <button type="button" className="crm-lead-card" key={lead.id} onClick={() => onOpen(lead)}><div><strong>{lead.fullName}</strong>{isOverdue(lead) && <Badge tone="danger" dot>Telat</Badge>}</div><p>{lead.productName || lead.campaign || SOURCE_LABEL[lead.source]}</p><dl><div><dt><IconUsers size={13} />Pax</dt><dd>{lead.pax}</dd></div><div><dt>Nilai</dt><dd>{formatIDR(lead.estimatedValueIdr)}</dd></div></dl><footer><span><IconUser size={13} />{lead.assigneeName || "Belum ada PIC"}</span><span><IconClock size={13} />{lead.nextFollowUpAt ? lead.nextFollowUpAt.toDate().toLocaleDateString("id-ID", { day: "numeric", month: "short" }) : "—"}</span></footer></button>) : <p className="crm-column__empty">Belum ada lead</p>}</div></section>;
}

function LeadFields({ form, setForm, assignees, seasons, products, editing }: { form: LeadForm; setForm: React.Dispatch<React.SetStateAction<LeadForm>>; assignees: CRMAssignee[]; seasons: Season[]; products: Product[]; editing: boolean }) {
  const field = (key: keyof LeadForm) => (event: React.ChangeEvent<HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement>) => setForm((value) => ({ ...value, [key]: event.target.value }));
  return <div className="crm-form"><div className="crm-form__split"><label>Nama lengkap<input autoFocus value={form.fullName} onChange={field("fullName")} maxLength={150} /></label><label>Jumlah pax<input type="number" min="1" max="1000" value={form.pax} onChange={field("pax")} /></label></div><div className="crm-form__split"><label>Nomor WhatsApp<input type="tel" value={form.phone} onChange={field("phone")} placeholder="08…" /></label><label>Email<input type="email" value={form.email} onChange={field("email")} /></label></div><label>Sumber lead<select value={form.source} onChange={field("source")}>{Object.entries(SOURCE_LABEL).map(([value, label]) => <option key={value} value={value}>{label}</option>)}</select></label><label>Kampanye<input value={form.campaign} onChange={field("campaign")} placeholder="Contoh: Meta Ads Ramadhan" /></label><div className="crm-form__split"><label>Musim<select value={form.seasonId} onChange={(event) => setForm((value) => ({ ...value, seasonId: event.target.value, productId: "" }))}><option value="">Belum ditentukan</option>{seasons.map((season) => <option key={season.id} value={season.id}>{season.name}</option>)}</select></label><label>Paket diminati<select value={form.productId} onChange={field("productId")} disabled={!form.seasonId}><option value="">Belum ditentukan</option>{products.map((product) => <option key={product.id} value={product.id}>{product.name}</option>)}</select></label></div><label>PIC<select value={form.assigneeUserId} onChange={field("assigneeUserId")}><option value="">Belum ditugaskan</option>{assignees.map((person) => <option key={person.userId} value={person.userId}>{person.name} · {person.email}</option>)}</select></label><label>Nilai estimasi (Rp)<input inputMode="numeric" value={form.estimatedValue} onChange={field("estimatedValue")} placeholder="35000000" /></label><label>Langkah berikutnya<input value={form.nextAction} onChange={field("nextAction")} placeholder="Telepon, kirim proposal, jadwalkan pertemuan…" /></label><label>Jadwal follow-up<input type="datetime-local" value={form.nextFollowUp} onChange={field("nextFollowUp")} /></label>{!editing && <label>Catatan awal<textarea rows={4} value={form.note} onChange={field("note")} placeholder="Kebutuhan, preferensi, atau konteks percakapan" /></label>}<p className="crm-form__hint">Minimal isi nomor WhatsApp atau email. Setiap perubahan setelah disimpan akan masuk ke timeline audit.</p></div>;
}

function LeadDetailDrawer({ detail, open, saving, onClose, onEdit, onSaving, onNotice, onReload }: { detail?: CRMLeadDetail; open: boolean; saving: boolean; onClose: () => void; onEdit: () => void; onSaving: (value: boolean) => void; onNotice: (value: string) => void; onReload: (id: string) => Promise<void> }) {
  const [stage, setStage] = useState(CRMLeadStage.CRM_LEAD_STAGE_UNSPECIFIED), [reason, setReason] = useState("");
  const [kind, setKind] = useState(CRMActivityKind.CRM_ACTIVITY_KIND_CONTACT), [note, setNote] = useState(""), [nextAction, setNextAction] = useState(""), [followUp, setFollowUp] = useState("");
  const lead = detail?.lead;
  useEffect(() => { if (lead) { setStage(lead.stage); setNextAction(lead.nextAction); setFollowUp(dateInput(lead.nextFollowUpAt?.toDate())); } }, [lead]);
  async function move() { if (!lead || stage === lead.stage || !reason.trim()) { onNotice("Pilih tahap baru dan tulis alasan perpindahan."); return; } onSaving(true); try { await crmClient.moveLeadStage({ leadId: lead.id, stage, reason: reason.trim(), idempotencyKey: nextKey() }); setReason(""); await onReload(lead.id); onNotice("Tahap lead berhasil dipindahkan."); } catch (error) { onNotice(`Tahap gagal dipindahkan. ${errorMessage(error, "Periksa alur tahap.")}`); } finally { onSaving(false); } }
  async function addActivity() { if (!lead || !note.trim()) { onNotice("Isi catatan aktivitas terlebih dahulu."); return; } onSaving(true); try { await crmClient.addLeadActivity({ leadId: lead.id, kind, note: note.trim(), nextAction: nextAction.trim(), nextFollowUpAt: followUp ? Timestamp.fromDate(new Date(followUp)) : undefined, idempotencyKey: nextKey() }); setNote(""); await onReload(lead.id); onNotice("Aktivitas tersimpan di timeline."); } catch (error) { onNotice(`Aktivitas gagal disimpan. ${errorMessage(error, "Coba lagi.")}`); } finally { onSaving(false); } }
  return <DetailDrawer open={open} onClose={onClose} title={lead?.fullName || "Detail lead"} subtitle={lead ? <span>{SOURCE_LABEL[lead.source]} · {formatIDR(lead.estimatedValueIdr)}</span> : "Memuat…"} footer={<><Button variant="outline" onClick={onEdit} disabled={!lead}>Edit profil</Button><Button variant="ghost" onClick={onClose}>Tutup</Button></>}>{lead && <div className="crm-detail"><section className="crm-detail__summary"><Badge tone={lead.stage === CRMLeadStage.CRM_LEAD_STAGE_CLOSING ? "success" : lead.stage === CRMLeadStage.CRM_LEAD_STAGE_CANCELLED ? "danger" : "brand"} dot>{STAGE_LABEL[lead.stage]}</Badge><dl><div><dt>Kontak</dt><dd>{lead.phone || lead.email}</dd></div><div><dt>PIC</dt><dd>{lead.assigneeName || "Belum ditugaskan"}</dd></div><div><dt>Paket</dt><dd>{lead.productName || lead.seasonName || "Belum ditentukan"}</dd></div><div><dt>Pax</dt><dd>{lead.pax}</dd></div><div><dt>Langkah berikutnya</dt><dd>{lead.nextAction || "Belum ditentukan"}</dd></div><div><dt>Follow-up</dt><dd data-overdue={isOverdue(lead) || undefined}>{formatDateTime(lead.nextFollowUpAt?.toDate())}</dd></div></dl><div className="crm-contact-links">{lead.phone && <a href={`tel:${lead.phone}`}><IconPhone size={15} />Telepon</a>}{lead.phone && <a href={`https://wa.me/${lead.phone.replace(/\D/g, "").replace(/^0/, "62")}`} target="_blank" rel="noreferrer"><IconBrandWhatsapp size={15} />WhatsApp</a>}{lead.email && <a href={`mailto:${lead.email}`}><IconMail size={15} />Email</a>}</div></section><section className="crm-action-form"><h3>Pindahkan tahap</h3><select value={stage} onChange={(event) => setStage(Number(event.target.value) as CRMLeadStage)}>{Object.entries(STAGE_LABEL).map(([value, label]) => <option key={value} value={value}>{label}</option>)}</select><input value={reason} onChange={(event) => setReason(event.target.value)} placeholder="Alasan wajib untuk jejak audit" /><Button variant="outline" disabled={saving || stage === lead.stage} onClick={() => void move()}>Pindahkan tahap</Button></section><section className="crm-action-form"><h3>Catat aktivitas</h3><select value={kind} onChange={(event) => setKind(Number(event.target.value) as CRMActivityKind)}><option value={CRMActivityKind.CRM_ACTIVITY_KIND_CONTACT}>Kontak / follow-up</option><option value={CRMActivityKind.CRM_ACTIVITY_KIND_OFFER_SENT}>Penawaran dikirim</option><option value={CRMActivityKind.CRM_ACTIVITY_KIND_NOTE}>Catatan internal</option></select><textarea rows={3} value={note} onChange={(event) => setNote(event.target.value)} placeholder="Apa yang terjadi?" /><input value={nextAction} onChange={(event) => setNextAction(event.target.value)} placeholder="Langkah berikutnya" /><input type="datetime-local" value={followUp} onChange={(event) => setFollowUp(event.target.value)} /><Button variant="emerald" disabled={saving} onClick={() => void addActivity()}><IconMessageCircle size={15} />{saving ? "Menyimpan…" : "Simpan aktivitas"}</Button></section><section className="crm-timeline"><h3>Timeline</h3>{detail.activities.length ? detail.activities.map((activity) => <article key={activity.id}><span><IconNotes size={14} /></span><div><header><strong>{ACTIVITY_LABEL[activity.kind]}</strong><time>{formatDateTime(activity.occurredAt?.toDate())}</time></header>{activity.fromStage && activity.toStage && <p>{STAGE_LABEL[activity.fromStage]} → {STAGE_LABEL[activity.toStage]}</p>}<p>{activity.note || "Tanpa catatan"}</p><small>{activity.actorName || "Staf operator"}</small></div></article>) : <p className="crm-muted">Memuat timeline…</p>}</section></div>}</DetailDrawer>;
}
