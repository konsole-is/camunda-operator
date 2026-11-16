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
  await expect(catalog.getByText("aws-kms", { exact: true })).toBeVisible();
  await expect(
    page
      .getByRole("navigation", { name: "Setup" })
      .getByRole("link", { name: "Plugins" }),
  ).toHaveAttribute("aria-current", "page");
  // Until each watch has its first snapshot, the page names the lists that
  // it has not read.
  await expect(page.getByText(/^Not read:/)).toHaveCount(0);
  await checkPage(page, "plugins");
});
