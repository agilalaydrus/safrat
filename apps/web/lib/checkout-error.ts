// Converts database-enforced fraud controls into messages a buyer can act on.
// Connect's raw error includes protocol codes and internal sentinel text;
// neither belongs in a customer-facing form.
export function checkoutErrorMessage(error: unknown, fallback: string): string {
  const message = error instanceof Error ? error.message : "";
  if (/checkout attempt limit|maximal 5 checkout/i.test(message)) {
    return "Batas 5 checkout dalam 1 jam sudah tercapai. Gunakan tautan pembayaran yang sudah dibuat atau coba lagi setelah 1 jam.";
  }
  if (/unresolved held payment|sedang diperiksa/i.test(message)) {
    return "Masih ada pembayaran yang sedang diperiksa. Hubungi petugas atau tunggu transaksi tersebut diselesaikan sebelum membuat checkout baru.";
  }
  return fallback;
}
