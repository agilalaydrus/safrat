"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import {
  IconAlertTriangle, IconArrowLeft, IconBuildingStore, IconExternalLink,
  IconEyeglass, IconLock, IconLockOpen, IconShieldCheck, IconShieldOff, IconWorld,
} from "@tabler/icons-react";
import type { GetTenantDetailResponse, ImpersonationRow, PersonalDataReadRow, PrivilegedActionRow } from "@hajj-saas/proto-gen/hajj/v1/platform_pb";
import { startImpersonationLocally } from "@/lib/impersonation";
import { Badge } from "@/components/ui/Badge";
import { EmptyState } from "@/components/ui/EmptyState";
import { PageHeader } from "@/components/ui/PageHeader";
import { StatCard } from "@/components/ui/StatCard";
import { buildTenantLink } from "@/lib/tenant-link";
import { platformClient } from "@/lib/rpc";

const rupiah = (n: bigint | number) => `Rp${Number(n).toLocaleString("id-ID")}`;
const count = (n: number) => new Intl.NumberFormat("id-ID").format(n);
const date = (at?: { toDate(): Date }) =>
  at ? at.toDate().toLocaleDateString("id-ID", { day: "2-digit", month: "short", year: "numeric" }) : "—";
const dateTime = (at?: { toDate(): Date }) =>
  at ? at.toDate().toLocaleString("id-ID", { day: "2-digit", month: "short", hour: "2-digit", minute: "2-digit" }) : "—";

const METRIC_LABEL: Record<string, string> = { pilgrims: "Jamaah", branches: "Cabang", storage_bytes: "Penyimpanan" };
const formatUsage = (metric: string, value: bigint) => {
  if (metric !== "storage_bytes") return count(Number(value));
  const mb = Number(value) / 1_048_576;
  return mb >= 1024 ? `${(mb / 1024).toFixed(1)} GB` : `${mb.toFixed(1)} MB`;
};

