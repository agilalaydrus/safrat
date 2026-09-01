"use client";

import { useEffect, useMemo, useState } from "react";
import { IconSpeakerphone, IconTrash } from "@tabler/icons-react";
import { Broadcast } from "@hajj-saas/proto-gen/hajj/v1/broadcast_pb";
import { broadcastClient, seasonClient } from "@/lib/rpc";
import { getMyAccessCached } from "@/lib/access-cache";

export default function BroadcastDashboard() {
  const [seasonId, setSeasonId] = useState("");
  const [seasons, setSeasons] = useState<{ id: string; name: string; isActive: boolean }[]>([]);
  const [broadcasts, setBroadcasts] = useState<Broadcast[]>([]);
  const [loading, setLoading] = useState(true);
  const [title, setTitle] = useState("");
  const [body, setBody] = useState("");
  const [sending, setSending] = useState(false);
  const [notice, setNotice] = useState("");
  const [branchId, setBranchId] = useState<string | null>();

  useEffect(() => {
    getMyAccessCached().then((access) => setBranchId(access.branchId || null)).catch(() => {
      setBranchId("unavailable");
      setNotice("Hak akses komunikasi belum dapat diverifikasi. Muat ulang halaman sebelum mengirim pengumuman.");
    });
  }, []);

  useEffect(() => {
    seasonClient.listSeasons({}).then((response) => {
      setSeasons(response.seasons);
      setSeasonId(response.seasons.find((s) => s.isActive)?.id ?? response.seasons[0]?.id ?? "");
    }).catch(() => setNotice("Gagal memuat data musim."));
  }, []);

  const refresh = () => {
    if (!seasonId) return;
    setLoading(true);
    broadcastClient.listBroadcasts({ seasonId }).then((response) => setBroadcasts(response.broadcasts)).catch(() => setNotice("Gagal memuat pengumuman.")).finally(() => setLoading(false));
  };
  useEffect(refresh, [seasonId]);

  const activeName = useMemo(() => seasons.find((s) => s.id === seasonId)?.name ?? "Pilih musim", [seasons, seasonId]);
  const canManage = branchId === null;

  const send = async () => {
    if (!title.trim() || !body.trim()) { setNotice("Judul dan isi pengumuman wajib diisi."); return; }
    setSending(true);
    try {
      await broadcastClient.createBroadcast({ seasonId, title, body });
      setTitle("");
      setBody("");
      setNotice("Pengumuman berhasil dikirim.");
      refresh();
    } catch (error) {
      setNotice(`Gagal mengirim: ${error instanceof Error ? error.message : "tidak diketahui"}`);
    } finally {
      setSending(false);
    }
  };

  const remove = async (broadcast: Broadcast) => {
    try {
      await broadcastClient.deleteBroadcast({ broadcastId: broadcast.id });
      setNotice("Pengumuman dihapus.");
      refresh();
    } catch (error) {
      setNotice(`Gagal menghapus: ${error instanceof Error ? error.message : "tidak diketahui"}`);
    }
  };

  return <main style={page}>
    <header style={header}>
      <div><p style={eyebrow}>OPERASIONAL / KOMUNIKASI</p><h1 style={title2}>Pengumuman</h1><p style={{ color: "var(--color-warm-500)", margin: 0 }}>{`${broadcasts.length} pengumuman${activeName ? ` · ${activeName}` : ""}`}</p></div>
      <select aria-label="Musim" value={seasonId} onChange={(e) => setSeasonId(e.target.value)} style={select}>
        {seasons.length ? seasons.map((s) => <option key={s.id} value={s.id}>{s.name}{s.isActive ? " · Aktif" : ""}</option>) : <option>{activeName}</option>}
      </select>
    </header>
    <div className="gold-divider" />
    {notice && <p role="status" style={{ color: "var(--color-gold-800)" }}>{notice}</p>}
    {branchId === undefined ? <section style={composer} aria-busy="true">Memverifikasi cakupan komunikasi…</section> : canManage ? <section style={composer}>
      <h2 style={{ margin: 0 }}>Kirim Pengumuman Baru</h2>
      <p style={{ margin: 0, color: "var(--color-warm-500)", fontSize: 13 }}>Pengumuman ini tampil kepada seluruh jamaah pada musim yang dipilih, di semua cabang.</p>
      <label style={field}>Judul
        <input value={title} onChange={(e) => setTitle(e.target.value)} placeholder="Judul pengumuman" style={input} maxLength={150} />
      </label>
      <label style={field}>Isi Pengumuman
        <textarea value={body} onChange={(e) => setBody(e.target.value)} rows={4} placeholder="Tulis pengumuman untuk seluruh jamaah pada musim ini..." style={{ ...input, minHeight: 100, resize: "vertical" }} maxLength={1000} />
      </label>
      <button disabled={sending || !seasonId} onClick={send} style={emerald}><IconSpeakerphone size={18} />{sending ? "Mengirim..." : "Kirim ke Semua Jamaah"}</button>
    </section> : <section style={branchNotice}><IconSpeakerphone size={22} /><div><h2 style={{ margin: 0, fontSize: 18 }}>Pengumuman dikelola kantor pusat</h2><p style={{ margin: "5px 0 0", color: "var(--color-warm-500)", fontSize: 13 }}>Anda tetap dapat membaca pengumuman untuk seluruh jamaah. Hubungi kantor pusat jika cabang membutuhkan pengumuman baru atau koreksi.</p></div></section>}
    <section style={{ marginTop: 24 }}>
      <h2>Riwayat Pengumuman</h2>
      {loading ? <p style={{ color: "var(--color-warm-500)" }}>Memuat...</p> : broadcasts.length ? <div style={{ display: "grid", gap: 12 }}>
        {broadcasts.map((b) => <article key={b.id} style={card}>
          <div style={{ display: "flex", justifyContent: "space-between", gap: 12, alignItems: "start" }}>
            <div><h3 style={{ margin: "0 0 4px" }}>{b.title}</h3><p style={{ margin: 0, whiteSpace: "pre-wrap" }}>{b.body}</p></div>
            {canManage && <button onClick={() => remove(b)} aria-label={`Hapus pengumuman ${b.title}`} style={deleteBtn}><IconTrash size={16} /></button>}
          </div>
          <p style={{ margin: "8px 0 0", fontSize: 12, color: "var(--color-warm-400)" }}>{b.createdAt?.toDate().toLocaleString("id-ID") ?? ""}</p>
        </article>)}
      </div> : <p style={{ color: "var(--color-warm-500)" }}>Belum ada pengumuman yang dikirim untuk musim ini. Isi formulir Kirim Pengumuman Baru di atas; riwayat pengiriman akan muncul di sini.</p>}
    </section>
  </main>;
}

