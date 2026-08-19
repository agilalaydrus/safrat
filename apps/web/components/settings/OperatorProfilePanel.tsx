"use client";

import { useEffect, useState } from "react";
import { operatorClient } from "@/lib/rpc";

export default function OperatorProfilePanel() {
  const [name, setName] = useState("");
  const [country, setCountry] = useState("");
  const [email, setEmail] = useState("");
  const [licenseNumber, setLicenseNumber] = useState("");
  const [loaded, setLoaded] = useState(false);
  const [saving, setSaving] = useState(false);
  const [notice, setNotice] = useState("");

  useEffect(() => {
    operatorClient.getMyOperator({}).then((operator) => {
      setName(operator.name);
      setCountry(operator.country);
      setEmail(operator.email);
      setLicenseNumber(operator.licenseNumber);
      setLoaded(true);
    }).catch(() => setNotice("Gagal memuat profil operator."));
  }, []);

  const save = async () => {
    if (!name.trim()) { setNotice("Nama operator wajib diisi."); return; }
    if (country && country.trim().length !== 2) { setNotice("Kode negara harus 2 huruf, mis. ID."); return; }
    setSaving(true);
    setNotice("");
    try {
      await operatorClient.updateOperator({ name: name.trim(), country: country.trim().toUpperCase(), email: email.trim(), licenseNumber: licenseNumber.trim() });
      setNotice("Profil operator diperbarui.");
    } catch (error) {
      setNotice(`Gagal menyimpan: ${error instanceof Error ? error.message : "tidak diketahui"}`);
    } finally {
      setSaving(false);
    }
  };

  if (!loaded) return <p style={{ color: "var(--color-warm-500)" }}>Memuat...</p>;

  return <section style={card}>
    {notice && <p role="status" style={{ color: "var(--color-gold-800)" }}>{notice}</p>}
    <label style={field}>Nama Operator
      <input value={name} onChange={(e) => setName(e.target.value)} style={input} />
    </label>
    <label style={field}>Kode Negara (2 huruf)
      <input value={country} onChange={(e) => setCountry(e.target.value)} maxLength={2} placeholder="ID" style={input} />
    </label>
    <label style={field}>Email
      <input type="email" value={email} onChange={(e) => setEmail(e.target.value)} style={input} />
    </label>
    <label style={field}>Nomor Izin Usaha (PPIU/PIHK)
      <input value={licenseNumber} onChange={(e) => setLicenseNumber(e.target.value)} style={input} />
    </label>
    <button disabled={saving} onClick={save} style={emerald}>{saving ? "Menyimpan..." : "Simpan Perubahan"}</button>
  </section>;
}

const card: React.CSSProperties = { display: "grid", gap: 14, background: "var(--color-cream-200)", border: "1px solid var(--color-cream-400)", borderRadius: 12, padding: 20 };
const field: React.CSSProperties = { display: "grid", gap: 6, color: "var(--color-warm-500)", fontSize: 14 };
const input: React.CSSProperties = { minHeight: 44, width: "100%", border: "1px solid var(--color-cream-500)", borderRadius: 8, padding: "10px 12px", background: "white", color: "var(--color-warm-900)", font: "inherit" };
const emerald: React.CSSProperties = { minHeight: 48, border: 0, borderRadius: 8, background: "var(--color-emerald-900)", color: "white", fontWeight: 700, padding: "0 18px", justifySelf: "start" };
