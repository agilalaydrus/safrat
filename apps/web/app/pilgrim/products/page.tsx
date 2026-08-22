"use client";

import { useEffect, useState } from "react";
import { useSearchParams } from "next/navigation";
import { IconChevronDown, IconChevronUp, IconWifiOff } from "@tabler/icons-react";
import { Product } from "@hajj-saas/proto-gen/hajj/v1/product_pb";
import { pilgrimAppClient, orderClient } from "@/lib/rpc";
import { cachedFetch } from "@/lib/offline";
import { usePilgrimCode } from "@/lib/pilgrim-context";

const money = (n: bigint) => new Intl.NumberFormat("id-ID", { style: "currency", currency: "IDR", maximumFractionDigits: 0 }).format(Number(n));

export default function PilgrimProductsPage() {
  const code = usePilgrimCode();
  const searchParams = useSearchParams();
  const [products, setProducts] = useState<Product[]>([]);
  const [fromCache, setFromCache] = useState(false);
  const [loaded, setLoaded] = useState(false);
  const [buying, setBuying] = useState<string | null>(null);
  const [error, setError] = useState("");
  const [expanded, setExpanded] = useState<string>("");

  useEffect(() => {
    if (!code) return;
    cachedFetch(`pilgrim-products:${code}`, () => pilgrimAppClient.listMyProducts({ appAccessCode: code })).then((result) => {
      if (result.data) setProducts(result.data.products);
      setFromCache(result.fromCache);
      setLoaded(true);
    });
  }, [code]);

  const orderStatus = searchParams.get("order");

  async function buy(product: Product) {
    if (!code) return;
    setBuying(product.id);
    setError("");
    try {
      const result = await orderClient.createOrder({ appAccessCode: code, productId: product.id, quantity: 1 });
      if (result.checkoutUrl) {
        window.location.href = result.checkoutUrl;
        return;
      }
      setError("Gagal membuat tautan pembayaran. Silakan coba lagi.");
    } catch (caught) {
      const message = caught instanceof Error ? caught.message : "";
      setError(/failed_precondition/i.test(message) ? "Pembayaran belum tersedia untuk operator ini. Hubungi petugas Anda." : "Gagal memproses pesanan. Silakan coba lagi.");
    } finally {
      setBuying(null);
    }
  }

  return (
    <main style={page}>
      <p style={eyebrow}>PRODUK TERSEDIA</p>
      <h1 style={title}>Produk</h1>
      {orderStatus === "success" && <p style={successBanner}>Pembayaran berhasil! Produk Anda akan segera diproses.</p>}
      {orderStatus === "failed" && <p style={{ ...offlineBanner, background: "#ffe4e6", color: "var(--color-danger-600)" }}>Pembayaran tidak berhasil. Silakan coba lagi.</p>}
      {fromCache && <p style={offlineBanner}><IconWifiOff size={16} />Menampilkan produk tersimpan — Anda sedang offline</p>}
      {error && <p role="alert" style={{ color: "var(--color-danger-600)", fontSize: 13 }}>{error}</p>}
      {loaded && !products.length && <p style={{ color: "var(--color-warm-400)" }}>Belum ada produk tersedia untuk musim Anda.</p>}
      <div style={list}>
        {products.map((product) => (
          <article key={product.id} style={card}>
            <div style={row}><h2 style={{ margin: 0, fontSize: 18 }}>{product.name}</h2><span style={badge}>{product.type}</span></div>
            {product.description && <p style={desc}>{product.description}</p>}
            {product.inclusions.length > 0 && <ul style={inclusions}>{product.inclusions.map((item) => <li key={item}>{item}</li>)}</ul>}
            <p style={price}>{money(product.priceIdr)}{product.durationDays > 0 && <span style={duration}> · {product.durationDays} hari</span>}</p>
            {product.itineraryDays.length > 0 && (
              <>
                <button type="button" onClick={() => setExpanded(expanded === product.id ? "" : product.id)} style={itineraryToggle}>
                  {expanded === product.id ? <IconChevronUp size={14} /> : <IconChevronDown size={14} />}
                  Lihat Itinerary ({product.itineraryDays.length} hari)
                </button>
                {expanded === product.id && (
                  <div style={itineraryList}>
                    {product.itineraryDays.map((day) => (
                      <div key={day.dayNumber} style={itineraryDay}>
                        <span style={itineraryDayBadge}>Hari {day.dayNumber}</span>
                        <div style={{ flex: 1, minWidth: 0 }}>
                          <strong style={{ fontSize: 13 }}>{day.title}{day.city && ` — ${day.city}`}</strong>
                          {day.activities && <p style={{ margin: "2px 0 0", fontSize: 12, color: "var(--color-warm-500)" }}>{day.activities}</p>}
                          {(day.mealBreakfast || day.mealLunch || day.mealDinner) && (
                            <p style={{ margin: "2px 0 0", fontSize: 11, color: "var(--color-warm-400)" }}>
                              {[day.mealBreakfast && "Sarapan", day.mealLunch && "Makan Siang", day.mealDinner && "Makan Malam"].filter(Boolean).join(" · ")}
                            </p>
                          )}
                        </div>
                      </div>
                    ))}
                  </div>
                )}
              </>
            )}
            <button onClick={() => void buy(product)} disabled={buying === product.id || fromCache} style={buyButton}>{buying === product.id ? "Memproses..." : "Beli"}</button>
          </article>
        ))}
      </div>
    </main>
  );
}

