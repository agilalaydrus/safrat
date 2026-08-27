import { expect, test } from "@playwright/test";
import { mkdirSync } from "node:fs";
import path from "node:path";
import { pilgrimFixture, query } from "./fixture";

/**
 * The three history screens that were built and never looked at: a jamaah's own
 * transactions, a Muttawwif's commission ledger, and the agent portal's recap
 * of everyone they referred.
 *
 * They need different identities from the money-screens spec, which runs as
 * operator staff — the pilgrim app bounces a staff account, and the agent
 * portal resolves the agent from the signed-in user.
 */

const shots = path.join(__dirname, ".screens");
mkdirSync(shots, { recursive: true });

async function capture(page: import("@playwright/test").Page, name: string) {
  await page.screenshot({ path: path.join(shots, `${name}.png`), fullPage: true });
}

/**
 * A paid transaction and a refunded one for the fixture jamaah, so the history
 * has both the ordinary case and the one somebody actually goes looking for.
 *
 * Clears what a previous run left first. Accumulating fixtures is how a local
 * database fills with rubbish that later makes a screen look broken when it is
 * not — which is exactly what the transport tests had been doing for weeks.
 */
async function seedPilgrimTransactions(): Promise<void> {
  await clearPilgrimTransactions();
  const [pilgrim] = await query<{ id: string; operator_id: string; season_id: string }>(
    `SELECT id, operator_id, season_id FROM pilgrims WHERE email = $1`, [pilgrimFixture.email]);
  if (!pilgrim) throw new Error("fixture pilgrim is missing — did setup run?");

  const [product] = await query<{ id: string }>(
    `INSERT INTO products (operator_id, season_id, name, code, category, price_idr, nominal_idr)
     VALUES ($1, $2, 'Kuota Roaming 5GB', $3, 'ROAMING_DATA', 275000, 250000)
     RETURNING id::text AS id`,
    [pilgrim.operator_id, pilgrim.season_id, `ROAM-5GB-${Date.now()}`]);
  if (!product) throw new Error("could not seed a product");

  const [paid] = await query<{ id: string }>(
    `INSERT INTO orders (operator_id, season_id, pilgrim_id, product_id, quantity,
       unit_price_idr, total_price_idr, status, paid_at, paid_amount_idr)
     VALUES ($1,$2,$3,$4,1,275000,275000,'PAID',NOW(),275000) RETURNING id::text AS id`,
    [pilgrim.operator_id, pilgrim.season_id, pilgrim.id, product.id]);
  const [refunded] = await query<{ id: string }>(
    `INSERT INTO orders (operator_id, season_id, pilgrim_id, product_id, quantity,
       unit_price_idr, total_price_idr, status, paid_at, paid_amount_idr)
     VALUES ($1,$2,$3,$4,1,275000,275000,'REFUNDED',NOW(),275000) RETURNING id::text AS id`,
    [pilgrim.operator_id, pilgrim.season_id, pilgrim.id, product.id]);
  if (!paid || !refunded) throw new Error("could not seed orders");

  await query(
    `INSERT INTO order_refunds (operator_id, order_id, amount_idr, reason)
     VALUES ($1, $2, 275000, 'Paket dibatalkan jamaah')
     ON CONFLICT DO NOTHING`,
    [pilgrim.operator_id, refunded.id]);
}

/**
 * Removes what this spec created. Ledger rows refuse deletion without the
 * teardown flag, which is the point of them, so this asks for it explicitly the
 * way a real tenant teardown would.
 */
async function clearPilgrimTransactions(): Promise<void> {
  const [pilgrim] = await query<{ id: string }>(
    `SELECT id FROM pilgrims WHERE email = $1`, [pilgrimFixture.email]);
  if (!pilgrim) return;
  // One statement, because the purge flag is session-scoped and the helper
  // takes a fresh connection each call — setting it separately would land on a
  // different session and the delete would be refused by the append-only
  // trigger, which is the trigger doing its job.
  await query(`
    DO $$
    BEGIN
      PERFORM set_config('app.allow_ledger_purge', 'on', true);
      DELETE FROM order_refunds
      WHERE order_id IN (SELECT id FROM orders WHERE pilgrim_id = '${pilgrim.id}');
      DELETE FROM orders WHERE pilgrim_id = '${pilgrim.id}';
      DELETE FROM products WHERE code LIKE 'ROAM-5GB-%';
    END $$;
  `);
}

test.describe("portal history screens render", () => {
  test.afterAll(clearPilgrimTransactions);
  test("a jamaah sees their transactions, including the refunded one", async ({ page }) => {
    await seedPilgrimTransactions();
    await page.goto("/pilgrim/transactions");

    await expect(page.getByRole("heading", { name: /Transaksi Saya/i })).toBeVisible();
    await expect(page.getByText("Kuota Roaming 5GB").first()).toBeVisible();

    // A refunded transaction stays visible rather than disappearing — somebody
    // whose money came back needs to see that it did.
    await expect(page.getByText(/Dana Dikembalikan/i).first()).toBeVisible();
    await expect(page.getByText(/dikembalikan/i).first()).toBeVisible();
    await capture(page, "10-pilgrim-transactions");

    // Every transaction can be shown as a receipt, whatever its state.
    await page.getByRole("button", { name: /Lihat Struk/ }).first().click();
    const receipt = page.getByRole("dialog", { name: /Struk transaksi/i });
    await expect(receipt).toBeVisible();
    await expect(receipt.getByText(/No\. Struk/)).toBeVisible();
    await expect(receipt.getByText(/INV-/)).toBeVisible();
    await capture(page, "11-pilgrim-receipt");
  });

  test("the customer service route is reachable from the transaction history", async ({ page }) => {
    await seedPilgrimTransactions();
    await page.goto("/pilgrim/transactions");

    const help = page.getByRole("link", { name: /Hubungi Customer Service TawafiqHub/i });
    await expect(help).toBeVisible();
    const href = await help.getAttribute("href");
    // The number, and the context prefilled so nobody has to explain twice.
    expect(href).toContain("6281283031003");
    expect(href).toContain("riwayat");
  });
});
