"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { IconAlertTriangle, IconCheck, IconMail, IconPlus, IconSpeakerphone, IconX } from "@tabler/icons-react";
import { Timestamp } from "@bufbuild/protobuf";
import type { Announcement } from "@hajj-saas/proto-gen/hajj/v1/announcement_pb";
import type { PlatformOperator } from "@hajj-saas/proto-gen/hajj/v1/platform_pb";
import { EmptyState } from "@/components/ui/EmptyState";
import { PageHeader } from "@/components/ui/PageHeader";
import { Wizard, type WizardReadinessCheck, type WizardStep, type WizardValidationIssue } from "@/components/ui/Wizard";
import { platformClient } from "@/lib/rpc";

const MODE_LABEL: Record<string, string> = {
  ALL: "Semua travel", PLAN: "Per paket", TRIALING: "Sedang trial",
  MULTI_BRANCH: "Multi-cabang", OVERDUE: "Menunggak", MANUAL: "Pilih manual",
};
const MODES = ["ALL", "PLAN", "TRIALING", "MULTI_BRANCH", "OVERDUE", "MANUAL"] as const;
const PLANS = ["STARTER", "GROWTH", "PRO"];

const dateTime = (at?: { toDate(): Date }) =>
  at ? at.toDate().toLocaleString("id-ID", { day: "2-digit", month: "short", year: "numeric", hour: "2-digit", minute: "2-digit" }) : "—";

const STEPS: WizardStep[] = [
  { id: "penerima", title: "Penerima" },
  { id: "kanal", title: "Kanal" },
  { id: "isi", title: "Isi" },
  { id: "jadwal", title: "Jadwal" },
];

