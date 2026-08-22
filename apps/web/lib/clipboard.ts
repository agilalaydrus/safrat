"use client";

/**
 * navigator.clipboard.writeText() is a Promise that silently rejects in a
 * lot of real conditions (non-secure context, iframe, lost focus, denied
 * permission) — code that fires it without awaiting/catching (the previous
 * pattern here) reports success regardless of whether anything was
 * actually copied. This awaits it and falls back to the legacy
 * execCommand("copy") path, only reporting success if one of the two
 * actually worked.
 */
export async function copyToClipboard(text: string): Promise<boolean> {
  if (typeof navigator !== "undefined" && navigator.clipboard && window.isSecureContext) {
    try {
      await navigator.clipboard.writeText(text);
      return true;
    } catch {
      // fall through to the legacy path below
    }
  }
  try {
    const textarea = document.createElement("textarea");
    textarea.value = text;
    textarea.style.position = "fixed";
    textarea.style.opacity = "0";
    document.body.appendChild(textarea);
    textarea.focus();
    textarea.select();
    const ok = document.execCommand("copy");
    document.body.removeChild(textarea);
    return ok;
  } catch {
    return false;
  }
}
