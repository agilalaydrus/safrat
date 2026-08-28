import { expect, test } from "@playwright/test";
import { mkdirSync } from "node:fs";
import path from "node:path";
import { fixture, pilgrimFixture, query } from "./fixture";

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

  // Platform-owned: a digital product belongs to TawafiqHub, not to the travel
  // selling it, so it carries no operator and no season. The travel's own price
  // is the base plus the markup row below.
  const [product] = await query<{ id: string }>(
    `INSERT INTO products (operator_id, season_id, name, code, category, price_idr, base_price_idr, nominal_idr)
     VALUES (NULL, NULL, 'Kuota Roaming 5GB', $1, 'ROAMING_DATA', 275000, 250000, 250000)
     RETURNING id::text AS id`,
    [`ROAM-5GB-${Date.now()}`]);
  if (!product) throw new Error("could not seed a product");
  await query(
    `INSERT INTO product_markups (product_id, operator_id, operator_markup_idr, agent_markup_idr)
     VALUES ($1, $2, 25000, 0) ON CONFLICT DO NOTHING`,
    [product.id, pilgrim.operator_id]);

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
  await query(
    `INSERT INTO pilgrim_balance_entries
       (operator_id, pilgrim_id, amount_idr, kind, order_id, note, idempotency_key)
     VALUES ($1, $2, 275000, 'REFUND', $3, 'Refund transaksi E2E', $4)
     ON CONFLICT DO NOTHING`,
    [pilgrim.operator_id, pilgrim.id, refunded.id, `e2e-refund-${refunded.id}`]);
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
      DELETE FROM pilgrim_refund_payout_requests WHERE pilgrim_id = '${pilgrim.id}';
      DELETE FROM pilgrim_balance_entries WHERE pilgrim_id = '${pilgrim.id}';
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

  test("a jamaah can request their refund balance without exposing payout details", async ({ page }) => {
    await seedPilgrimTransactions();
    // Payout creation is deliberately stricter than reading the history.
    await query(`UPDATE "user" SET "twoFactorEnabled" = true WHERE email = $1`, [pilgrimFixture.email]);
    try {
      await page.goto("/pilgrim/transactions");
      await expect(page.getByRole("heading", { name: /Dana yang dikembalikan/i })).toBeVisible();
      await expect(page.getByText(/Rp\s?275\.000/).first()).toBeVisible();

      await page.getByLabel(/^Jumlah$/).fill("275000");
      await page.getByLabel(/Metode yang diinginkan/i).selectOption(String(3));
      await page.getByLabel(/Catatan untuk travel/i).fill("Konfirmasi lewat WhatsApp");
      await page.getByRole("button", { name: /Ajukan Pencairan/i }).click();

		await expect(page.getByText(/Permintaan pencairan tunai tercatat/i)).toBeVisible();
      await expect(page.getByText(/Menunggu diproses/i)).toBeVisible();
      const [stored] = await query<{ amount_idr: string; method: string; status: string }>(
        `SELECT pr.amount_idr::text, pr.method, pr.status FROM pilgrim_refund_payout_requests pr
         JOIN pilgrims p ON p.id = pr.pilgrim_id WHERE p.email = $1 ORDER BY pr.created_at DESC LIMIT 1`,
        [pilgrimFixture.email],
      );
      expect(stored).toEqual({ amount_idr: "275000", method: "CASH", status: "REQUESTED" });
      await capture(page, "16-pilgrim-refund-wallet");
    } finally {
      await query(`UPDATE "user" SET "twoFactorEnabled" = false WHERE email = $1`, [pilgrimFixture.email]);
    }
  });
});

/**
 * The fixture staff account, additionally made an agent who leads a group.
 *
 * Both portals resolve their identity from the signed-in user — the agent
 * portal through the linked agent record, the leader pages through the groups
 * they lead — so one account can reach both. It stays an owner, and an owner is
 * never treated as a restricted member, so this does not narrow what the
 * account can otherwise do.
 */
