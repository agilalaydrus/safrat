import { expect, test } from "@playwright/test";
import { mkdirSync } from "node:fs";
import path from "node:path";
import { fixture, operatorID, query } from "./fixture";

/**
 * Every screen built for the money work — transaction history, the refund and
 * hold-review dialogs, two-factor enrolment, the platform panel — passed
 * typecheck and lint and had never been opened in a browser. This spec renders
 * each one and writes a screenshot, because the things that go wrong here
 * (truncated text, unreachable buttons, unreadable contrast) are invisible to
 * every other kind of test in this repo.
 *
 * Assertions are deliberately shallow: the point is that the page renders its
 * own content rather than an error state or an empty shell.
 */

const shots = path.join(__dirname, ".screens");
mkdirSync(shots, { recursive: true });

async function capture(page: import("@playwright/test").Page, name: string) {
  await page.screenshot({ path: path.join(shots, `${name}.png`), fullPage: true });
}

/**
 * A paid order and a held one, so the dashboard's Refund and Tinjau actions
 * have something to open. Written directly rather than through checkout: this
 * spec is about rendering, and driving Xendit from a browser test would make it
 * about something else.
 */
async function seedOrders(): Promise<{ paid: string; held: string }> {
  const operator = await operatorID();
  const [season] = await query<{ id: string }>(
    "SELECT id FROM seasons WHERE operator_id = $1 ORDER BY created_at DESC LIMIT 1", [operator]);
  if (!season) throw new Error("fixture operator has no season");

  const [pilgrim] = await query<{ id: string }>(
    `INSERT INTO pilgrims (season_id, operator_id, full_name, passport_number, nationality, date_of_birth, gender)
     VALUES ($1, $2, 'Jamaah Layar', $3, 'ID', '1990-01-01'::timestamptz, 'MALE')
     RETURNING id::text AS id`,
    [season.id, operator, `P-SCREEN-${Date.now()}`]);
  const [product] = await query<{ id: string }>(
    `INSERT INTO products (operator_id, season_id, name, price_idr, agent_margin_bps)
     VALUES ($1, $2, 'Paket Layar Uji', 4500000, 1500) RETURNING id::text AS id`,
    [operator, season.id]);
  if (!pilgrim || !product) throw new Error("could not seed fixture rows");

  const [paid] = await query<{ id: string }>(
    `INSERT INTO orders (operator_id, season_id, pilgrim_id, product_id, quantity,
       unit_price_idr, total_price_idr, status, paid_at, paid_amount_idr)
     VALUES ($1,$2,$3,$4,1,4500000,4500000,'PAID',NOW(),4500000) RETURNING id::text AS id`,
    [operator, season.id, pilgrim.id, product.id]);
  const [held] = await query<{ id: string }>(
    `INSERT INTO orders (operator_id, season_id, pilgrim_id, product_id, quantity,
       unit_price_idr, total_price_idr, status, paid_amount_idr, held_reason)
     VALUES ($1,$2,$3,$4,1,4500000,4500000,'HELD',4200000,
             -- Written exactly as OrderService.SettlePayment writes it, thousand
             -- separators included. A fixture that formats money differently
             -- from production hides the very mismatch this screen is for.
             'Nominal dibayar Rp4.200.000 tidak sama dengan tagihan Rp4.500.000')
     RETURNING id::text AS id`,
    [operator, season.id, pilgrim.id, product.id]);
  if (!paid || !held) throw new Error("could not seed orders");
  return { paid: paid.id, held: held.id };
}

/**
 * Staff must enrol a second factor before the dashboard opens. The fixture
 * account is enrolled here rather than in each test: this spec is about how the
 * screens look, and every one of them lives behind that gate.
 */
async function enrolFixtureStaff() {
  await query(`UPDATE "user" SET "twoFactorEnabled" = true WHERE email = $1`, [fixture.email]);
}

