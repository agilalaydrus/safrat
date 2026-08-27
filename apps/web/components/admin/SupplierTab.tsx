"use client";

import { useCallback, useEffect, useState } from "react";
import { IconPlus, IconFlask, IconAlertTriangle } from "@tabler/icons-react";
import { PlatformSupplier, SupplierResponseRule } from "@hajj-saas/proto-gen/hajj/v1/platform_pb";
import { platformClient } from "@/lib/rpc";

export default function SupplierTab() {
  const [suppliers, setSuppliers] = useState<PlatformSupplier[]>([]);
  const [selected, setSelected] = useState<PlatformSupplier | null>(null);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const [creating, setCreating] = useState(false);
  const [draft, setDraft] = useState({ name: "", code: "", baseUrl: "", credentialEnvVar: "", status: "INACTIVE", notes: "" });

  const load = useCallback(() => {
    setLoading(true);
    platformClient.listSuppliers({}).then((r) => setSuppliers(r.suppliers))
      .catch(() => setError("Gagal memuat supplier.")).finally(() => setLoading(false));
  }, []);
  useEffect(() => { load(); }, [load]);

  if (selected) return <SupplierRules supplier={selected} onBack={() => { setSelected(null); load(); }} />;

  const save = async () => {
    setError("");
    try {
      await platformClient.saveSupplier(draft);
      setCreating(false);
      setDraft({ name: "", code: "", baseUrl: "", credentialEnvVar: "", status: "INACTIVE", notes: "" });
      load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Gagal menyimpan supplier.");
    }
  };

  return (
    <section style={{ display: "grid", gap: 16 }}>
      <p style={muted}>
        Produk digital dipasok oleh TawafiqHub. Kredensial supplier tidak pernah disimpan di sini —
        yang dicatat hanya nama variabel environment tempat kredensialnya dibaca.
      </p>
      {error && <p style={{ color: "var(--color-danger-600)", margin: 0 }}>{error}</p>}

      {!creating ? (
        <button style={primary} onClick={() => setCreating(true)}><IconPlus size={17} />Tambah Supplier</button>
      ) : (
        <div style={card}>
          <div style={grid}>
            <Field label="Nama" value={draft.name} onChange={(v) => setDraft({ ...draft, name: v })} />
            <Field label="Kode (huruf kecil, angka, -)" value={draft.code} onChange={(v) => setDraft({ ...draft, code: v })} />
            <Field label="Base URL" value={draft.baseUrl} onChange={(v) => setDraft({ ...draft, baseUrl: v })} />
            <Field label="Nama variabel kredensial" value={draft.credentialEnvVar} onChange={(v) => setDraft({ ...draft, credentialEnvVar: v })} />
            <label style={label}>Status
              <select value={draft.status} onChange={(e) => setDraft({ ...draft, status: e.target.value })} style={input}>
                {["ACTIVE", "INACTIVE", "SUSPENDED"].map((s) => <option key={s} value={s}>{s}</option>)}
              </select>
            </label>
          </div>
          <div style={{ display: "flex", gap: 10 }}>
            <button style={ghost} onClick={() => setCreating(false)}>Batal</button>
            <button style={primary} onClick={save}>Simpan</button>
          </div>
        </div>
      )}

      {loading ? <p style={muted}>Memuat...</p> : suppliers.length === 0 ? (
        <p style={muted}>Belum ada supplier terdaftar.</p>
      ) : (
        <div style={{ overflowX: "auto" }}>
          <table style={table}>
            <thead><tr>{["Supplier", "Kode", "Status", "Routing", "Aturan", ""].map((h) => <th key={h} style={th}>{h}</th>)}</tr></thead>
            <tbody>
              {suppliers.map((supplier) => (
                <tr key={supplier.id} style={tr}>
                  <td style={td}><strong>{supplier.name}</strong>
                    {supplier.baseUrl && <small style={{ display: "block", color: "var(--color-warm-400)" }}>{supplier.baseUrl}</small>}
                  </td>
                  <td style={td}>{supplier.code}</td>
                  <td style={td}>{supplier.status}</td>
                  <td style={td}>{supplier.routeCount}</td>
                  <td style={td}>
                    {/* An active supplier with no rules reads every answer as
                        unrecognised. Worth flagging before that happens. */}
                    {supplier.ruleCount === 0 && supplier.status === "ACTIVE"
                      ? <span style={warn}><IconAlertTriangle size={13} />belum ada</span>
                      : supplier.ruleCount}
                  </td>
                  <td style={td}><button style={ghost} onClick={() => setSelected(supplier)}>Aturan Baca</button></td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </section>
  );
}

function SupplierRules({ supplier, onBack }: { supplier: PlatformSupplier; onBack: () => void }) {
  const [rules, setRules] = useState<SupplierResponseRule[]>([]);
  const [sample, setSample] = useState("");
  const [reading, setReading] = useState<{ outcome: string; reference: string; cost: bigint; costReported: boolean; skipped: string[] } | null>(null);
  const [draft, setDraft] = useState({ priority: 10, pattern: "", outcome: "SUCCESS", referenceGroup: "", costGroup: "", description: "" });
  const [error, setError] = useState("");

  const load = useCallback(() => {
    platformClient.listResponseRules({ supplierId: supplier.id })
      .then((r) => setRules(r.rules)).catch(() => setError("Gagal memuat aturan."));
  }, [supplier.id]);
  useEffect(() => { load(); }, [load]);

  const create = async () => {
    setError("");
    try {
      await platformClient.createResponseRule({ supplierId: supplier.id, ...draft });
      setDraft({ priority: 10, pattern: "", outcome: "SUCCESS", referenceGroup: "", costGroup: "", description: "" });
      load();
    } catch (err) {
      // The server compiles the pattern before storing it, so this message is
      // the actual reason it will not work — shown verbatim.
      setError(err instanceof Error ? err.message : "Pola ditolak.");
    }
  };

  const test = async () => {
    setError("");
    try {
      const result = await platformClient.testResponseRules({ supplierId: supplier.id, sampleResponse: sample });
      setReading({
        outcome: result.outcome, reference: result.reference, cost: result.costIdr,
        costReported: result.costReported, skipped: result.skippedRules.map((s) => s.reason),
      });
    } catch (err) {
      setError(err instanceof Error ? err.message : "Gagal menguji.");
    }
  };

  return (
    <section style={{ display: "grid", gap: 16 }}>
      <button style={ghost} onClick={onBack}>← Kembali ke daftar supplier</button>
      <h3 style={{ margin: 0, fontSize: 18, fontWeight: 500 }}>Aturan Baca — {supplier.name}</h3>
      <p style={muted}>
        Respons supplier dibaca dengan pola berurutan; yang cocok pertama menentukan hasilnya.
        Pola yang tidak bisa dikompilasi ditolak di sini, bukan ditemukan saat transaksi berjalan.
      </p>
      {error && <p style={{ color: "var(--color-danger-600)", margin: 0 }}>{error}</p>}

      <div style={card}>
        <strong style={{ fontSize: 14 }}>Uji sebuah respons</strong>
        <textarea value={sample} onChange={(e) => setSample(e.target.value)} rows={3}
          placeholder={'{"status":"OK","sn":"SN-123","harga":9500}'} style={{ ...input, minHeight: 70, padding: 10 }} />
        <button style={ghost} onClick={test} disabled={!sample.trim()}><IconFlask size={16} />Uji</button>
        {reading && (
          <div style={{ display: "grid", gap: 6, fontSize: 13 }}>
            <span>Hasil: <strong>{reading.outcome}</strong></span>
            {reading.reference && <span>Referensi: <strong>{reading.reference}</strong></span>}
            <span>Harga modal: <strong>{reading.costReported ? `Rp${Number(reading.cost).toLocaleString("id-ID")}` : "tidak dilaporkan"}</strong></span>
            {reading.outcome === "UNMATCHED" && (
              <span style={{ color: "#b45309" }}>
                Tidak ada aturan yang mengenali respons ini — transaksinya akan menunggu peninjauan manusia, bukan dianggap gagal.
              </span>
            )}
            {reading.skipped.length > 0 && (
              <span style={{ color: "var(--color-danger-600)" }}>
                {reading.skipped.length} aturan tidak bisa dipakai: {reading.skipped.join("; ")}
              </span>
            )}
          </div>
        )}
      </div>

      <div style={card}>
        <strong style={{ fontSize: 14 }}>Tambah aturan</strong>
        <div style={grid}>
          <label style={label}>Prioritas (kecil = lebih dulu)
            <input type="number" value={draft.priority} onChange={(e) => setDraft({ ...draft, priority: Number(e.target.value) })} style={input} />
          </label>
          <label style={label}>Hasil
            <select value={draft.outcome} onChange={(e) => setDraft({ ...draft, outcome: e.target.value })} style={input}>
              {["SUCCESS", "FAILED", "PENDING"].map((o) => <option key={o} value={o}>{o}</option>)}
            </select>
          </label>
          <Field label="Grup referensi" value={draft.referenceGroup} onChange={(v) => setDraft({ ...draft, referenceGroup: v })} />
          <Field label="Grup harga modal" value={draft.costGroup} onChange={(v) => setDraft({ ...draft, costGroup: v })} />
        </div>
        <label style={label}>Pola (RE2)
          <textarea value={draft.pattern} onChange={(e) => setDraft({ ...draft, pattern: e.target.value })} rows={2} style={{ ...input, minHeight: 60, padding: 10 }} />
        </label>
        <Field label="Keterangan" value={draft.description} onChange={(v) => setDraft({ ...draft, description: v })} />
        <button style={primary} onClick={create} disabled={!draft.pattern.trim()}>Simpan Aturan</button>
      </div>

      {rules.length > 0 && (
        <div style={{ overflowX: "auto" }}>
          <table style={table}>
            <thead><tr>{["Prio", "Hasil", "Pola", "Status", ""].map((h) => <th key={h} style={th}>{h}</th>)}</tr></thead>
            <tbody>
              {rules.map((rule) => (
                <tr key={rule.id} style={tr}>
                  <td style={td}>{rule.priority}</td>
                  <td style={td}>{rule.outcome}</td>
                  <td style={{ ...td, maxWidth: 320, whiteSpace: "normal", wordBreak: "break-all", fontSize: 12 }}>
                    {rule.pattern}
                    {rule.description && <small style={{ display: "block", color: "var(--color-warm-400)" }}>{rule.description}</small>}
                  </td>
                  <td style={td}>{rule.isActive ? "Aktif" : "Nonaktif"}</td>
                  <td style={td}>
                    {/* Deactivate, never edit: changing a pattern in place would
                        silently change how past logs should have been read. */}
                    <button style={ghost} onClick={async () => {
                      await platformClient.setResponseRuleActive({ ruleId: rule.id, isActive: !rule.isActive });
                      load();
                    }}>{rule.isActive ? "Nonaktifkan" : "Aktifkan"}</button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </section>
  );
}

function Field({ label: text, value, onChange }: { label: string; value: string; onChange: (v: string) => void }) {
  return <label style={label}>{text}<input value={value} onChange={(e) => onChange(e.target.value)} style={input} /></label>;
}

const muted: React.CSSProperties = { color: "var(--color-warm-500)", fontSize: 13, margin: 0 };
const card: React.CSSProperties = { display: "grid", gap: 12, padding: 18, background: "#fff", border: "1px solid var(--color-cream-400)", borderRadius: 10 };
const grid: React.CSSProperties = { display: "grid", gridTemplateColumns: "repeat(auto-fit,minmax(200px,1fr))", gap: 12 };
const label: React.CSSProperties = { display: "grid", gap: 6, fontSize: 13, color: "var(--color-warm-700)" };
const input: React.CSSProperties = { minHeight: 44, border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 12px", fontFamily: "inherit", fontSize: 14 };
const primary: React.CSSProperties = { minHeight: 44, border: 0, borderRadius: 8, padding: "0 18px", background: "var(--color-emerald-800)", color: "#fff", fontWeight: 700, display: "inline-flex", alignItems: "center", gap: 7, justifySelf: "start" };
const ghost: React.CSSProperties = { minHeight: 40, border: "1px solid var(--color-cream-400)", borderRadius: 8, padding: "0 14px", background: "transparent", color: "var(--color-warm-700)", display: "inline-flex", alignItems: "center", gap: 6, justifySelf: "start", fontSize: 13 };
const table: React.CSSProperties = { width: "100%", borderCollapse: "collapse", background: "#fff" };
const th: React.CSSProperties = { textAlign: "left", padding: 12, fontSize: 11, color: "var(--color-warm-400)", borderBottom: "1px solid var(--color-cream-300)" };
const tr: React.CSSProperties = { borderBottom: "1px solid var(--color-cream-300)" };
const td: React.CSSProperties = { padding: 12, color: "var(--color-warm-700)", verticalAlign: "top", fontSize: 13 };
const warn: React.CSSProperties = { display: "inline-flex", alignItems: "center", gap: 4, color: "#b45309", fontWeight: 700, fontSize: 12 };
