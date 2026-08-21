import { betterAuth } from "better-auth";
import { organization } from "better-auth/plugins";
import { Pool } from "pg";
import { invitationEmail, resetPasswordEmail, sendEmail, verifyEmailEmail } from "./email";

// No connectionString — a URL string breaks if POSTGRES_PASSWORD contains
// characters pg-connection-string's strict parser rejects (this happened in
// production: "TypeError: Invalid URL"). Pool() with no args reads PGHOST/
// PGPORT/PGUSER/PGPASSWORD/PGDATABASE directly, sidestepping URL parsing.
const pool = new Pool();

export const auth = betterAuth({
  database: pool,
  plugins: [
    organization({
      allowUserToCreateOrganization: true,
      // Real leader/staff onboarding — admin enters an email
      // (authClient.organization.inviteMember from GroupFormDialog), the
      // invitee gets a real email with a link, accepts it, and becomes an
      // org member (assignable as a group's leader from the same dropdown
      // that already lists ListOperatorMembers). Replaces creating leader
      // accounts by hand via SQL/curl.
      sendInvitationEmail: async (data) => {
        const url = `${process.env.NEXT_PUBLIC_APP_URL}/accept-invitation?id=${data.id}`;
        await sendEmail({
          to: data.email,
          subject: `Undangan bergabung dengan ${data.organization.name} di Tawafiq Hub`,
          html: invitationEmail(data.inviter.user.name, data.organization.name, url),
        });
      },
    }),
  ],
  emailAndPassword: {
    enabled: true,
    // No unverified account can sign in and use the app indefinitely —
    // closes the gap flagged in the security audit (DEPLOY.md §13): this
    // is also what makes account-linking's `requireLocalEmailVerified`
    // guard (see socialProviders note below) mean anything for an
    // organically-signed-up account, not just a Google-first one.
    requireEmailVerification: true,
    sendResetPassword: async ({ user, url }) => {
      await sendEmail({ to: user.email, subject: "Atur Ulang Kata Sandi Tawafiq Hub", html: resetPasswordEmail(user.name, url) });
    },
  },
  emailVerification: {
    sendOnSignUp: true,
    autoSignInAfterVerification: true,
    sendVerificationEmail: async ({ user, url }) => {
      await sendEmail({ to: user.email, subject: "Verifikasi Email Tawafiq Hub", html: verifyEmailEmail(user.name, url) });
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
  // No session.cookieCache here, deliberately — it previously cached
  // get-session results in a signed cookie for up to 60s, served without
  // ever touching the DB. That directly undermined the single-session
  // enforcement below: signing in on a second device deletes the first
  // device's session row immediately, but the first device's cached
  // cookie kept authenticating RequireAccess/PublicOnly (both call
  // authClient.getSession) for up to another 60 seconds — a real,
  // observed "double login" window, not a theoretical one. Every
  // get-session call now hits the DB, so a killed session is dead on its
  // very next check, matching the "no grace period" this app promises
  // (money flows through this login — see the hook below). The Go API's
  // own session check (internal/middleware/auth.go) was never cached and
  // was already correct; this makes the Next.js side match it.
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
