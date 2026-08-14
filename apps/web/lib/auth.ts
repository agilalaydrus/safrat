import { betterAuth } from "better-auth";
import { organization } from "better-auth/plugins";
import { Pool } from "pg";

export const auth = betterAuth({
  database: new Pool({ connectionString: process.env.DATABASE_URL }),
  plugins: [organization({ allowUserToCreateOrganization: true })],
  emailAndPassword: { enabled: true },
  secret: process.env.BETTER_AUTH_SECRET!,
});
