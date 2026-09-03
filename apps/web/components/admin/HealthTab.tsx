"use client";

import { useCallback, useEffect, useState } from "react";
import { IconAlertTriangle, IconCircleCheck, IconEyeOff, IconRefresh } from "@tabler/icons-react";
import type { GetPlatformHealthResponse, HealthSignal } from "@hajj-saas/proto-gen/hajj/v1/platform_pb";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { PageHeader } from "@/components/ui/PageHeader";
import type { DashboardTone } from "@/components/ui/tone";

import { platformClient } from "@/lib/rpc";

const STATUS: Record<string, { tone: DashboardTone; card: string; label: string }> = {
  OK: { tone: "success", card: "ok", label: "Aman" },
  WARN: { tone: "warning", card: "warning", label: "Perlu dilihat" },
  ALERT: { tone: "danger", card: "danger", label: "Perlu tindakan" },
  UNMONITORED: { tone: "neutral", card: "neutral", label: "Tidak dipantau" },
};

const timeOf = (at?: { toDate(): Date }) =>
  at ? at.toDate().toLocaleString("id-ID", { day: "2-digit", month: "short", hour: "2-digit", minute: "2-digit" }) : "";

function StatusIcon({ status }: { status: string }) {
  if (status === "OK") return <IconCircleCheck size={17} />;
  if (status === "UNMONITORED") return <IconEyeOff size={17} />;
  return <IconAlertTriangle size={17} />;
}

export default function HealthTab() {
  const [health, setHealth] = useState<GetPlatformHealthResponse>();
  const [loading, setLoading] = useState(true);
  const [failure, setFailure] = useState("");

  const load = useCallback(() => {
    setLoading(true);
    setFailure("");
    platformClient
      .getPlatformHealth({})
      .then(setHealth)
      .catch(() => setFailure("Gagal memuat kesehatan platform."))
      .finally(() => setLoading(false));
  }, []);

  useEffect(() => { load(); }, [load]);

  if (loading) return <p className="admin-note">Memeriksa…</p>;
  if (failure) return <p className="admin-inline-alert" data-tone="danger"><IconAlertTriangle size={16} />{failure}</p>;
  if (!health) return null;

  const attention = health.signals.filter((signal) => signal.status === "WARN" || signal.status === "ALERT");
  const unmonitored = health.signals.filter((signal) => signal.status === "UNMONITORED");

  return (
    <section className="admin-tab">
      <PageHeader
        eyebrow="TAWAFIQHUB / KESEHATAN"
        title="Kesehatan Platform"
        subtitle={
          <>
            Diperbarui {timeOf(health.checkedAt)} · {attention.length} butir perlu perhatian
            {unmonitored.length > 0 && ` · ${unmonitored.length} tidak dipantau`}
            <br />
            <span style={{ fontSize: 12 }}>
              Hanya yang berdampak ke pelanggan. Bukan konsol infrastruktur — grafik CPU membuat daftar ini terlalu
              panjang untuk dibaca justru saat ada yang benar-benar rusak.
            </span>
          </>
        }
        primaryAction={
          <Button variant="outline" onClick={load}><IconRefresh size={15} />Periksa lagi</Button>
        }
      />

      <div className="admin-grid-3 tw-stagger">
        {health.signals.map((signal) => <SignalCard key={signal.key} signal={signal} />)}
      </div>

      <div className="admin-note">
        <p>
          <strong>Yang sehat ikut ditampilkan, dengan sengaja.</strong> Layar yang hanya menampilkan masalah tidak bisa
          dibedakan dari layar yang berhenti bekerja — &ldquo;tidak ada peringatan&rdquo; harus berarti &ldquo;sudah
          diperiksa, aman&rdquo;, bukan &ldquo;mungkin tidak ada yang memeriksa&rdquo;.
        </p>
        <p>
          <strong>&ldquo;Tidak dipantau&rdquo; bukan hijau.</strong> Butir yang belum punya sumber data ditandai apa
          adanya. Lampu hijau yang tidak memeriksa apa pun lebih buruk daripada tidak ada lampu sama sekali.
        </p>
      </div>
    </section>
  );
}

function SignalCard({ signal }: { signal: HealthSignal }) {
  const status = STATUS[signal.status] ?? STATUS.UNMONITORED!;
  return (
    <article className="admin-signal tw-enter" data-tone={status.card}>
      <div className="admin-signal__head">
        <Badge tone={status.tone} dot={signal.status !== "UNMONITORED"}>
          <StatusIcon status={signal.status} />
          {status.label}
        </Badge>
      </div>
      <h3 className="admin-signal__title">{signal.title}</h3>
      <p className="admin-signal__detail">{signal.detail}</p>
      <dl className="admin-signal__meta">
        {signal.affectedTenants > 0 && (
          <div>
            <dt>Travel terdampak</dt>
            <dd>{signal.affectedTenants}</dd>
          </div>
        )}
        {signal.oldestSeen && (
          <div>
            <dt>{signal.status === "OK" ? "Terakhir" : "Sejak"}</dt>
            <dd>{timeOf(signal.oldestSeen)}</dd>
          </div>
        )}
        <div>
          <dt>Sumber</dt>
          <dd className="is-quiet">{signal.source}</dd>
        </div>
      </dl>
    </article>
  );
}
