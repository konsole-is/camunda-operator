import { expect, test } from "./fixture";
import { checkPage } from "./themes";

test.use({ scenarioName: "plane-ready" });

test("the plane page shows the header and the conditions of a ready OIDC plane with Web Modeler", async ({
  page,
}) => {
  await page.goto("/fleet/planes");
  await expect(page.getByRole("row", { name: /plane-a/ })).toContainText(
    "Identity, Console, Web Modeler",
  );
  // The link of a plane names the plane and its namespace.
  await page.getByRole("link", { name: "plane-a platform" }).click();
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
    await expect(
      table.getByRole("rowheader", { name: type, exact: true }),
    ).toBeVisible();
  }
  await expect(
    table.getByRole("row", { name: /OptimizeCallbacksReady/ }),
  ).toContainText("Disabled");
  for (const type of ["IdentityReady", "WebModelerReady"]) {
    await expect(
      table.getByRole("rowheader", { name: type, exact: true }),
    ).toHaveCount(0);
  }

  await checkPage(page, "plane-overview");
});
