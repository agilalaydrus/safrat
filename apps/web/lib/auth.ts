import { betterAuth } from "better-auth";
import { organization } from "better-auth/plugins";
import { Pool } from "pg";
import { resetPasswordEmail, sendEmail, verifyEmailEmail } from "./email";

const pool = new Pool({ connectionString: process.env.DATABASE_URL });

export const auth = betterAuth({
  database: pool,
  plugins: [organization({ allowUserToCreateOrganization: true })],
  emailAndPassword: {
    enabled: true,
    // No unverified account can sign in and use the app indefinitely —
    // closes the gap flagged in the security audit (DEPLOY.md §13): this
    // is also what makes account-linking's `requireLocalEmailVerified`
    // guard (see socialProviders note below) mean anything for an
    // organically-signed-up account, not just a Google-first one.
    requireEmailVerification: true,
    sendResetPassword: async ({ user, url }) => {
      await sendEmail({ to: user.email, subject: "Atur Ulang Kata Sandi Safrat", html: resetPasswordEmail(user.name, url) });
    },
  },
  emailVerification: {
    sendOnSignUp: true,
    autoSignInAfterVerification: true,
    sendVerificationEmail: async ({ user, url }) => {
      await sendEmail({ to: user.email, subject: "Verifikasi Email Safrat", html: verifyEmailEmail(user.name, url) });
    },
  },
  // Google is additive, not a replacement — email/password keeps working.
  // No-op (button 401s) until GOOGLE_CLIENT_ID/GOOGLE_CLIENT_SECRET are set;
  // same "safe to leave unset locally" pattern as Firebase/Sentry in
  // .env.example. Every account type (operator staff, group leader, and a
  // pilgrim who links via /pilgrim/[code]) goes through this one provider.
  socialProviders: {
    google: {
      clientId: process.env.GOOGLE_CLIENT_ID ?? "",
      clientSecret: process.env.GOOGLE_CLIENT_SECRET ?? "",
    },
  },
  secret: process.env.BETTER_AUTH_SECRET!,
  // Without this, every /api/auth/get-session call — and the app makes one
  // per RPC request via lib/transport.ts's interceptor — hits the database.
  // A short signed cookie cache lets most of those resolve without a query.
  session: {
    cookieCache: { enabled: true, maxAge: 60 },
  },
  databaseHooks: {
    session: {
      create: {
        // Single-session, enforced server-side for every account this app
        // issues a session to (operator staff, leaders, and Google-linked
        // pilgrims): signing in anywhere immediately revokes every other
        // active session for that same user, so a lost/stolen device's
        // session dies the moment the real owner signs back in elsewhere —
        // deliberately no grace period, since money will flow through this
        // login eventually (Module 7 orders/payments).
        after: async (session) => {
          await pool.query('DELETE FROM session WHERE "userId" = $1 AND id <> $2', [session.userId, session.id]);
          // Pilgrim account access — "the same as admin/leader": a pilgrim
          // signs up/in at this same shared login, no app_access_code link
          // required. If this identity's email matches a pilgrim record
          // an operator already entered (pilgrims.email, set from the
          // admin dashboard) and that record isn't linked to anyone yet,
          // link it now. Idempotent (linked_user_id IS NULL guard) and
          // safe to run on every session, not just first sign-in, so a
          // pilgrim added *after* someone already has an account still
          // gets linked on their next sign-in.
          await pool.query(
            `UPDATE pilgrims p SET linked_user_id = $1
             FROM "user" u
             WHERE u.id = $1 AND p.email = u.email AND p.linked_user_id IS NULL`,
            [session.userId],
          );
        },
      },
    },
  },
});
