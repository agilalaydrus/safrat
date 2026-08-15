"use client";
import { useEffect,useState } from "react";
import { IconPencil,IconPlus,IconTrash,IconUserCheck,IconUsersGroup } from "@tabler/icons-react";
import { Group } from "@hajj-saas/proto-gen/hajj/v1/group_pb";
import { groupClient,seasonClient } from "@/lib/rpc";
import GroupFormDialog from "./GroupFormDialog";

export default function GroupsDashboard(){
  const[seasons,setSeasons]=useState<{id:string;name:string;isActive:boolean}[]>([]),[seasonId,setSeasonId]=useState(""),[groups,setGroups]=useState<Group[]>([]),[open,setOpen]=useState(false),[edit,setEdit]=useState<Group|undefined>(),[notice,setNotice]=useState("");
  const load=async(id=seasonId)=>{if(!id)return;try{setGroups((await groupClient.listGroups({seasonId:id})).groups)}catch{setNotice("Unable to load groups.")}};
  useEffect(()=>{seasonClient.listSeasons({}).then(r=>{setSeasons(r.seasons);setSeasonId(r.seasons.find(s=>s.isActive)?.id??r.seasons[0]?.id??"")}).catch(()=>setNotice("Unable to load seasons."))},[]);
  useEffect(()=>{void load()},[seasonId]);
  return <main style={page}>
    <header style={header}><div><p style={eyebrow}>OPERATIONS / GROUPS</p><h1 style={title}>Groups</h1></div>
      <div style={actions}><select value={seasonId} onChange={e=>setSeasonId(e.target.value)} style={select}>{seasons.map(s=><option key={s.id} value={s.id}>{s.name}</option>)}</select><button style={emerald} onClick={()=>{setEdit(undefined);setOpen(true)}}><IconPlus size={18}/>Add Group</button></div>
    </header>
    <div className="gold-divider"/>
    {notice&&<p style={{color:"var(--color-danger-600)"}}>{notice}</p>}
    <div style={stats}>{[["Total Groups",groups.length],["Total Pilgrims Grouped",groups.reduce((n,g)=>n+g.pilgrimCount,0)],["Groups Without a Leader",groups.filter(g=>!g.leaderId).length]].map(x=><div style={stat} key={String(x[0])}><small>{x[0]}</small><strong>{x[1]}</strong></div>)}</div>
    {groups.length?<div style={grid}>{groups.map(g=>
      <article style={card} key={g.id}>
        <div style={row}><h2 style={{margin:0,fontSize:18}}>{g.name}</h2><span style={capBadge}>{g.pilgrimCount}/{g.capacity}</span></div>
        <div style={barTrack}><div style={{...barFill,width:`${g.capacity?Math.min(100,(g.pilgrimCount/g.capacity)*100):0}%`}}/></div>
        <p style={leaderRow}><IconUserCheck size={15}/>{g.leaderName||"No leader assigned"}</p>
        <div style={row}><span/><span><button style={ghost} onClick={()=>{setEdit(g);setOpen(true)}}><IconPencil size={15}/>Edit</button><button style={{...ghost,color:"var(--color-danger-600)"}} onClick={async()=>{if(window.confirm(`Delete group ${g.name}? Pilgrims will be unassigned, not deleted.`)){await groupClient.deleteGroup({groupId:g.id});void load()}}}><IconTrash size={15}/>Delete</button></span></div>
      </article>
    )}</div>:<section style={empty}><IconUsersGroup size={48} color="var(--color-warm-400)"/><h2 style={{margin:0}}>No groups yet</h2><p>Create groups to organize pilgrims and assign leaders.</p><button style={gold} onClick={()=>setOpen(true)}>Add Group</button></section>}
    <GroupFormDialog open={open} seasonId={seasonId} initial={edit} onClose={()=>setOpen(false)} onSaved={n=>{setNotice(`${n} saved.`);void load()}}/>
  </main>
}
const page:React.CSSProperties={maxWidth:1400,margin:"0 auto",padding:"32px 24px"};const header:React.CSSProperties={display:"flex",justifyContent:"space-between",alignItems:"flex-start",gap:20,flexWrap:"wrap"};const eyebrow:React.CSSProperties={color:"var(--color-gold-800)",fontSize:11,fontWeight:700,letterSpacing:".08em",margin:"4px 0 8px"};const title:React.CSSProperties={fontSize:"clamp(32px,5vw,48px)",fontWeight:500,margin:0};const actions:React.CSSProperties={display:"flex",gap:10};const select:React.CSSProperties={minHeight:48,border:"1px solid var(--color-cream-400)",borderRadius:8,padding:"0 12px",background:"#fff"};const emerald:React.CSSProperties={minHeight:48,border:0,borderRadius:8,padding:"0 18px",display:"inline-flex",alignItems:"center",gap:8,background:"var(--color-emerald-900)",color:"var(--color-cream-100)",fontWeight:700};const gold:React.CSSProperties={...emerald,background:"var(--color-gold-500)",color:"var(--color-warm-900)"};const stats:React.CSSProperties={display:"grid",gridTemplateColumns:"repeat(auto-fit,minmax(190px,1fr))",gap:14,margin:"24px 0"};const stat:React.CSSProperties={display:"grid",gap:6,padding:18,background:"#fff",border:"1px solid var(--color-cream-400)",borderTop:"2px solid var(--color-gold-500)",borderRadius:10};const grid:React.CSSProperties={display:"grid",gridTemplateColumns:"repeat(auto-fit,minmax(280px,1fr))",gap:16};const card:React.CSSProperties={background:"white",border:"1px solid var(--color-cream-400)",borderRadius:12,padding:20,display:"grid",gap:12};const row:React.CSSProperties={display:"flex",justifyContent:"space-between",alignItems:"center",gap:8};const capBadge:React.CSSProperties={padding:"4px 8px",borderRadius:99,background:"var(--color-gold-50)",color:"var(--color-gold-800)",fontSize:12};const barTrack:React.CSSProperties={height:6,borderRadius:6,overflow:"hidden",background:"var(--color-emerald-100)"};const barFill:React.CSSProperties={height:"100%",background:"var(--color-emerald-900)"};const leaderRow:React.CSSProperties={display:"flex",alignItems:"center",gap:6,margin:0,color:"var(--color-warm-500)",fontSize:13};const ghost:React.CSSProperties={border:0,background:"transparent",display:"inline-flex",alignItems:"center",gap:4,color:"var(--color-warm-500)",marginRight:8};const empty:React.CSSProperties={minHeight:280,display:"grid",placeItems:"center",alignContent:"center",gap:12,border:"1px dashed var(--color-cream-400)",borderRadius:12};
