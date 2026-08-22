"use client";

import { ChangeEvent, useMemo, useState } from "react";
import Papa from "papaparse";
import { Timestamp } from "@bufbuild/protobuf";
import { Gender } from "@hajj-saas/proto-gen/hajj/v1/pilgrim_pb";
import { pilgrimClient } from "@/lib/rpc";

type Row = Record<string,string>;
const required = ["full_name","passport_number","nationality","date_of_birth","gender"];
const LABELS:Record<string,string>={full_name:"Nama Lengkap",passport_number:"Nomor Paspor",nationality:"Kewarganegaraan",date_of_birth:"Tanggal Lahir",gender:"Jenis Kelamin",phone:"Telepon",email:"Email",emergency_contact:"Kontak Darurat",preferred_lang:"Bahasa Pilihan",medical_notes:"Catatan Medis",requires_wheelchair:"Butuh Kursi Roda",nik:"NIK",address:"Alamat",place_of_birth:"Tempat Lahir",marital_status:"Status Pernikahan",occupation:"Pekerjaan",father_name:"Nama Ayah Kandung"};
// Column headers can arrive in Indonesian (our own template) or the old
// English snake_case (still accepted for backward compatibility) — this
// maps each internal field to every header spelling we recognize.
const HEADER_ALIASES:Record<string,string[]>={
  full_name:["nama_lengkap","nama"], passport_number:["nomor_paspor","paspor"], nationality:["kewarganegaraan"],
  date_of_birth:["tanggal_lahir"], gender:["jenis_kelamin"], phone:["telepon","no_telepon","nomor_telepon"], email:["email"],
  emergency_contact:["kontak_darurat"], preferred_lang:["bahasa_pilihan","bahasa"], medical_notes:["catatan_medis"],
  requires_wheelchair:["butuh_kursi_roda","kursi_roda"], nik:["nik"], address:["alamat"], place_of_birth:["tempat_lahir"],
  marital_status:["status_pernikahan"], occupation:["pekerjaan"], father_name:["nama_ayah","nama_ayah_kandung"],
};
const norm=(s:string)=>s.toLowerCase().trim().replace(/[^a-z0-9]+/g,"_").replace(/^_+|_+$/g,"");
function genderCode(raw:string):string{const v=norm(raw);if(["f","female","p","perempuan","wanita"].includes(v))return"FEMALE";if(["m","male","l","laki_laki","pria"].includes(v))return"MALE";return"";}
function maritalCode(raw:string):string{const v=norm(raw);if(["married","menikah"].includes(v))return"MARRIED";if(["divorced","cerai"].includes(v))return"DIVORCED";if(["widowed","janda","duda","janda_duda"].includes(v))return"WIDOWED";if(["single","belum_menikah"].includes(v))return"SINGLE";return"";}
function boolCode(raw:string):boolean{return["true","ya","yes","1"].includes(norm(raw));}

