import { request, test } from "@playwright/test";
import { mkdirSync } from "node:fs";
import path from "node:path";
import { Pool } from "pg";
import { apiURL, appURL, authFile, fixture, loadWebEnv, pilgrimAuthFile, pilgrimFixture } from "./fixture";

/**
 * Brings the fixture operator into existence through the same endpoints the app
 * uses — Better Auth over HTTP and the Connect RPC the onboarding wizard calls
 * — rather than inserting rows by hand, so the account these specs sign in as
 * is shaped exactly like a real one.
 *
 * The single unavoidable shortcut is marking the address verified directly in
 * the database: `requireEmailVerification` is on, and no mail is delivered
 * locally.
 *
 * Idempotent — a second run signs the existing fixture in and reuses it.
 *
 * Runs as the "setup" project, which the CMS and audio projects depend on. The
 * offline project deliberately does not — it needs no account, and coupling it
 * here would make an offline run fail whenever the dev server is down.
 */
test.describe.configure({ mode: "serial" });
test("provision the fixture operator", async () => {
  loadWebEnv();
  const pool = new Pool();
  try {
    // Better Auth rejects state-changing calls without an Origin (CSRF), which
    // a browser always sends and a bare request context does not.
    const api = await request.newContext({ baseURL: appURL, extraHTTPHeaders: { origin: appURL } });

    // 1. The account. A repeat run just finds it already there.
    await api.post("/api/auth/sign-up/email", {
      data: { name: fixture.name, email: fixture.email, password: fixture.password },
      failOnStatusCode: false,
    });
    const verified = await pool.query(
      'UPDATE "user" SET "emailVerified" = true WHERE email = $1 RETURNING id',
      [fixture.email],
    );
    if (verified.rowCount === 0) {
      throw new Error(`fixture user ${fixture.email} was not created — is the web app running on ${appURL}?`);
    }
    const userID = verified.rows[0].id as string;

    // 2. Sign in. This context's cookies become the saved storage state.
    const signIn = await api.post("/api/auth/sign-in/email", {
      data: { email: fixture.email, password: fixture.password },
      failOnStatusCode: false,
    });
    if (!signIn.ok()) {
      throw new Error(`fixture sign-in failed (${signIn.status()}): ${await signIn.text()}`);
    }

    // 3. The organization, which is what the API resolves as the operator ID.
    const existingMember = await pool.query('SELECT "organizationId" FROM member WHERE "userId" = $1 LIMIT 1', [userID]);
    let organizationID = existingMember.rows[0]?.organizationId as string | undefined;
    if (!organizationID) {
      const created = await api.post("/api/auth/organization/create", {
        data: { name: fixture.operatorName, slug: `${fixture.operatorSlug}-org` },
        failOnStatusCode: false,
      });
      if (!created.ok()) {
        throw new Error(`fixture organization create failed (${created.status()}): ${await created.text()}`);
      }
      organizationID = (await created.json()).id as string;
    }
    await api.post("/api/auth/organization/set-active", {
      data: { organizationId: organizationID },
      failOnStatusCode: false,
    });

    // 4. The operator, created through the same RPC the onboarding wizard calls.
    //    The bearer token is the Better Auth session token, exactly as
    //    lib/transport.ts attaches it.
    const operator = await pool.query("SELECT id FROM operators WHERE better_auth_org_id = $1", [organizationID]);
    if (operator.rowCount === 0) {
      const session = await pool.query(
        'SELECT token FROM session WHERE "userId" = $1 AND "expiresAt" > NOW() ORDER BY "expiresAt" DESC LIMIT 1',
        [userID],
      );
      if (session.rowCount === 0) throw new Error("no live session row for the fixture user");
      const response = await api.post(`${apiURL}/hajj.v1.OperatorService/CreateOperator`, {
        headers: { "content-type": "application/json", authorization: `Bearer ${session.rows[0].token}` },
        data: {
          betterAuthOrgId: organizationID,
          name: fixture.operatorName,
          country: fixture.country,
          email: fixture.email,
          licenseNumber: fixture.licenseNumber,
          slug: fixture.operatorSlug,
        },
        failOnStatusCode: false,
      });
      if (!response.ok()) {
        throw new Error(`CreateOperator failed (${response.status()}): ${await response.text()}`);
      }
    }

    mkdirSync(path.dirname(authFile), { recursive: true });
    await api.storageState({ path: authFile });
    await api.dispose();
  } finally {
    await pool.end();
  }
});

test("provision a linked pilgrim identity", async () => {
  loadWebEnv();
  const pool = new Pool();
  try {
    const operator = await pool.query("SELECT id FROM operators WHERE slug = $1", [fixture.operatorSlug]);
    if (operator.rowCount === 0) throw new Error("fixture operator is missing — the previous setup step must run first");
    const operatorID = operator.rows[0].id as string;

    // A pilgrim needs a season to belong to. Fixture scaffolding, not the
    // behaviour under test, so it is inserted directly.
    const season = await pool.query(
      `INSERT INTO seasons (operator_id, name, type, start_date, end_date, is_active, capacity)
       SELECT $1, 'E2E Season', 'UMRAH_REGULER', NOW() - INTERVAL '1 day', NOW() + INTERVAL '90 days', true, 50
       WHERE NOT EXISTS (SELECT 1 FROM seasons WHERE operator_id = $1 AND name = 'E2E Season')
       RETURNING id`,
      [operatorID],
    );
    const seasonID = (season.rows[0]?.id as string | undefined)
      ?? (await pool.query("SELECT id FROM seasons WHERE operator_id = $1 AND name = 'E2E Season'", [operatorID])).rows[0].id;

    await pool.query(
      `INSERT INTO pilgrims (season_id, operator_id, full_name, passport_number, nationality, date_of_birth, gender, email)
       SELECT $1, $2, $3, 'E2E-PASSPORT', 'ID', '1990-01-01'::timestamptz, 'MALE', $4
       WHERE NOT EXISTS (SELECT 1 FROM pilgrims WHERE email = $4)`,
      [seasonID, operatorID, pilgrimFixture.name, pilgrimFixture.email],
    );

    const api = await request.newContext({ baseURL: appURL, extraHTTPHeaders: { origin: appURL } });
    await api.post("/api/auth/sign-up/email", {
      data: { name: pilgrimFixture.name, email: pilgrimFixture.email, password: pilgrimFixture.password },
      failOnStatusCode: false,
    });
    await pool.query('UPDATE "user" SET "emailVerified" = true WHERE email = $1', [pilgrimFixture.email]);

    // Signing in is what links the account: the databaseHooks.session.create
    // hook in lib/auth.ts matches pilgrims.email to the user's email.
    const signIn = await api.post("/api/auth/sign-in/email", {
      data: { email: pilgrimFixture.email, password: pilgrimFixture.password },
      failOnStatusCode: false,
    });
    if (!signIn.ok()) throw new Error(`pilgrim sign-in failed (${signIn.status()}): ${await signIn.text()}`);

    const linked = await pool.query("SELECT linked_user_id FROM pilgrims WHERE email = $1", [pilgrimFixture.email]);
    if (!linked.rows[0]?.linked_user_id) {
      throw new Error("the session hook did not link the pilgrim record to the account");
    }

    await api.storageState({ path: pilgrimAuthFile });
    await api.dispose();
  } finally {
    await pool.end();
  }
});
