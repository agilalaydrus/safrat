import { expect, test } from "@playwright/test";
import { createHash } from "node:crypto";
import { mkdirSync } from "node:fs";
import path from "node:path";
import { enrolFixtureStaff, fixture, operatorID, query, unenrolFixtureStaff } from "./fixture";

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

async function seedRefundPayout(): Promise<{ requestId: string; pilgrimId: string; pilgrimName: string }> {
  const operator = await operatorID();
  const [season] = await query<{ id: string }>(
    "SELECT id FROM seasons WHERE operator_id = $1 ORDER BY created_at DESC LIMIT 1", [operator]);
  const pilgrimName = `Jamaah Pencairan ${Date.now().toString().slice(-5)}`;
  const [pilgrim] = await query<{ id: string }>(
    `INSERT INTO pilgrims (season_id, operator_id, full_name, passport_number, nationality, date_of_birth, gender, phone)
     VALUES ($1,$2,$3,$4,'ID','1990-01-01'::timestamptz,'MALE','08123456789') RETURNING id::text AS id`,
    [season!.id, operator, pilgrimName, `P-PAYOUT-${Date.now()}`]);
  await query(
    `INSERT INTO pilgrim_balance_entries (operator_id,pilgrim_id,amount_idr,kind,note,idempotency_key)
     VALUES ($1,$2,350000,'REFUND','Refund untuk layar payout',$3)`,
    [operator, pilgrim!.id, `e2e-payout-credit-${pilgrim!.id}`]);
  const [request] = await query<{ id: string }>(
    `INSERT INTO pilgrim_refund_payout_requests
       (operator_id,pilgrim_id,amount_idr,method,note,idempotency_key,requested_by_user_id)
     VALUES ($1,$2,350000,'BANK_TRANSFER','Hubungi sebelum transfer',$3,'e2e-pilgrim')
     RETURNING id::text AS id`,
    [operator, pilgrim!.id, `e2e-payout-request-${pilgrim!.id}`]);
  return { requestId: request!.id, pilgrimId: pilgrim!.id, pilgrimName };
}

async function clearRefundPayout(pilgrimId: string): Promise<void> {
  await query(`
    DO $$
    BEGIN
      PERFORM set_config('app.allow_ledger_purge', 'on', true);
      DELETE FROM pilgrim_refund_payout_requests WHERE pilgrim_id = '${pilgrimId}';
      DELETE FROM pilgrim_balance_entries WHERE pilgrim_id = '${pilgrimId}';
      DELETE FROM pilgrims WHERE id = '${pilgrimId}';
    END $$;
  `);
}

