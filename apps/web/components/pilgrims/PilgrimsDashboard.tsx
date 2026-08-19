"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { IconFileImport, IconPlus, IconReplace, IconSearch, IconUsers, IconWheelchair } from "@tabler/icons-react";
import { Gender, Pilgrim } from "@hajj-saas/proto-gen/hajj/v1/pilgrim_pb";
import { Group } from "@hajj-saas/proto-gen/hajj/v1/group_pb";
import { Kloter } from "@hajj-saas/proto-gen/hajj/v1/kloter_pb";
import { pilgrimClient, seasonClient, groupClient, accommodationClient, kloterClient } from "@/lib/rpc";
import PilgrimFormDialog from "./PilgrimFormDialog";
import CsvImportWizard from "./CsvImportWizard";

const ROOM_TYPE_LABEL: Record<string, string> = { single: "Single", double: "Double", triple: "Triple", quad: "Quadruple", quadruple: "Quadruple" };
const pageSize = 20;
export default function PilgrimsDashboard() {
  const [seasonId, setSeasonId] = useState(""); const [seasons, setSeasons] = useState<{id:string;name:string;isActive:boolean}[]>([]); const [pilgrims, setPilgrims] = useState<Pilgrim[]>([]); const [total, setTotal] = useState(0); const [offset, setOffset] = useState(0); const [search, setSearch] = useState(""); const [term, setTerm] = useState(""); const [filter, setFilter] = useState("all"); const [loading, setLoading] = useState(true); const [formOpen, setFormOpen] = useState(false); const [importOpen, setImportOpen] = useState(false); const [notice, setNotice] = useState("");
  const [groups, setGroups] = useState<Group[]>([]);
  const [kloters, setKloters] = useState<Kloter[]>([]);
  const [rooms, setRooms] = useState<Record<string, { hotelName: string; roomNumber: string; roomType: string }>>({});
  const [assigning, setAssigning] = useState<string>("");
  const groupNames = useMemo(() => { const map: Record<string, { name: string; leaderName: string }> = {}; for (const g of groups) map[g.id] = { name: g.name, leaderName: g.leaderName }; return map; }, [groups]);
  const loadGroups = () => { if (!seasonId) return; groupClient.listGroups({ seasonId }).then((response) => setGroups(response.groups)).catch(() => {}); };
  const loadKloters = () => { if (!seasonId) return; kloterClient.listKloters({ seasonId }).then((response) => setKloters(response.kloters)).catch(() => {}); };
  const loadRooms = () => { if (!seasonId) return; accommodationClient.listPilgrimRoomAssignments({ seasonId }).then((response) => { const map: Record<string, { hotelName: string; roomNumber: string; roomType: string }> = {}; for (const a of response.assignments) map[a.pilgrimId] = { hotelName: a.hotelName, roomNumber: a.roomNumber, roomType: a.roomType }; setRooms(map); }).catch(() => {}); };
  useEffect(() => { seasonClient.listSeasons({}).then((response) => { const values=response.seasons; setSeasons(values); setSeasonId(values.find((s)=>s.isActive)?.id ?? values[0]?.id ?? ""); }).catch(() => setNotice("Gagal memuat data musim.")); }, []);
  useEffect(loadGroups, [seasonId]);
  useEffect(loadKloters, [seasonId]);
  useEffect(loadRooms, [seasonId]);
  useEffect(() => { const timeout = window.setTimeout(() => { setTerm(search); setOffset(0); }, 300); return () => window.clearTimeout(timeout); }, [search]);
  const refresh = () => { if (!seasonId) return; setLoading(true); pilgrimClient.listPilgrims({ seasonId, limit:pageSize, offset }).then((response) => { setPilgrims(response.pilgrims); setTotal(Number(response.totalCount)); }).catch(() => setNotice("Gagal memuat data jamaah." )).finally(() => setLoading(false)); };
  useEffect(refresh, [seasonId, offset]);
  async function updatePilgrimFields(pilgrim: Pilgrim, patch: Partial<{ groupId: string; kloterId: string; requiresWheelchair: boolean }>) {
    setAssigning(pilgrim.id);
    try {
      await pilgrimClient.updatePilgrim({
        pilgrimId: pilgrim.id, groupId: pilgrim.groupId, requiresWheelchair: pilgrim.requiresWheelchair, ...patch,
        fullName: pilgrim.fullName, passportNumber: pilgrim.passportNumber, nationality: pilgrim.nationality,
        dateOfBirth: pilgrim.dateOfBirth, gender: pilgrim.gender, photoUrl: pilgrim.photoUrl, phone: pilgrim.phone,
        emergencyContact: pilgrim.emergencyContact, preferredLang: pilgrim.preferredLang, medicalNotes: pilgrim.medicalNotes,
        mahramId: pilgrim.mahramId, kloterId: pilgrim.kloterId,
      });
      refresh();
      if (patch.groupId !== undefined) loadGroups();
      if (patch.kloterId !== undefined) loadKloters();
    } finally {
      setAssigning("");
    }
  }
  async function assignGroup(pilgrim: Pilgrim, groupId: string) {
    try {
      await updatePilgrimFields(pilgrim, { groupId });
      setNotice(`${pilgrim.fullName} dipindahkan ke ${groupId ? (groupNames[groupId]?.name ?? "rombongan terpilih") : "Belum Ada Rombongan"}.`);
    } catch {
      setNotice(`Gagal memindahkan ${pilgrim.fullName} ke rombongan.`);
    }
  }
  async function assignKloter(pilgrim: Pilgrim, kloterId: string) {
    try {
      await updatePilgrimFields(pilgrim, { kloterId });
      setNotice(`${pilgrim.fullName} dipindahkan ke ${kloterId ? (kloters.find((k) => k.id === kloterId)?.code ?? "kloter terpilih") : "Belum Ada Kloter"}.`);
    } catch {
      setNotice(`Gagal memindahkan ${pilgrim.fullName} ke kloter.`);
    }
  }
  async function toggleWheelchair(pilgrim: Pilgrim) {
    const next = !pilgrim.requiresWheelchair;
    try {
      await updatePilgrimFields(pilgrim, { requiresWheelchair: next });
      setNotice(`${pilgrim.fullName} ${next ? "ditandai membutuhkan" : "tidak lagi membutuhkan"} kursi roda.`);
    } catch {
      setNotice(`Gagal memperbarui kebutuhan kursi roda untuk ${pilgrim.fullName}.`);
    }
  }
  const filtered = useMemo(() => pilgrims.filter((p) => {
    const matchesSearch = `${p.fullName}${p.passportNumber}`.toLowerCase().includes(term.toLowerCase());
    const matchesFilter = filter === "all" || (filter === "male" && p.gender === Gender.MALE) || (filter === "female" && p.gender === Gender.FEMALE) || (filter === "wheelchair" && p.requiresWheelchair) || (filter === "unassigned" && !p.groupId) || (filter === "unpaid" && (p.paymentStatus || "UNPAID") === "UNPAID") || (filter === "dp" && p.paymentStatus === "DP") || (filter === "paid" && p.paymentStatus === "PAID");
    return matchesSearch && matchesFilter;
  }), [pilgrims, term, filter]);
  const paymentCounts = useMemo(() => {
    const counts = { UNPAID: 0, DP: 0, PAID: 0 };
    for (const p of pilgrims) { const status = p.paymentStatus || "UNPAID"; if (status in counts) counts[status as keyof typeof counts]++; }
    return counts;
  }, [pilgrims]);
  const activeName = seasons.find((s)=>s.id===seasonId)?.name ?? "Pilih musim";
  return <main style={page}><header style={header}><div><p style={eyebrow}>OPERASIONAL / JAMAAH</p><h1 style={title}>Jamaah</h1><p style={{ color:"var(--color-warm-500)", margin:0 }}>{`${total} jamaah terdaftar${activeName ? ` · ${activeName}` : ""}`}</p></div><div style={actions}><select aria-label="Musim" value={seasonId} onChange={(e)=>setSeasonId(e.target.value)} style={select}>{seasons.length ? seasons.map((s)=><option key={s.id} value={s.id}>{s.name}{s.isActive ? " · Aktif" : ""}</option>) : <option>{activeName}</option>}</select><Link href={`/dashboard/pilgrims/substitution?seasonId=${seasonId}`} style={{...ghost,minHeight:48,textDecoration:"none",display:"inline-flex",alignItems:"center",gap:8,paddingInline:14}}><IconReplace size={18}/>Riwayat Substitusi</Link><button onClick={()=>setImportOpen(true)} style={gold}><IconFileImport size={18}/>Impor CSV</button><button disabled={!seasonId} onClick={()=>setFormOpen(true)} style={emerald}><IconPlus size={18}/>Tambah Jamaah</button></div></header><div className="gold-divider" />{notice && <p role="status" style={{ color:"var(--color-gold-800)" }}>{notice}</p>}<section style={statGrid}><div style={statCard}><span style={statLabel}>Belum Bayar (di halaman ini)</span><strong style={{ ...statValue, color:"var(--color-danger-600)" }}>{paymentCounts.UNPAID}</strong></div><div style={statCard}><span style={statLabel}>DP (di halaman ini)</span><strong style={{ ...statValue, color:"var(--color-gold-800)" }}>{paymentCounts.DP}</strong></div><div style={statCard}><span style={statLabel}>Lunas (di halaman ini)</span><strong style={{ ...statValue, color:"var(--color-emerald-900)" }}>{paymentCounts.PAID}</strong></div></section><section style={toolbar}><label style={{ position:"relative", flex:"1 1 280px" }}><span style={sr}>Cari jamaah</span><IconSearch size={20} style={{ position:"absolute", insetInlineStart:14, top:14, color:"var(--color-warm-400)" }}/><input value={search} onChange={(e)=>setSearch(e.target.value)} placeholder="Cari nama atau nomor paspor" style={{ ...select, width:"100%", paddingInlineStart:44 }} /></label><div style={chips}>{[["all","Semua"],["male","Pria"],["female","Wanita"],["wheelchair","Kursi Roda"],["unassigned","Belum Ada Rombongan"],["unpaid","Belum Bayar"],["dp","DP"],["paid","Lunas"]].map(([key,label])=><button key={key} onClick={()=>setFilter(key ?? "all")} style={filter===key?chipActive:chip}>{label}</button>)}</div></section><section aria-busy={loading} style={tableCard}>{loading ? <div style={skeleton}>Memuat data jamaah…</div> : filtered.length ? <div><div style={{ overflowX:"auto" }}><table style={table}><thead><tr>{["Nama Lengkap","No. Paspor","Kewarganegaraan","Jenis Kelamin","Kloter","Rombongan","Akomodasi","Kursi Roda","Akun","Status"].map((name)=><th key={name} style={th}>{name}</th>)}</tr></thead><tbody>{filtered.map((pilgrim)=>{const room=rooms[pilgrim.id];return <tr key={pilgrim.id} style={tr}><td style={td}><Link href={`/dashboard/pilgrims/${pilgrim.id}`} style={{ color:"var(--color-emerald-900)", fontWeight:700 }}>{pilgrim.fullName}</Link></td><td style={{...td,fontFamily:"ui-monospace,monospace",color:"var(--color-warm-500)"}}>{pilgrim.passportNumber}</td><td style={td}>{pilgrim.nationality}</td><td style={td}>{pilgrim.gender===Gender.FEMALE?"Wanita":"Pria"}</td><td style={td}><select aria-label={`Kloter untuk ${pilgrim.fullName}`} value={pilgrim.kloterId} disabled={assigning===pilgrim.id} onChange={(e)=>assignKloter(pilgrim,e.target.value)} style={{...select,minHeight:38,fontSize:13}}><option value="">Belum Ada Kloter</option>{kloters.map((k)=><option key={k.id} value={k.id}>{k.code}</option>)}</select></td><td style={td}><select aria-label={`Rombongan untuk ${pilgrim.fullName}`} value={pilgrim.groupId} disabled={assigning===pilgrim.id} onChange={(e)=>assignGroup(pilgrim,e.target.value)} style={{...select,minHeight:38,fontSize:13}}><option value="">Belum Ada Rombongan</option>{groups.map((g)=><option key={g.id} value={g.id}>{g.name} ({g.pilgrimCount}/{g.capacity})</option>)}</select>{pilgrim.groupId&&groupNames[pilgrim.groupId]?.leaderName?<span style={{display:"block",color:"var(--color-warm-400)",fontSize:12,marginTop:4}}>Ketua: {groupNames[pilgrim.groupId]?.leaderName}</span>:null}</td><td style={td}>{room?<span>{room.hotelName}<span style={{display:"block",color:"var(--color-warm-400)",fontSize:12}}>Kamar {room.roomNumber} · {ROOM_TYPE_LABEL[room.roomType]??room.roomType}</span></span>:<span style={{color:"var(--color-warm-400)"}}>Belum ditempatkan</span>}</td><td style={td}><button onClick={()=>toggleWheelchair(pilgrim)} disabled={assigning===pilgrim.id} style={pilgrim.requiresWheelchair?wheelchairOn:wheelchairOff} aria-pressed={pilgrim.requiresWheelchair} aria-label={`Kebutuhan kursi roda untuk ${pilgrim.fullName}`}><IconWheelchair size={16}/>{pilgrim.requiresWheelchair?"Ya":"Tidak"}</button></td><td style={td}>{pilgrim.hasAccount?<span style={{color:"var(--color-emerald-800)",fontWeight:600,fontSize:12}}>Terhubung</span>:pilgrim.email?<span style={{color:"var(--color-gold-800)",fontSize:12}}>Menunggu</span>:<span style={{color:"var(--color-warm-400)",fontSize:12}}>Belum ada email</span>}</td><td style={td}><Status pilgrim={pilgrim}/></td></tr>;})}</tbody></table></div><footer style={pagination}><span>{Math.min(offset+1,total)}–{Math.min(offset+pageSize,total)} dari {total} jamaah</span><div><button onClick={()=>setOffset(Math.max(0,offset-pageSize))} disabled={!offset} style={ghost}>Sebelumnya</button><button onClick={()=>setOffset(offset+pageSize)} disabled={offset+pageSize>=total} style={ghost}>Berikutnya</button></div></footer></div> : <Empty onImport={()=>setImportOpen(true)} onAdd={()=>setFormOpen(true)} />}</section><PilgrimFormDialog open={formOpen} onClose={()=>setFormOpen(false)} seasonId={seasonId} pilgrims={pilgrims} onSaved={(name)=>{setNotice(`${name} berhasil disimpan.`);refresh();}}/><CsvImportWizard open={importOpen} onClose={()=>setImportOpen(false)} seasonId={seasonId} onComplete={(message)=>{setNotice(message);refresh();}}/></main>;
}
function Status({pilgrim}:{pilgrim:Pilgrim}) { const label=pilgrim.isSubstituted?"Digantikan":"Aktif"; const style=pilgrim.isSubstituted?badgeAmber:badgeGreen; return <span role="status" style={style}>{label}</span>; }
function Empty({onImport,onAdd}:{onImport:()=>void;onAdd:()=>void}) { return <div style={empty}><IconUsers size={48} color="var(--color-warm-400)"/><h2 style={{margin:0}}>Belum ada jamaah</h2><p style={{color:"var(--color-warm-500)",maxWidth:360,margin:"0 auto 8px"}}>Mulai dengan menambah data satu per satu atau impor manifest yang sudah ada.</p><div style={{display:"flex",justifyContent:"center",gap:12,flexWrap:"wrap"}}><button onClick={onImport} style={gold}><IconFileImport size={18}/>Impor CSV</button><button onClick={onAdd} style={emerald}><IconPlus size={18}/>Tambah Jamaah</button></div></div>; }
const page:React.CSSProperties={maxWidth:1400,margin:"0 auto",padding:"32px 24px"}; const header:React.CSSProperties={display:"flex",justifyContent:"space-between",gap:24,alignItems:"flex-start",flexWrap:"wrap"}; const actions:React.CSSProperties={display:"flex",gap:10,flexWrap:"wrap"}; const eyebrow:React.CSSProperties={color:"var(--color-gold-800)",fontSize:11,fontWeight:700,letterSpacing:".08em",margin:"4px 0 8px"}; const title:React.CSSProperties={fontSize:"clamp(32px,5vw,48px)",fontWeight:500,margin:0}; const select:React.CSSProperties={minHeight:48,border:"1px solid var(--color-cream-400)",borderRadius:8,padding:"0 12px",background:"var(--color-cream-200)",font:"inherit",color:"var(--color-warm-900)"}; const gold:React.CSSProperties={minHeight:48,border:0,borderRadius:8,padding:"0 18px",display:"inline-flex",gap:8,alignItems:"center",background:"var(--color-gold-500)",color:"white",fontWeight:700}; const emerald:React.CSSProperties={...gold,background:"var(--color-emerald-900)"}; const toolbar:React.CSSProperties={display:"flex",gap:16,alignItems:"center",flexWrap:"wrap",margin:"24px 0 16px"}; const chips:React.CSSProperties={display:"flex",gap:8,flexWrap:"wrap"}; const chip:React.CSSProperties={minHeight:40,borderWidth:1,borderStyle:"solid",borderColor:"var(--color-cream-400)",borderRadius:999,padding:"0 14px",background:"transparent",color:"var(--color-warm-500)"}; const chipActive:React.CSSProperties={...chip,background:"var(--color-emerald-900)",borderColor:"var(--color-emerald-900)",color:"var(--color-cream-100)"}; const tableCard:React.CSSProperties={border:"1px solid var(--color-cream-400)",borderRadius:12,background:"white",overflow:"hidden",minHeight:260}; const table:React.CSSProperties={width:"100%",borderCollapse:"collapse",minWidth:760}; const th:React.CSSProperties={background:"var(--color-cream-200)",padding:"14px 16px",textAlign:"start",fontSize:11,textTransform:"uppercase",letterSpacing:".08em",color:"var(--color-warm-400)"}; const tr:React.CSSProperties={borderTop:"1px solid var(--color-cream-300)"}; const td:React.CSSProperties={padding:"16px",fontSize:14}; const pagination:React.CSSProperties={display:"flex",justifyContent:"space-between",gap:16,padding:16,color:"var(--color-warm-500)",alignItems:"center",flexWrap:"wrap"}; const ghost:React.CSSProperties={minHeight:40,border:"1px solid var(--color-cream-400)",background:"transparent",borderRadius:8,padding:"0 12px",color:"var(--color-warm-500)",marginInlineStart:8}; const badgeGreen:React.CSSProperties={display:"inline-flex",padding:"5px 10px",borderRadius:99,background:"var(--color-emerald-50)",color:"var(--color-emerald-900)",fontSize:12}; const badgeAmber:React.CSSProperties={...badgeGreen,background:"#fef8e6",color:"var(--color-gold-800)"}; const wheelchairOff:React.CSSProperties={minHeight:34,border:"1px solid var(--color-cream-400)",borderRadius:8,padding:"0 10px",background:"transparent",color:"var(--color-warm-400)",display:"inline-flex",alignItems:"center",gap:6,fontSize:13}; const wheelchairOn:React.CSSProperties={...wheelchairOff,background:"var(--color-gold-50)",borderColor:"var(--color-gold-500)",color:"var(--color-gold-800)",fontWeight:700}; const empty:React.CSSProperties={padding:"64px 24px",textAlign:"center",display:"grid",justifyItems:"center",gap:12}; const skeleton:React.CSSProperties={padding:32,color:"var(--color-warm-500)"}; const sr:React.CSSProperties={position:"absolute",width:1,height:1,overflow:"hidden",clip:"rect(0 0 0 0)"}; const statGrid:React.CSSProperties={display:"grid",gridTemplateColumns:"repeat(auto-fit, minmax(200px, 1fr))",gap:12,margin:"20px 0"}; const statCard:React.CSSProperties={border:"1px solid var(--color-cream-400)",borderRadius:12,background:"var(--color-cream-200)",padding:"16px 20px",display:"grid",gap:6}; const statLabel:React.CSSProperties={fontSize:12,color:"var(--color-warm-500)",textTransform:"uppercase",letterSpacing:".06em"}; const statValue:React.CSSProperties={fontSize:28,fontWeight:700};
