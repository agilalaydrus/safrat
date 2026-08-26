import { StorefrontAssetKind } from "@hajj-saas/proto-gen/hajj/v1/operator_pb";
import { operatorClient } from "@/lib/rpc";

const MAX_SOURCE_BYTES = 20 * 1024 * 1024;
const MAX_UPLOAD_BYTES = 5 * 1024 * 1024;
export const MAX_AUDIO_UPLOAD_BYTES = 10 * 1024 * 1024;

const DIMENSIONS: Record<StorefrontAssetKind, number> = {
  [StorefrontAssetKind.UNSPECIFIED]: 0,
  [StorefrontAssetKind.LOGO]: 1024,
  [StorefrontAssetKind.HERO]: 2400,
  [StorefrontAssetKind.GALLERY]: 2000,
  [StorefrontAssetKind.PACKAGE]: 2000,
  [StorefrontAssetKind.BACKGROUND_MUSIC]: 0,
  [StorefrontAssetKind.ARTICLE]: 2000,
};

export async function uploadStorefrontImage(file: File, kind: StorefrontAssetKind): Promise<{ url: string; originalBytes: number; optimizedBytes: number }> {
  if (!file.type.startsWith("image/")) throw new Error("File harus berupa gambar.");
  if (file.size > MAX_SOURCE_BYTES) throw new Error("Gambar asli maksimal 20 MB.");
  const blob = await compressToWebP(file, DIMENSIONS[kind] || 2000);
  if (blob.size > MAX_UPLOAD_BYTES) throw new Error("Hasil optimasi masih lebih dari 5 MB. Pilih gambar yang lebih kecil.");

  const ticket = await operatorClient.createStorefrontUpload({ kind, sizeBytes: BigInt(blob.size) });
  const response = await fetch(ticket.uploadUrl, {
    method: ticket.method || "PUT",
    headers: { "Content-Type": ticket.contentType || "image/webp" },
    body: blob,
  });
  if (!response.ok) throw new Error(`Object storage menolak upload (${response.status}). Periksa CORS bucket.`);
  const confirmed = await operatorClient.confirmStorefrontUpload({ objectKey: ticket.objectKey });
  return { url: confirmed.publicUrl, originalBytes: file.size, optimizedBytes: blob.size };
}

export async function uploadStorefrontAudio(file: File): Promise<{ url: string; bytes: number }> {
  const isMP3 = file.type === "audio/mpeg" || file.type === "audio/mp3" || file.name.toLowerCase().endsWith(".mp3");
  if (!isMP3) throw new Error("Musik latar harus berupa MP3.");
  if (file.size > MAX_AUDIO_UPLOAD_BYTES) throw new Error("Musik latar maksimal 10 MB.");
  if (!(await hasMP3Signature(file))) throw new Error("Isi file bukan MP3 yang valid.");

  const ticket = await operatorClient.createStorefrontUpload({ kind: StorefrontAssetKind.BACKGROUND_MUSIC, sizeBytes: BigInt(file.size) });
  const response = await fetch(ticket.uploadUrl, {
    method: ticket.method || "PUT",
    headers: { "Content-Type": ticket.contentType || "audio/mpeg" },
    body: file,
  });
  if (!response.ok) throw new Error(`Object storage menolak upload (${response.status}). Periksa CORS bucket.`);
  const confirmed = await operatorClient.confirmStorefrontUpload({ objectKey: ticket.objectKey });
  return { url: confirmed.publicUrl, bytes: file.size };
}

async function hasMP3Signature(file: File): Promise<boolean> {
  const bytes = new Uint8Array(await file.slice(0, 3).arrayBuffer());
  const [first = 0, second = 0, third = 0] = bytes;
  return bytes.length >= 3 && (
    (first === 0x49 && second === 0x44 && third === 0x33)
    || (first === 0xff && (second & 0xe0) === 0xe0)
  );
}

async function compressToWebP(file: File, maxDimension: number): Promise<Blob> {
  const image = await loadImage(file);
  const scale = Math.min(1, maxDimension / Math.max(image.width, image.height));
  const width = Math.max(1, Math.round(image.width * scale));
  const height = Math.max(1, Math.round(image.height * scale));
  const canvas = document.createElement("canvas");
  canvas.width = width;
  canvas.height = height;
  const context = canvas.getContext("2d", { alpha: true });
  if (!context) {
    image.close?.();
    throw new Error("Browser tidak mendukung kompresi gambar.");
  }
  context.drawImage(image.source, 0, 0, width, height);
  image.close?.();
  const result = await new Promise<Blob | null>((resolve) => canvas.toBlob(resolve, "image/webp", 0.82));
  if (!result || result.type !== "image/webp") throw new Error("Browser gagal membuat WebP. Gunakan Chrome, Edge, atau Safari terbaru.");
  return result;
}

async function loadImage(file: File): Promise<{ source: CanvasImageSource; width: number; height: number; close?: () => void }> {
  if ("createImageBitmap" in window) {
    const bitmap = await createImageBitmap(file, { imageOrientation: "from-image" });
    return { source: bitmap, width: bitmap.width, height: bitmap.height, close: () => bitmap.close() };
  }
  const objectURL = URL.createObjectURL(file);
  try {
    const image = await new Promise<HTMLImageElement>((resolve, reject) => {
      const element = new Image();
      element.onload = () => resolve(element);
      element.onerror = () => reject(new Error("Gambar tidak dapat dibaca."));
      element.src = objectURL;
    });
    return { source: image, width: image.naturalWidth, height: image.naturalHeight };
  } finally {
    URL.revokeObjectURL(objectURL);
  }
}
