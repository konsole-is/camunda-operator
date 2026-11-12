import { blockingViolations } from "../axe";
import { expect, test } from "./fixture";

test.use({ scenarioName: "environment-named-new" });

test("an Environment named new opens its own page", async ({ page }) => {
  await page.goto("/setup/environments/new");
  await expect(
    page.getByRole("heading", { level: 1, name: "Lab" }),
  ).toBeVisible();
  await expect(
    page
      .getByRole("list", { name: "Namespaces of the environment" })
      .getByText("team-a", { exact: true }),
  ).toBeVisible();
  const crumbs = page.getByRole("navigation", { name: "Breadcrumb" });
  await expect(crumbs.locator('[aria-current="page"]')).toHaveText("new");
  expect(await blockingViolations(page)).toEqual([]);
});

test("New environment opens the create page beside the lists", async ({
  page,
}) => {
  await page.goto("/setup/environments");
  await page.getByRole("link", { name: "New environment" }).click();
  await expect(page).toHaveURL(/\/setup\/new-environment$/);
  await expect(
    page.getByRole("heading", { level: 1, name: "New environment" }),
  ).toBeVisible();
  await expect(
    page
      .getByRole("navigation", { name: "Setup" })
      .getByRole("link", { name: "Environments" }),
  ).toHaveAttribute("aria-current", "page");
});
