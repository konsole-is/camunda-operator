import { docShot } from "./docs-shot";
import { expect, test } from "./fixture";
import { checkPage } from "./themes";

test.use({ scenarioName: "claim-ready" });

test("the Plugins page groups the built-in registrations by section", async ({
  page,
}) => {
  await page.goto("/setup/plugins");
  await expect(
    page.getByRole("heading", { level: 1, name: "Plugins" }),
  ).toBeVisible();
  await expect(page.getByRole("heading", { level: 2 })).toHaveText([
    "Cloud identity",
    "Encryption key",
    "Volume encryption",
    "Backup storage",
    "Secondary storage",
    "Access",
    "Connectors permissions",
    "Databases",
    "Charts you can install",
  ]);
  const storage = page.getByRole("region", {
    name: "Secondary storage",
    exact: true,
  });
  const row = storage.getByRole("row", { name: /existing-secondary-storage/ });
  await expect(row.getByText("camunda-operator 0.0.0-dev")).toBeVisible();
  await expect(row.getByText("Production 1")).toBeVisible();
  await expect(page.getByText("6 healthy, 0 not healthy")).toBeVisible();
  const catalog = page.getByRole("region", { name: "Charts you can install" });
  await expect(catalog.getByText("aws", { exact: true })).toBeVisible();
  await expect(
    page
      .getByRole("navigation", { name: "Setup" })
      .getByRole("link", { name: "Plugins" }),
  ).toHaveAttribute("aria-current", "page");
  // Until each watch has its first snapshot, the page names the lists that
  // it has not read.
  await expect(page.getByText(/^Not read:/)).toHaveCount(0);
  await checkPage(page, "plugins");
  await docShot(page, "plugins-options-by-section", {
    ready: row,
    target: storage,
  });
  await docShot(page, "plugins-charts-to-install", {
    ready: catalog,
    target: catalog,
  });
});

test("a registration opens with its tabs and the claims that use it", async ({
  page,
}) => {
  await page.goto("/setup/plugins");
  await page
    .getByRole("region", { name: "Secondary storage", exact: true })
    .getByRole("link", { name: /existing-secondary-storage/ })
    .click();
  await expect(page).toHaveURL(/\/setup\/plugins\/existing-secondary-storage$/);
  await expect(
    page.getByRole("heading", { level: 1, name: "Existing secondary storage" }),
  ).toBeVisible();
  await expect(page.getByRole("tab")).toHaveText([
    "About",
    "Contract",
    "Parameters",
    "Grants",
    "Health",
    "Used by",
  ]);
  await page.getByRole("tab", { name: "Contract" }).click();
  await expect(
    page
      .getByRole("region", { name: "Outputs", exact: true })
      .getByText("secondaryStorageConfig", { exact: true }),
  ).toBeVisible();
  await page.getByRole("tab", { name: "Used by" }).click();
  await expect(page).toHaveURL(/\?tab=used-by$/);
  const used = page.getByRole("region", {
    name: "team-orders/orders-secondary-storage",
  });
  await expect(
    used.getByText("SecondaryStorageConfig: orders-storage, Ready"),
  ).toBeVisible();
  await expect(used.getByText("Production", { exact: true })).toBeVisible();
  await expect(used.getByText("Ready: True", { exact: true })).toBeVisible();
  // Until each watch has its first snapshot, the page names the lists that
  // it has not read.
  await expect(page.getByText(/^Not read:/)).toHaveCount(0);
  await checkPage(page, "registration-used-by");
  await page.getByRole("tab", { name: "Grants" }).click();
  await expect(
    page.getByText("This option has no grant template, so it writes no Grant."),
  ).toBeVisible();
});
