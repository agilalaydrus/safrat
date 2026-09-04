"use client";

import { useEffect, useState } from "react";
import { IconPhoto, IconTrash, IconUpload } from "@tabler/icons-react";
import type { Moment } from "@hajj-saas/proto-gen/hajj/v1/moment_pb";
import type { Group } from "@hajj-saas/proto-gen/hajj/v1/group_pb";
import { groupClient, momentClient, pilgrimClient, seasonClient } from "@/lib/rpc";

export default function MomentDashboard() {
  const [seasons, setSeasons] = useState<{ id: string; name: string; isActive: boolean }[]>([]);
  const [seasonId, setSeasonId] = useState("");
  const [groups, setGroups] = useState<Group[]>([]);
  const [pilgrims, setPilgrims] = useState<{ id: string; fullName: string }[]>([]);
  const [moments, setMoments] = useState<Moment[]>([]);
  const [notice, setNotice] = useState("");

  const [targetKind, setTargetKind] = useState<"pilgrim" | "group">("group");
  const [targetGroupId, setTargetGroupId] = useState("");
  const [pilgrimQuery, setPilgrimQuery] = useState("");
  const [targetPilgrimId, setTargetPilgrimId] = useState("");
  const [caption, setCaption] = useState("");
  const [photo, setPhoto] = useState<File>();
  const [sending, setSending] = useState(false);

  useEffect(() => {
    seasonClient.listSeasons({}).then((r) => {
      setSeasons(r.seasons);
      setSeasonId(r.seasons.find((s) => s.isActive)?.id ?? r.seasons[0]?.id ?? "");
    }).catch(() => setNotice("Gagal memuat musim."));
  }, []);

  const refresh = () => {
    if (!seasonId) return;
    Promise.all([
      momentClient.listMoments({ seasonId }).then((r) => setMoments(r.moments)),
      groupClient.listGroups({ seasonId }).then((r) => setGroups(r.groups)),
      pilgrimClient.listPilgrims({ seasonId, limit: 1000 }).then((r) => setPilgrims(r.pilgrims.map((p) => ({ id: p.id, fullName: p.fullName })))),
    ]).catch(() => setNotice("Gagal memuat momen."));
  };
  useEffect(refresh, [seasonId]);

  const pilgrimMatches = pilgrimQuery.trim()
    ? pilgrims.filter((p) => p.fullName.toLowerCase().includes(pilgrimQuery.trim().toLowerCase())).slice(0, 8)
    : [];
  const selectedPilgrim = pilgrims.find((p) => p.id === targetPilgrimId);

  const send = async () => {
    setNotice("");
    if (!photo) { setNotice("Pilih foto terlebih dahulu."); return; }
    if (targetKind === "group" && !targetGroupId) { setNotice("Pilih grup tujuan."); return; }
    if (targetKind === "pilgrim" && !targetPilgrimId) { setNotice("Pilih jamaah tujuan."); return; }
    setSending(true);
    try {
      // Uploaded straight to storage with a one-shot link, so the photo
      // never passes through this app server. The key comes back from the
      // server, not chosen here, so an upload cannot be aimed at another
      // operator's prefix.
      const { uploadUrl, objectKey, contentType } = await momentClient.createMomentUpload({ sizeBytes: BigInt(photo.size) });
      const putResponse = await fetch(uploadUrl, { method: "PUT", body: photo, headers: { "Content-Type": contentType } });
      if (!putResponse.ok) throw new Error("Gagal mengunggah foto. Coba lagi.");
      await momentClient.createMoment({
        seasonId,
        pilgrimId: targetKind === "pilgrim" ? targetPilgrimId : undefined,
        groupId: targetKind === "group" ? targetGroupId : undefined,
        objectKey, caption: caption.trim(),
      });
      setPhoto(undefined);
      setCaption("");
      setTargetPilgrimId("");
      setPilgrimQuery("");
      refresh();
    } catch (caught) {
      setNotice(caught instanceof Error ? caught.message : "Gagal mengirim momen.");
    } finally {
      setSending(false);
    }
  };

  const remove = async (m: Moment) => {
    if (!window.confirm("Hapus momen ini?")) return;
    try { await momentClient.deleteMoment({ momentId: m.id }); refresh(); }
    catch (caught) { setNotice(caught instanceof Error ? caught.message : "Gagal menghapus momen."); }
  };

  return (
    <main style={page}>
      <header>
        <p style={eyebrow}>UNTUK KELUARGA</p>
        <h1 style={{ margin: 0, fontSize: 32 }}>Momen</h1>
        <p style={{ margin: "4px 0 0", color: "var(--color-warm-500)" }}>
          Foto dan kabar dari lapangan — tampil di halaman pelacak keluarga. Posisi GPS, nomor kamar, dan paspor tidak pernah dibagikan.
        </p>
      </header>
      <div style={{ marginTop: 12 }}>
        <select value={seasonId} onChange={(e) => setSeasonId(e.target.value)} style={seasonSelect}>
          {seasons.map((s) => <option key={s.id} value={s.id}>{s.name}{s.isActive ? " (aktif)" : ""}</option>)}
        </select>
      </div>
      {notice && <p style={{ color: "var(--color-danger-600)" }}>{notice}</p>}
      <div className="gold-divider" />

      <section style={card}>
        <h2 style={sectionTitle}>Kirim Momen Baru</h2>
        <div style={{ display: "grid", gap: 12, marginTop: 12 }}>
          <div style={{ display: "flex", gap: 8 }}>
            <button type="button" onClick={() => setTargetKind("group")} style={targetKind === "group" ? tabActive : tabBtn}>Seluruh Grup</button>
            <button type="button" onClick={() => setTargetKind("pilgrim")} style={targetKind === "pilgrim" ? tabActive : tabBtn}>Satu Jamaah</button>
          </div>

          {targetKind === "group" ? (
            <select value={targetGroupId} onChange={(e) => setTargetGroupId(e.target.value)} style={input}>
              <option value="">— pilih grup —</option>
              {groups.map((g) => <option key={g.id} value={g.id}>{g.name}</option>)}
            </select>
          ) : (
            <div style={{ position: "relative" }}>
              {selectedPilgrim ? (
                <div style={{ ...input, display: "flex", alignItems: "center", justifyContent: "space-between" }}>
                  <span>{selectedPilgrim.fullName}</span>
                  <button type="button" onClick={() => setTargetPilgrimId("")} style={{ border: 0, background: "transparent", color: "var(--color-warm-400)", fontSize: 12 }}>Ganti</button>
                </div>
              ) : (
                <>
                  <input style={input} placeholder="Cari nama jamaah..." value={pilgrimQuery} onChange={(e) => setPilgrimQuery(e.target.value)} />
                  {pilgrimMatches.length > 0 && (
                    <div style={suggestBox}>
                      {pilgrimMatches.map((p) => (
                        <button key={p.id} type="button" style={suggestItem} onClick={() => { setTargetPilgrimId(p.id); setPilgrimQuery(""); }}>{p.fullName}</button>
                      ))}
                    </div>
                  )}
                </>
              )}
            </div>
          )}

          <label style={fileLabel}>
            <IconUpload size={16} />
            {photo ? photo.name : "Pilih foto..."}
            <input type="file" accept="image/jpeg,image/png" style={{ display: "none" }} onChange={(e) => setPhoto(e.target.files?.[0])} />
          </label>
          <input placeholder="Catatan singkat (opsional)" value={caption} onChange={(e) => setCaption(e.target.value)} style={input} />
          <button type="button" onClick={() => void send()} disabled={sending} style={primaryBtn}>
            <IconUpload size={14} /> {sending ? "Mengirim..." : "Kirim Momen"}
          </button>
        </div>
      </section>

      <section style={card}>
        <h2 style={sectionTitle}>Riwayat Momen ({moments.length})</h2>
        {moments.length ? (
          <div style={grid}>
            {moments.map((m) => (
              <div key={m.id} style={momentCard}>
                {m.photoViewUrl ? <img src={m.photoViewUrl} alt={m.caption || "Momen"} style={momentImg} /> : <div style={{ ...momentImg, display: "grid", placeItems: "center", color: "var(--color-warm-400)" }}><IconPhoto size={32} /></div>}
                <div style={{ padding: "10px 12px" }}>
                  <div style={{ display: "flex", alignItems: "center", gap: 6, flexWrap: "wrap" }}>
                    <span style={miniBadge}>{m.groupName || m.pilgrimName}</span>
                  </div>
                  {m.caption && <p style={{ margin: "6px 0 0", fontSize: 13 }}>{m.caption}</p>}
                  <p style={{ margin: "4px 0 0", fontSize: 11, color: "var(--color-warm-400)" }}>
                    {m.createdBy} · {m.createdAt?.toDate().toLocaleString("id-ID", { day: "2-digit", month: "short", hour: "2-digit", minute: "2-digit" })}
                  </p>
                  <button type="button" onClick={() => void remove(m)} style={{ ...iconBtnDanger, marginTop: 8 }}><IconTrash size={14} /> Hapus</button>
                </div>
              </div>
            ))}
          </div>
        ) : <p style={{ color: "var(--color-warm-400)", fontSize: 13, marginTop: 12 }}>Belum ada momen di musim ini.</p>}
      </section>
    </main>
  );
}

