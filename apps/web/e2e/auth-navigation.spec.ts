import { expect, test } from "@playwright/test";

// Both public auth pages use the same session guard. This regression test
// exercises the actual client-side transition so a completed/pending probe on
// one route cannot leave the other route visually locked.
test("visitor can switch between sign-up and sign-in without a stuck auth guard", async ({ page }) => {
  await page.goto("/sign-up");
  await expect(page.getByRole("heading", { name: "Buat akun Anda" })).toBeVisible();

  await page.getByRole("link", { name: "Masuk" }).click();
  await expect(page).toHaveURL(/\/sign-in$/);
  await expect(page.getByRole("heading", { name: "Masuk ke akun Anda" })).toBeVisible();

  await page.getByRole("link", { name: "Buat akun" }).click();
  await expect(page).toHaveURL(/\/sign-up$/);
  await expect(page.getByRole("heading", { name: "Buat akun Anda" })).toBeVisible();
  await expect(page.getByText("Checking your session...")).toHaveCount(0);
});
