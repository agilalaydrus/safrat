"use client";
import { FormEvent,useEffect,useState } from "react"; import { IconX } from "@tabler/icons-react"; import { Group,OperatorMember } from "@hajj-saas/proto-gen/hajj/v1/group_pb"; import { groupClient } from "@/lib/rpc";
type P={open:boolean;seasonId:string;initial?:Group;onClose:()=>void;onSaved:(n:string)=>void};
export default function GroupFormDialog({open,seasonId,initial,onClose,onSaved}:P){
  const[name,setName]=useState(""),[capacity,setCapacity]=useState("40"),[leaderId,setLeaderId]=useState(""),[members,setMembers]=useState<OperatorMember[]>([]),[error,setError]=useState(""),[saving,setSaving]=useState(false);
  useEffect(()=>{if(open){setName(initial?.name??"");setCapacity(String(initial?.capacity??40));setLeaderId(initial?.leaderId??"");setError("");groupClient.listOperatorMembers({}).then(r=>setMembers(r.members)).catch(()=>{})}},[open,initial]);
  if(!open)return null;
  async function submit(event:FormEvent){
    event.preventDefault();
    if(!name.trim()){setError("Nama rombongan wajib diisi.");return}
    if(Number(capacity)<1){setError("Kapasitas minimal 1.");return}
    setSaving(true);
    try{
      if(initial)await groupClient.updateGroup({groupId:initial.id,name:name.trim(),capacity:Number(capacity),leaderId});
      else await groupClient.createGroup({seasonId,name:name.trim(),capacity:Number(capacity)});
      onSaved(name.trim());onClose();
    }catch(caught){setError(caught instanceof Error?caught.message:"Gagal menyimpan rombongan.")}
    finally{setSaving(false)}
  }
  return <div style={o}><aside style={s}><div style={h}><div><p style={ey}>ROMBONGAN</p><h2 style={{margin:0}}>{initial?"Ubah rombongan":"Tambah rombongan"}</h2></div><button className="btn-close-sheet" onClick={onClose} style={x}><IconX size={18}/></button></div>
    <div style={b}><form id="group-form" onSubmit={submit} style={{display:"grid",gap:16}}>
      <label style={{display:"grid",gap:6}}><span style={lab}>Nama rombongan</span><input className="safrat-input" value={name} onChange={e=>setName(e.target.value)} placeholder="mis. GROUP-A" style={i}/></label>
      <label style={{display:"grid",gap:6}}><span style={lab}>Kapasitas</span><input className="safrat-input" type="number" min="1" value={capacity} onChange={e=>setCapacity(e.target.value)} style={i}/></label>
      {initial&&<label style={{display:"grid",gap:6}}><span style={lab}>Ketua rombongan</span><select className="safrat-input" value={leaderId} onChange={e=>setLeaderId(e.target.value)} style={i}><option value="">Belum ditentukan</option>{members.map(m=><option key={m.userId} value={m.userId}>{m.name} ({m.email})</option>)}</select></label>}
      {!initial&&<p style={{margin:0,fontSize:12,color:"var(--color-warm-400)"}}>Anda dapat menetapkan ketua rombongan setelah rombongan dibuat.</p>}
      {error&&<p style={err}>{error}</p>}
    </form></div>
    <div style={foot}><button form="group-form" disabled={saving} style={primary}>{saving?"Menyimpan...":"Simpan rombongan"}</button></div>
  </aside></div>
}
const o:React.CSSProperties={position:"fixed",inset:0,zIndex:30,display:"flex",justifyContent:"flex-end",background:"rgba(26,20,16,.48)",backdropFilter:"blur(2px)"},s:React.CSSProperties={width:"min(480px,100%)",height:"100vh",display:"flex",flexDirection:"column",background:"#fff",borderRadius:"16px 0 0 16px",overflow:"hidden"},h:React.CSSProperties={display:"flex",justifyContent:"space-between",alignItems:"center",padding:"20px 24px 16px",borderBottom:"1px solid var(--color-cream-300)"},b:React.CSSProperties={flex:1,overflowY:"auto",padding:24},foot:React.CSSProperties={padding:"16px 24px",borderTop:"1px solid var(--color-cream-300)"},x:React.CSSProperties={width:40,height:40,borderRadius:"50%",border:"1px solid var(--color-cream-400)",background:"transparent",color:"var(--color-warm-400)"},ey:React.CSSProperties={margin:0,color:"var(--color-gold-800)",fontSize:11,fontWeight:700,letterSpacing:".1em"},i:React.CSSProperties={minHeight:48,width:"100%",border:"1.5px solid var(--color-cream-400)",borderRadius:10,padding:"0 14px",background:"#fff",font:"inherit"},lab:React.CSSProperties={fontSize:13,fontWeight:600,color:"var(--color-warm-700)"},primary:React.CSSProperties={minHeight:48,width:"100%",border:0,borderRadius:10,background:"var(--color-gold-500)",color:"var(--color-warm-900)",fontWeight:700},err:React.CSSProperties={margin:0,padding:10,borderRadius:8,background:"#fdf0f0",color:"var(--color-danger-600)"};
