import { createHmac } from "node:crypto";
import { expect, test } from "@playwright/test";
import { loadWebEnv, query } from "./fixture";

const account = {
  email: "e2e-two-factor@safrat.local",
  password: "E2eTwoFactor!2026",
  name: "Keamanan E2E",
};

test.beforeAll(() => loadWebEnv());

function decodeBase32(value: string): Buffer {
  const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567";
  const clean = value.toUpperCase().replace(/=+$/g, "");
  let bits = "";
  for (const char of clean) {
    const index = alphabet.indexOf(char);
    if (index < 0) throw new Error(`invalid base32 character: ${char}`);
    bits += index.toString(2).padStart(5, "0");
  }
  const bytes: number[] = [];
  for (let offset = 0; offset + 8 <= bits.length; offset += 8) {
    bytes.push(Number.parseInt(bits.slice(offset, offset + 8), 2));
  }
  return Buffer.from(bytes);
}

function currentTotp(uri: string): string {
  const secret = new URL(uri).searchParams.get("secret");
  if (!secret) throw new Error("TOTP URI did not contain a secret");
  const counter = Math.floor(Date.now() / 30_000);
  const message = Buffer.alloc(8);
  message.writeBigUInt64BE(BigInt(counter));
  const digest = createHmac("sha1", decodeBase32(secret)).update(message).digest();
  const offset = (digest[digest.length - 1] ?? 0) & 0x0f;
  const binary = ((digest[offset] ?? 0) & 0x7f) << 24
    | ((digest[offset + 1] ?? 0) & 0xff) << 16
    | ((digest[offset + 2] ?? 0) & 0xff) << 8
    | ((digest[offset + 3] ?? 0) & 0xff);
  return String(binary % 1_000_000).padStart(6, "0");
}

async function cleanAccount() {
  const users = await query<{ id: string }>('SELECT id FROM "user" WHERE email = $1', [account.email]);
  for (const user of users) {
    await query("DELETE FROM audit_logs WHERE user_id = $1", [user.id]);
    await query("DELETE FROM verification WHERE identifier LIKE '%' || $1 || '%'", [user.id]);
  }
  await query('DELETE FROM "user" WHERE email = $1', [account.email]);
}

test("backup code login and sensitive 2FA management require a step-up", async ({ page }) => {
  await cleanAccount();
  try {
    // The pending Google challenge has no authenticated session, so this route
    // must remain reachable through middleware before its signed cookie is
    // verified by Better Auth.
    await page.goto("/two-factor-challenge");
    await expect(page.getByText(/Login Google berhasil/i)).toBeVisible();

    await page.goto("/sign-in");
    const signUpStatus = await page.evaluate(async (body) => {
      const response = await fetch("/api/auth/sign-up/email", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      });
      return response.status;
    }, account);
    expect(signUpStatus).toBe(200);
    await query('UPDATE "user" SET "emailVerified" = true WHERE email = $1', [account.email]);

    await page.getByLabel(/Email/i).fill(account.email);
    await page.getByLabel(/Kata Sandi/i).fill(account.password);
    await page.getByRole("button", { name: /^Masuk$/ }).click();
    await expect(page).toHaveURL(/\/onboarding/);

    await page.goto("/keamanan");
    await page.getByLabel(/Kata sandi akun/i).fill(account.password);
    await page.getByRole("button", { name: /^Lanjutkan$/ }).click();
    await expect(page.getByLabel(/QR Code untuk Google Authenticator atau Authy/i)).toBeVisible();
    await page.getByText(/Tidak bisa memindai/i).click();
    const uri = await page.locator("details code").textContent();
    if (!uri) throw new Error("manual TOTP URI was not rendered");
    await page.getByLabel(/Kode 6 digit dari aplikasi/i).fill(currentTotp(uri));
    await page.getByRole("button", { name: /^Aktifkan$/ }).click();
    await expect(page.getByText(/Berhasil diaktifkan/i)).toBeVisible();

    await page.reload();
    const managementBypassStatus = await page.evaluate(async (password) => {
      const response = await fetch("/api/auth/two-factor/generate-backup-codes", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ password }),
      });
      return response.status;
    }, account.password);
    expect(managementBypassStatus).toBe(403);

    await page.getByRole("button", { name: /^Kelola 2FA$/ }).click();
    await page.getByLabel("Kode authenticator").fill(currentTotp(uri));
    await page.getByRole("button", { name: /^Verifikasi$/ }).click();
    await page.getByLabel(/Konfirmasi kata sandi akun/i).fill(account.password);
    await page.getByRole("button", { name: /Buat kode cadangan baru/i }).click();
    await expect(page.getByText(/Kode cadangan lama sudah tidak berlaku/i)).toBeVisible();
    const replacementCode = (await page.locator("code").allTextContents()).find((value) => /^[A-Za-z0-9]{5}-[A-Za-z0-9]{5}$/.test(value));
    if (!replacementCode) throw new Error("replacement backup codes were not rendered");

    await page.evaluate(async () => {
      await fetch("/api/auth/sign-out", { method: "POST", headers: { "Content-Type": "application/json" }, body: "{}" });
    });
    await page.goto("/sign-in");
    await page.getByLabel(/Email/i).fill(account.email);
    await page.getByLabel(/Kata Sandi/i).fill(account.password);
    await page.getByRole("button", { name: /^Masuk$/ }).click();
    await expect(page.getByText(/Verifikasi dua langkah/i)).toBeVisible();
    await page.getByLabel("Kode authenticator").fill("000000");
    await page.getByRole("button", { name: /^Verifikasi$/ }).click();
    await expect(page.getByText(/Kode tidak cocok\. Gunakan kode terbaru/i)).toBeVisible();
    await expect(page).toHaveURL(/\/sign-in/);
    await page.getByRole("button", { name: /Gunakan kode cadangan/i }).click();
    await page.getByLabel("Kode cadangan").fill(replacementCode);
    await page.getByRole("button", { name: /^Verifikasi$/ }).click();
    await expect(page).toHaveURL(/\/onboarding/);

    // Turn the still-authenticated fixture into the shape of a Google-only
    // account. Management must remain protected by TOTP while no longer
    // asking for a password that does not exist.
    await query(
      `DELETE FROM account a USING "user" u
       WHERE a."userId" = u.id AND u.email = $1 AND a."providerId" = 'credential'`,
      [account.email],
    );
    await page.goto("/keamanan");
    await page.getByRole("button", { name: /^Kelola 2FA$/ }).click();
    await page.getByLabel("Kode authenticator").fill(currentTotp(uri));
    await page.getByRole("button", { name: /^Verifikasi$/ }).click();
    await expect(page.getByLabel(/Konfirmasi kata sandi akun/i)).toHaveCount(0);
    await page.getByRole("button", { name: /Nonaktifkan dan pasang ulang/i }).click();
    await expect(page).toHaveURL(/\/keamanan/);
    await expect(page.getByText(/Langkah 1 — konfirmasi kata sandi/i)).toBeVisible();

    const [user] = await query<{ id: string }>('SELECT id FROM "user" WHERE email = $1', [account.email]);
    if (!user) throw new Error("2FA fixture user disappeared");
    const audits = await query<{ action: string }>(
      "SELECT action FROM audit_logs WHERE user_id = $1 ORDER BY created_at",
      [user.id],
    );
    expect(audits.map((entry) => entry.action)).toEqual(expect.arrayContaining([
      "two_factor_step_up_verified",
      "two_factor_backup_codes_regenerated",
      "two_factor_login_verified",
      "two_factor_disabled",
    ]));
  } finally {
    await cleanAccount();
  }
});
