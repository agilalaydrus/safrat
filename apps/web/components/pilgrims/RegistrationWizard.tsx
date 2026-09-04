"use client";

import { useEffect, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { Timestamp } from "@bufbuild/protobuf";
import { IconCheck, IconChevronRight } from "@tabler/icons-react";
import { Gender, Pilgrim } from "@hajj-saas/proto-gen/hajj/v1/pilgrim_pb";
import { Product, ProductRoomTier } from "@hajj-saas/proto-gen/hajj/v1/product_pb";
import { ManualOrderPaymentMethod, Order } from "@hajj-saas/proto-gen/hajj/v1/order_pb";
import type { Group } from "@hajj-saas/proto-gen/hajj/v1/group_pb";
import type { Agent } from "@hajj-saas/proto-gen/hajj/v1/agent_pb";
import { agentClient, groupClient, orderClient, pilgrimClient, productClient, seasonClient } from "@/lib/rpc";
import { NATIONALITIES } from "./PilgrimFormDialog";
import PilgrimDocumentChecklist from "./PilgrimDocumentChecklist";

const rupiah = (n: bigint | number) => new Intl.NumberFormat("id-ID", { style: "currency", currency: "IDR", maximumFractionDigits: 0 }).format(Number(n));
const PAYMENT_LABEL: Record<number, string> = {
  [ManualOrderPaymentMethod.XENDIT_LINK]: "Tautan Xendit (bayar online)",
  [ManualOrderPaymentMethod.CASH]: "Tunai — sudah diterima",
  [ManualOrderPaymentMethod.BANK_TRANSFER]: "Transfer bank — sudah diterima",
};
const nextKey = () => globalThis.crypto?.randomUUID?.() ?? `${Date.now()}-${Math.random().toString(16).slice(2)}`;

type IdentityForm = {
  fullName: string; passportNumber: string; nationality: string; dateOfBirth: string;
  gender: "" | "MALE" | "FEMALE"; phone: string; email: string; groupId: string; agentId: string;
};
const EMPTY_IDENTITY: IdentityForm = { fullName: "", passportNumber: "", nationality: "ID", dateOfBirth: "", gender: "", phone: "", email: "", groupId: "", agentId: "" };

const STEP_LABEL = ["Data Diri", "Paket & Kamar", "Pembayaran", "Konfirmasi"];

export default function RegistrationWizard() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const [step, setStep] = useState(1);
  const [seasons, setSeasons] = useState<{ id: string; name: string; isActive: boolean }[]>([]);
  const [seasonId, setSeasonId] = useState("");
  const [groups, setGroups] = useState<Group[]>([]);
  const [agents, setAgents] = useState<Agent[]>([]);
  const [notice, setNotice] = useState("");

  // Step 1
  const [identity, setIdentity] = useState<IdentityForm>(EMPTY_IDENTITY);
  const [pilgrim, setPilgrim] = useState<Pilgrim>();
  const [saving, setSaving] = useState(false);

  // Step 2
  const [products, setProducts] = useState<Product[]>([]);
  const [productId, setProductId] = useState("");
  const [roomTiers, setRoomTiers] = useState<ProductRoomTier[]>([]);
  const [roomTier, setRoomTier] = useState("");

  // Step 3
  const [paymentMethod, setPaymentMethod] = useState<ManualOrderPaymentMethod>(ManualOrderPaymentMethod.CASH);
  const [order, setOrder] = useState<Order>();
  const [orderKey] = useState(nextKey());

  useEffect(() => {
    const fromQuery = searchParams.get("seasonId") ?? "";
    seasonClient.listSeasons({}).then((r) => {
      setSeasons(r.seasons);
      setSeasonId(fromQuery || r.seasons.find((s) => s.isActive)?.id || r.seasons[0]?.id || "");
    }).catch(() => setNotice("Gagal memuat musim."));
    agentClient.listAgents({}).then((r) => setAgents(r.agents)).catch(() => {});
  }, []);

  useEffect(() => {
    if (!seasonId) return;
    groupClient.listGroups({ seasonId }).then((r) => setGroups(r.groups)).catch(() => {});
    productClient.listProducts({ seasonId }).then((r) => setProducts(r.products.filter((p) => p.isActive))).catch(() => {});
  }, [seasonId]);

  useEffect(() => {
    if (!productId) { setRoomTiers([]); setRoomTier(""); return; }
    productClient.listProductRoomTiers({ productId }).then((r) => setRoomTiers(r.tiers.filter((t) => t.isActive))).catch(() => setRoomTiers([]));
  }, [productId]);

  const selectedProduct = products.find((p) => p.id === productId);
  const selectedTier = roomTiers.find((t) => t.tier === roomTier);
  const totalPreview = selectedTier ? selectedTier.priceIdr : (selectedProduct?.priceIdr ?? 0n);

  async function submitIdentity() {
    setNotice("");
    if (identity.fullName.trim().length < 2) { setNotice("Nama lengkap wajib diisi, sesuai KTP & paspor."); return; }
    if (identity.passportNumber.trim().length < 5) { setNotice("Nomor paspor wajib diisi (minimal 5 karakter)."); return; }
    if (identity.nationality.length !== 2) { setNotice("Kewarganegaraan wajib dipilih."); return; }
    if (!identity.dateOfBirth) { setNotice("Tanggal lahir wajib diisi."); return; }
    if (!identity.gender) { setNotice("Jenis kelamin wajib dipilih."); return; }
    setSaving(true);
    try {
      const payload = {
        seasonId, groupId: identity.groupId, fullName: identity.fullName.trim(),
        passportNumber: identity.passportNumber.toUpperCase().trim(), nationality: identity.nationality,
        dateOfBirth: Timestamp.fromDate(new Date(`${identity.dateOfBirth}T00:00:00Z`)),
        gender: identity.gender === "FEMALE" ? Gender.FEMALE : Gender.MALE,
        phone: identity.phone.trim(), email: identity.email.trim().toLowerCase(), agentId: identity.agentId,
      };
      const saved = pilgrim ? await pilgrimClient.updatePilgrim({ ...payload, pilgrimId: pilgrim.id }) : await pilgrimClient.createPilgrim(payload);
      setPilgrim(saved);
      setStep(2);
    } catch (caught) {
      setNotice(caught instanceof Error ? caught.message : "Gagal menyimpan data diri.");
    } finally {
      setSaving(false);
    }
  }

  function proceedToPayment() {
    setNotice("");
    if (!productId) { setNotice("Pilih paket terlebih dahulu."); return; }
    if (roomTiers.length > 0 && !roomTier) { setNotice("Pilih tipe kamar terlebih dahulu."); return; }
    setStep(3);
  }

  async function confirmPayment() {
    setNotice("");
    if (!pilgrim) return;
    // Cross-step validation: a Xendit link or a payment reminder both need a
    // number to reach the pilgrim on, and that only lives on step 1.
    if (!pilgrim.phone.trim()) { setNotice("Isi nomor WhatsApp pada langkah Data Diri sebelum melanjutkan."); return; }
    setSaving(true);
    try {
      const response = await orderClient.createManualOrder({
        pilgrimId: pilgrim.id, productId, quantity: 1, paymentMethod, roomTier, idempotencyKey: orderKey,
      });
      setOrder(response.order);
      setStep(4);
    } catch (caught) {
      setNotice(caught instanceof Error ? caught.message : "Gagal membuat pesanan.");
    } finally {
      setSaving(false);
    }
  }

  return (
    <main style={page}>
      <header>
        <p style={eyebrow}>PENDAFTARAN TERPANDU</p>
        <h1 style={{ margin: 0, fontSize: 32 }}>Jamaah Baru</h1>
        <p style={{ margin: "4px 0 0", color: "var(--color-warm-500)" }}>Empat langkah: identitas, paket, pembayaran, konfirmasi.</p>
      </header>

      {step === 1 && (
        <div style={{ marginTop: 12 }}>
          <select value={seasonId} onChange={(e) => setSeasonId(e.target.value)} style={input} disabled={!!pilgrim}>
            {seasons.map((s) => <option key={s.id} value={s.id}>{s.name}{s.isActive ? " (aktif)" : ""}</option>)}
          </select>
        </div>
      )}

      <ol style={stepper}>
        {STEP_LABEL.map((label, i) => (
          <li key={label} style={{ ...stepItem, ...(step === i + 1 ? stepItemActive : {}), ...(step > i + 1 ? stepItemDone : {}) }}>
            <span style={stepBadge}>{step > i + 1 ? <IconCheck size={13} /> : i + 1}</span> {label}
          </li>
        ))}
      </ol>

      {notice && <p style={{ color: "var(--color-danger-600)" }}>{notice}</p>}
      <div className="gold-divider" />

      {step === 1 && (
        <section style={card}>
          <h2 style={sectionTitle}>Data Diri</h2>
          <p style={{ margin: "2px 0 12px", fontSize: 12, color: "var(--color-warm-500)" }}>Tulis persis seperti yang tertera di KTP &amp; paspor.</p>
          <div style={grid2}>
            <label style={label1}><span>Nama Lengkap</span><input style={input} value={identity.fullName} onChange={(e) => setIdentity({ ...identity, fullName: e.target.value })} /></label>
            <label style={label1}><span>Nomor Paspor</span><input style={input} value={identity.passportNumber} onChange={(e) => setIdentity({ ...identity, passportNumber: e.target.value })} /></label>
          </div>
          <div style={grid2}>
            <label style={label1}>
              <span>Kewarganegaraan</span>
              <select style={input} value={identity.nationality} onChange={(e) => setIdentity({ ...identity, nationality: e.target.value })}>
                {NATIONALITIES.map(([code, name]) => <option key={code} value={code}>{name}</option>)}
              </select>
            </label>
            <label style={label1}><span>Tanggal Lahir</span><input type="date" style={input} value={identity.dateOfBirth} onChange={(e) => setIdentity({ ...identity, dateOfBirth: e.target.value })} /></label>
          </div>
          <div style={grid2}>
            <label style={label1}>
              <span>Jenis Kelamin</span>
              <select style={input} value={identity.gender} onChange={(e) => setIdentity({ ...identity, gender: e.target.value as IdentityForm["gender"] })}>
                <option value="">— pilih —</option>
                <option value="MALE">Laki-laki</option>
                <option value="FEMALE">Perempuan</option>
              </select>
            </label>
            <label style={label1}><span>Nomor WhatsApp</span><input style={input} value={identity.phone} onChange={(e) => setIdentity({ ...identity, phone: e.target.value })} placeholder="08xxxxxxxxxx" /></label>
          </div>
          <div style={grid2}>
            <label style={label1}><span>Email (opsional)</span><input type="email" style={input} value={identity.email} onChange={(e) => setIdentity({ ...identity, email: e.target.value })} /></label>
            <label style={label1}>
              <span>Grup (opsional)</span>
              <select style={input} value={identity.groupId} onChange={(e) => setIdentity({ ...identity, groupId: e.target.value })}>
                <option value="">— belum ditentukan —</option>
                {groups.map((g) => <option key={g.id} value={g.id}>{g.name}</option>)}
              </select>
            </label>
          </div>
          {agents.length > 0 && (
            <label style={label1}>
              <span>Agen/Perujuk (opsional)</span>
              <select style={input} value={identity.agentId} onChange={(e) => setIdentity({ ...identity, agentId: e.target.value })}>
                <option value="">— tanpa agen —</option>
                {agents.map((a) => <option key={a.id} value={a.id}>{a.name}</option>)}
              </select>
              <small style={{ color: "var(--color-warm-400)" }}>Hanya bisa diisi di sini — komisi agen dihitung dari data ini saat pesanan dibuat.</small>
            </label>
          )}
          <button type="button" onClick={() => void submitIdentity()} disabled={saving} style={primaryBtn}>
            {saving ? "Menyimpan..." : "Lanjut ke Paket & Kamar"} <IconChevronRight size={14} />
          </button>
        </section>
      )}

      {step === 2 && pilgrim && (
        <section style={card}>
          <h2 style={sectionTitle}>Paket &amp; Kamar untuk {pilgrim.fullName}</h2>
          <label style={label1}>
            <span>Paket</span>
            <select style={input} value={productId} onChange={(e) => setProductId(e.target.value)}>
              <option value="">— pilih paket —</option>
              {products.map((p) => <option key={p.id} value={p.id}>{p.name} · {rupiah(p.priceIdr)}</option>)}
            </select>
          </label>
          {roomTiers.length > 0 && (
            <label style={label1}>
              <span>Tipe Kamar</span>
              <select style={input} value={roomTier} onChange={(e) => setRoomTier(e.target.value)}>
                <option value="">— pilih tipe kamar —</option>
                {roomTiers.map((t) => <option key={t.tier} value={t.tier} disabled={t.seatQuota !== undefined && t.seatsTaken >= t.seatQuota}>
                  {t.tier} · {rupiah(t.priceIdr)}{t.seatQuota !== undefined ? ` (${t.seatQuota - t.seatsTaken} tersisa)` : ""}
                </option>)}
              </select>
            </label>
          )}
          {selectedProduct && (
            <div style={priceBox}><span>Total harga paket</span><strong>{rupiah(totalPreview)}</strong></div>
          )}
          <div style={{ display: "flex", gap: 8 }}>
            <button type="button" onClick={() => setStep(1)} style={ghostBtn}>Kembali</button>
            <button type="button" onClick={proceedToPayment} style={primaryBtn}>Lanjut ke Pembayaran <IconChevronRight size={14} /></button>
          </div>
        </section>
      )}

      {step === 3 && pilgrim && selectedProduct && (
        <section style={card}>
          <h2 style={sectionTitle}>Pembayaran</h2>
          <div style={priceBox}><span>{selectedProduct.name}{roomTier ? ` · ${roomTier}` : ""}</span><strong>{rupiah(totalPreview)}</strong></div>
          <label style={label1}>
            <span>Metode</span>
            <select style={input} value={paymentMethod} onChange={(e) => setPaymentMethod(Number(e.target.value) as ManualOrderPaymentMethod)}>
              {Object.entries(PAYMENT_LABEL).map(([value, label]) => <option key={value} value={value}>{label}</option>)}
            </select>
          </label>
          {paymentMethod !== ManualOrderPaymentMethod.XENDIT_LINK && (
            <p style={{ fontSize: 12, color: "var(--color-warm-500)" }}>Pesanan akan langsung tercatat lunas — pastikan dana benar-benar sudah diterima.</p>
          )}
          <p style={{ fontSize: 12, color: "var(--color-warm-400)" }}>
            Untuk cicilan bertahap, buat rencana pembayaran di halaman Arus Kas setelah pesanan ini dibuat — jadwal dan simulasinya dihitung dari sana.
          </p>
          <div style={{ display: "flex", gap: 8 }}>
            <button type="button" onClick={() => setStep(2)} style={ghostBtn}>Kembali</button>
            <button type="button" onClick={() => void confirmPayment()} disabled={saving} style={primaryBtn}>
              {saving ? "Memproses..." : "Buat Pesanan"} <IconChevronRight size={14} />
            </button>
          </div>
        </section>
      )}

      {step === 4 && pilgrim && order && (
        <>
          <section style={card}>
            <h2 style={sectionTitle}>Pesanan Dibuat</h2>
            <div style={priceBox}><span>{order.productName}</span><strong>{rupiah(order.totalPriceIdr)}</strong></div>
            <p style={{ fontSize: 13 }}>Status: <strong>{order.status === "PAID" ? "Lunas" : order.status === "PENDING" ? "Menunggu pembayaran" : order.status}</strong></p>
            {order.checkoutUrl && (
              <p style={{ fontSize: 13 }}>Tautan pembayaran: <a href={order.checkoutUrl} target="_blank" rel="noreferrer">{order.checkoutUrl}</a></p>
            )}
          </section>
          <section style={card}>
            <h2 style={sectionTitle}>Kelengkapan Berkas</h2>
            <PilgrimDocumentChecklist pilgrim={pilgrim} onUpdated={setPilgrim} />
          </section>
          <button type="button" onClick={() => router.push(`/dashboard/pilgrims/${pilgrim.id}`)} style={primaryBtn}>Selesai — Buka Profil Jamaah</button>
        </>
      )}
    </main>
  );
}

