import { createHash, createHmac, randomInt, timingSafeEqual } from "node:crypto";
import { APIError, createAuthEndpoint, createAuthMiddleware, getSessionFromCtx, sessionMiddleware } from "better-auth/api";
import { z } from "zod";
import { sendEmail, twoFactorEnrollmentOtpEmail } from "./email";

const OTP_TTL_SECONDS = 5 * 60;
const GRANT_TTL_SECONDS = 5 * 60;
const OTP_RESEND_DELAY_SECONDS = 60;
const MAX_ATTEMPTS = 5;

type StoredOtp = {
  hash: string;
  attempts: number;
  sentAt: number;
};

function otpIdentifier(userId: string) {
  return `tawafiqhub:2fa-enrollment:otp:${userId}`;
}

function grantIdentifier(userId: string, sessionToken: string) {
  const sessionHash = createHash("sha256").update(sessionToken).digest("hex");
  return `tawafiqhub:2fa-enrollment:grant:${userId}:${sessionHash}`;
}

function otpHash(userId: string, otp: string) {
  const secret = process.env.BETTER_AUTH_SECRET;
  if (!secret) throw new Error("BETTER_AUTH_SECRET is required");
  return createHmac("sha256", secret).update(`${userId}:${otp}`).digest("hex");
}

function parseStoredOtp(value: string): StoredOtp | null {
  try {
    const parsed = JSON.parse(value) as Partial<StoredOtp>;
    if (typeof parsed.hash !== "string" || typeof parsed.attempts !== "number" || typeof parsed.sentAt !== "number") return null;
    return { hash: parsed.hash, attempts: parsed.attempts, sentAt: parsed.sentAt };
  } catch {
    return null;
  }
}

async function hasCredentialPassword(ctx: Parameters<typeof getSessionFromCtx>[0], userId: string) {
  const accounts = await ctx.context.internalAdapter.findAccounts(userId);
  return accounts.some((account) => account.providerId === "credential" && Boolean(account.password));
}

/**
 * Step-up proof for passwordless (Google-only) accounts enrolling TOTP.
 *
 * Better Auth's allowPasswordless option deliberately trusts an authenticated
 * session. TawafiqHub requires one more proof: a short-lived email OTP. The
 * before hook below consumes that proof server-side, so calling the built-in
 * /two-factor/enable endpoint directly cannot bypass the email check.
 */
