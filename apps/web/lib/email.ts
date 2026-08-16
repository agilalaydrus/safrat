// Transactional email via Resend's HTTP API directly — one POST call, not
// worth adding the `resend` SDK as a dependency for. No-op (logged, not
// thrown) when RESEND_API_KEY is unset, same "safe to leave unset locally"
// pattern as Firebase/Sentry elsewhere in this app — auth flows that call
// this (password reset, email verification) still complete without
// erroring, they just don't deliver anything until it's configured.
export async function sendEmail({ to, subject, html }: { to: string; subject: string; html: string }) {
  const apiKey = process.env.RESEND_API_KEY;
  if (!apiKey) {
    console.warn(`[email] RESEND_API_KEY not set — skipping send to ${to}: "${subject}"`);
    return;
  }
  const from = process.env.RESEND_FROM_EMAIL || "onboarding@resend.dev";
  const response = await fetch("https://api.resend.com/emails", {
    method: "POST",
    headers: {
      Authorization: `Bearer ${apiKey}`,
      "Content-Type": "application/json",
    },
    body: JSON.stringify({ from: `Safrat <${from}>`, to, subject, html }),
  });
  if (!response.ok) {
    const body = await response.text().catch(() => "");
    throw new Error(`Resend API error (${response.status}): ${body}`);
  }
}

function emailShell(title: string, bodyHtml: string, ctaLabel: string, ctaUrl: string): string {
  return `<!doctype html>
<html>
  <body style="margin:0;padding:0;background:#fdf9f0;font-family:'Segoe UI',Roboto,Arial,sans-serif;">
    <table role="presentation" width="100%" style="padding:32px 16px;">
      <tr><td align="center">
        <table role="presentation" width="480" style="max-width:100%;background:#ffffff;border-radius:14px;overflow:hidden;border:1px solid #e9dfc8;">
          <tr><td style="background:#0d3d27;padding:24px 32px;">
            <span style="font-family:Georgia,serif;color:#c9a84c;font-size:24px;font-weight:700;">Safrat</span>
          </td></tr>
          <tr><td style="padding:32px;">
            <h1 style="margin:0 0 16px;font-size:20px;color:#1a1a1a;">${title}</h1>
            <div style="font-size:14px;line-height:1.6;color:#4a4a4a;">${bodyHtml}</div>
            <table role="presentation" style="margin:28px 0 8px;">
              <tr><td style="background:#c9a84c;border-radius:8px;">
                <a href="${ctaUrl}" style="display:inline-block;padding:12px 28px;color:#1a1a1a;font-weight:700;font-size:14px;text-decoration:none;">${ctaLabel}</a>
              </td></tr>
            </table>
            <p style="margin:16px 0 0;font-size:12px;color:#9a9a9a;">Jika tombol di atas tidak berfungsi, salin dan tempel tautan berikut ke browser Anda:<br/><a href="${ctaUrl}" style="color:#0d3d27;word-break:break-all;">${ctaUrl}</a></p>
          </td></tr>
        </table>
      </td></tr>
    </table>
  </body>
</html>`;
}

export function resetPasswordEmail(name: string, url: string): string {
  return emailShell(
    "Atur Ulang Kata Sandi",
    `<p>Assalamualaikum ${name || ""},</p><p>Kami menerima permintaan untuk mengatur ulang kata sandi akun Safrat Anda. Tautan ini berlaku selama 1 jam. Jika Anda tidak meminta ini, abaikan email ini — kata sandi Anda tidak akan berubah.</p>`,
    "Atur Ulang Kata Sandi",
    url,
  );
}

export function verifyEmailEmail(name: string, url: string): string {
  return emailShell(
    "Verifikasi Alamat Email Anda",
    `<p>Assalamualaikum ${name || ""},</p><p>Terima kasih telah mendaftar di Safrat. Mohon verifikasi alamat email Anda untuk mengaktifkan akun sepenuhnya. Tautan ini berlaku selama 1 jam.</p>`,
    "Verifikasi Email",
    url,
  );
}
