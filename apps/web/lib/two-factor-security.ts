import { createHash } from "node:crypto";
import type { BetterAuthPlugin } from "better-auth";
import { createAuthMiddleware, getSessionFromCtx, isAPIError } from "better-auth/api";
import { deleteSessionCookie, parseCookies } from "better-auth/cookies";
import { generateRandomString } from "better-auth/crypto";
import type { Pool } from "pg";

const CHALLENGE_TTL_SECONDS = 10 * 60;
const STEP_UP_TTL_SECONDS = 5 * 60;
const TWO_FACTOR_COOKIE_NAME = "two_factor";

function managementGrantIdentifier(userId: string, sessionToken: string) {
  const sessionHash = createHash("sha256").update(sessionToken).digest("hex");
  return `tawafiqhub:2fa-management:${userId}:${sessionHash}`;
}

async function writeSecurityAudit(pool: Pool, userId: string, action: string, method?: string) {
  await pool.query(
    `INSERT INTO audit_logs (operator_id, user_id, action, entity_type, entity_id, metadata)
     VALUES (NULL, $1, $2, 'account_security', $1,
       jsonb_build_object('message', $3::text, 'method', $4::text))`,
    [userId, action, `Keamanan akun: ${action}`, method ?? null],
  );
}

async function auditWithoutBreakingAuth(
  pool: Pool,
  ctx: { context: { logger: { error: (message: string, error?: unknown) => void } } },
  userId: string,
  action: string,
  method?: string,
) {
  try {
    await writeSecurityAudit(pool, userId, action, method);
  } catch (error) {
    // Authentication may already have succeeded by the time an after-hook
    // writes its audit event. Reporting success as failure would invite a
    // retry of an operation that already happened, so log the audit failure
    // loudly without lying about the authentication result.
    ctx.context.logger.error(`[2fa-security] failed to audit ${action}`, error);
  }
}

/**
 * Completes the parts of Better Auth's 2FA lifecycle that its built-in plugin
 * intentionally leaves to the application:
 *
 * - OAuth callbacks: an enrolled Google account's freshly-created session is
 *   destroyed before it reaches the browser and replaced with the same signed
 *   pending challenge used by email/password login.
 * - Sensitive management: disabling 2FA or replacing backup codes requires a
 *   TOTP/backup-code step-up tied to the exact active session.
 * - Audit: successful challenges and security changes leave account-level
 *   records without exposing a secret or code.
 */
export function twoFactorSecurity(pool: Pool): BetterAuthPlugin {
  return {
    id: "tawafiqhub-two-factor-security",
    hooks: {
      before: [{
        matcher: (ctx) =>
          ctx.path === "/two-factor/disable" ||
          ctx.path === "/two-factor/generate-backup-codes",
        handler: createAuthMiddleware(async (ctx) => {
          const current = await getSessionFromCtx(ctx, { disableCookieCache: true });
          if (!current?.session || !current.user.twoFactorEnabled) return;

          const identifier = managementGrantIdentifier(current.user.id, current.session.token);
          const grant = await ctx.context.internalAdapter.findVerificationValue(identifier);
          if (!grant || grant.value !== current.user.id || grant.expiresAt <= new Date()) {
            throw ctx.error("FORBIDDEN", {
              code: "TWO_FACTOR_STEP_UP_REQUIRED",
              message: "Verifikasi kode authenticator atau kode cadangan terlebih dahulu.",
            });
          }
        }),
      }],
      after: [
        {
          matcher: (ctx) => ctx.path === "/callback/:id",
          handler: createAuthMiddleware(async (ctx) => {
            if (ctx.params?.id !== "google") return;
            const data = ctx.context.newSession;
            if (!data?.user.twoFactorEnabled) return;

            // OAuth has already created a normal session by this point. Remove
            // both its database row and every pending Set-Cookie entry before
            // issuing a challenge, otherwise the redirect response itself
            // would contain a reusable session token that bypasses TOTP.
            deleteSessionCookie(ctx, true);
            await ctx.context.internalAdapter.deleteSession(data.session.token);
            ctx.context.setNewSession(null);

            const identifier = `2fa-${generateRandomString(20)}`;
            const expiresAt = new Date(Date.now() + CHALLENGE_TTL_SECONDS * 1000);
            await ctx.context.internalAdapter.createVerificationValue({
              identifier,
              value: data.user.id,
              expiresAt,
            });
            await ctx.context.internalAdapter.createVerificationValue({
              identifier: `2fa-attempts-${identifier}`,
              value: "0",
              expiresAt,
            });

            const cookie = ctx.context.createAuthCookie(TWO_FACTOR_COOKIE_NAME, {
              maxAge: CHALLENGE_TTL_SECONDS,
            });
            await ctx.setSignedCookie(cookie.name, identifier, ctx.context.secret, cookie.attributes);
            await auditWithoutBreakingAuth(pool, ctx, data.user.id, "two_factor_challenge_started", "google");
            throw ctx.redirect("/two-factor-challenge");
          }),
        },
        {
          matcher: (ctx) =>
            ctx.path === "/two-factor/verify-totp" ||
            ctx.path === "/two-factor/verify-backup-code",
          handler: createAuthMiddleware(async (ctx) => {
            if (isAPIError(ctx.context.returned)) return;
            const method = ctx.path === "/two-factor/verify-backup-code" ? "backup_code" : "totp";
            const requestCookies = parseCookies(ctx.headers?.get("cookie") ?? "");
            const requestHadSession = requestCookies.has(ctx.context.authCookies.sessionToken.name);
            const current = await getSessionFromCtx(ctx, { disableCookieCache: true });

            if (current?.session && current.user.twoFactorEnabled) {
              const identifier = managementGrantIdentifier(current.user.id, current.session.token);
              await ctx.context.internalAdapter.deleteVerificationByIdentifier(identifier);
              await ctx.context.internalAdapter.createVerificationValue({
                identifier,
                value: current.user.id,
                expiresAt: new Date(Date.now() + STEP_UP_TTL_SECONDS * 1000),
              });
              await auditWithoutBreakingAuth(pool, ctx, current.user.id, "two_factor_step_up_verified", method);
              return;
            }

            if (requestHadSession) {
              const enrolled = ctx.context.newSession;
              if (enrolled?.user.twoFactorEnabled) {
                await auditWithoutBreakingAuth(pool, ctx, enrolled.user.id, "two_factor_enrolled", method);
              }
              return;
            }

            // A sign-in challenge has no session on the request. Successful
            // verification creates one and publishes it as newSession.
            const completed = ctx.context.newSession;
            if (completed?.user.twoFactorEnabled) {
              await auditWithoutBreakingAuth(pool, ctx, completed.user.id, "two_factor_login_verified", method);
            }
          }),
        },
        {
          matcher: (ctx) =>
            ctx.path === "/two-factor/disable" ||
            ctx.path === "/two-factor/generate-backup-codes",
          handler: createAuthMiddleware(async (ctx) => {
            if (isAPIError(ctx.context.returned)) return;
            const current = ctx.context.newSession ?? await getSessionFromCtx(ctx, { disableCookieCache: true });
            if (!current?.user) return;
            const action = ctx.path === "/two-factor/disable"
              ? "two_factor_disabled"
              : "two_factor_backup_codes_regenerated";
            await auditWithoutBreakingAuth(pool, ctx, current.user.id, action);
          }),
        },
      ],
    },
    rateLimit: [{
      pathMatcher: (path) => path.startsWith("/two-factor/"),
      window: 60,
      max: 20,
    }],
  };
}
