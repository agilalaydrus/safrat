import { readFileSync } from "node:fs";
import path from "node:path";

/**
 * A dedicated operator that exists only for these specs. Everything the suite
 * creates hangs off this operator, so a run never touches real local data —
 * and `pnpm e2e:clean` removes it in one statement.
 */
export const fixture = {
  email: "e2e-operator@safrat.local",
  password: "E2eOperator!2026",
  name: "Agen E2E",
  operatorName: "E2E Fixture Travel",
  operatorSlug: "e2e-fixture",
  licenseNumber: "E2E-LOCAL",
  country: "ID",
} as const;

/**
 * A pilgrim belonging to the fixture operator. The /pilgrim PWA resolves its
 * identity from the signed-in account, so the offline specs need a real linked
 * pilgrim — an operator account is bounced by RequireAccess.
 */
export const pilgrimFixture = {
  email: "e2e-pilgrim@safrat.local",
  password: "E2ePilgrim!2026",
  name: "Jamaah E2E",
} as const;

export const pilgrimAuthFile = path.join(__dirname, ".auth", "pilgrim.json");

export const authFile = path.join(__dirname, ".auth", "operator.json");

export const appURL = process.env.E2E_APP_URL ?? "http://127.0.0.1:3131";
export const apiURL = process.env.E2E_API_URL ?? "http://127.0.0.1:8131";

/**
 * Next.js loads .env.local itself; the Playwright process does not. Parse it
 * directly rather than adding a dotenv dependency for one file.
 */
export function loadWebEnv(): void {
  const envPath = path.join(__dirname, "..", ".env.local");
  let contents: string;
  try {
    contents = readFileSync(envPath, "utf8");
  } catch {
    return;
  }
  for (const line of contents.split("\n")) {
    const match = /^\s*([A-Z0-9_]+)\s*=\s*(.*)\s*$/.exec(line);
    if (!match) continue;
    const key = match[1];
    const rawValue = match[2] ?? "";
    if (!key || process.env[key] !== undefined) continue;
    process.env[key] = rawValue.replace(/^["']|["']$/g, "");
  }
}

/** One-off query helper so specs can assert against the real database. */
export async function query<T = Record<string, unknown>>(text: string, values: unknown[] = []): Promise<T[]> {
  const { Pool } = await import("pg");
  const pool = new Pool();
  try {
    const result = await pool.query(text, values);
    return result.rows as T[];
  } finally {
    await pool.end();
  }
}

/** The fixture operator's UUID, as the API knows it. */
export async function operatorID(): Promise<string> {
  const rows = await query<{ id: string }>("SELECT id FROM operators WHERE slug = $1", [fixture.operatorSlug]);
  const row = rows[0];
  if (!row) throw new Error("fixture operator is missing — did the setup project run?");
  return row.id;
}

/**
 * Staff cannot reach the dashboard without a second factor, so any spec that
 * drives a staff screen has to enrol the fixture first.
 *
 * Shared rather than repeated, because it has a matching half that is easy to
 * forget: an enrolled account cannot sign in again — Better Auth answers with
 * a TOTP challenge instead of a session — so the next run's setup would save
 * an empty storage state. Enrol before, restore after, always as a pair.
 */
export async function enrolFixtureStaff(): Promise<void> {
  await query(`UPDATE "user" SET "twoFactorEnabled" = true WHERE email = $1`, [fixture.email]);
}

export async function unenrolFixtureStaff(): Promise<void> {
  await query(`UPDATE "user" SET "twoFactorEnabled" = false WHERE email = $1`, [fixture.email]);
}
