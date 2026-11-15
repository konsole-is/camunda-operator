import { expect, test } from "./fixture";
import { checkPage } from "./themes";

test.use({ scenarioName: "plane-ready" });

test("the plane page shows the header and the conditions of a ready OIDC plane", async ({
  page,
}) => {
  await page.goto("/fleet/planes");
  await page.getByRole("link", { name: "plane-a", exact: true }).click();
  await expect(page).toHaveURL(/\/fleet\/planes\/platform\/plane-a$/);

  const header = page
    .locator("header")
    .filter({ has: page.getByRole("heading", { level: 1, name: "plane-a" }) });
  await expect(header.getByText("Ready", { exact: true })).toBeVisible();
  const fact = (term: string) =>
    header
      .locator("dt", { hasText: term })
      .locator("xpath=following-sibling::dd");
  await expect(fact("Identity provider")).toHaveText("OIDC");
  await expect(fact("Platform config")).toHaveText("oidc-platform");
  await expect(fact("Serves")).toHaveText("platform");

  await expect(
    page
      .getByRole("navigation", { name: "Main" })
      .getByRole("link", { name: "Fleet" }),
  ).toHaveAttribute("aria-current", "page");

  const table = page.getByRole("region", {
    name: "Conditions outside the components",
  });
  for (const type of [
    "SecretsReady",
    "MirroredSecretsReady",
    "ServiceAccountsReady",
    "ManagementAuthReady",
    "RequestsReady",
    "ManagementClusterReady",
  ]) {
    await expect(table.getByRole("rowheader", { name: type })).toBeVisible();
  }
  await expect(
    table.getByRole("row", { name: /OptimizeCallbacksReady/ }),
  ).toContainText("Disabled");
  await expect(
    table.getByRole("rowheader", { name: "IdentityReady" }),
  ).toHaveCount(0);

  await checkPage(page, "plane-overview");
});
