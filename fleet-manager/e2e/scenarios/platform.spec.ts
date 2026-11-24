import type { CamundaPlatformConfig } from "../../src/generated/crds/camundaplatformconfig";
import { kubeRequest } from "../../test/scenarios/kube";
import { expect, test } from "./fixture";
import { checkPage } from "./themes";

test.use({ scenarioName: "claim-ready" });

// The tests of this file share one envtest, so they run in this order.

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

test("New platform config creates a basic config with the license Secret", async ({
  page,
  scenario,
}) => {
  await page.goto("/setup/platform");
  await page.getByRole("link", { name: "New platform config" }).click();
  await expect(page).toHaveURL(/\/setup\/new-platform-config$/);
  await expect(
    page.getByRole("heading", { level: 1, name: "New platform config" }),
  ).toBeVisible();
  await page.getByLabel("Name", { exact: true }).fill("basic-lab");
  const license = page.getByRole("group", { name: "License key" });
  await license.getByLabel("Namespace").selectOption("camunda-system");
  await license.getByLabel("Secret").selectOption("camunda-license");
  await license.getByLabel("Key").selectOption("license-key");
  await page.getByRole("button", { name: "Review" }).click();
  const create = page.getByRole("button", { name: "Create" });
  await expect(create).toBeEnabled();
  await checkPage(page, "platform-create-review");
  await create.click();
  await expect(page).toHaveURL(/\/setup\/platform\/basic-lab$/);

  const created = await kubeRequest<CamundaPlatformConfig>(
    scenario.admin,
    "GET",
    "/apis/core.camunda.io/v1/camundaplatformconfigs/basic-lab",
  );
  expect(created.spec).toEqual({
    auth: { method: "basic" },
    licenseSecretRef: {
      namespace: "camunda-system",
      name: "camunda-license",
      key: "license-key",
    },
  });
});
