"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import {
  IconAlertTriangle, IconArrowLeft, IconBuildingStore, IconExternalLink,
  IconEyeglass, IconShieldCheck, IconShieldOff, IconWorld,
} from "@tabler/icons-react";
import type { GetTenantDetailResponse, ImpersonationRow } from "@hajj-saas/proto-gen/hajj/v1/platform_pb";
import { startImpersonationLocally } from "@/lib/impersonation";
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
  }, [operatorId]);

  if (loading) return <main style={page}><p style={muted}>Memuat data travel…</p></main>;

  if (failure || !detail?.operator) {
    return (
      <main style={page}>
        <Link href="/admin" style={back}><IconArrowLeft size={15} />Kembali ke panel</Link>
        <section style={emptyState}>
          <IconAlertTriangle size={40} color="var(--color-danger-600)" />
          <h1 style={{ margin: 0, fontSize: 22, fontWeight: 500 }}>Travel tidak dapat dibuka</h1>
          <p style={{ ...muted, maxWidth: 520 }}>
            Tautannya mungkin salah, atau travel ini sudah dihapus. Alamat halaman memakai id travel, bukan namanya.
          </p>
          {failure && <code style={failureBox}>{failure}</code>}
        </section>
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

      <header style={header}>
        <div>
          <p style={eyebrow}>TAWAFIQHUB / TRAVEL</p>
          <h1 style={title}>{operator.name}</h1>
          <p style={muted}>
            {operator.slug || "tanpa slug"} · bergabung {date(operator.createdAt)} · {operator.plan || "tanpa paket"}
          </p>
        </div>
        <div style={{ display: "flex", gap: 10, flexWrap: "wrap" }}>
          <StartImpersonation operatorId={operator.id} operatorName={operator.name} />
          {storefront && (
            <a href={storefront} target="_blank" rel="noreferrer" style={storefrontButton}>
              <IconBuildingStore size={16} />Buka storefront<IconExternalLink size={13} />
            </a>
          )}
        </div>
      </header>
      <div className="gold-divider" />

      {(suspended || lapsed || operator.dunningStage) && (
        <div style={alertRow}>
          {suspended && (
            <span style={{ ...pill, background: "var(--color-danger-100)", color: "var(--color-danger-600)" }}>
              Ditangguhkan {date(operator.suspendedAt)} — dihentikan dengan sengaja, bukan karena tagihan
            </span>
          )}
          {lapsed && !suspended && (
            <span style={{ ...pill, background: "var(--color-warning-50)", color: "var(--color-warning-700)" }}>
              Akses habis {date(operator.effectiveAccessUntil)} — termasuk masa tenggang
            </span>
          )}
          {operator.dunningStage && (
            <span style={{ ...pill, background: "var(--color-cream-200)", color: "var(--color-warm-700)" }}>
              Tagihan tahap {operator.dunningStage}
            </span>
          )}
        </div>
      )}

      <div style={statGrid}>
        {[
          { label: "Status langganan", value: operator.subscriptionStatus || "—", hint: `akses sampai ${date(operator.accessUntil)}` },
          { label: "Tagihan berjalan", value: rupiah(operator.outstandingIdr), hint: operator.outstandingIdr > 0n ? "belum dibayar" : "tidak ada tagihan terbuka" },
          { label: "Saldo kredit", value: rupiah(operator.creditBalanceIdr), hint: "dipakai otomatis pada tagihan berikutnya" },
          { label: "Transaksi tertahan", value: count(operator.heldOrderCount), hint: "uang masuk yang menunggu keputusan" },
        ].map((card) => (
          <div key={card.label} style={statCard}>
            <p style={statLabel}>{card.label}</p>
            <p style={statValue}>{card.value}</p>
            <p style={statHint}>{card.hint}</p>
          </div>
        ))}
      </div>

      <div style={twoCol}>
        <section style={card}>
          <h2 style={cardTitle}>Pemakaian terhadap kuota</h2>
          {detail.usage.length === 0 ? (
            <p style={muted}>
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

        <section style={card}>
          <h2 style={cardTitle}>Isi akun</h2>
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
                <dt style={statLabel}>{label}</dt>
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

      <section style={{ ...card, marginTop: 16 }}>
        <h2 style={cardTitle}>Tim</h2>
        {twoFactorMissing.length > 0 && (
          <p style={warnLine}>
            <IconShieldOff size={15} />
            {twoFactorMissing.length} dari {detail.team.length} akun belum memakai verifikasi dua langkah.
          </p>
        )}
        {detail.team.length === 0 ? (
          <p style={muted}>Tidak ada anggota tim. Travel tanpa akun tidak bisa masuk sama sekali.</p>
        ) : (
          <div style={{ overflowX: "auto" }}>
            <table style={table}>
              <thead><tr>{["Nama", "Email", "Peran", "2FA", "Bergabung"].map((head) => <th key={head} style={th}>{head}</th>)}</tr></thead>
              <tbody>
                {detail.team.map((member) => (
                  <tr key={member.userId} style={tr}>
                    <td style={{ ...td, fontWeight: 600 }}>{member.name || "—"}</td>
                    <td style={td}>{member.email}</td>
                    <td style={td}>{member.role}</td>
                    <td style={td}>
                      {member.twoFactorEnabled
                        ? <span style={{ ...tag, background: "var(--color-emerald-50)", color: "var(--color-emerald-800)" }}><IconShieldCheck size={12} />aktif</span>
                        : <span style={{ ...tag, background: "var(--color-warning-50)", color: "var(--color-warning-700)" }}><IconShieldOff size={12} />belum</span>}
                    </td>
                    <td style={td}>{date(member.joinedAt)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>

      <div style={twoCol}>
        <section style={card}>
          <h2 style={cardTitle}>Tagihan langganan</h2>
          {detail.invoices.length === 0 ? (
            <p style={muted}>Belum pernah ditagih.</p>
          ) : (
            <div style={{ overflowX: "auto" }}>
              <table style={table}>
                <thead><tr>{["Dibuat", "Jumlah", "Status", "Jatuh tempo"].map((head) => <th key={head} style={th}>{head}</th>)}</tr></thead>
                <tbody>
                  {detail.invoices.map((invoice) => (
                    <tr key={invoice.id} style={tr}>
                      <td style={td}>{date(invoice.createdAt)}</td>
                      <td style={{ ...td, fontWeight: 600 }}>{rupiah(invoice.amountIdr)}</td>
                      <td style={td}>
                        {invoice.voidedAt
                          ? <span title={invoice.voidedReason} style={{ color: "var(--color-warm-400)" }}>dibatalkan</span>
                          : invoice.paidAt
                            ? <span style={{ color: "var(--color-emerald-800)", fontWeight: 600 }}>lunas {date(invoice.paidAt)}</span>
                            : <span style={{ color: "var(--color-warning-700)", fontWeight: 600 }}>{invoice.status.toLowerCase()}</span>}
                      </td>
                      <td style={td}>{date(invoice.dueAt)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </section>

        <section style={card}>
          <h2 style={cardTitle}><IconWorld size={16} style={{ verticalAlign: "-3px", marginRight: 6 }} />Domain</h2>
          {detail.domains.length === 0 ? (
            <p style={muted}>
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

      <section style={{ ...card, marginTop: 16 }}>
        <h2 style={cardTitle}>Jejak audit</h2>
        <p style={{ ...muted, fontSize: 12, marginBottom: 12 }}>
          Empat puluh baris terakhir milik travel ini saja.
        </p>
        {detail.audit.length === 0 ? (
          <p style={muted}>Belum ada jejak.</p>
        ) : (
          <div style={{ overflowX: "auto" }}>
            <table style={table}>
              <thead><tr>{["Waktu", "Pelaku", "Tindakan", "Objek"].map((head) => <th key={head} style={th}>{head}</th>)}</tr></thead>
              <tbody>
                {detail.audit.map((entry, index) => (
                  <tr key={`${entry.at?.seconds ?? index}-${index}`} style={tr}>
                    <td style={td}>{dateTime(entry.at)}</td>
                    <td style={td}>{entry.actor}</td>
                    <td style={{ ...td, fontWeight: 600 }}>{entry.action}</td>
                    <td style={{ ...td, color: "var(--color-warm-400)" }}>{entry.entityType}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>

      <section style={{ ...card, marginTop: 16 }}>
        <h2 style={cardTitle}><IconEyeglass size={16} style={{ verticalAlign: "-3px", marginRight: 6 }} />Riwayat sesi lihat-saja</h2>
        <p style={{ ...muted, fontSize: 12, marginBottom: 12 }}>
          Setiap kali seseorang dari TawafiqHub membuka akun ini sebagai pemiliknya. Barisnya tetap ada setelah
          sesinya berakhir — itulah gunanya.
        </p>
        {impersonations.length === 0 ? (
          <p style={muted}>Belum pernah ada yang membuka akun ini sebagai pemiliknya.</p>
        ) : (
          <div style={{ overflowX: "auto" }}>
            <table style={table}>
              <thead><tr>{["Mulai", "Oleh", "Alasan", "Alamat", "Selesai"].map((head) => <th key={head} style={th}>{head}</th>)}</tr></thead>
              <tbody>
                {impersonations.map((session) => (
                  <tr key={session.id} style={tr}>
                    <td style={td}>{dateTime(session.startedAt)}</td>
                    <td style={td}>{session.admin}</td>
                    <td style={{ ...td, maxWidth: 320 }}>{session.reason}</td>
                    <td style={{ ...td, color: "var(--color-warm-400)" }}>{session.ip || "—"}</td>
                    <td style={td}>
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

      <p style={{ ...muted, fontSize: 12, marginTop: 16 }}>
        Halaman ini hanya membaca. Mengubah paket, kuota, masa tenggang, atau menangguhkan travel dilakukan di tab
        yang punya konfirmasi dan jejaknya sendiri — <Link href="/admin" style={inlineLink}>Paket &amp; Kuota</Link> dan{" "}
        <Link href="/admin" style={inlineLink}>Langganan</Link>.
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

const page: React.CSSProperties = { maxWidth: 1100, margin: "0 auto", padding: "32px 24px" };
const back: React.CSSProperties = { display: "inline-flex", alignItems: "center", gap: 6, color: "var(--color-warm-500)", fontSize: 13, fontWeight: 600, textDecoration: "none", marginBottom: 16 };
const header: React.CSSProperties = { display: "flex", justifyContent: "space-between", gap: 20, flexWrap: "wrap", alignItems: "flex-start" };
const eyebrow: React.CSSProperties = { color: "var(--color-gold-800)", fontSize: 11, fontWeight: 700, letterSpacing: ".08em", margin: "0 0 8px" };
const title: React.CSSProperties = { fontSize: "clamp(26px,4vw,38px)", fontWeight: 500, margin: "0 0 6px" };
const muted: React.CSSProperties = { color: "var(--color-warm-500)", fontSize: 14, margin: 0 };
const storefrontButton: React.CSSProperties = { display: "inline-flex", alignItems: "center", gap: 8, minHeight: 44, padding: "0 18px", borderRadius: 8, border: "1px solid var(--color-cream-400)", background: "#fff", color: "var(--color-emerald-900)", fontWeight: 700, fontSize: 13, textDecoration: "none" };
const alertRow: React.CSSProperties = { display: "flex", gap: 10, flexWrap: "wrap", margin: "18px 0 0" };
const pill: React.CSSProperties = { display: "inline-flex", alignItems: "center", gap: 6, padding: "8px 14px", borderRadius: 99, fontSize: 12, fontWeight: 700 };
const statGrid: React.CSSProperties = { display: "grid", gridTemplateColumns: "repeat(auto-fit,minmax(200px,1fr))", gap: 12, margin: "20px 0 16px" };
const statCard: React.CSSProperties = { background: "#fff", border: "1px solid var(--color-cream-300)", borderRadius: 10, padding: "16px 18px" };
const statLabel: React.CSSProperties = { margin: "0 0 6px", fontSize: 12, color: "var(--color-warm-500)", fontWeight: 600 };
const statValue: React.CSSProperties = { margin: 0, fontSize: 22, fontWeight: 700, color: "var(--color-emerald-900)" };
const statHint: React.CSSProperties = { margin: "4px 0 0", fontSize: 11, color: "var(--color-warm-400)" };
const twoCol: React.CSSProperties = { display: "grid", gridTemplateColumns: "repeat(auto-fit,minmax(340px,1fr))", gap: 16, marginTop: 16 };
const card: React.CSSProperties = { background: "#fff", border: "1px solid var(--color-cream-300)", borderRadius: 12, padding: 22 };
const cardTitle: React.CSSProperties = { margin: "0 0 14px", fontSize: 16, fontWeight: 700 };
const rowHead: React.CSSProperties = { display: "flex", justifyContent: "space-between", gap: 12, fontSize: 13, marginBottom: 5 };
const track: React.CSSProperties = { height: 10, background: "var(--color-cream-300)", borderRadius: 5, overflow: "hidden" };
const overrideBox: React.CSSProperties = { marginTop: 16, padding: "12px 14px", borderRadius: 8, background: "var(--color-cream-100)", border: "1px solid var(--color-cream-300)", fontSize: 13 };
const countGrid: React.CSSProperties = { display: "grid", gridTemplateColumns: "repeat(auto-fit,minmax(120px,1fr))", gap: 12, margin: 0 };
const countItem: React.CSSProperties = { padding: "10px 12px", borderRadius: 8, background: "var(--color-cream-100)" };
const countValue: React.CSSProperties = { margin: 0, fontSize: 18, fontWeight: 700, color: "var(--color-warm-900)" };
const table: React.CSSProperties = { width: "100%", borderCollapse: "collapse", fontSize: 13 };
const th: React.CSSProperties = { textAlign: "left", padding: "10px 12px", fontSize: 11, color: "var(--color-warm-400)", borderBottom: "1px solid var(--color-cream-300)", whiteSpace: "nowrap" };
const tr: React.CSSProperties = { borderBottom: "1px solid var(--color-cream-300)" };
const td: React.CSSProperties = { padding: "10px 12px", color: "var(--color-warm-700)" };
const tag: React.CSSProperties = { display: "inline-flex", alignItems: "center", gap: 4, padding: "3px 9px", borderRadius: 99, fontSize: 11, fontWeight: 700 };
const warnLine: React.CSSProperties = { display: "flex", alignItems: "center", gap: 8, margin: "0 0 12px", padding: "10px 14px", borderRadius: 8, background: "var(--color-warning-50)", color: "var(--color-warning-700)", fontSize: 13, fontWeight: 600 };
const domainRow: React.CSSProperties = { display: "flex", alignItems: "center", gap: 8, flexWrap: "wrap", padding: "10px 12px", borderRadius: 8, background: "var(--color-cream-100)", fontSize: 13 };
const emptyState: React.CSSProperties = { minHeight: 280, display: "grid", placeItems: "center", alignContent: "center", gap: 12, textAlign: "center", border: "1px dashed var(--color-cream-400)", borderRadius: 12, padding: 32 };
const failureBox: React.CSSProperties = { display: "block", maxWidth: 520, padding: 12, background: "var(--color-cream-100)", borderRadius: 8, fontSize: 13, color: "var(--color-danger-600)", overflowWrap: "anywhere" };
const inlineLink: React.CSSProperties = { color: "var(--color-emerald-800)", fontWeight: 700 };
const impersonateButton: React.CSSProperties = { display: "inline-flex", alignItems: "center", gap: 8, minHeight: 44, padding: "0 18px", borderRadius: 8, border: "1px solid var(--color-warning-200)", background: "var(--color-warning-50)", color: "var(--color-warning-700)", fontWeight: 700, fontSize: 13, cursor: "pointer" };
const impersonateForm: React.CSSProperties = { width: "min(420px, 100%)", padding: 16, borderRadius: 10, border: "1px solid var(--color-warning-200)", background: "var(--color-warning-50)" };
const fieldLabel: React.CSSProperties = { display: "block", margin: "0 0 4px", fontSize: 11, fontWeight: 700, color: "var(--color-warm-700)", letterSpacing: ".03em" };
const textarea: React.CSSProperties = { width: "100%", padding: "8px 10px", borderRadius: 8, border: "1px solid var(--color-cream-400)", background: "#fff", font: "inherit", fontSize: 13, resize: "vertical", marginBottom: 10 };
const select: React.CSSProperties = { minHeight: 40, padding: "0 10px", borderRadius: 8, border: "1px solid var(--color-cream-400)", background: "#fff", font: "inherit", fontSize: 13 };
const confirmButton: React.CSSProperties = { minHeight: 42, padding: "0 18px", borderRadius: 8, border: 0, background: "var(--color-warning-700)", color: "#fff", font: "inherit", fontWeight: 700, fontSize: 13, cursor: "pointer" };
const cancelButton: React.CSSProperties = { minHeight: 42, padding: "0 16px", borderRadius: 8, border: "1px solid var(--color-cream-400)", background: "#fff", font: "inherit", fontWeight: 700, fontSize: 13, color: "var(--color-warm-700)", cursor: "pointer" };
