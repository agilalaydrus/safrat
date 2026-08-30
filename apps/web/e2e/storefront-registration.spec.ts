import { expect, test } from "@playwright/test";
import { query } from "./fixture";

/**
 * The public registration form: filled by a jamaah, often on a phone, often by
 * somebody who is not comfortable with forms. Nothing covered it before.
 *
 * What is asserted here is not layout but the things that decide whether the
 * form is usable on a phone — the keyboard that opens, whether autofill can
 * help, and whether a failure is announced rather than rendered silently above
 * a button that is already off screen.
 */
test.describe("public registration", () => {
  test("the form is usable on a phone and refuses an empty submission", async ({ page }) => {
    const [target] = await query<{ operator_id: string; season_id: string }>(
      `SELECT o.id::text AS operator_id, s.id::text AS season_id
       FROM operators o JOIN seasons s ON s.operator_id = o.id LIMIT 1`);
    if (!target) throw new Error("no operator/season to register against");

    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto(`/register/${target.operator_id}/${target.season_id}`);

    const phone = page.getByLabel("Telepon");
    await expect(phone).toBeVisible();
    // type=tel opens the number pad. Without it a jamaah types a phone number
    // on a full qwerty keyboard.
    await expect(phone).toHaveAttribute("type", "tel");
    await expect(phone).toHaveAttribute("autocomplete", "tel");

    const email = page.getByLabel("Email");
    await expect(email).toHaveAttribute("autocomplete", "email");
    // Autocapitalising an email address is how "Budi@..." reaches the server.
    await expect(email).toHaveAttribute("autocapitalize", "none");

    // A birthday cannot be in the future; the picker refuses it where the
    // mistake is made rather than after a round trip.
    const born = page.getByLabel("Tanggal lahir");
    await expect(born).toHaveAttribute("max", /\d{4}-\d{2}-\d{2}/);

    // Submitting empty is refused by the browser itself, because every required
    // field now carries the attribute. That is better than a custom message:
    // native validation focuses the offending field and says which one, rather
    // than printing one sentence at the top of a form the reader has scrolled
    // past. The assertion is that the form did not submit and the first empty
    // required field is where focus went.
    await page.getByRole("button", { name: /Kirim pendaftaran/i }).click();
    await expect(page.getByRole("button", { name: /Kirim pendaftaran/i })).toBeVisible();
    await expect(page.getByLabel("Nama lengkap (sesuai paspor)")).toBeFocused();
  });
});