const page: React.CSSProperties = { maxWidth: 760, margin: "0 auto", padding: "32px 24px", display: "grid", gap: 16 };
const eyebrow: React.CSSProperties = { margin: "0 0 6px", fontSize: 11, fontWeight: 700, color: "var(--color-gold-800)", letterSpacing: ".08em" };
const stepper: React.CSSProperties = { display: "flex", gap: 6, listStyle: "none", padding: 0, margin: "16px 0 0", flexWrap: "wrap" };
const stepItem: React.CSSProperties = { display: "flex", alignItems: "center", gap: 6, fontSize: 12, fontWeight: 600, color: "var(--color-warm-400)", padding: "6px 10px", borderRadius: 999, border: "1px solid var(--color-cream-400)", background: "#fff" };
const stepItemActive: React.CSSProperties = { color: "var(--color-emerald-900)", border: "1px solid var(--color-emerald-900)" };
const stepItemDone: React.CSSProperties = { color: "var(--color-emerald-800)" };
const stepBadge: React.CSSProperties = { width: 18, height: 18, borderRadius: "50%", background: "var(--color-cream-200)", display: "grid", placeItems: "center", fontSize: 10 };
const card: React.CSSProperties = { background: "var(--color-cream-200)", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 20, display: "grid", gap: 12 };
const sectionTitle: React.CSSProperties = { margin: 0, fontSize: 16 };
const grid2: React.CSSProperties = { display: "grid", gridTemplateColumns: "1fr 1fr", gap: 12 };
const label1: React.CSSProperties = { display: "flex", flexDirection: "column", gap: 6, fontSize: 13, fontWeight: 600, color: "var(--color-warm-700)" };
const input: React.CSSProperties = { minHeight: 40, border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 10px", font: "inherit", background: "#fff", width: "100%" };
const primaryBtn: React.CSSProperties = { minHeight: 40, border: 0, borderRadius: 8, padding: "0 16px", background: "var(--color-gold-500)", color: "#fff", fontWeight: 700, display: "inline-flex", alignItems: "center", justifyContent: "center", gap: 6, fontSize: 13, width: "fit-content" };
const ghostBtn: React.CSSProperties = { minHeight: 40, border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 16px", background: "transparent", color: "var(--color-emerald-900)", fontSize: 13, fontWeight: 600 };
const priceBox: React.CSSProperties = { display: "flex", justifyContent: "space-between", alignItems: "center", background: "#fff", border: "1px solid var(--color-cream-300)", borderRadius: 8, padding: "10px 14px", fontSize: 14 };
