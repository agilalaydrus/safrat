import { betterAuth } from "better-auth";
import { organization } from "better-auth/plugins";
import { Pool } from "pg";

export const auth = betterAuth({
  database: new Pool({ connectionString: process.env.DATABASE_URL }),
  plugins: [organization({ allowUserToCreateOrganization: true })],
  emailAndPassword: { enabled: true },
  secret: process.env.BETTER_AUTH_SECRET!,
  // Without this, every /api/auth/get-session call — and the app makes one
  // per RPC request via lib/transport.ts's interceptor — hits the database.
  // A short signed cookie cache lets most of those resolve without a query.
  session: {
    cookieCache: { enabled: true, maxAge: 60 },
  },
});
