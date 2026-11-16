import type { CamundaRelease } from "../../src/generated/crds/camundarelease";
import { kubeRequest } from "../../test/scenarios/kube";
import { expect, test } from "./fixture";
import { checkPage } from "./themes";

test.use({ scenarioName: "claim-ready" });

// The tests of this file share one envtest, so they run in this order.

test("the Releases list shows the facts and the usage of a release", async ({
  page,
}) => {
  await page.goto("/setup/releases");
  const row = page.getByRole("row", { name: /camunda-8-9-0/ });
  await expect(row.getByText("Default in Production")).toBeVisible();
  await expect(row.getByText("Allowed in Production")).toBeVisible();
  await expect(row.getByText("1 cluster", { exact: true })).toBeVisible();
  await expect(
    row.getByRole("checkbox", { name: "Select camunda-8-9-0" }),
  ).toBeDisabled();
  await checkPage(page, "releases-list");
});

test("the release page shows the settings, the default images, and the Environments card", async ({
  page,
}) => {
  await page.goto("/setup/releases");
  await page.getByRole("link", { name: "camunda-8-9-0" }).click();
  await expect(page).toHaveURL(/\/setup\/releases\/camunda-8-9-0$/);
  await expect(page.getByRole("tab", { name: "Settings" })).toHaveAttribute(
    "aria-selected",
    "true",
  );
  // The tag is the version of the release, not the version of the table.
  await expect(page.getByText("camunda/camunda:8.9.0")).toBeVisible();
  await expect(page.getByText("camunda/connectors-bundle:8.9.7")).toBeVisible();
  await expect(page.getByRole("note")).toContainText(
    "A change reaches every resource that uses this release.",
  );
  const card = page.getByRole("region", { name: "Environments" });
  const production = card.getByRole("link", { name: "Production" });
  await expect(production).toHaveAttribute(
    "href",
    "/setup/environments/production",
  );
  await expect(
    card.getByRole("listitem").filter({ hasText: "Production" }),
  ).toContainText("Default");
  const crumbs = page.getByRole("navigation", { name: "Breadcrumb" });
  await expect(crumbs.locator('[aria-current="page"]')).toHaveText(
    "camunda-8-9-0",
  );
  await checkPage(page, "release-settings");
});

test("New release copies a release and creates it", async ({
  page,
  scenario,
}) => {
  await page.goto("/setup/releases");
  await page.getByRole("link", { name: "New release" }).click();
  await expect(page).toHaveURL(/\/setup\/new-release$/);
  await page.getByLabel("Copy from").selectOption("camunda-8-9-0");
  await page.getByLabel("Release name").fill("camunda-8-9-1");
  await page.getByLabel("Camunda version").fill("8.9.1");
  await page.getByRole("button", { name: "Review" }).click();
  const create = page.getByRole("button", { name: "Create" });
  await expect(create).toBeEnabled();
  await checkPage(page, "release-create-review");
  await create.click();
  await expect(page).toHaveURL(/\/setup\/releases\/camunda-8-9-1$/);

  const created = await kubeRequest<CamundaRelease>(
    scenario.admin,
    "GET",
    "/apis/core.camunda.io/v1/camundareleases/camunda-8-9-1",
  );
  expect(created.spec.version).toBe("8.9.1");
  expect(created.spec.extraEnv).toEqual([
    { name: "CAMUNDA_FEATURE_FLAG", value: "true" },
  ]);

  await page.goto("/setup/releases");
  const row = page.getByRole("row", { name: /camunda-8-9-1/ });
  await expect(row.getByText("Unused")).toBeVisible();
  await expect(row.getByText("Not offered to new clusters")).toBeVisible();
  await expect(
    row.getByRole("checkbox", { name: "Select camunda-8-9-1" }),
  ).toBeEnabled();
});
