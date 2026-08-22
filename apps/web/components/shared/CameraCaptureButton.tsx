"use client";

import { useEffect, useRef, useState } from "react";
import { IconCamera, IconX } from "@tabler/icons-react";

type Props = {
  label?: string;
  onCapture: (file: File) => void;
  disabled?: boolean;
  style?: React.CSSProperties;
};

/**
 * Opens the actual camera device via getUserMedia and lets the user shoot
 * a still frame, instead of relying on <input capture="environment">.
 * That attribute is only a hint — desktop Chrome ignores it outright, and
 * plenty of mobile browsers/webviews fall back to the regular file/gallery
 * picker instead of the camera. This is the only way to force real camera
 * access reliably across environments.
 */
export default function CameraCaptureButton({ label = "Ambil Foto", onCapture, disabled, style }: Props) {
  const [open, setOpen] = useState(false);
  const [error, setError] = useState("");
  const [ready, setReady] = useState(false);
  const videoRef = useRef<HTMLVideoElement>(null);
  const streamRef = useRef<MediaStream | null>(null);

  useEffect(() => {
    if (!open) return;
    let cancelled = false;
    setError("");
    setReady(false);

    if (!navigator.mediaDevices?.getUserMedia) {
      setError("Perangkat/browser ini tidak mendukung akses kamera langsung.");
      return;
    }

    navigator.mediaDevices
      .getUserMedia({ video: { facingMode: { ideal: "environment" } }, audio: false })
      .then((stream) => {
        if (cancelled) {
          stream.getTracks().forEach((t) => t.stop());
          return;
        }
        streamRef.current = stream;
        if (videoRef.current) {
          videoRef.current.srcObject = stream;
          void videoRef.current.play();
        }
        setReady(true);
      })
      .catch(() => setError("Tidak bisa mengakses kamera — periksa izin kamera untuk browser ini di pengaturan perangkat."));

    return () => {
      cancelled = true;
      streamRef.current?.getTracks().forEach((t) => t.stop());
      streamRef.current = null;
    };
  }, [open]);

  function stop() {
    streamRef.current?.getTracks().forEach((t) => t.stop());
    streamRef.current = null;
    setOpen(false);
    setReady(false);
  }

  function capture() {
    const video = videoRef.current;
    if (!video || !video.videoWidth) return;
    const canvas = document.createElement("canvas");
    canvas.width = video.videoWidth;
    canvas.height = video.videoHeight;
    const ctx = canvas.getContext("2d");
    if (!ctx) return;
    ctx.drawImage(video, 0, 0, canvas.width, canvas.height);
    canvas.toBlob(
      (blob) => {
        if (!blob) return;
        onCapture(new File([blob], `capture-${Date.now()}.jpg`, { type: "image/jpeg" }));
        stop();
      },
      "image/jpeg",
      0.9,
    );
  }

  return (
    <>
      <button type="button" onClick={() => setOpen(true)} disabled={disabled} style={style}>
        <IconCamera size={14} />{label}
      </button>
      {open && (
        <div style={overlay} onClick={stop}>
          <div style={panel} onClick={(e) => e.stopPropagation()}>
            <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 12 }}>
              <strong style={{ color: "#fff", fontSize: 15 }}>Ambil Foto</strong>
              <button type="button" onClick={stop} style={closeBtn} aria-label="Tutup"><IconX size={18} /></button>
            </div>
            {error ? (
              <p style={{ color: "#fca5a5", margin: "20px 0" }}>{error}</p>
            ) : (
              <video ref={videoRef} playsInline muted style={{ width: "100%", borderRadius: 12, background: "#000", maxHeight: "60vh", objectFit: "cover" }} />
            )}
            {!error && (
              <button type="button" onClick={capture} disabled={!ready} style={captureBtn}>
                <IconCamera size={18} />{ready ? "Jepret" : "Menyalakan kamera..."}
              </button>
            )}
          </div>
        </div>
      )}
    </>
  );
}

const overlay: React.CSSProperties = { position: "fixed", inset: 0, zIndex: 60, display: "flex", alignItems: "center", justifyContent: "center", background: "rgba(0,0,0,.7)", padding: 20 };
const panel: React.CSSProperties = { width: "100%", maxWidth: 480, background: "#0f172a", borderRadius: 16, padding: 18 };
const closeBtn: React.CSSProperties = { width: 32, height: 32, borderRadius: "50%", border: "1px solid rgba(255,255,255,.3)", background: "transparent", color: "#fff" };
const captureBtn: React.CSSProperties = { width: "100%", minHeight: 48, marginTop: 14, border: 0, borderRadius: 10, background: "var(--color-gold-500)", color: "#fff", fontWeight: 700, display: "inline-flex", alignItems: "center", justifyContent: "center", gap: 8 };
