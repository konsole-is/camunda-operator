import { blockingViolations } from "../axe";
import { expect, test } from "./fixture";
import { checkPage, inBothThemes } from "./themes";

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
  await inBothThemes(page, async (theme) => {
    expect(
      await blockingViolations(page),
      `axe violations in the ${theme} theme`,
    ).toEqual([]);
  });
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

test("an allowed built-in option warns at its required preset", async ({
  page,
}) => {
  await page.goto("/setup/environments/new");
  await page
    .getByRole("region", { name: "Clusters" })
    .getByRole("group", { name: "Secondary storage" })
    .getByRole("checkbox", { name: "New Elasticsearch cluster" })
    .click();
  const option = page.getByRole("region", {
    name: "New Elasticsearch cluster",
  });
  await expect(
    option
      .getByRole("group", { name: "Preset" })
      .getByText(
        "This parameter needs a value. A new claim that picks this option is refused until you give it a value or let a claim change it.",
      ),
  ).toBeVisible();
  await checkPage(page, "environment-required-parameter");
});