const page: React.CSSProperties = { maxWidth: 480, margin: "0 auto", padding: "28px 20px" };
const eyebrow: React.CSSProperties = { color: "var(--color-gold-800)", fontSize: 11, fontWeight: 700, letterSpacing: ".08em", margin: "0 0 6px" };
const title: React.CSSProperties = { fontSize: 30, margin: "0 0 16px" };
const offlineBanner: React.CSSProperties = { display: "flex", alignItems: "center", gap: 6, background: "var(--color-gold-50)", color: "var(--color-gold-800)", padding: "8px 12px", borderRadius: 8, fontSize: 12, marginBottom: 16 };
const successBanner: React.CSSProperties = { background: "var(--color-emerald-50)", color: "var(--color-emerald-900)", padding: "10px 12px", borderRadius: 8, fontSize: 13, marginBottom: 16, fontWeight: 600 };
const list: React.CSSProperties = { display: "grid", gap: 12 };
const card: React.CSSProperties = { background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 16 };
const row: React.CSSProperties = { display: "flex", justifyContent: "space-between", alignItems: "center", gap: 8 };
const badge: React.CSSProperties = { padding: "3px 8px", borderRadius: 99, background: "var(--color-gold-50)", color: "var(--color-gold-800)", fontSize: 11, fontWeight: 700 };
const desc: React.CSSProperties = { margin: "8px 0 0", color: "var(--color-warm-500)", fontSize: 13 };
const inclusions: React.CSSProperties = { margin: "8px 0 0", paddingInlineStart: 18, color: "var(--color-warm-700)", fontSize: 13 };
const price: React.CSSProperties = { margin: "10px 0 0", fontWeight: 700, color: "var(--color-emerald-900)", fontSize: 16 };
const duration: React.CSSProperties = { fontWeight: 400, color: "var(--color-warm-400)", fontSize: 13 };
const buyButton: React.CSSProperties = { marginTop: 12, width: "100%", minHeight: 44, border: "none", borderRadius: 8, background: "var(--color-gold-500)", color: "#fff", fontWeight: 700, fontSize: 14, cursor: "pointer", fontFamily: "'Plus Jakarta Sans',sans-serif" };
const itineraryToggle: React.CSSProperties = { marginTop: 10, display: "flex", alignItems: "center", gap: 4, border: 0, background: "transparent", color: "var(--color-emerald-800)", fontSize: 12, fontWeight: 600, padding: 0 };
const itineraryList: React.CSSProperties = { marginTop: 8, display: "grid", gap: 8 };
const itineraryDay: React.CSSProperties = { display: "flex", gap: 8, alignItems: "flex-start", background: "var(--color-cream-100)", border: "1px solid var(--color-cream-300)", borderRadius: 8, padding: "8px 10px" };
const itineraryDayBadge: React.CSSProperties = { flexShrink: 0, padding: "2px 8px", borderRadius: 99, background: "var(--color-emerald-50)", color: "var(--color-emerald-900)", fontSize: 10, fontWeight: 700, whiteSpace: "nowrap" };
