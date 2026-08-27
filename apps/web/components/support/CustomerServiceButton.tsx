"use client";

import { IconBrandWhatsapp } from "@tabler/icons-react";

/**
 * The way somebody reaches a human when a transaction goes wrong.
 *
 * Deliberately present on the surfaces where money moves rather than buried in
 * a settings page: the moment a person needs this is the moment a payment
 * failed or something they paid for never arrived, and asking them to go
 * looking then is asking them to give up.
 *
 * WhatsApp because that is what people here actually use, and because it
 * survives a bad connection in a way a web form does not.
 */
const CS_NUMBER = "6281283031003";

export function CustomerServiceButton({
  context,
  variant = "floating",
}: {
  /** What the person was doing, prefilled so they do not have to explain twice. */
  context?: string;
  variant?: "floating" | "inline";
}) {
  const message = context
    ? `Halo Customer Service TawafiqHub, saya butuh bantuan terkait: ${context}`
    : "Halo Customer Service TawafiqHub, saya butuh bantuan.";
  const href = `https://wa.me/${CS_NUMBER}?text=${encodeURIComponent(message)}`;

  if (variant === "inline") {
    return (
      <a href={href} target="_blank" rel="noopener noreferrer" style={inline}>
        <IconBrandWhatsapp size={17} />Hubungi Customer Service
      </a>
    );
  }

  return (
    <a
      href={href}
      target="_blank"
      rel="noopener noreferrer"
      aria-label="Hubungi Customer Service TawafiqHub lewat WhatsApp"
      style={floating}
      className="no-print"
    >
      <IconBrandWhatsapp size={22} />
      <span style={floatingLabel}>Bantuan</span>
    </a>
  );
}

const floating: React.CSSProperties = {
  position: "fixed",
  // Above the pilgrim app's bottom tab bar, so it never sits on top of the
  // navigation it is meant to sit beside.
  bottom: 84,
  right: 16,
  zIndex: 45,
  display: "inline-flex",
  alignItems: "center",
  gap: 8,
  minHeight: 48,
  padding: "0 16px",
  borderRadius: 999,
  background: "#25D366",
  color: "#fff",
  fontWeight: 700,
  fontSize: 14,
  textDecoration: "none",
  boxShadow: "0 6px 20px rgba(0,0,0,.18)",
};
const floatingLabel: React.CSSProperties = { whiteSpace: "nowrap" };
const inline: React.CSSProperties = {
  display: "inline-flex",
  alignItems: "center",
  gap: 7,
  minHeight: 44,
  padding: "0 16px",
  borderRadius: 8,
  background: "#25D366",
  color: "#fff",
  fontWeight: 700,
  fontSize: 14,
  textDecoration: "none",
};