async function makeFixtureStaffAnAgentAndLeader(): Promise<{ agentId: string }> {
  const [session] = await query<{ userId: string }>(
    `SELECT s."userId" FROM session s JOIN "user" u ON u.id = s."userId"
     WHERE u.email = $1 ORDER BY s."expiresAt" DESC LIMIT 1`, [fixture.email]);
  if (!session) throw new Error("fixture operator has no session — did setup run?");

  const [operator] = await query<{ id: string }>(
    `SELECT id FROM operators WHERE slug = $1`, [fixture.operatorSlug]);
  const [season] = await query<{ id: string }>(
    `SELECT id FROM seasons WHERE operator_id = $1 ORDER BY created_at DESC LIMIT 1`, [operator!.id]);

  const [agent] = await query<{ id: string }>(
    `INSERT INTO agents (operator_id, name, linked_user_id, is_active)
     VALUES ($1, 'Agen Portal E2E', $2, true)
     ON CONFLICT DO NOTHING
     RETURNING id::text AS id`, [operator!.id, session.userId]);
  const agentId = agent?.id ?? (await query<{ id: string }>(
    `SELECT id::text AS id FROM agents WHERE linked_user_id = $1 LIMIT 1`, [session.userId]))[0]!.id;

  await query(
    `INSERT INTO groups (operator_id, season_id, name, leader_id)
     SELECT $1, $2, 'Rombongan E2E', $3
     WHERE NOT EXISTS (SELECT 1 FROM groups WHERE leader_id = $3)`,
    [operator!.id, season!.id, session.userId]);

  // Commission the ledger will show: one earning, and one reversal, so the
  // screen has to explain a balance rather than just state it.
  await query(
    `INSERT INTO agent_commission_entries (operator_id, agent_id, amount_idr, kind, note, idempotency_key)
     VALUES ($1, $2, 450000, 'EARNED', 'Komisi referral dari transaksi', 'e2e-earn')
     ON CONFLICT DO NOTHING`, [operator!.id, agentId]);
  await query(
    `INSERT INTO agent_commission_entries (operator_id, agent_id, amount_idr, kind, note, idempotency_key)
     VALUES ($1, $2, -450000, 'REVERSED', 'Refund pesanan', 'e2e-reverse')
     ON CONFLICT DO NOTHING`, [operator!.id, agentId]);

  return { agentId };
}

test.describe("agent and muttawwif screens render", () => {
  test.use({ storageState: "./e2e/.auth/operator.json" });

  test.beforeEach(async () => {
    await query(`UPDATE "user" SET "twoFactorEnabled" = true WHERE email = $1`, [fixture.email]);
    await makeFixtureStaffAnAgentAndLeader();
  });

  test("a Muttawwif sees the commission ledger, reversal included", async ({ page }) => {
    await page.goto("/leader/transactions");

    await expect(page.getByRole("heading", { name: /Transaksi Komisi/i })).toBeVisible();
    // Both sides of the story. A reversal shown as an ordinary credit — or not
    // shown at all — would leave a balance nothing on screen accounts for.
    await expect(page.getByText(/Komisi masuk/).first()).toBeVisible();
    await expect(page.getByText(/Komisi ditarik/).first()).toBeVisible();
    await capture(page, "14-muttawwif-commission");
  });

  test("an agent sees the recap of everyone they referred", async ({ page }) => {
    await page.goto("/agent");

    await page.getByRole("button", { name: /Rekap Transaksi/ }).click();
    await expect(page.getByText(/Nilai bersih/i)).toBeVisible();
    await capture(page, "15-agent-recap");
  });

  test("an agent can withdraw a refund from their own purchase ledger", async ({ page }) => {
    const { agentId } = await makeFixtureStaffAnAgentAndLeader();
    const [agent] = await query<{ operator_id: string }>(`SELECT operator_id::text FROM agents WHERE id=$1`, [agentId]);
    await query(`DO $$ BEGIN PERFORM set_config('app.allow_ledger_purge','on',true); DELETE FROM pilgrim_refund_payout_requests WHERE agent_id='${agentId}'; DELETE FROM agent_refund_balance_entries WHERE agent_id='${agentId}'; END $$;`);
    try {
      await query(`INSERT INTO agent_refund_balance_entries (operator_id,agent_id,amount_idr,kind,note,idempotency_key) VALUES ($1,$2,185000,'REFUND','Refund pembelian agen',$3)`, [agent!.operator_id, agentId, `e2e-agent-refund-${Date.now()}`]);
      await page.goto("/agent");
      await page.getByRole("button", { name: /Beli Produk/ }).click();
      await expect(page.getByRole("heading", { name: /Dana yang dikembalikan/i })).toBeVisible();
      await page.getByLabel(/^Jumlah$/).fill("185000");
      await page.getByLabel(/Metode yang diinginkan/i).selectOption(String(3));
      await page.getByRole("button", { name: /Ajukan Pencairan/i }).click();
      await expect(page.getByText(/Permintaan pencairan tunai tercatat/i)).toBeVisible();
      const [stored] = await query<{ beneficiary_kind: string; amount_idr: string }>(`SELECT beneficiary_kind,amount_idr::text FROM pilgrim_refund_payout_requests WHERE agent_id=$1 ORDER BY created_at DESC LIMIT 1`, [agentId]);
      expect(stored).toEqual({ beneficiary_kind: "AGENT", amount_idr: "185000" });
      await capture(page, "21-agent-refund-wallet");
    } finally {
      await query(`DO $$ BEGIN PERFORM set_config('app.allow_ledger_purge','on',true); DELETE FROM pilgrim_refund_payout_requests WHERE agent_id='${agentId}'; DELETE FROM agent_refund_balance_entries WHERE agent_id='${agentId}'; END $$;`);
    }
  });
});