test.describe("money screens render", () => {
  test.beforeEach(enrolFixtureStaff);
  // Restored, because an enrolled fixture cannot sign in again: Better Auth
  // answers with a TOTP challenge instead of a session, and the next run's
  // setup would save an empty storage state.
  test.afterAll(async () => {
    await query(`UPDATE "user" SET "twoFactorEnabled" = false WHERE email = $1`, [fixture.email]);
  });

  test("orders dashboard shows both money actions, and each dialog opens", async ({ page }) => {
    await seedOrders();
    await page.goto("/dashboard/orders");

    // The season picker drives the list; the seeded orders are in the newest.
    await expect(page.getByRole("heading", { name: /Pesanan Produk Digital/i })).toBeVisible();
    await expect(page.getByText("Paket Layar Uji").first()).toBeVisible();
    await capture(page, "01-orders-dashboard");

    // Held orders must be visibly different from failures — amber, not red.
    await expect(page.getByText("Perlu Ditinjau").first()).toBeVisible();

    await page.getByRole("button", { name: /Refund/ }).first().click();
    const refund = page.getByRole("dialog", { name: /Refund pesanan/i });
    await expect(refund).toBeVisible();
    await expect(refund.getByText(/seluruh nilai transaksi/i)).toBeVisible();
    await capture(page, "02-refund-dialog");
    await refund.getByRole("button", { name: /Batal/ }).click();

    await page.getByRole("button", { name: /Tinjau/ }).first().click();
    const review = page.getByRole("dialog", { name: /Tinjau transaksi/i });
    await expect(review).toBeVisible();
    // The shortfall is the whole reason this screen exists.
    await expect(review.getByText(/Kurang/)).toBeVisible();
    await capture(page, "03-review-held-dialog");
  });

  test("two-factor enrolment renders its first step", async ({ page }) => {
    // Un-enrolled for this one: the first step is what somebody setting it up
    // actually sees.
    await query(`UPDATE "user" SET "twoFactorEnabled" = false WHERE email = $1`, [fixture.email]);
    await page.goto("/keamanan");
    await expect(page.getByRole("heading", { name: /Verifikasi Dua Langkah/i })).toBeVisible();
    await expect(page.getByText(/Langkah 1/)).toBeVisible();
    await capture(page, "04-two-factor-enrolment");
  });

  test("staff without a second factor are sent to enrol before the dashboard opens", async ({ page }) => {
    await query(`UPDATE "user" SET "twoFactorEnabled" = false WHERE email = $1`, [fixture.email]);
    await page.goto("/dashboard/orders");
    await expect(page).toHaveURL(/\/keamanan/);
    await capture(page, "09-staff-must-enrol");
    await enrolFixtureStaff();
  });

  test("the platform panel is not found without access, and asks for 2FA with it", async ({ page }) => {
    const [session] = await query<{ userId: string }>(
      `SELECT s."userId" FROM session s JOIN "user" u ON u.id = s."userId"
       WHERE u.email = $1 ORDER BY s."expiresAt" DESC LIMIT 1`, [fixture.email]);
    if (!session) throw new Error("fixture operator has no session — did setup run?");

    // Start from no access at all, whatever a previous run left behind. A
    // failed run used to leave the grant in place, and the next one then began
    // in the wrong state and failed for a reason that had nothing to do with
    // the code.
    await query("DELETE FROM platform_admins WHERE user_id = $1", [session.userId]);
    // Un-enrolled too, because this test walks all three states and the middle
    // one only exists before enrolment. beforeEach enrols the fixture so the
    // dashboard opens; here that has to be undone deliberately.
    await query(`UPDATE "user" SET "twoFactorEnabled" = false WHERE id = $1`, [session.userId]);

    // No access at all: the page reports itself as not existing.
    await page.goto("/admin");
    await expect(page.getByText(/404|not found|tidak ditemukan/i).first()).toBeVisible();
    await capture(page, "05-admin-not-found");

    // Granted but unenrolled: told to enrol, not told they lack access.
    await query("INSERT INTO platform_admins (user_id, note) VALUES ($1, 'e2e') ON CONFLICT DO NOTHING", [session.userId]);
    await page.goto("/admin");
    await expect(page.getByRole("heading", { name: /Aktifkan verifikasi dua langkah/i })).toBeVisible();
    await capture(page, "06-admin-needs-2fa");

    // Enrolled: the panel itself.
    await query(`UPDATE "user" SET "twoFactorEnabled" = true WHERE id = $1`, [session.userId]);
    await page.goto("/admin");
    await expect(page.getByRole("heading", { name: /Panel Platform/i })).toBeVisible();
    await expect(page.getByText(fixture.operatorName).first()).toBeVisible();
    await capture(page, "07-admin-operators");

    await page.getByRole("button", { name: /Harga Modal/ }).click();
    await expect(page.getByText(/tanpa harga modal dijual tanpa batas bawah/i)).toBeVisible();
    await capture(page, "08-admin-supplier-costs");

    // Leave the fixture as found: platform access is not something to leave
    // switched on in a shared local database.
    await query("DELETE FROM platform_admins WHERE user_id = $1", [session.userId]);
    await query(`UPDATE "user" SET "twoFactorEnabled" = false WHERE id = $1`, [session.userId]);
  });
});