export default function TenantDetail({ operatorId }: { operatorId: string }) {
  const [detail, setDetail] = useState<GetTenantDetailResponse>();
  const [impersonations, setImpersonations] = useState<ImpersonationRow[]>([]);
  const [privileged, setPrivileged] = useState<PrivilegedActionRow[]>([]);
  const [reads, setReads] = useState<PersonalDataReadRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [failure, setFailure] = useState("");

  useEffect(() => {
    setLoading(true);
    platformClient
      .getTenantDetail({ operatorId })
      .then(setDetail)
      .catch((error: unknown) => setFailure(error instanceof Error ? error.message : String(error)))
      .finally(() => setLoading(false));
    // Its own call rather than part of the detail: the history of who looked at
    // this account is worth showing even when it is empty, and a failure to
    // read it must not blank the whole page.
    platformClient
      .listImpersonations({ operatorId, limit: 10 })
      .then((response) => setImpersonations(response.sessions))
      .catch(() => setImpersonations([]));
    platformClient
      .listPrivilegedActions({ operatorId, limit: 20 })
      .then((response) => setPrivileged(response.actions))
      .catch(() => setPrivileged([]));
    platformClient
      .listPersonalDataReads({ operatorId, limit: 50 })
      .then((response) => setReads(response.reads))
      .catch(() => setReads([]));
  }, [operatorId]);

  if (loading) return <main style={page}><p className="tw-note">Memuat data travel…</p></main>;

  if (failure || !detail?.operator) {
    return (
      <main style={page}>
        <Link href="/admin" style={back}><IconArrowLeft size={15} />Kembali ke panel</Link>
        <EmptyState
          icon={<IconAlertTriangle size={26} />}
          title="Travel tidak dapat dibuka"
          cause="Alamat halaman ini memakai id travel, bukan namanya — tautannya mungkin salah, atau travelnya sudah dihapus."
          nextStep={failure || "Kembali ke daftar Travel dan buka lewat namanya."}
          actionHref="/admin"
          actionLabel="Kembali ke panel"
        />
      </main>
    );
  }

  const operator = detail.operator;
  const storefront = buildTenantLink(operator.slug, "/");
  // Suspension and lapsed access are different things and must not be shown as
  // one: the first was a decision somebody made, the second is a bill nobody
  // paid, and the way out of each is different.
  const suspended = Boolean(operator.suspendedAt);
  const effectiveUntil = operator.effectiveAccessUntil?.toDate();
  const lapsed = Boolean(effectiveUntil && effectiveUntil.getTime() < Date.now());
  const twoFactorMissing = detail.team.filter((member) => !member.twoFactorEnabled);

  return (
    <main style={page}>
      <Link href="/admin" style={back}><IconArrowLeft size={15} />Kembali ke panel</Link>

      <PageHeader
        eyebrow="TAWAFIQHUB / TRAVEL"
        title={operator.name}
        subtitle={`${operator.slug || "tanpa slug"} · bergabung ${date(operator.createdAt)} · ${operator.plan || "tanpa paket"}`}
        primaryAction={
          <div style={{ display: "flex", gap: 10, flexWrap: "wrap" }}>
            <StartImpersonation operatorId={operator.id} operatorName={operator.name} />
            <SuspensionControl operatorId={operator.id} operatorName={operator.name} suspended={suspended} />
            {storefront && (
              <a href={storefront} target="_blank" rel="noreferrer" className="tw-btn tw-btn--ghost tw-btn--md">
                <IconBuildingStore size={16} />Buka storefront<IconExternalLink size={13} />
              </a>
            )}
          </div>
        }
      />

      {(suspended || lapsed || operator.dunningStage) && (
        <div style={alertRow}>
          {suspended && (
            <Badge tone="danger" dot>
              Ditangguhkan {date(operator.suspendedAt)} — dihentikan dengan sengaja, bukan karena tagihan
            </Badge>
          )}
          {lapsed && !suspended && (
            <Badge tone="warning" dot>
              Akses habis {date(operator.effectiveAccessUntil)} — termasuk masa tenggang
            </Badge>
          )}
          {operator.dunningStage && <Badge tone="neutral">Tagihan tahap {operator.dunningStage}</Badge>}
        </div>
      )}

      <div className="tw-stat-grid tw-stagger">
        <StatCard label="Status langganan" value={operator.subscriptionStatus || "—"} unit={`akses sampai ${date(operator.accessUntil)}`}
          tone={suspended ? "danger" : lapsed ? "warning" : "success"} />
        <StatCard label="Tagihan berjalan" value={rupiah(operator.outstandingIdr)} unit={operator.outstandingIdr > 0n ? "belum dibayar" : "tidak ada tagihan terbuka"}
          tone={operator.outstandingIdr > 0n ? "warning" : "neutral"} />
        <StatCard label="Saldo kredit" value={rupiah(operator.creditBalanceIdr)} unit="dipakai otomatis pada tagihan berikutnya" tone="info" />
        <StatCard label="Transaksi tertahan" value={count(operator.heldOrderCount)} unit="uang masuk menunggu keputusan"
          tone={operator.heldOrderCount > 0 ? "danger" : "neutral"} />
      </div>

      <div className="tw-grid-2">
        <section className="tw-card tw-panel tw-enter">
          <h2 className="tw-panel__title">Pemakaian terhadap kuota</h2>
          {detail.usage.length === 0 ? (
            <p className="tw-panel__lede" style={{ marginBottom: 0 }}>
              Belum ada snapshot pemakaian. Diambil worker harian, jadi travel yang baru bergabung belum punya baris.
            </p>
          ) : (
            <div style={{ display: "grid", gap: 14 }}>
              {detail.usage.map((row) => {
                const limit = row.limit ?? undefined;
                const ratio = limit === undefined ? undefined : Number(limit) === 0 ? (Number(row.value) > 0 ? 1 : 0) : Number(row.value) / Number(limit);
                const tone = ratio === undefined ? "var(--color-emerald-700)" : ratio >= 1 ? "var(--color-danger-600)" : ratio >= 0.8 ? "var(--color-warning-600)" : "var(--color-emerald-700)";
                return (
                  <div key={row.metric}>
                    <div style={rowHead}>
                      <span>{METRIC_LABEL[row.metric] ?? row.metric}</span>
                      <span style={{ fontWeight: 700 }}>
                        {formatUsage(row.metric, row.value)}
                        <span style={{ color: "var(--color-warm-400)", fontWeight: 500 }}>
                          {" / "}{limit === undefined ? "tanpa batas" : formatUsage(row.metric, limit)}
                        </span>
                      </span>
                    </div>
                    <div style={track}>
                      <div style={{ width: `${Math.min((ratio ?? 0) * 100, 100)}%`, height: "100%", background: tone, borderRadius: 5 }} />
                    </div>
                  </div>
                );
              })}
            </div>
          )}
          {detail.hasOverride && detail.override && (
            <div style={overrideBox}>
              <strong>Override berlaku</strong>
              <p style={{ ...muted, fontSize: 12, margin: "6px 0 0" }}>
                {detail.override.note || "tanpa catatan"} · sampai{" "}
                {detail.override.expiresAt ? date(detail.override.expiresAt) : "dicabut manual"} · diubah oleh{" "}
                {detail.override.updatedBy || "—"}
              </p>
              <p style={{ ...muted, fontSize: 12, margin: "4px 0 0" }}>
                Angka batas di atas sudah memakai override ini, bukan batas paketnya.
              </p>
            </div>
          )}
        </section>

        <section className="tw-card tw-panel tw-enter">
          <h2 className="tw-panel__title">Isi akun</h2>
          <dl style={countGrid}>
            {[
              ["Jamaah", detail.counts?.pilgrims ?? 0],
              ["Cabang aktif", detail.counts?.branches ?? 0],
              ["Musim", detail.counts?.seasons ?? 0],
              ["Produk", detail.counts?.products ?? 0],
              ["Pendaftaran", detail.counts?.registrations ?? 0],
              ["Staf", detail.counts?.staff ?? 0],
              ["KYC menunggu", detail.counts?.kycPending ?? 0],
              ["KYC terverifikasi", detail.counts?.kycVerified ?? 0],
            ].map(([label, value]) => (
              <div key={String(label)} style={countItem}>
                <dt style={{ margin: "0 0 6px", fontSize: 12, color: "var(--color-warm-500)", fontWeight: 600 }}>{label}</dt>
                <dd style={countValue}>{count(Number(value))}</dd>
              </div>
            ))}
          </dl>
          {detail.funnel && (
            <p style={{ ...muted, fontSize: 12, marginTop: 14 }}>
              Storefront 30 hari terakhir: {count(detail.funnel.visitors)} pengunjung,{" "}
              {count(detail.funnel.registrations)} pendaftar
              {detail.funnel.visitors > 0 ? ` (${(detail.funnel.conversion * 100).toFixed(1)}%)` : ""}.
              {detail.funnel.visitors === 0 && " Belum ada yang membuka storefront ini — biasanya tautannya belum disebar."}
            </p>
          )}
        </section>
      </div>

      <section className="tw-card tw-panel tw-enter">
        <h2 className="tw-panel__title">Tim</h2>
        {twoFactorMissing.length > 0 && (
          <p style={warnLine}>
            <IconShieldOff size={15} />
            {twoFactorMissing.length} dari {detail.team.length} akun belum memakai verifikasi dua langkah.
          </p>
        )}
        {detail.team.length === 0 ? (
          <p className="tw-panel__lede" style={{ marginBottom: 0 }}>Tidak ada anggota tim. Travel tanpa akun tidak bisa masuk sama sekali.</p>
        ) : (
          <div className="tw-table-wrap">
            <table className="tw-table">
              <thead><tr>{["Nama", "Email", "Peran", "2FA", "Bergabung"].map((head) => <th key={head}>{head}</th>)}</tr></thead>
              <tbody>
                {detail.team.map((member) => (
                  <tr key={member.userId}>
                    <td style={{ fontWeight: 600 }}>{member.name || "—"}</td>
                    <td>{member.email}</td>
                    <td>{member.role}</td>
                    <td>
                      {member.twoFactorEnabled
                        ? <span style={{ ...tag, background: "var(--color-emerald-50)", color: "var(--color-emerald-800)" }}><IconShieldCheck size={12} />aktif</span>
                        : <span style={{ ...tag, background: "var(--color-warning-50)", color: "var(--color-warning-700)" }}><IconShieldOff size={12} />belum</span>}
                    </td>
                    <td>{date(member.joinedAt)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>

      <div className="tw-grid-2">
        <section className="tw-card tw-panel tw-enter">
          <h2 className="tw-panel__title">Tagihan langganan</h2>
          {detail.invoices.length === 0 ? (
            <p className="tw-panel__lede" style={{ marginBottom: 0 }}>Belum pernah ditagih.</p>
          ) : (
            <div className="tw-table-wrap">
              <table className="tw-table">
                <thead><tr>{["Dibuat", "Jumlah", "Status", "Jatuh tempo"].map((head) => <th key={head}>{head}</th>)}</tr></thead>
                <tbody>
                  {detail.invoices.map((invoice) => (
                    <tr key={invoice.id}>
                      <td>{date(invoice.createdAt)}</td>
                      <td style={{ fontWeight: 600 }}>{rupiah(invoice.amountIdr)}</td>
                      <td>
                        {invoice.voidedAt
                          ? <span title={invoice.voidedReason} style={{ color: "var(--color-warm-400)" }}>dibatalkan</span>
                          : invoice.paidAt
                            ? <span style={{ color: "var(--color-emerald-800)", fontWeight: 600 }}>lunas {date(invoice.paidAt)}</span>
                            : <span style={{ color: "var(--color-warning-700)", fontWeight: 600 }}>{invoice.status.toLowerCase()}</span>}
                      </td>
                      <td>{date(invoice.dueAt)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </section>

        <section className="tw-card tw-panel tw-enter">
          <h2 className="tw-panel__title"><IconWorld size={16} style={{ verticalAlign: "-3px", marginRight: 6 }} />Domain</h2>
          {detail.domains.length === 0 ? (
            <p className="tw-panel__lede" style={{ marginBottom: 0 }}>
              Belum ada domain sendiri. Storefront-nya memakai subdomain {operator.slug || "—"} di alamat platform.
            </p>
          ) : (
            <ul style={{ margin: 0, padding: 0, listStyle: "none", display: "grid", gap: 10 }}>
              {detail.domains.map((domain) => (
                <li key={domain.hostname} style={domainRow}>
                  <span style={{ fontWeight: 600 }}>{domain.hostname}</span>
                  {domain.isPrimary && <span style={{ ...tag, background: "var(--color-cream-200)", color: "var(--color-warm-700)" }}>utama</span>}
                  {domain.verifiedAt
                    ? <span style={{ ...tag, background: "var(--color-emerald-50)", color: "var(--color-emerald-800)" }}>terverifikasi</span>
                    : <span style={{ ...tag, background: "var(--color-warning-50)", color: "var(--color-warning-700)" }}>belum diverifikasi</span>}
                </li>
              ))}
            </ul>
          )}
        </section>
      </div>

      <section className="tw-card tw-panel tw-enter">
        <h2 className="tw-panel__title">Jejak audit</h2>
        <p className="tw-panel__lede">
          Empat puluh baris terakhir milik travel ini saja.
        </p>
        {detail.audit.length === 0 ? (
          <p className="tw-panel__lede" style={{ marginBottom: 0 }}>Belum ada jejak.</p>
        ) : (
          <div className="tw-table-wrap">
            <table className="tw-table">
              <thead><tr>{["Waktu", "Pelaku", "Tindakan", "Objek"].map((head) => <th key={head}>{head}</th>)}</tr></thead>
              <tbody>
                {detail.audit.map((entry, index) => (
                  <tr key={`${entry.at?.seconds ?? index}-${index}`} style={tr}>
                    <td>{dateTime(entry.at)}</td>
                    <td>{entry.actor}</td>
                    <td style={{ fontWeight: 600 }}>{entry.action}</td>
                    <td style={{ color: "var(--color-warm-400)" }}>{entry.entityType}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>

      <section className="tw-card tw-panel tw-enter">
        <h2 className="tw-panel__title"><IconLock size={16} style={{ verticalAlign: "-3px", marginRight: 6 }} />Tindakan yang tidak bisa ditarik</h2>
        <p className="tw-panel__lede">
          Penangguhan, pembukaan kembali, dan perubahan batas paket. Kolom terakhir menyebut berapa admin platform
          yang ada saat itu — satu berarti persetujuan orang kedua memang belum mungkin, bukan bahwa aturannya
          dilewati.
        </p>
        {privileged.length === 0 ? (
          <p className="tw-panel__lede" style={{ marginBottom: 0 }}>Belum ada tindakan istimewa pada travel ini.</p>
        ) : (
          <div className="tw-table-wrap">
            <table className="tw-table">
              <thead><tr>{["Waktu", "Tindakan", "Alasan", "Diminta oleh", "Disetujui oleh", "Admin saat itu"].map((head) => <th key={head}>{head}</th>)}</tr></thead>
              <tbody>
                {privileged.map((action) => (
                  <tr key={action.id}>
                    <td>{dateTime(action.requestedAt)}</td>
                    <td style={{ fontWeight: 700 }}>{PRIVILEGED_LABEL[action.kind] ?? action.kind}</td>
                    <td style={{ maxWidth: 280 }}>{action.reason}</td>
                    <td>{action.requestedBy}</td>
                    <td>
                      {action.approvedBy}
                      {action.approvedBy === action.requestedBy && action.adminCountAtRequest <= 1 && (
                        <span style={{ ...tag, background: "var(--color-cream-200)", color: "var(--color-warm-700)", marginLeft: 6 }}>
                          admin tunggal
                        </span>
                      )}
                    </td>
                    <td>{action.adminCountAtRequest}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>

      <section className="tw-card tw-panel tw-enter">
        <h2 className="tw-panel__title"><IconEyeglass size={16} style={{ verticalAlign: "-3px", marginRight: 6 }} />Riwayat sesi lihat-saja</h2>
        <p className="tw-panel__lede">
          Setiap kali seseorang dari TawafiqHub membuka akun ini sebagai pemiliknya. Barisnya tetap ada setelah
          sesinya berakhir — itulah gunanya.
        </p>
        {impersonations.length === 0 ? (
          <p className="tw-panel__lede" style={{ marginBottom: 0 }}>Belum pernah ada yang membuka akun ini sebagai pemiliknya.</p>
        ) : (
          <div className="tw-table-wrap">
            <table className="tw-table">
              <thead><tr>{["Mulai", "Oleh", "Alasan", "Alamat", "Selesai"].map((head) => <th key={head}>{head}</th>)}</tr></thead>
              <tbody>
                {impersonations.map((session) => (
                  <tr key={session.id}>
                    <td>{dateTime(session.startedAt)}</td>
                    <td>{session.admin}</td>
                    <td style={{ maxWidth: 320 }}>{session.reason}</td>
                    <td style={{ color: "var(--color-warm-400)" }}>{session.ip || "—"}</td>
                    <td>
                      {session.endedAt
                        ? dateTime(session.endedAt)
                        : session.expiresAt && session.expiresAt.toDate().getTime() < Date.now()
                          ? <span style={{ color: "var(--color-warm-400)" }}>kedaluwarsa {dateTime(session.expiresAt)}</span>
                          : <span style={{ color: "var(--color-warning-700)", fontWeight: 700 }}>masih terbuka</span>}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>

      <section className="tw-card tw-panel tw-enter">
        <h2 className="tw-panel__title"><IconShieldCheck size={16} style={{ verticalAlign: "-3px", marginRight: 6 }} />Pembacaan data pribadi oleh TawafiqHub</h2>
        <p className="tw-panel__lede">
          Perubahan data selalu tercatat; pembacaan dulu tidak. Satu baris per orang, per layar, per hari — bukan per
          permintaan. Angkanya menghitung <strong>percobaan</strong> membaca, termasuk yang lalu ditolak, karena
          dicatat sebelum permintaannya dilayani.
        </p>
        {reads.length === 0 ? (
          <p className="tw-panel__lede" style={{ marginBottom: 0 }}>Belum ada data pribadi travel ini yang dibaca dari sisi TawafiqHub.</p>
        ) : (
          <div className="tw-table-wrap">
            <table className="tw-table">
              <thead><tr>{["Tanggal", "Siapa", "Layar", "Berapa kali", "Terakhir", "Dari"].map((head) => <th key={head}>{head}</th>)}</tr></thead>
              <tbody>
                {reads.map((row) => (
                  <tr key={`${row.day}-${row.actor}-${row.procedure}`} style={tr}>
                    <td>{row.day}</td>
                    <td>{row.actor}</td>
                    <td style={{ color: "var(--color-warm-400)" }}>{screenName(row.procedure)}</td>
                    <td style={{ fontWeight: 700 }}>{count(row.readCount)}</td>
                    <td>{row.lastAt} WIB</td>
                    <td>
                      {row.insideTenantView
                        ? <span style={{ ...tag, background: "var(--color-warning-50)", color: "var(--color-warning-700)" }}>sesi lihat-saja</span>
                        : <span style={{ ...tag, background: "var(--color-cream-200)", color: "var(--color-warm-700)" }}>panel platform</span>}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>

      <p style={{ ...muted, fontSize: 12, marginTop: 16 }}>
        Selain menangguhkan dan membuka kembali, halaman ini hanya membaca. Mengubah paket, kuota, dan masa tenggang
        dilakukan di tab yang punya konfirmasi dan jejaknya sendiri —{" "}
        <Link href="/admin" style={inlineLink}>Paket &amp; Kuota</Link> dan{" "}
        <Link href="/admin" style={inlineLink}>Langganan</Link>. Dua jalan menuju tindakan yang sama berarti dua tempat
        yang harus sama-sama benar.
      </p>
    </main>
  );
}

/**
 * Opening a read-only window onto this tenant's own dashboard.
 *
 * The reason is required and typed by hand every time, on purpose. A dropdown
 * of canned reasons produces a column full of the first option; a sentence
 * somebody had to write is the only thing that will explain the session to
 * whoever reads the audit trail a year from now.
 */
function StartImpersonation({ operatorId, operatorName }: { operatorId: string; operatorName: string }) {
  const [open, setOpen] = useState(false);
  const [reason, setReason] = useState("");
  const [minutes, setMinutes] = useState(15);
  const [busy, setBusy] = useState(false);
  const [failure, setFailure] = useState("");

  const start = useCallback(async () => {
    setBusy(true);
    setFailure("");
    try {
      const response = await platformClient.startImpersonation({
        operatorId, reason: reason.trim(), minutes,
        // New for each attempt, not per tenant: a retry must not be able to
        // settle a session opened minutes ago for a different reason.
        idempotencyKey: `imp-${operatorId}-${crypto.randomUUID()}`,
      });
      startImpersonationLocally({
        token: response.token,
        operatorId,
        operatorName: response.operatorName || operatorName,
        expiresAt: response.expiresAt ? response.expiresAt.toDate().getTime() : Date.now() + minutes * 60_000,
      });
      // A full navigation, not a client-side push: every cached RPC result on
      // this page belongs to the admin's own identity, and carrying it into the
      // tenant's dashboard would show one customer's numbers under another's
      // name.
      window.location.href = "/dashboard";
    } catch (error: unknown) {
      setFailure(error instanceof Error ? error.message : String(error));
      setBusy(false);
    }
  }, [operatorId, operatorName, reason, minutes]);

  if (!open) {
    return (
      <button type="button" onClick={() => setOpen(true)} style={impersonateButton}>
        <IconEyeglass size={16} />Lihat sebagai travel ini
      </button>
    );
  }

  return (
    <div style={impersonateForm}>
      <p style={{ margin: "0 0 10px", fontSize: 13, lineHeight: 1.6, color: "var(--color-warm-700)" }}>
        Sesi <strong>hanya membaca</strong> dan berakhir sendiri. Perubahan untuk pelanggan tetap dilakukan lewat panel
        platform, yang punya konfirmasi dan jejaknya sendiri. Alasan di bawah tercatat di jejak audit travel ini.
      </p>
      <label style={fieldLabel} htmlFor="impersonation-reason">Alasan (minimal 10 huruf)</label>
      <textarea
        id="impersonation-reason"
        value={reason}
        onChange={(event) => setReason(event.target.value)}
        rows={2}
        placeholder="mis. pelanggan melaporkan daftar jamaah kosong setelah impor"
        style={textarea}
      />
      <label style={fieldLabel} htmlFor="impersonation-minutes">Durasi</label>
      <select id="impersonation-minutes" value={minutes} onChange={(event) => setMinutes(Number(event.target.value))} style={select}>
        {[15, 30, 60].map((value) => <option key={value} value={value}>{value} menit</option>)}
      </select>
      {failure && <p style={{ margin: "10px 0 0", fontSize: 12, color: "var(--color-danger-600)" }}>{failure}</p>}
      <div style={{ display: "flex", gap: 8, marginTop: 12 }}>
        <button type="button" onClick={start} disabled={busy || reason.trim().length < 10} style={confirmButton}>
          {busy ? "Membuka…" : "Mulai sesi lihat-saja"}
        </button>
        <button type="button" onClick={() => { setOpen(false); setFailure(""); }} style={cancelButton}>Batal</button>
      </div>
    </div>
  );
}

// The procedure name is precise and unreadable. This turns it into the screen
// a person would recognise, and falls back to the raw name rather than hiding
// a surface nobody has labelled yet.
function screenName(procedure: string): string {
  const known: Record<string, string> = {
    "/hajj.v1.PilgrimService/ListPilgrims": "Daftar jamaah",
    "/hajj.v1.PilgrimService/GetPilgrim": "Detail jamaah",
    "/hajj.v1.RegistrationService/ListRegistrations": "Pendaftaran masuk",
    "/hajj.v1.HealthReportService/ListHealthReports": "Laporan kesehatan",
    "/hajj.v1.DocumentService/ListDocuments": "Dokumen jamaah",
    "/hajj.v1.ChatService/ListMessages": "Percakapan",
    "/hajj.v1.MonitoringService/ListPilgrimLocations": "Lokasi jamaah",
    "/hajj.v1.PlatformService/ListKycRecords": "Daftar identitas (panel)",
    "/hajj.v1.PlatformService/GetKycRecord": "Nomor identitas dibuka (panel)",
  };
  return known[procedure] ?? procedure.split("/").pop() ?? procedure;
}

const PRIVILEGED_LABEL: Record<string, string> = {
  SUSPEND: "Ditangguhkan",
  REINSTATE: "Dibuka kembali",
  SET_PLAN_LIMIT: "Batas paket diubah",
  SET_SETTLEMENT: "Rekening settlement diubah",
  DELETE_TENANT: "Travel dihapus",
};

/**
 * Locking a travel agency out, and letting them back in.
 *
 * The name has to be typed. Not because typing proves authority — the server
 * already knows who is asking — but because it proves the admin is looking at
 * the row they think they are. Suspending the wrong tenant is a phone call
 * from a customer whose staff cannot log in.
 */
function SuspensionControl({ operatorId, operatorName, suspended }: { operatorId: string; operatorName: string; suspended: boolean }) {
  const [open, setOpen] = useState(false);
  const [reason, setReason] = useState("");
  const [typed, setTyped] = useState("");
  const [busy, setBusy] = useState(false);
  const [failure, setFailure] = useState("");

  const submit = useCallback(async () => {
    setBusy(true);
    setFailure("");
    const payload = {
      operatorId, reason: reason.trim(), confirmation: typed.trim(),
      idempotencyKey: `${suspended ? "rein" : "susp"}-${operatorId}-${crypto.randomUUID()}`,
    };
    try {
      if (suspended) await platformClient.reinstateTenant(payload);
      else await platformClient.suspendTenant(payload);
      // Reloaded rather than patched into state: this changes the tenant's
      // access, and half the page would still be describing the old answer.
      window.location.reload();
    } catch (error: unknown) {
      setFailure(error instanceof Error ? error.message : String(error));
      setBusy(false);
    }
  }, [operatorId, reason, typed, suspended]);

  if (!open) {
    return (
      <button type="button" onClick={() => setOpen(true)} style={suspended ? reinstateButton : suspendButton}>
        {suspended ? <IconLockOpen size={16} /> : <IconLock size={16} />}
        {suspended ? "Buka kembali travel ini" : "Tangguhkan travel ini"}
      </button>
    );
  }

  return (
    <div style={suspended ? reinstateForm : suspendForm}>
      <p style={{ margin: "0 0 10px", fontSize: 13, lineHeight: 1.6, color: "var(--color-warm-700)" }}>
        {suspended
          ? "Travel ini bisa masuk lagi seketika. Sisa waktu yang sudah mereka bayar tidak pernah dikurangi selama ditangguhkan, jadi mereka mendapatkannya kembali utuh."
          : "Seluruh staf travel ini langsung tidak bisa masuk. Storefront dan portal jamaah tidak terpengaruh, dan sisa waktu yang sudah dibayar tetap berjalan — membuka kembali mengembalikannya utuh."}
      </p>
      <label style={fieldLabel} htmlFor="suspension-reason">Alasan (minimal 10 huruf)</label>
      <textarea
        id="suspension-reason"
        value={reason}
        onChange={(event) => setReason(event.target.value)}
        rows={2}
        placeholder={suspended ? "mis. kesalahpahaman selesai, dibuka kembali atas permintaan pemilik" : "mis. permintaan resmi pemilik travel lewat surat tertanggal 3 September"}
        style={textarea}
      />
      <label style={fieldLabel} htmlFor="suspension-confirm">
        Ketik nama travel persis: <strong>{operatorName}</strong>
      </label>
      <input
        id="suspension-confirm"
        value={typed}
        onChange={(event) => setTyped(event.target.value)}
        placeholder={operatorName}
        style={confirmInput}
        autoComplete="off"
      />
      {failure && <p style={{ margin: "10px 0 0", fontSize: 12, color: "var(--color-danger-600)" }}>{failure}</p>}
      <div style={{ display: "flex", gap: 8, marginTop: 12 }}>
        <button
          type="button"
          onClick={submit}
          disabled={busy || reason.trim().length < 10 || typed.trim().toLowerCase() !== operatorName.trim().toLowerCase()}
          style={suspended ? confirmReinstate : confirmSuspend}
        >
          {busy ? "Menyimpan…" : suspended ? "Buka kembali" : "Tangguhkan"}
        </button>
        <button type="button" onClick={() => { setOpen(false); setFailure(""); }} style={cancelButton}>Batal</button>
      </div>
    </div>
  );
}

const page: React.CSSProperties = { maxWidth: 1100, margin: "0 auto", padding: "32px 24px" };
const back: React.CSSProperties = { display: "inline-flex", alignItems: "center", gap: 6, color: "var(--color-warm-500)", fontSize: 13, fontWeight: 600, textDecoration: "none", marginBottom: 16 };
const muted: React.CSSProperties = { color: "var(--color-warm-500)", fontSize: 14, margin: 0 };
const alertRow: React.CSSProperties = { display: "flex", gap: 10, flexWrap: "wrap", margin: "18px 0 0" };
const rowHead: React.CSSProperties = { display: "flex", justifyContent: "space-between", gap: 12, fontSize: 13, marginBottom: 5 };
const track: React.CSSProperties = { height: 10, background: "var(--color-cream-300)", borderRadius: 5, overflow: "hidden" };
const overrideBox: React.CSSProperties = { marginTop: 16, padding: "12px 14px", borderRadius: 8, background: "var(--color-cream-100)", border: "1px solid var(--color-cream-300)", fontSize: 13 };
const countGrid: React.CSSProperties = { display: "grid", gridTemplateColumns: "repeat(auto-fit,minmax(120px,1fr))", gap: 12, margin: 0 };
const countItem: React.CSSProperties = { padding: "10px 12px", borderRadius: 8, background: "var(--color-cream-100)" };
const countValue: React.CSSProperties = { margin: 0, fontSize: 18, fontWeight: 700, color: "var(--color-warm-900)" };
const tr: React.CSSProperties = { borderBottom: "1px solid var(--color-cream-300)" };
const tag: React.CSSProperties = { display: "inline-flex", alignItems: "center", gap: 4, padding: "3px 9px", borderRadius: 99, fontSize: 11, fontWeight: 700 };
const warnLine: React.CSSProperties = { display: "flex", alignItems: "center", gap: 8, margin: "0 0 12px", padding: "10px 14px", borderRadius: 8, background: "var(--color-warning-50)", color: "var(--color-warning-700)", fontSize: 13, fontWeight: 600 };
const domainRow: React.CSSProperties = { display: "flex", alignItems: "center", gap: 8, flexWrap: "wrap", padding: "10px 12px", borderRadius: 8, background: "var(--color-cream-100)", fontSize: 13 };
const inlineLink: React.CSSProperties = { color: "var(--color-emerald-800)", fontWeight: 700 };
const impersonateButton: React.CSSProperties = { display: "inline-flex", alignItems: "center", gap: 8, minHeight: 44, padding: "0 18px", borderRadius: 8, border: "1px solid var(--color-warning-200)", background: "var(--color-warning-50)", color: "var(--color-warning-700)", fontWeight: 700, fontSize: 13, cursor: "pointer" };
const impersonateForm: React.CSSProperties = { width: "min(420px, 100%)", padding: 16, borderRadius: 10, border: "1px solid var(--color-warning-200)", background: "var(--color-warning-50)" };
const fieldLabel: React.CSSProperties = { display: "block", margin: "0 0 4px", fontSize: 11, fontWeight: 700, color: "var(--color-warm-700)", letterSpacing: ".03em" };
const textarea: React.CSSProperties = { width: "100%", padding: "8px 10px", borderRadius: 8, border: "1px solid var(--color-cream-400)", background: "#fff", font: "inherit", fontSize: 13, resize: "vertical", marginBottom: 10 };
const select: React.CSSProperties = { minHeight: 40, padding: "0 10px", borderRadius: 8, border: "1px solid var(--color-cream-400)", background: "#fff", font: "inherit", fontSize: 13 };
const confirmButton: React.CSSProperties = { minHeight: 42, padding: "0 18px", borderRadius: 8, border: 0, background: "var(--color-warning-700)", color: "#fff", font: "inherit", fontWeight: 700, fontSize: 13, cursor: "pointer" };
const suspendButton: React.CSSProperties = { display: "inline-flex", alignItems: "center", gap: 8, minHeight: 44, padding: "0 18px", borderRadius: 8, border: "1px solid var(--color-danger-100)", background: "var(--color-danger-100)", color: "var(--color-danger-600)", fontWeight: 700, fontSize: 13, cursor: "pointer" };
const reinstateButton: React.CSSProperties = { display: "inline-flex", alignItems: "center", gap: 8, minHeight: 44, padding: "0 18px", borderRadius: 8, border: "1px solid var(--color-emerald-200)", background: "var(--color-emerald-50)", color: "var(--color-emerald-800)", fontWeight: 700, fontSize: 13, cursor: "pointer" };
const suspendForm: React.CSSProperties = { width: "min(460px, 100%)", padding: 16, borderRadius: 10, border: "1px solid var(--color-danger-100)", background: "var(--color-danger-100)" };
const reinstateForm: React.CSSProperties = { width: "min(460px, 100%)", padding: 16, borderRadius: 10, border: "1px solid var(--color-emerald-200)", background: "var(--color-emerald-50)" };
const confirmInput: React.CSSProperties = { width: "100%", minHeight: 40, padding: "0 10px", borderRadius: 8, border: "1px solid var(--color-cream-400)", background: "#fff", font: "inherit", fontSize: 13 };
const confirmSuspend: React.CSSProperties = { minHeight: 42, padding: "0 18px", borderRadius: 8, border: 0, background: "var(--color-danger-600)", color: "#fff", font: "inherit", fontWeight: 700, fontSize: 13, cursor: "pointer" };
const confirmReinstate: React.CSSProperties = { minHeight: 42, padding: "0 18px", borderRadius: 8, border: 0, background: "var(--color-emerald-800)", color: "#fff", font: "inherit", fontWeight: 700, fontSize: 13, cursor: "pointer" };
const cancelButton: React.CSSProperties = { minHeight: 42, padding: "0 16px", borderRadius: 8, border: "1px solid var(--color-cream-400)", background: "#fff", font: "inherit", fontWeight: 700, fontSize: 13, color: "var(--color-warm-700)", cursor: "pointer" };
