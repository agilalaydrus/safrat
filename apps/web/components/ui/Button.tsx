import React from "react";
type Variant="gold"|"emerald"|"outline"|"ghost"|"danger"; type Size="sm"|"md"|"lg";
interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement>{variant?:Variant;size?:Size;children:React.ReactNode;}
const base:React.CSSProperties={display:"inline-flex",alignItems:"center",gap:7,fontFamily:"'Plus Jakarta Sans', sans-serif",fontWeight:600,borderRadius:8,border:"none",cursor:"pointer",transition:"opacity .15s, transform .1s",whiteSpace:"nowrap"};
const variants:Record<Variant,React.CSSProperties>={gold:{background:"var(--color-gold-500)",color: "#fff",border:"none"},emerald:{background:"var(--color-emerald-900)",color:"var(--color-cream-100)",border:"none"},outline:{background:"transparent",color:"var(--color-emerald-900)",border:"1.5px solid var(--color-emerald-900)"},ghost:{background:"transparent",color:"var(--color-warm-500)",border:"1px solid var(--color-cream-500)"},danger:{background:"var(--color-danger-600)",color:"#fff",border:"none"}};
const sizes:Record<Size,React.CSSProperties>={sm:{height:34,padding:"0 14px",fontSize:12},md:{height:42,padding:"0 20px",fontSize:13},lg:{height:52,padding:"0 28px",fontSize:15}};
export function Button({variant="gold",size="md",style,children,...props}:ButtonProps){return <button style={{...base,...variants[variant],...sizes[size],...style}} {...props}>{children}</button>;}
