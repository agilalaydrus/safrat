"use client";
import { FormEvent, useState } from "react";
import { useRouter } from "next/navigation";
import { authClient } from "@/lib/auth-client";
import { resolveLandingPath } from "@/lib/post-login";
import { invalidateMyAccessCache } from "@/lib/access-cache";
import { GoogleSignInButton } from "@/components/auth/GoogleSignInButton";
import { TwoFactorChallenge } from "@/components/auth/TwoFactorChallenge";
type Mode="sign-in"|"sign-up";
// Better Auth's error.message is raw English straight from the library —
// jamaah/leader users only ever see Indonesian elsewhere in this app, so an
// unmapped code would read as a confusing, out-of-place error. Known codes
// get a plain-language Indonesian message; anything unmapped falls back to
// a generic message rather than leaking the raw English string.
function friendlyAuthError(code:string|undefined,signUp:boolean):string{
  switch(code){
    case "INVALID_EMAIL_OR_PASSWORD":
    case "INVALID_PASSWORD":
      return "Email atau kata sandi salah. Silakan periksa kembali.";
    case "USER_NOT_FOUND":
      return "Akun dengan email ini tidak ditemukan.";
    case "USER_ALREADY_EXISTS":
      return "Email ini sudah terdaftar. Silakan masuk, atau gunakan email lain.";
    case "PASSWORD_TOO_SHORT":
      return "Kata sandi terlalu pendek — minimal 8 karakter.";
    case "PASSWORD_TOO_LONG":
      return "Kata sandi terlalu panjang.";
    default:
      return signUp?"Gagal membuat akun Anda. Silakan coba lagi.":"Gagal masuk. Silakan coba lagi.";
  }
}
export function AuthForm({mode}:{mode:Mode}) { const router=useRouter(); const [error,setError]=useState<string|null>(null); const [isSubmitting,setSubmitting]=useState(false); const [focused,setFocused]=useState(""); const [checkEmail,setCheckEmail]=useState<string|null>(null); const [unverifiedEmail,setUnverifiedEmail]=useState<string|null>(null); const [resending,setResending]=useState(false);
 // Set when the password was accepted but a second factor is still owed. The
 // form stays put rather than navigating away, so a mistyped code does not
 // cost the user their password entry.
 const [awaitingCode,setAwaitingCode]=useState(false);
 const signUp=mode==="sign-up";
 async function submit(event:FormEvent<HTMLFormElement>){event.preventDefault();setError(null);setUnverifiedEmail(null);setSubmitting(true);try{const form=new FormData(event.currentTarget);const email=String(form.get("email")??"");const result=signUp?await authClient.signUp.email({name:String(form.get("name")??""),email,password:String(form.get("password")??"")}):await authClient.signIn.email({email,password:String(form.get("password")??"")});if(result.error){if(result.error.code==="EMAIL_NOT_VERIFIED"){setUnverifiedEmail(email);setError("Email Anda belum diverifikasi. Periksa kotak masuk Anda, atau kirim ulang di bawah.");}else{setError(friendlyAuthError(result.error.code,signUp));}return;}
  // requireEmailVerification means sign-up never establishes a session —
  // there's nothing to redirect to yet, just tell them to check their inbox.
  if(signUp){setCheckEmail(email);return;}
  // Password accepted, second factor owed. Better Auth reports this instead of
  // returning a session.
  if((result.data as {twoFactorRedirect?:boolean}|undefined)?.twoFactorRedirect){setAwaitingCode(true);return;}
  await completeSignIn();}catch{setError("Gagal menyelesaikan proses masuk. Silakan coba lagi.");}finally{setSubmitting(false);}}

 // Shared by the password path and the second-factor path: whichever one
 // established the session, the same cache clearing and routing has to follow.
 async function completeSignIn(){
  const session=await authClient.getSession({fetchOptions:{cache:"no-store"}});if(!session.data?.user){setError("Sesi Anda tidak dapat dikonfirmasi. Silakan coba lagi.");return;}
  // A previous account's MyAccess may still be cached client-side (see
  // access-cache.ts's TTL) — without this, signing in as a different
  // identity right after another one (e.g. testing admin then leader in
  // the same tab) reads the wrong account's role data and misroutes.
  invalidateMyAccessCache();
  const path=await resolveLandingPath();router.replace(path);}

 async function resendVerification(){if(!unverifiedEmail||resending)return;setResending(true);try{await authClient.sendVerificationEmail({email:unverifiedEmail,callbackURL:"/sign-in"});setError("Email verifikasi baru telah dikirim.");}catch{setError("Gagal mengirim ulang email verifikasi.");}finally{setResending(false);}}
 const input=(name:string,type="text")=><input name={name} type={type} required minLength={name==="password"?8:undefined} style={{...inputStyle,borderWidth:1,borderStyle:"solid",borderColor:focused===name?"var(--color-emerald-800)":"var(--color-cream-500)"}} onFocus={()=>setFocused(name)} onBlur={()=>setFocused("")} />;
 if(awaitingCode){return <TwoFactorChallenge onVerified={completeSignIn}/>;}
 if(checkEmail){return <div style={{display:"grid",gap:10,textAlign:"center",padding:"12px 0"}}><p style={{fontWeight:700,fontSize:15}}>Periksa email Anda</p><p style={{fontSize:13,color:"var(--color-warm-500)"}}>Kami mengirim tautan verifikasi ke <strong>{checkEmail}</strong>. Klik tautan tersebut untuk mengaktifkan akun Anda.</p></div>;}
 return <div style={{display:"grid",gap:16}}><form onSubmit={submit} style={{display:"grid",gap:16}}>{signUp&&<label style={labelStyle}>Nama{input("name")}</label>}<label style={labelStyle}>Email{input("email","email")}</label><label style={labelStyle}>Kata Sandi{input("password","password")}</label>{!signUp&&<a href="/forgot-password" style={{fontSize:12,color:"var(--color-emerald-900)",fontWeight:600,justifySelf:"end",marginTop:-8}}>Lupa kata sandi?</a>}{error&&<p role="alert" style={{color:"var(--color-danger-600)",fontSize:13}}>{error}</p>}{unverifiedEmail&&<button type="button" onClick={()=>void resendVerification()} disabled={resending} style={{...submitStyle,background:"transparent",border:"1px solid var(--color-emerald-800)",color:"var(--color-emerald-900)",marginTop:0}}>{resending?"Mengirim...":"Kirim ulang email verifikasi"}</button>}<button type="submit" disabled={isSubmitting} style={submitStyle}>{isSubmitting?"Mohon tunggu...":signUp?"Buat akun":"Masuk"}</button></form><div style={{display:"flex",alignItems:"center",gap:10,color:"var(--color-warm-400)",fontSize:12}}><div style={{flex:1,height:1,background:"var(--color-cream-400)"}}/>atau<div style={{flex:1,height:1,background:"var(--color-cream-400)"}}/></div><GoogleSignInButton callbackURL="/sign-in" /></div>; }
const labelStyle:React.CSSProperties={display:"block",fontSize:13,fontWeight:600,color:"var(--color-warm-700)"}; const inputStyle:React.CSSProperties={display:"block",width:"100%",marginTop:6,padding:"10px 12px",fontSize:14,borderRadius:8,background:"var(--color-cream-200)",fontFamily:"'Plus Jakarta Sans',sans-serif",outline:"none"}; const submitStyle:React.CSSProperties={width:"100%",height:44,background:"var(--color-gold-500)",color: "#fff",border:"none",borderRadius:8,fontSize:14,fontWeight:700,cursor:"pointer",fontFamily:"'Plus Jakarta Sans',sans-serif",marginTop:8};