const page: React.CSSProperties = { maxWidth: 900, margin: "0 auto", padding: "32px 24px" };
const header: React.CSSProperties = { display: "flex", justifyContent: "space-between", gap: 24, alignItems: "flex-start", flexWrap: "wrap" };
const eyebrow: React.CSSProperties = { color: "var(--color-gold-800)", fontSize: 11, fontWeight: 700, letterSpacing: ".08em", margin: "4px 0 8px" };
const title2: React.CSSProperties = { fontSize: "clamp(32px,5vw,48px)", fontWeight: 500, margin: 0 };
const select: React.CSSProperties = { minHeight: 48, border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 12px", background: "var(--color-cream-200)", font: "inherit", color: "var(--color-warm-900)" };
const composer: React.CSSProperties = { display: "grid", gap: 12, background: "var(--color-cream-200)", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 20 };
const field: React.CSSProperties = { display: "grid", gap: 6, color: "var(--color-warm-500)", fontSize: 14 };
const input: React.CSSProperties = { minHeight: 44, width: "100%", border: "1px solid var(--color-cream-500)", borderRadius: 8, padding: "10px 12px", background: "white", color: "var(--color-warm-900)", font: "inherit" };
const emerald: React.CSSProperties = { minHeight: 48, border: 0, borderRadius: 8, background: "var(--color-emerald-900)", color: "white", fontWeight: 700, padding: "0 18px", display: "inline-flex", gap: 8, alignItems: "center", justifySelf: "start" };
const card: React.CSSProperties = { border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 16, background: "white" };
const deleteBtn: React.CSSProperties = { minHeight: 36, minWidth: 36, border: "1px solid var(--color-cream-400)", borderRadius: 8, background: "transparent", color: "var(--color-danger-600)", display: "grid", placeItems: "center", flexShrink: 0 };
const branchNotice: React.CSSProperties = { display: "flex", alignItems: "flex-start", gap: 12, border: "1px solid var(--color-warning-200)", borderRadius: 12, background: "var(--color-warning-50)", padding: 18, color: "var(--color-warning-700)" };
