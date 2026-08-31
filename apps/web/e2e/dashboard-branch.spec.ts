import { expect, test } from "@playwright/test";

test("branch dashboard exposes live summary, actions and an accessible create drawer", async ({ page }) => {
  await page.goto("/dashboard/cabang");

  await expect(page.getByRole("heading", { name: "Cabang", exact: true })).toBeVisible();
  await expect(page.getByText(/cabang · .* agen aktif · .* jamaah terealisasi/)).toBeVisible();
  await expect(page.getByText("Omzet jaringan")).toBeVisible();
  await expect(page.getByRole("heading", { name: "Papan Peringkat" })).toBeVisible();
  await expect(page.getByRole("region", { name: "Pusat Aksi Cabang" })).toBeVisible();

  await page.getByRole("button", { name: "Tambah cabang" }).click();
  const drawer = page.getByRole("dialog", { name: "Tambah cabang" });
  await expect(drawer).toBeVisible();
  await expect(drawer.getByLabel("Nama cabang")).toBeFocused();
  await drawer.getByLabel("Nama cabang").fill("Cabang Uji Browser");
  await page.keyboard.press("Escape");
  await expect(drawer).toBeHidden();
});