export function twoFactorEmailGate() {
  const requestEnrollmentOtp = createAuthEndpoint(
    "/two-factor/enrollment-otp/request",
    {
      method: "POST",
      use: [sessionMiddleware],
      body: z.object({}),
    },
    async (ctx) => {
      const { user } = ctx.context.session;
      if (await hasCredentialPassword(ctx, user.id)) {
        throw APIError.from("BAD_REQUEST", { code: "PASSWORD_REQUIRED", message: "Konfirmasikan kata sandi akun Anda." });
      }
      if (!user.email || !user.emailVerified) {
        throw APIError.from("FORBIDDEN", { code: "EMAIL_NOT_VERIFIED", message: "Email akun harus terverifikasi." });
      }

      const identifier = otpIdentifier(user.id);
      const existing = await ctx.context.internalAdapter.findVerificationValue(identifier);
      const existingOtp = existing ? parseStoredOtp(existing.value) : null;
      const secondsSinceLastSend = existingOtp ? Math.floor(Date.now() / 1000) - existingOtp.sentAt : OTP_RESEND_DELAY_SECONDS;
      if (existing && existing.expiresAt > new Date() && secondsSinceLastSend < OTP_RESEND_DELAY_SECONDS) {
        throw APIError.from("TOO_MANY_REQUESTS", {
          code: "OTP_RESEND_TOO_SOON",
          message: `Tunggu ${OTP_RESEND_DELAY_SECONDS - secondsSinceLastSend} detik sebelum mengirim ulang.`,
        });
      }

      const otp = randomInt(0, 1_000_000).toString().padStart(6, "0");
      const value = JSON.stringify({ hash: otpHash(user.id, otp), attempts: 0, sentAt: Math.floor(Date.now() / 1000) } satisfies StoredOtp);
      await ctx.context.internalAdapter.deleteVerificationByIdentifier(identifier);
      await ctx.context.internalAdapter.createVerificationValue({
        identifier,
        value,
        expiresAt: new Date(Date.now() + OTP_TTL_SECONDS * 1000),
      });

      try {
        await sendEmail({
          to: user.email,
          subject: "Kode OTP Keamanan Tawafiq Hub",
          html: twoFactorEnrollmentOtpEmail(user.name, otp),
        });
      } catch (error) {
        await ctx.context.internalAdapter.deleteVerificationByIdentifier(identifier);
        throw error;
      }

      return ctx.json({ success: true, expiresIn: OTP_TTL_SECONDS, resendAfter: OTP_RESEND_DELAY_SECONDS });
    },
  );

  const verifyEnrollmentOtp = createAuthEndpoint(
    "/two-factor/enrollment-otp/verify",
    {
      method: "POST",
      use: [sessionMiddleware],
      body: z.object({ otp: z.string().regex(/^\d{6}$/) }),
    },
    async (ctx) => {
      const { session, user } = ctx.context.session;
      if (await hasCredentialPassword(ctx, user.id)) {
        throw APIError.from("BAD_REQUEST", { code: "PASSWORD_REQUIRED", message: "Konfirmasikan kata sandi akun Anda." });
      }

      const identifier = otpIdentifier(user.id);
      const verification = await ctx.context.internalAdapter.findVerificationValue(identifier);
      if (!verification || verification.expiresAt <= new Date()) {
        if (verification) await ctx.context.internalAdapter.deleteVerificationByIdentifier(identifier);
        throw APIError.from("BAD_REQUEST", { code: "OTP_EXPIRED", message: "Kode OTP sudah kedaluwarsa. Kirim kode baru." });
      }

      const stored = parseStoredOtp(verification.value);
      if (!stored) {
        await ctx.context.internalAdapter.deleteVerificationByIdentifier(identifier);
        throw APIError.from("BAD_REQUEST", { code: "INVALID_OTP", message: "Kode OTP tidak valid." });
      }
      if (stored.attempts >= MAX_ATTEMPTS) {
        await ctx.context.internalAdapter.deleteVerificationByIdentifier(identifier);
        throw APIError.from("FORBIDDEN", { code: "TOO_MANY_ATTEMPTS", message: "Terlalu banyak percobaan. Kirim kode baru." });
      }

      const actual = Buffer.from(otpHash(user.id, ctx.body.otp), "hex");
      const expected = Buffer.from(stored.hash, "hex");
      if (actual.length !== expected.length || !timingSafeEqual(actual, expected)) {
        const attempts = stored.attempts + 1;
        if (attempts >= MAX_ATTEMPTS) {
          await ctx.context.internalAdapter.deleteVerificationByIdentifier(identifier);
        } else {
          await ctx.context.internalAdapter.updateVerificationByIdentifier(identifier, {
            value: JSON.stringify({ ...stored, attempts } satisfies StoredOtp),
          });
        }
        throw APIError.from("BAD_REQUEST", {
          code: "INVALID_OTP",
          message: attempts >= MAX_ATTEMPTS ? "Terlalu banyak percobaan. Kirim kode baru." : "Kode OTP tidak cocok.",
        });
      }

      await ctx.context.internalAdapter.deleteVerificationByIdentifier(identifier);
      const grant = grantIdentifier(user.id, session.token);
      await ctx.context.internalAdapter.deleteVerificationByIdentifier(grant);
      await ctx.context.internalAdapter.createVerificationValue({
        identifier: grant,
        value: user.id,
        expiresAt: new Date(Date.now() + GRANT_TTL_SECONDS * 1000),
      });
      return ctx.json({ success: true });
    },
  );

  return {
    id: "two-factor-email-gate",
    endpoints: { requestEnrollmentOtp, verifyEnrollmentOtp },
    hooks: {
      before: [{
        matcher: (ctx: { path?: string }) => ctx.path === "/two-factor/enable",
        handler: createAuthMiddleware(async (ctx) => {
          const current = await getSessionFromCtx(ctx, { disableCookieCache: true });
          if (!current?.session) {
            throw APIError.from("UNAUTHORIZED", { code: "UNAUTHORIZED", message: "Sesi tidak valid." });
          }
          if (await hasCredentialPassword(ctx, current.user.id)) return;

          const identifier = grantIdentifier(current.user.id, current.session.token);
          const grant = await ctx.context.internalAdapter.findVerificationValue(identifier);
          if (!grant || grant.value !== current.user.id || grant.expiresAt <= new Date()) {
            if (grant) await ctx.context.internalAdapter.deleteVerificationByIdentifier(identifier);
            throw APIError.from("FORBIDDEN", { code: "EMAIL_OTP_REQUIRED", message: "Verifikasi OTP email diperlukan." });
          }
          // One successful call only. A failed/replayed enable request must go
          // through email verification again instead of retaining the grant.
          await ctx.context.internalAdapter.deleteVerificationByIdentifier(identifier);
        }),
      }],
    },
    rateLimit: [{
      pathMatcher: (path: string) => path.startsWith("/two-factor/enrollment-otp"),
      window: 60,
      max: 10,
    }],
  };
}
