import { expect, test } from "@playwright/test";
import path from "node:path";
import { enrolFixtureStaff, loadWebEnv, operatorID, query, unenrolFixtureStaff } from "./fixture";

loadWebEnv();

const heroFixture = path.join(__dirname, "..", "public", "images", "tenant-umrah-hero.webp");

type AssetRow = { object_key: string; public_url: string; state: string; kind: string; size_bytes: string };

test.describe.configure({ mode: "serial" });

/** Reads the "N file aktif" figure the CMS shows for the storage quota. */
async function activeFileCount(page: import("@playwright/test").Page): Promise<number> {
  const label = page.getByText(/\d+ file aktif/).first();
  await expect(label).toBeVisible();
  const text = (await label.textContent()) ?? "";
  return Number(/(\d+) file aktif/.exec(text)?.[1] ?? NaN);
}

// Staff cannot reach the dashboard without a second factor. This spec drives
// staff screens, so the gate applies to it — it was simply never updated when
// the requirement landed, and every test here has been redirected to the
// enrolment page since.
test.beforeEach(enrolFixtureStaff);
test.afterAll(unenrolFixtureStaff);

test("uploading a hero image runs the full presign, verify, promote, register chain", async ({ page }) => {
  const operator = await operatorID();
  await page.goto("/dashboard/settings");
  await expect(page.getByText("Landing page travel")).toBeVisible();

  // Relative to whatever this fixture already holds, so repeat runs stay
  // deterministic without wiping state the previous run left behind.
  const activeFilesBefore = await activeFileCount(page);

  const heroField = page.locator('label:has(> span:text-is("Foto hero"))');
  await heroField.locator('input[type="file"]').setInputFiles(heroFixture);

  // The browser converts to WebP and PUTs to the presigned URL; the API then
  // HEADs, downloads, signature- and dimension-checks, and promotes it. The
  // resolved public URL only appears once ConfirmStorefrontUpload has returned.
  const preview = heroField.locator("img");
  await expect(preview).toHaveAttribute("src", /\/storefront\/.+\.webp$/, { timeout: 45_000 });
  const publicURL = await preview.getAttribute("src");
  expect(publicURL).toBeTruthy();

  // The object is really in the bucket, publicly readable, and still a WebP.
  const object = await page.request.get(publicURL!);
  expect(object.status()).toBe(200);
  expect(object.headers()["content-type"]).toBe("image/webp");

  // ...and it is registered, so it counts against the quota and the cleanup
  // worker can manage it.
  const assets = await query<AssetRow>(
    "SELECT object_key, public_url, state, kind, size_bytes FROM operator_storefront_assets WHERE operator_id = $1 AND public_url = $2",
    [operator, publicURL],
  );
  expect(assets).toHaveLength(1);
  const asset = assets[0]!;
  expect(asset.state).toBe("LIVE");
  expect(asset.kind).toBe("hero");
  expect(Number(asset.size_bytes)).toBeGreaterThan(0);

  // The pending copy is deliberately left for the lifecycle rule to expire, so
  // a failed registry write can be retried. Confirm the promoted key is live.
  expect(asset.object_key).toMatch(/^storefront\//);

  // The CMS reports the new usage back to the operator — but only after a
  // reload, save, or publish. setStorageUsage in OperatorProfilePanel is called
  // from load/saveDraft/publish and never from the upload handler, so the
  // indicator is stale for the rest of the session immediately after an upload.
  expect(await activeFileCount(page)).toBe(activeFilesBefore);
  await page.reload();
  await expect(page.getByText("Landing page travel")).toBeVisible();
  expect(await activeFileCount(page)).toBe(activeFilesBefore + 1);
});

test("a draft is saved, published, and only then visible to the public renderer", async ({ page }) => {
  const operator = await operatorID();
  await page.goto("/dashboard/settings");
  await expect(page.getByText("Landing page travel")).toBeVisible();

  const tagline = `E2E draft ${Date.now()}`;
  const heroTitle = page.locator('label:has(> span:text-is("Judul utama"))').locator("textarea").first();
  await heroTitle.fill(tagline);

  await page.getByRole("button", { name: /Simpan Draft/ }).click();
  await expect(page.getByText(/Draft tersimpan/)).toBeVisible();

  // Saved to draft only — the published snapshot must not have moved yet.
  const afterDraft = await query<{ draft: string; published: string | null }>(
    "SELECT draft::text AS draft, published::text AS published FROM operator_storefronts WHERE operator_id = $1",
    [operator],
  );
  expect(afterDraft).toHaveLength(1);
  const draftRow = afterDraft[0]!;
  expect(draftRow.draft).toContain(tagline);
  expect(draftRow.published ?? "").not.toContain(tagline);

  await page.getByRole("button", { name: /Publikasikan/ }).click();
  await expect(page.getByText(/berhasil dipublikasikan/)).toBeVisible();

  const afterPublish = await query<{ published: string | null }>(
    "SELECT published::text AS published FROM operator_storefronts WHERE operator_id = $1",
    [operator],
  );
  expect(afterPublish[0]?.published ?? "").toContain(tagline);
});
