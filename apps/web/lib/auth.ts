import { betterAuth } from "better-auth";
import { organization } from "better-auth/plugins";
import { Pool } from "pg";

const pool = new Pool({ connectionString: process.env.DATABASE_URL });

export const auth = betterAuth({
  database: pool,
  plugins: [organization({ allowUserToCreateOrganization: true })],
  emailAndPassword: { enabled: true },
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
        },
      },
    },
  },
});
