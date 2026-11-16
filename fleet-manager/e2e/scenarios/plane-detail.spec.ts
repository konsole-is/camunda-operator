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

test("the Overview shows a card for each component that runs", async ({
  page,
}) => {
  await page.goto("/fleet/planes/platform/plane-a");
  const cards = page.getByRole("region", { name: "Components" });
  const provider = cards.getByRole("article", { name: "Identity provider" });
  await expect(provider).toContainText("OIDC");
  await expect(provider).toContainText(
    "https://login.example.com/realms/camunda",
  );
  await expect(provider.getByRole("link")).toHaveCount(0);
  for (const label of [
    "Management Identity",
    "Console",
    "Web Modeler REST API",
    "Web Modeler websockets",
  ]) {
    await expect(
      cards.getByRole("link", { name: label, exact: true }),
    ).toBeVisible();
  }
  await expect(cards.getByRole("article", { name: "Keycloak" })).toHaveCount(0);
  await expect(
    cards.getByRole("img", { name: "Management Identity: Ready" }),
  ).toBeVisible();
  await checkPage(page, "plane-overview-cards");
});

test("a card opens the component detail with its condition and its settings", async ({
  page,
}) => {
  await page.goto("/fleet/planes/platform/plane-a");
  await page
    .getByRole("region", { name: "Components" })
    .getByRole("link", { name: "Management Identity", exact: true })
    .click();
  await expect(page).toHaveURL(/\?tab=overview&component=identity$/);
  const detail = page.getByRole("region", { name: "Management Identity" });
  await expect(
    detail.getByRole("heading", { name: "Management Identity" }),
  ).toBeVisible();
  await expect(
    detail.getByRole("list", { name: "Management Identity conditions" }),
  ).toContainText("IdentityReady");
  await checkPage(page, "plane-identity-detail");

  await detail.getByRole("tab", { name: "Settings" }).click();
  const claimName = detail
    .locator("dt", { hasText: "Administrator claim name" })
    .locator("xpath=following-sibling::dd");
  await expect(claimName).toContainText("email");
  await expect(claimName).toContainText("Fixed after Identity started");

  await page.getByRole("link", { name: "All components" }).click();
  await expect(page.getByRole("region", { name: "Components" })).toBeVisible();
});