test.describe("money screens render", () => {
  test.beforeEach(enrolFixtureStaff);
  // Restored, because an enrolled fixture cannot sign in again: Better Auth
  // answers with a TOTP challenge instead of a session, and the next run's
  // setup would save an empty storage state.
  test.afterAll(unenrolFixtureStaff);

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

  test("refund payout moves from requested to paid and debits the ledger once", async ({ page }) => {
    const seeded = await seedRefundPayout();
    try {
      await page.goto("/dashboard/refunds");
      await expect(page.getByRole("heading", { name: /Pencairan Saldo Jamaah/i })).toBeVisible();
      const card = page.getByRole("article").filter({ hasText: seeded.pilgrimName });
      await expect(card).toBeVisible();
      await card.getByRole("button", { name: /Mulai proses/i }).click();
      await expect(card.getByText(/^Diproses$/)).toBeVisible();

      await card.getByRole("button", { name: /Tandai dibayar/i }).click();
      await card.getByLabel(/Referensi pembayaran/i).fill("E2E-TRX-350K");
      await card.getByLabel(/Catatan operasional/i).fill("Transfer diverifikasi");
      await card.getByRole("button", { name: /Simpan keputusan/i }).click();
      await expect(card.getByText(/^Dibayar$/)).toBeVisible();

      const [proof] = await query<{ status: string; balance: string; withdrawals: string }>(
        `SELECT pr.status,
                COALESCE(SUM(e.amount_idr),0)::text AS balance,
                COUNT(*) FILTER (WHERE e.kind='WITHDRAWAL')::text AS withdrawals
         FROM pilgrim_refund_payout_requests pr
         JOIN pilgrim_balance_entries e ON e.pilgrim_id = pr.pilgrim_id
         WHERE pr.id = $1 GROUP BY pr.status`, [seeded.requestId]);
      expect(proof).toEqual({ status: "PAID", balance: "0", withdrawals: "1" });
      await capture(page, "18-refund-payout-paid");
    } finally {
      await clearRefundPayout(seeded.pilgrimId);
    }
  });

  // The travel's own pricing screen. Every level of the price is shown here,
  // and the two buyer prices are computed by the server — so this is the one
  // place a person can see whether the layered pricing agrees with itself.
  test("the pricing screen shows each level and both buyer prices", async ({ page }) => {
    await page.goto("/dashboard/products/harga");
    await expect(page.getByRole("heading", { name: /Harga & Markup/i })).toBeVisible();

    // The rule the screen exists to make visible: the base is TawafiqHub's and
    // the travel cannot move it.
    await expect(page.getByText(/Harga dasar ditetapkan TawafiqHub/i)).toBeVisible();

    // Column headers, so a layout change that drops a level is caught rather
    // than silently shipping a price nobody can explain.
    for (const heading of ["HARGA DASAR", "MARKUP TRAVEL", "MARKUP AGEN", "HARGA AGEN", "HARGA JAMAAH"]) {
      await expect(page.getByRole("columnheader", { name: heading })).toBeVisible();
    }
    await capture(page, "15-pricing-levels");
  });

  test("an enrolled account can continue from the security screen", async ({ page }) => {
    await page.goto("/keamanan");
    await expect(page.getByText(/Sudah aktif/i)).toBeVisible();
    await page.getByRole("button", { name: /Lanjutkan ke aplikasi/i }).click();
    await expect(page).toHaveURL(/\/dashboard(?:\/|$)/);
  });

  test("two-factor enrolment renders its first step", async ({ page }) => {
    // Un-enrolled for this one: the first step is what somebody setting it up
    // actually sees.
    await query(`UPDATE "user" SET "twoFactorEnabled" = false WHERE email = $1`, [fixture.email]);
    await page.goto("/keamanan");
    await expect(page.getByRole("heading", { name: /Verifikasi Dua Langkah/i })).toBeVisible();
    await expect(page.getByText(/Langkah 1/)).toBeVisible();
    await capture(page, "04-two-factor-enrolment");

    await page.getByLabel(/Kata sandi akun/i).fill(fixture.password);
    await page.getByRole("button", { name: /^Lanjutkan$/i }).click();
    await expect(page.getByLabel(/QR Code untuk Google Authenticator atau Authy/i)).toBeVisible();
    await capture(page, "17-two-factor-qr");
  });

  // A Google account has no password: it proves ownership with an email OTP,
  // and the built-in enable endpoint must reject a direct bypass attempt.
  test("an account with no password uses a server-gated email OTP", async ({ page }) => {
    const [credential] = await query<{ id: string; accountId: string; password: string }>(
      `SELECT a.id, a."accountId", a.password FROM account a
       JOIN "user" u ON u.id = a."userId"
       WHERE u.email = $1 AND a."providerId" = 'credential'`, [fixture.email]);
    if (!credential) throw new Error("fixture has no credential account — did setup run?");

    // Restored below whatever happens: without a credential row the fixture
    // cannot sign in again, and the next run's setup would save an empty
    // storage state.
    try {
      await query(`DELETE FROM account WHERE id = $1`, [credential.id]);
      // The describe block enrols the fixture before every test so the
      // dashboard opens; this one is about the enrolment screen itself, so
      // that has to be undone deliberately.
      await unenrolFixtureStaff();

      await page.goto("/keamanan");
      await expect(page.getByText(/Langkah 1 — verifikasi email akun/i)).toBeVisible();
      // The dead end was a password field on an account that has none.
      await expect(page.getByLabel(/Kata sandi akun/i)).toHaveCount(0);
      await expect(page.getByRole("button", { name: /Kirim OTP ke email/i })).toBeVisible();

      const bypassStatus = await page.evaluate(async () => {
        const response = await fetch("/api/auth/two-factor/enable", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({}),
        });
        return response.status;
      });
      expect(bypassStatus).toBe(403);

      // CI deliberately has no SMTP credentials, so sendEmail is a no-op; the
      // endpoint still exercises OTP creation and reveals the code input.
      await page.getByRole("button", { name: /Kirim OTP ke email/i }).click();
      await expect(page.getByLabel(/Kode OTP dari email/i)).toBeVisible();
      await capture(page, "16-keamanan-no-password");

      // Install the same short-lived, session-bound proof a correct OTP would
      // create. This lets the browser spec cover the complete passwordless UI
      // transition without exposing or weakening the HMAC-protected OTP.
      const [current] = await query<{ userId: string; token: string }>(
        `SELECT s.\"userId\", s.token FROM session s
         JOIN \"user\" u ON u.id = s.\"userId\"
         WHERE u.email = $1 ORDER BY s.\"expiresAt\" DESC LIMIT 1`,
        [fixture.email],
      );
      if (!current) throw new Error("fixture has no current session");
      const sessionHash = createHash("sha256").update(current.token).digest("hex");
      const grantIdentifier = `tawafiqhub:2fa-enrollment:grant:${current.userId}:${sessionHash}`;
      await query(`DELETE FROM verification WHERE identifier = $1`, [grantIdentifier]);
      await query(
        `INSERT INTO verification (id, identifier, value, \"expiresAt\", \"createdAt\", \"updatedAt\")
         VALUES ($1, $2, $3, NOW() + INTERVAL '5 minutes', NOW(), NOW())`,
        [
          `e2e-2fa-grant-${Date.now()}`,
          grantIdentifier,
          current.userId,
        ],
      );
      await page.getByLabel(/Kode OTP dari email/i).fill("000000");
      await page.getByRole("button", { name: /Verifikasi dan lanjutkan/i }).click();
      await expect(page.getByLabel(/QR Code untuk Google Authenticator atau Authy/i)).toBeVisible();

      // A remount must not discard the secret before the person scans it.
      await page.reload();
      await expect(page.getByLabel(/QR Code untuk Google Authenticator atau Authy/i)).toBeVisible();
    } finally {
      await query(
        `DELETE FROM verification v USING "user" u
         WHERE u.email = $1 AND v.identifier LIKE '%' || u.id || '%'`,
        [fixture.email],
      );
      await query(
        `INSERT INTO account (id, "accountId", "providerId", "userId", password, "createdAt", "updatedAt")
         SELECT $1, $2, 'credential', u.id, $3, NOW(), NOW() FROM "user" u WHERE u.email = $4
         ON CONFLICT (id) DO NOTHING`,
        [credential.id, credential.accountId, credential.password, fixture.email]);
    }
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
    await expect(page.getByText(/Harga dasar adalah yang\s+TawafiqHub tagih ke travel/i)).toBeVisible();
    await capture(page, "08-admin-supplier-costs");

    // The catalogue the platform supplies to every travel. This is the screen
    // that replaced writing product rows by hand in SQL, so it is worth
    // driving rather than trusting: the RPC being green says nothing about
    // whether a person can reach it.
    await page.getByRole("button", { name: /^Katalog$/ }).click();
    await expect(page.getByText(/dipasok TawafiqHub dan dijual oleh semua travel/i)).toBeVisible();

    // Whatever a previous run left behind — a failure before the delete below
    // leaks a row, and a leaked platform product is visible to every travel.
    await query("DELETE FROM products WHERE code LIKE 'E2E-PULSA-%' AND operator_id IS NULL", []);

    const code = `E2E-PULSA-${Date.now().toString().slice(-6)}`;
    await page.getByRole("button", { name: /Tambah Produk Platform/ }).click();
    await page.getByLabel("Nama").fill("Pulsa E2E 10K");
    await page.getByLabel("Kode").fill(code);
    await page.getByLabel("Nominal (Rp)").fill("10000");
    await page.getByLabel("Harga Dasar (Rp)").fill("10500");
    await page.getByRole("button", { name: /^Simpan$/ }).click();

    // It has to come back in the list, formatted as rupiah. Unformatted money
    // has slipped through here before — the value was right and the screen was
    // still wrong.
    await expect(page.getByText(code)).toBeVisible();
    // Rp\u00a010.500: Intl puts a non-breaking space after the symbol, so the
    // separator is matched loosely rather than asserted as a literal.
    await expect(page.getByText(/Rp\s?10\.500/).first()).toBeVisible();
    await capture(page, "14-admin-catalogue");

    await query("DELETE FROM products WHERE code = $1 AND operator_id IS NULL", [code]);

    // Accounts: the tab that removed the last need for a SQL client.
    await page.getByRole("button", { name: /^Akun$/ }).click();
    await expect(page.getByPlaceholder(/Cari nama atau email/i)).toBeVisible();
    await page.getByPlaceholder(/Cari nama atau email/i).fill("safrat.local");
    await page.getByRole("button", { name: /^Cari$/ }).click();
    await expect(page.getByText(/2FA/).first()).toBeVisible();
    await capture(page, "12-admin-accounts");

    // Identity: the list must never carry the numbers themselves.
    await page.getByRole("button", { name: /^Identitas$/ }).click();
    await expect(page.getByText(/tidak ditampilkan di daftar ini/i)).toBeVisible();
    await capture(page, "13-admin-identity");

    // Leave the fixture as found: platform access is not something to leave
    // switched on in a shared local database.
    await query("DELETE FROM platform_admins WHERE user_id = $1", [session.userId]);
    await query(`UPDATE "user" SET "twoFactorEnabled" = false WHERE id = $1`, [session.userId]);
  });
});
