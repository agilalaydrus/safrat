import nodemailer from "nodemailer";

let transporter: ReturnType<typeof nodemailer.createTransport> | null = null;

// Transactional email through the operator's Hostinger mailbox. Port 465 uses
// implicit TLS; port 587 can be selected with SMTP_PORT and STARTTLS is then
// negotiated by Nodemailer. Locally this stays a logged no-op when credentials
// are absent, matching the optional-service pattern used elsewhere in the app.
export async function sendEmail({ to, subject, html }: { to: string; subject: string; html: string }) {
  const user = process.env.SMTP_USER;
  const password = process.env.SMTP_PASSWORD;
  if (!user || !password) {
    console.warn(`[email] SMTP_USER/SMTP_PASSWORD not set — skipping send to ${to}: "${subject}"`);
    return;
  }
  const port = Number(process.env.SMTP_PORT || "465");
  if (!transporter) {
    transporter = nodemailer.createTransport({
      host: process.env.SMTP_HOST || "smtp.hostinger.com",
      port,
      secure: port === 465,
      auth: { user, pass: password },
    });
  }
  const from = process.env.SMTP_FROM_EMAIL || user;
  await transporter.sendMail({ from: `Tawafiq Hub <${from}>`, to, subject, html });
}

function emailShell(title: string, bodyHtml: string, ctaLabel: string, ctaUrl: string): string {
  return `<!doctype html>
<html>
  <body style="margin:0;padding:0;background:#fdf9f0;font-family:'Segoe UI',Roboto,Arial,sans-serif;">
    <table role="presentation" width="100%" style="padding:32px 16px;">
      <tr><td align="center">
        <table role="presentation" width="480" style="max-width:100%;background:#ffffff;border-radius:14px;overflow:hidden;border:1px solid #e9dfc8;">
          <tr><td style="background:#0d3d27;padding:24px 32px;">
            <span style="font-family:Georgia,serif;color:#c9a84c;font-size:24px;font-weight:700;">Tawafiq Hub</span>
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
    `<p>Assalamualaikum ${name || ""},</p><p>Kami menerima permintaan untuk mengatur ulang kata sandi akun Tawafiq Hub Anda. Tautan ini berlaku selama 1 jam. Jika Anda tidak meminta ini, abaikan email ini — kata sandi Anda tidak akan berubah.</p>`,
    "Atur Ulang Kata Sandi",
    url,
  );
}

export function verifyEmailEmail(name: string, url: string): string {
  return emailShell(
    "Verifikasi Alamat Email Anda",
    `<p>Assalamualaikum ${name || ""},</p><p>Terima kasih telah mendaftar di Tawafiq Hub. Mohon verifikasi alamat email Anda untuk mengaktifkan akun sepenuhnya. Tautan ini berlaku selama 1 jam.</p>`,
    "Verifikasi Email",
    url,
  );
}

export function invitationEmail(inviterName: string, organizationName: string, url: string): string {
  return emailShell(
    "Undangan Bergabung",
    `<p>Assalamualaikum,</p><p><strong>${inviterName}</strong> mengundang Anda untuk bergabung dengan <strong>${organizationName}</strong> di Tawafiq Hub sebagai Ketua Grup / Muttawwif. Klik tombol di bawah untuk membuat akun (atau masuk jika sudah punya) dan menerima undangan.</p>`,
    "Terima Undangan",
    url,
  );
}

export function twoFactorEnrollmentOtpEmail(name: string, otp: string): string {
  const safeName = escapeHtml(name || "");
  const safeOtp = escapeHtml(otp);
  return `<!doctype html>
<html>
  <body style="margin:0;padding:0;background:#fdf9f0;font-family:'Segoe UI',Roboto,Arial,sans-serif;">
    <table role="presentation" width="100%" style="padding:32px 16px;">
      <tr><td align="center">
        <table role="presentation" width="480" style="max-width:100%;background:#ffffff;border-radius:14px;overflow:hidden;border:1px solid #e9dfc8;">
          <tr><td style="background:#0d3d27;padding:24px 32px;"><span style="font-family:Georgia,serif;color:#c9a84c;font-size:24px;font-weight:700;">Tawafiq Hub</span></td></tr>
          <tr><td style="padding:32px;">
            <h1 style="margin:0 0 16px;font-size:20px;color:#1a1a1a;">Kode OTP Keamanan</h1>
            <p style="font-size:14px;line-height:1.6;color:#4a4a4a;">Assalamualaikum ${safeName}, gunakan kode berikut untuk memasang authenticator pada akun Anda:</p>
            <p style="margin:24px 0;padding:16px;background:#f8f2e4;border-radius:10px;text-align:center;font-size:30px;letter-spacing:.3em;font-weight:700;color:#0d3d27;">${safeOtp}</p>
            <p style="font-size:13px;line-height:1.6;color:#6b6b6b;">Kode berlaku selama 5 menit. Jangan berikan kode ini kepada siapa pun. Jika Anda tidak meminta pemasangan authenticator, abaikan email ini.</p>
          </td></tr>
        </table>
      </td></tr>
    </table>
  </body>
</html>`;
}

function escapeHtml(value: string): string {
  return value.replace(/[&<>'"]/g, (character) => ({
    "&": "&amp;",
    "<": "&lt;",
    ">": "&gt;",
    "'": "&#39;",
    '"': "&quot;",
  })[character] ?? character);
}