export default function CsvImportWizard({open,onClose,seasonId,onComplete}:{open:boolean;onClose:()=>void;seasonId:string;onComplete:(message:string)=>void}) {
  const [step,setStep]=useState(1); const [rows,setRows]=useState<Row[]>([]); const [mapping,setMapping]=useState<Record<string,string>>({}); const [progress,setProgress]=useState(0); const [summary,setSummary]=useState(""); const [notice,setNotice]=useState("");
  const headers=Object.keys(rows[0]??{});
  // Explicit mapping (step 2) wins; then an exact header match on the
  // internal key (old English CSVs); then an alias match against every
  // header spelling we recognize (new Indonesian CSVs), matched
  // punctuation/case-insensitively via norm().
  const guessHeader=(key:string)=>{
    if(headers.includes(key)) return key;
    const wanted=[norm(key),...(HEADER_ALIASES[key]??[]).map(norm)];
    return headers.find((header)=>wanted.includes(norm(header)))??"";
  };
  const mapped=(row:Row,key:string)=>{
    const header=mapping[key]||guessHeader(key)||key;
    return row[header]?.trim()??"";
  };
  const errors=(row:Row,index:number)=>{ const missing=required.filter((key)=>!mapped(row,key)); const date=mapped(row,"date_of_birth"); const duplicate=rows.slice(0,index).some((item)=>mapped(item,"passport_number").toUpperCase()===mapped(row,"passport_number").toUpperCase()); return [...missing, ...(date&&!/^\d{4}-\d{2}-\d{2}$/.test(date)?["invalid date"]:[]), ...(!genderCode(mapped(row,"gender"))?["invalid gender"]:[]), ...(duplicate?["duplicate passport"]:[])]; };
  const valid=useMemo(()=>rows.filter((row,index)=>!errors(row,index).length),[rows,mapping]);
  if(!open) return null;
  function upload(event:ChangeEvent<HTMLInputElement>){const file=event.target.files?.[0];if(!file)return;const MAX_SIZE=5*1024*1024;if(file.size>MAX_SIZE){setNotice("Ukuran file terlalu besar. Maksimal 5MB (±5.000 jamaah).");return;}Papa.parse<Row>(file,{header:true,skipEmptyLines:true,complete:(result)=>{if(result.data.length>500){setNotice("Maksimal 500 baris per impor. Bagi CSV Anda menjadi beberapa batch.");return;}setNotice("");setRows(result.data);setMapping({});setStep(2);}})}
  function template(){const content="Nama Lengkap,Nomor Paspor,Kewarganegaraan,Tanggal Lahir,Jenis Kelamin,Telepon,Email,Kontak Darurat,Bahasa Pilihan,Catatan Medis,Butuh Kursi Roda,NIK,Alamat,Tempat Lahir,Status Pernikahan,Pekerjaan,Nama Ayah Kandung\nAisha Ahmad,P1234567,Indonesia,1980-01-15,P,+628123456,aisha@example.com,+628123456999,ar,,Tidak,3271012345670001,Jl. Merdeka No. 1 Jakarta,Jakarta,Menikah,Guru,Ahmad Yusuf\n";const link=document.createElement("a");link.href=URL.createObjectURL(new Blob([content],{type:"text/csv"}));link.download="safrat-pilgrims-template.csv";link.click();URL.revokeObjectURL(link.href)}
  // KYC fields are optional in the CSV — if a row supplies any of them, we
  // follow up CreatePilgrim with UpdatePilgrimKyc so bulk-imported jamaah
  // land with identity data already filled in (kyc_source ADMIN,
  // PENDING_REVIEW, same as entering it by hand one at a time). A KYC
  // failure doesn't undo the pilgrim creation — it's still counted as
  // added, just without KYC filled in yet.
  const kycFields=["nik","address","place_of_birth","marital_status","occupation","father_name"] as const;
  async function run(){let added=0,failed=0;for(let i=0;i<valid.length;i++){const row=valid[i];if(!row)continue;try{const pilgrim=await pilgrimClient.createPilgrim({seasonId,fullName:mapped(row,"full_name"),passportNumber:mapped(row,"passport_number").toUpperCase(),nationality:mapped(row,"nationality"),dateOfBirth:Timestamp.fromDate(new Date(`${mapped(row,"date_of_birth")}T00:00:00Z`)),gender:genderCode(mapped(row,"gender"))==="FEMALE"?Gender.FEMALE:Gender.MALE,phone:mapped(row,"phone"),email:mapped(row,"email"),emergencyContact:mapped(row,"emergency_contact"),preferredLang:mapped(row,"preferred_lang")||"ar",medicalNotes:mapped(row,"medical_notes"),requiresWheelchair:boolCode(mapped(row,"requires_wheelchair"))});added++;if(kycFields.some((key)=>mapped(row,key))){try{await pilgrimClient.updatePilgrimKyc({pilgrimId:pilgrim.id,nik:mapped(row,"nik"),address:mapped(row,"address"),placeOfBirth:mapped(row,"place_of_birth"),maritalStatus:maritalCode(mapped(row,"marital_status")),occupation:mapped(row,"occupation"),fatherName:mapped(row,"father_name")});}catch{/* pilgrim still counts as added; KYC can be filled in manually */}}}catch{failed++;}setProgress(Math.round(((i+1)/valid.length)*100));}setSummary(`${added} berhasil ditambahkan, ${failed + rows.length-valid.length} gagal`);setStep(4);onComplete(`${added} jamaah berhasil diimpor`)}
  return <div role="dialog" aria-modal="true" aria-label="Impor jamaah" style={overlay}><section style={panel}><header style={{display:"flex",justifyContent:"space-between",gap:16}}><div><p style={eyebrow}>IMPOR CSV · LANGKAH {step} DARI 4</p><h2 style={{margin:0}}>Impor jamaah</h2></div><button onClick={onClose} style={button}>Tutup</button></header><div className="gold-divider" />{notice&&<p role="alert" style={{color:"var(--color-danger-600)"}}>{notice}</p>}{step===1&&<div style={drop}><input aria-label="Unggah CSV" type="file" accept=".csv,text/csv" onChange={upload}/><p>Seret file CSV ke sini atau pilih file. Kolom boleh berbahasa Indonesia (lihat templat) atau Inggris.</p><button onClick={template} style={button}>Unduh templat</button></div>}{step===2&&<><p>Petakan kolom wajib sebelum meninjau data yang akan diimpor.</p><div style={grid}>{required.map((key)=><label key={key}>{LABELS[key]??key}<select value={mapping[key]??guessHeader(key)??""} onChange={(e)=>setMapping({...mapping,[key]:e.target.value})} style={input}><option value="">— pilih kolom —</option>{headers.map((header)=><option key={header}>{header}</option>)}</select></label>)}</div><Preview rows={rows.slice(0,3)}/><button onClick={()=>setStep(3)} style={primary}>Tinjau {rows.length} baris</button></>}{step===3&&<><p>{valid.length} valid dari {rows.length} baris.</p><div style={preview}>{rows.map((row,index)=><div key={index} style={{padding:8,background:errors(row,index).length?"var(--color-danger-100)":"transparent",borderBottom:"1px solid var(--color-cream-300)"}}>{mapped(row,"full_name")} · {mapped(row,"passport_number")} {errors(row,index).length?`(${errors(row,index).join(", ")})`:""}</div>)}</div><button onClick={run} disabled={!valid.length} style={primary}>Impor {valid.length} baris valid</button></>}{step===4&&<><progress value={progress} max="100" style={{width:"100%"}}/><h3>{summary}</h3><button onClick={onClose} style={primary}>Selesai</button></>}</section></div>;
}
function Preview({rows}:{rows:Row[]}){return <div style={preview}>{rows.map((row,index)=><pre key={index} style={{margin:0,padding:8,whiteSpace:"pre-wrap"}}>{JSON.stringify(row,null,2)}</pre>)}</div>}; const overlay:React.CSSProperties={position:"fixed",inset:0,zIndex:30,background:"rgba(26,20,16,.52)",display:"grid",placeItems:"center",padding:20}; const panel:React.CSSProperties={width:"min(720px,100%)",maxHeight:"85vh",overflow:"auto",background:"var(--color-cream-100)",borderRadius:20,padding:24}; const drop:React.CSSProperties={border:"2px dashed var(--color-cream-400)",borderRadius:12,padding:48,textAlign:"center",display:"grid",gap:12}; const grid:React.CSSProperties={display:"grid",gridTemplateColumns:"repeat(auto-fit,minmax(180px,1fr))",gap:12,margin:"16px 0"}; const input:React.CSSProperties={display:"block",width:"100%",minHeight:44,marginTop:6,border:"1px solid var(--color-cream-400)",borderRadius:8,background:"var(--color-cream-200)",padding:8}; const button:React.CSSProperties={minHeight:40,border:"1px solid var(--color-cream-400)",borderRadius:8,background:"transparent",padding:"0 14px"}; const primary:React.CSSProperties={minHeight:48,border:0,borderRadius:8,background:"var(--color-gold-500)",color:"white",fontWeight:700,padding:"0 18px"}; const preview:React.CSSProperties={maxHeight:250,overflow:"auto",border:"1px solid var(--color-cream-300)",borderRadius:8,margin:"16px 0"}; const eyebrow:React.CSSProperties={margin:"0 0 6px",color:"var(--color-gold-800)",fontSize:11,fontWeight:700,letterSpacing:".08em"};