const page: React.CSSProperties = { maxWidth: 1000, margin: "0 auto", padding: "32px 24px", display: "grid", gap: 16 };
const eyebrow: React.CSSProperties = { margin: "0 0 6px", fontSize: 11, fontWeight: 700, color: "var(--color-gold-800)", letterSpacing: ".08em" };
const seasonSelect: React.CSSProperties = { minHeight: 40, border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 10px", font: "inherit", background: "#fff" };
const card: React.CSSProperties = { background: "var(--color-cream-200)", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 20 };
const sectionTitle: React.CSSProperties = { margin: 0, fontSize: 16 };
const input: React.CSSProperties = { minHeight: 40, border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 10px", font: "inherit", background: "#fff", width: "100%" };
const fileLabel: React.CSSProperties = { minHeight: 40, border: "1px dashed var(--color-cream-400)", borderRadius: 8, padding: "0 10px", display: "flex", alignItems: "center", gap: 8, fontSize: 13, color: "var(--color-warm-600)", background: "#fff", cursor: "pointer" };
const suggestBox: React.CSSProperties = { position: "absolute", top: "100%", left: 0, right: 0, marginTop: 4, background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 8, boxShadow: "0 4px 12px rgba(0,0,0,.08)", zIndex: 1, maxHeight: 200, overflowY: "auto" };
const suggestItem: React.CSSProperties = { display: "block", width: "100%", textAlign: "left", padding: "8px 10px", border: 0, background: "transparent", font: "inherit", cursor: "pointer" };
const tabBtn: React.CSSProperties = { minHeight: 34, border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 14px", background: "#fff", color: "var(--color-warm-600)", fontSize: 13, fontWeight: 600 };
const tabActive: React.CSSProperties = { ...tabBtn, background: "var(--color-emerald-900)", color: "#fff", borderColor: "var(--color-emerald-900)" };
const primaryBtn: React.CSSProperties = { minHeight: 40, border: 0, borderRadius: 8, padding: "0 16px", background: "var(--color-gold-500)", color: "#fff", fontWeight: 700, display: "inline-flex", alignItems: "center", justifyContent: "center", gap: 6, fontSize: 13, width: "fit-content" };
const grid: React.CSSProperties = { display: "grid", gridTemplateColumns: "repeat(auto-fill, minmax(220px, 1fr))", gap: 14, marginTop: 12 };
const momentCard: React.CSSProperties = { background: "#fff", border: "1px solid var(--color-cream-300)", borderRadius: 10, overflow: "hidden" };
const momentImg: React.CSSProperties = { display: "block", width: "100%", height: 160, objectFit: "cover", background: "var(--color-cream-100)" };
const miniBadge: React.CSSProperties = { padding: "2px 6px", borderRadius: 99, background: "var(--color-cream-200)", color: "var(--color-warm-500)", fontSize: 10, fontWeight: 700 };
const iconBtnDanger: React.CSSProperties = { border: "1px solid var(--color-cream-400)", borderRadius: 6, background: "#fff", color: "var(--color-danger-600)", fontSize: 11, padding: "4px 8px", display: "inline-flex", alignItems: "center", gap: 4 };
