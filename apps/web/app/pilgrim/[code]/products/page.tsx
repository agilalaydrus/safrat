"use client";

import { useEffect, useState } from "react";
import { useParams } from "next/navigation";
import { IconWifiOff } from "@tabler/icons-react";
import { Product } from "@hajj-saas/proto-gen/hajj/v1/product_pb";
import { pilgrimAppClient } from "@/lib/rpc";
import { cachedFetch } from "@/lib/offline";

const money = (n: bigint) => new Intl.NumberFormat("id-ID", { style: "currency", currency: "IDR", maximumFractionDigits: 0 }).format(Number(n));

export default function PilgrimProductsPage() {
  const { code } = useParams<{ code: string }>();
  const [products, setProducts] = useState<Product[]>([]);
  const [fromCache, setFromCache] = useState(false);
  const [loaded, setLoaded] = useState(false);

  useEffect(() => {
    cachedFetch(`pilgrim-products:${code}`, () => pilgrimAppClient.listMyProducts({ appAccessCode: code })).then((result) => {
      if (result.data) setProducts(result.data.products);
      setFromCache(result.fromCache);
      setLoaded(true);
    });
  }, [code]);

  return (
    <main style={page}>
      <p style={eyebrow}>PRODUK TERSEDIA</p>
      <h1 style={title}>Produk</h1>
      {fromCache && <p style={offlineBanner}><IconWifiOff size={16} />Menampilkan produk tersimpan — Anda sedang offline</p>}
      {loaded && !products.length && <p style={{ color: "var(--color-warm-400)" }}>Belum ada produk tersedia untuk musim Anda.</p>}
      <div style={list}>
        {products.map((product) => (
          <article key={product.id} style={card}>
            <div style={row}><h2 style={{ margin: 0, fontSize: 18 }}>{product.name}</h2><span style={badge}>{product.type}</span></div>
            {product.description && <p style={desc}>{product.description}</p>}
            {product.inclusions.length > 0 && <ul style={inclusions}>{product.inclusions.map((item) => <li key={item}>{item}</li>)}</ul>}
            <p style={price}>{money(product.priceIdr)}{product.durationDays > 0 && <span style={duration}> · {product.durationDays} hari</span>}</p>
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
const list: React.CSSProperties = { display: "grid", gap: 12 };
const card: React.CSSProperties = { background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 16 };
const row: React.CSSProperties = { display: "flex", justifyContent: "space-between", alignItems: "center", gap: 8 };
const badge: React.CSSProperties = { padding: "3px 8px", borderRadius: 99, background: "var(--color-gold-50)", color: "var(--color-gold-800)", fontSize: 11, fontWeight: 700 };
const desc: React.CSSProperties = { margin: "8px 0 0", color: "var(--color-warm-500)", fontSize: 13 };
const inclusions: React.CSSProperties = { margin: "8px 0 0", paddingInlineStart: 18, color: "var(--color-warm-700)", fontSize: 13 };
const price: React.CSSProperties = { margin: "10px 0 0", fontWeight: 700, color: "var(--color-emerald-900)", fontSize: 16 };
const duration: React.CSSProperties = { fontWeight: 400, color: "var(--color-warm-400)", fontSize: 13 };
