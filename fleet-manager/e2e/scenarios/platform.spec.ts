import { expect, test } from "./fixture";
import { checkPage } from "./themes";

test.use({ scenarioName: "claim-ready" });

test("the Platform list shows the config with its auth method, users, and status", async ({
  page,
}) => {
  await page.goto("/setup/platform");
  const row = page.getByRole("row", { name: /camundaplatformconfig-sample/ });
  await expect(row.getByText("OIDC", { exact: true })).toBeVisible();
  await expect(row.getByRole("cell", { name: "2", exact: true })).toBeVisible();
  await expect(row.getByText("Ready", { exact: true })).toBeVisible();
  await checkPage(page, "platform-list");
});

test("the detail of a used config shows its users and offers no delete", async ({
  page,
}) => {
  await page.goto("/setup/platform/camundaplatformconfig-sample");
  await expect(
    page.getByRole("heading", {
      level: 1,
      name: "camundaplatformconfig-sample",
    }),
  ).toBeVisible();
  await expect(
    page.getByRole("link", { name: "team-orders/orders" }),
  ).toHaveAttribute("href", "/fleet/clusters/team-orders/orders");
  await expect(page.getByRole("link", { name: "production" })).toHaveAttribute(
    "href",
    "/setup/environments/production",
  );
  await expect(
    page.getByRole("list", { name: "Defaults of Connectors" }),
  ).toHaveText("camunda/connectors-bundle");
  await expect(
    page.getByText(/^Delete is possible when no cluster/),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Review the delete" }),
  ).toHaveCount(0);
  const crumbs = page.getByRole("navigation", { name: "Breadcrumb" });
  await expect(crumbs.locator('[aria-current="page"]')).toHaveText(
    "camundaplatformconfig-sample",
  );
  await checkPage(page, "platform-detail");
});