export default function AnnouncementsTab() {
  const [history, setHistory] = useState<Announcement[]>([]);
  const [loading, setLoading] = useState(true);
  const [failure, setFailure] = useState("");
  const [composing, setComposing] = useState(false);

  const load = useCallback(() => {
    setLoading(true);
    platformClient
      .listPlatformAnnouncements({ limit: 50 })
      .then((r) => setHistory(r.announcements))
      .catch(() => setFailure("Gagal memuat riwayat pengumuman."))
      .finally(() => setLoading(false));
  }, []);

  useEffect(load, [load]);

  const last30Days = Date.now() - 30 * 24 * 60 * 60 * 1000;
  const sentLast30Days = history.filter((a) => a.sentAt && a.sentAt.toDate().getTime() >= last30Days).length;
  const scheduled = history.filter((a) => !a.sentAt).length;

  return (
    <section className="tw-screen">
      <PageHeader
        eyebrow="TAWAFIQHUB / PENGUMUMAN"
        title="Pengumuman"
        subtitle={`${sentLast30Days} terkirim 30 hari terakhir · ${scheduled} terjadwal`}
        primaryAction={
          !composing && (
            <button type="button" className="tw-btn tw-btn--emerald" onClick={() => setComposing(true)}>
              <IconPlus size={16} />Buat pengumuman
            </button>
          )
        }
      />

      {failure && <p className="tw-inline-alert" data-tone="danger"><IconAlertTriangle size={16} />{failure}</p>}

      {composing && (
        <ComposeAnnouncement
          onCancel={() => setComposing(false)}
          onSent={() => { setComposing(false); load(); }}
        />
      )}

      {!composing && (
        <>
          {loading && <p className="tw-note">Memuat…</p>}
          {!loading && history.length === 0 && (
            <EmptyState
              icon={<IconSpeakerphone size={26} />}
              title="Belum ada pengumuman"
              cause="Belum pernah ada yang dikirim ke tenant dari panel ini."
              nextStep="Buat pengumuman pertama lewat tombol di atas."
            />
          )}
          {!loading && history.length > 0 && (
            <div className="tw-table-wrap">
              <table className="tw-table">
                <thead>
                  <tr>{["Judul", "Penerima", "Terkirim", "Dibaca", "Kapan", "Oleh"].map((h) => <th key={h}>{h}</th>)}</tr>
                </thead>
                <tbody>
                  {history.map((a) => (
                    <tr key={a.id}>
                      <td style={{ fontWeight: 600, maxWidth: 320 }}>{a.title}</td>
                      <td>{a.recipientCount}</td>
                      <td>
                        {a.sentAt
                          ? <span style={{ color: "var(--color-emerald-800)", fontWeight: 700 }}>terkirim</span>
                          : <span style={{ color: "var(--color-warning-700)", fontWeight: 700 }}>terjadwal {dateTime(a.scheduledAt)}</span>}
                      </td>
                      <td>{a.sentAt ? `${a.readCount}/${a.recipientCount}` : "—"}</td>
                      <td>{dateTime(a.sentAt ?? a.createdAt)}</td>
                      <td style={{ color: "var(--color-warm-400)" }}>{a.adminEmail}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </>
      )}
    </section>
  );
}

function ComposeAnnouncement({ onCancel, onSent }: { onCancel: () => void; onSent: () => void }) {
  const [stepId, setStepId] = useState<string>("penerima");
  const [mode, setMode] = useState<(typeof MODES)[number]>("ALL");
  const [plan, setPlan] = useState("GROWTH");
  const [operators, setOperators] = useState<PlatformOperator[]>([]);
  const [operatorSearch, setOperatorSearch] = useState("");
  const [manualIds, setManualIds] = useState<string[]>([]);
  const [wantsEmail, setWantsEmail] = useState(false);
  const [title, setTitle] = useState("");
  const [body, setBody] = useState("");
  const [link, setLink] = useState("");
  const [scheduleMode, setScheduleMode] = useState<"now" | "later">("now");
  const [scheduledAtLocal, setScheduledAtLocal] = useState("");
  const [preview, setPreview] = useState<{ count: number; overlap: number }>();
  const [previewLoading, setPreviewLoading] = useState(false);
  const [sending, setSending] = useState(false);
  const [failure, setFailure] = useState("");
  const [idempotencyKey] = useState(() => `ann-${crypto.randomUUID()}`);

  useEffect(() => {
    if (mode !== "MANUAL" || operators.length > 0) return;
    platformClient.listOperators({}).then((r) => setOperators(r.operators)).catch(() => {});
  }, [mode, operators.length]);

  const filter = useMemo(() => ({
    mode, plan: mode === "PLAN" ? plan : "", operatorIds: mode === "MANUAL" ? manualIds : [],
  }), [mode, plan, manualIds]);

  useEffect(() => {
    if (mode === "MANUAL" && manualIds.length === 0) { setPreview({ count: 0, overlap: 0 }); return; }
    setPreviewLoading(true);
    const timer = setTimeout(() => {
      platformClient.previewAnnouncementRecipients({ filter })
        .then((r) => setPreview({ count: r.count, overlap: r.overlappingRecentCount }))
        .catch(() => setPreview(undefined))
        .finally(() => setPreviewLoading(false));
    }, 300);
    return () => clearTimeout(timer);
  }, [filter, mode, manualIds.length]);

  const readinessChecks: WizardReadinessCheck[] = [
    { id: "title", label: "Judul terisi", passed: title.trim().length > 0 },
    { id: "body", label: "Isi memadai (minimal 10 huruf)", passed: body.trim().length >= 10 },
    { id: "recipients", label: "Penerima lebih dari nol", passed: (preview?.count ?? 0) > 0 },
    { id: "overlap", label: "Tidak ada pengumuman lain ke penerima sama dalam 24 jam", passed: (preview?.overlap ?? 0) === 0 },
  ];
  const readinessScore = Math.round((readinessChecks.filter((c) => c.passed).length / readinessChecks.length) * 100);

  const validationIssues: WizardValidationIssue[] = [];
  if (title.trim().length === 0) validationIssues.push({ id: "title", message: "Judul belum diisi", stepId: "isi" });
  if (body.trim().length < 10) validationIssues.push({ id: "body", message: "Isi pesan terlalu pendek", stepId: "isi" });
  if ((preview?.count ?? 0) === 0) validationIssues.push({ id: "recipients", message: "Belum ada penerima yang cocok", stepId: "penerima" });
  if (mode === "PLAN" && !plan) validationIssues.push({ id: "plan", message: "Paket belum dipilih", stepId: "penerima" });
  if (mode === "MANUAL" && manualIds.length === 0) validationIssues.push({ id: "manual", message: "Belum ada travel yang dipilih", stepId: "penerima" });

  const canSend = validationIssues.length === 0 && !sending;

  const submit = useCallback(async () => {
    setSending(true);
    setFailure("");
    try {
      const scheduledAt = scheduleMode === "later" && scheduledAtLocal
        ? Timestamp.fromDate(new Date(scheduledAtLocal))
        : undefined;
      await platformClient.sendAnnouncement({
        title: title.trim(), body: body.trim(), link: link.trim(),
        channels: wantsEmail ? ["IN_APP", "EMAIL"] : ["IN_APP"],
        filter, scheduledAt, idempotencyKey,
      });
      onSent();
    } catch (caught) {
      setFailure(caught instanceof Error ? caught.message : "Gagal mengirim pengumuman.");
    } finally {
      setSending(false);
    }
  }, [title, body, link, wantsEmail, filter, scheduleMode, scheduledAtLocal, idempotencyKey, onSent]);

  const filteredOperators = operatorSearch.trim()
    ? operators.filter((o) => o.name.toLowerCase().includes(operatorSearch.trim().toLowerCase()))
    : operators;

  return (
    <Wizard
      title="Pengumuman baru"
      steps={STEPS}
      currentStepId={stepId}
      onStepChange={setStepId}
      readinessScore={readinessScore}
      readinessChecks={readinessChecks}
      validationIssues={validationIssues}
      footer={
        <div style={{ display: "flex", gap: 8, justifyContent: "flex-end", alignItems: "center", width: "100%" }}>
          {failure && <span style={{ color: "var(--color-danger-600)", fontSize: 13, marginRight: "auto" }}>{failure}</span>}
          <button type="button" className="tw-btn tw-btn--ghost" onClick={onCancel}><IconX size={15} />Batal</button>
          <button type="button" className="tw-btn tw-btn--emerald" disabled={!canSend} onClick={() => void submit()}>
            <IconCheck size={15} />{sending ? "Mengirim…" : scheduleMode === "later" ? "Jadwalkan" : "Kirim sekarang"}
          </button>
        </div>
      }
    >
      {stepId === "penerima" && (
        <div style={{ display: "grid", gap: 16 }}>
          <div className="tw-segmented" role="group" aria-label="Mode penerima">
            {MODES.map((m) => (
              <button key={m} type="button" onClick={() => setMode(m)} aria-pressed={mode === m}
                className={mode === m ? "tw-segmented__item is-active" : "tw-segmented__item"}>{MODE_LABEL[m]}</button>
            ))}
          </div>

          {mode === "PLAN" && (
            <label style={{ display: "grid", gap: 6, maxWidth: 240 }}>
              <span style={fieldLabel}>Paket</span>
              <select value={plan} onChange={(e) => setPlan(e.target.value)} style={select}>
                {PLANS.map((p) => <option key={p} value={p}>{p}</option>)}
              </select>
            </label>
          )}

          {mode === "MANUAL" && (
            <div style={{ display: "grid", gap: 8 }}>
              <input
                value={operatorSearch} onChange={(e) => setOperatorSearch(e.target.value)}
                placeholder="Cari nama travel…" style={textInput}
              />
              <div style={operatorList}>
                {filteredOperators.length === 0 && <p className="tw-note" style={{ padding: 12 }}>Tidak ada travel yang cocok.</p>}
                {filteredOperators.map((o) => {
                  const checked = manualIds.includes(o.id);
                  return (
                    <label key={o.id} style={operatorRow}>
                      <input
                        type="checkbox" checked={checked}
                        onChange={(e) => setManualIds((prev) => e.target.checked ? [...prev, o.id] : prev.filter((id) => id !== o.id))}
                      />
                      <span>{o.name}</span>
                      <span style={{ color: "var(--color-warm-400)", fontSize: 12 }}>{o.plan || "—"}</span>
                    </label>
                  );
                })}
              </div>
              <p className="tw-note">{manualIds.length} travel dipilih.</p>
            </div>
          )}

          <div style={previewBox}>
            {previewLoading
              ? <p className="tw-note" style={{ margin: 0 }}>Menghitung penerima…</p>
              : (
                <>
                  <p style={{ margin: 0, fontWeight: 700, fontSize: 15 }}>{preview?.count ?? 0} travel akan menerima pengumuman ini</p>
                  {(preview?.overlap ?? 0) > 0 && (
                    <p style={{ margin: "6px 0 0", color: "var(--color-warning-700)", fontSize: 13, display: "flex", alignItems: "center", gap: 6 }}>
                      <IconAlertTriangle size={14} />
                      {preview?.overlap} dari mereka sudah menerima pengumuman lain dalam 24 jam terakhir.
                    </p>
                  )}
                </>
              )}
          </div>
        </div>
      )}

      {stepId === "kanal" && (
        <div style={{ display: "grid", gap: 12 }}>
          <label style={channelRow}>
            <input type="checkbox" checked disabled />
            <span><strong>Dalam aplikasi</strong> — selalu aktif, muncul di lonceng notifikasi dashboard travel.</span>
          </label>
          <label style={channelRow}>
            <input type="checkbox" checked={wantsEmail} onChange={(e) => setWantsEmail(e.target.checked)} />
            <span><IconMail size={14} style={{ verticalAlign: "-2px", marginRight: 4 }} /><strong>Email</strong> — dikirim ke travel yang punya alamat email terdaftar. Yang tidak punya tetap menerima salinan dalam aplikasi.</span>
          </label>
        </div>
      )}

      {stepId === "isi" && (
        <div style={{ display: "grid", gap: 14 }}>
          <label style={{ display: "grid", gap: 6 }}>
            <span style={fieldLabel}>Judul</span>
            <input value={title} onChange={(e) => setTitle(e.target.value)} style={textInput} placeholder="mis. Pemeliharaan terjadwal Sabtu malam" />
          </label>
          <label style={{ display: "grid", gap: 6 }}>
            <span style={fieldLabel}>Isi</span>
            <textarea value={body} onChange={(e) => setBody(e.target.value)} rows={5} style={textarea}
              placeholder="Tuliskan pesan lengkapnya di sini…" />
          </label>
          <label style={{ display: "grid", gap: 6 }}>
            <span style={fieldLabel}>Tautan (opsional)</span>
            <input value={link} onChange={(e) => setLink(e.target.value)} style={textInput} placeholder="https://…" />
          </label>
          {(title.trim() || body.trim()) && (
            <div style={previewBox}>
              <p style={{ margin: 0, fontSize: 11, fontWeight: 700, color: "var(--color-warm-500)", letterSpacing: ".04em" }}>PRATINJAU</p>
              <p style={{ margin: "8px 0 0", fontWeight: 700 }}>{title || "(judul kosong)"}</p>
              <p style={{ margin: "6px 0 0", fontSize: 13, whiteSpace: "pre-wrap" }}>{body || "(isi kosong)"}</p>
              {link.trim() && <p style={{ margin: "6px 0 0", fontSize: 13, color: "var(--color-emerald-800)" }}>{link}</p>}
            </div>
          )}
        </div>
      )}

      {stepId === "jadwal" && (
        <div style={{ display: "grid", gap: 14 }}>
          <div className="tw-segmented" role="group" aria-label="Jadwal">
            <button type="button" onClick={() => setScheduleMode("now")} aria-pressed={scheduleMode === "now"}
              className={scheduleMode === "now" ? "tw-segmented__item is-active" : "tw-segmented__item"}>Sekarang</button>
            <button type="button" onClick={() => setScheduleMode("later")} aria-pressed={scheduleMode === "later"}
              className={scheduleMode === "later" ? "tw-segmented__item is-active" : "tw-segmented__item"}>Terjadwal</button>
          </div>
          {scheduleMode === "later" && (
            <label style={{ display: "grid", gap: 6, maxWidth: 260 }}>
              <span style={fieldLabel}>Kirim pada</span>
              <input type="datetime-local" value={scheduledAtLocal} onChange={(e) => setScheduledAtLocal(e.target.value)} style={textInput} />
            </label>
          )}
          <p className="tw-note">
            Setelah terkirim, judul dan isi tidak bisa diedit lagi. Kalau ada yang salah, kirim pengumuman ralat sebagai pengumuman baru.
          </p>
        </div>
      )}
    </Wizard>
  );
}

const fieldLabel: React.CSSProperties = { fontSize: 11, fontWeight: 700, color: "var(--color-warm-700)", letterSpacing: ".03em" };
const textInput: React.CSSProperties = { minHeight: 40, padding: "0 10px", borderRadius: 8, border: "1px solid var(--color-cream-400)", background: "#fff", font: "inherit", fontSize: 13 };
const textarea: React.CSSProperties = { padding: "10px 12px", borderRadius: 8, border: "1px solid var(--color-cream-400)", background: "#fff", font: "inherit", fontSize: 13, resize: "vertical" };
const select: React.CSSProperties = { minHeight: 40, padding: "0 10px", borderRadius: 8, border: "1px solid var(--color-cream-400)", background: "#fff", font: "inherit", fontSize: 13 };
const previewBox: React.CSSProperties = { padding: "12px 14px", borderRadius: 10, background: "var(--color-cream-100)", border: "1px solid var(--color-cream-300)" };
const operatorList: React.CSSProperties = { maxHeight: 220, overflowY: "auto", border: "1px solid var(--color-cream-300)", borderRadius: 8 };
const operatorRow: React.CSSProperties = { display: "flex", alignItems: "center", gap: 8, padding: "8px 12px", borderBottom: "1px solid var(--color-cream-200)", fontSize: 13, cursor: "pointer" };
const channelRow: React.CSSProperties = { display: "flex", alignItems: "flex-start", gap: 10, padding: "12px 14px", borderRadius: 8, background: "var(--color-cream-100)", fontSize: 13, cursor: "pointer" };
