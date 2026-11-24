import type { ElasticsearchClusterPreset } from "../../src/generated/crds/elasticsearchclusterpreset";
import { kubeRequest } from "../../test/scenarios/kube";
import { expect, test } from "./fixture";
import { checkPage } from "./themes";

test.use({ scenarioName: "claim-ready" });

// The tests of this file share one envtest, so they run in this order.

test("the Presets page shows the preset of a claim as used and offers no delete", async ({
  page,
}) => {
  await page.goto("/setup/presets");
  const row = page.getByRole("row", { name: /medium/ });
  await expect(row.getByText("Cluster", { exact: true })).toBeVisible();
  await expect(row.getByText("1 cluster, 1 claim")).toBeVisible();
  await expect(row.getByText("production", { exact: true })).toBeVisible();
  await expect(row.getByRole("checkbox")).toBeDisabled();
  await checkPage(page, "presets-list");
});

test("a preset page names the preset and lists the cluster that inherits it", async ({
  page,
}) => {
  await page.goto("/setup/presets");
  await page.getByRole("link", { name: "medium" }).click();
  await expect(page).toHaveURL(/\/setup\/presets\/cluster\/medium$/);
  await expect(
    page.getByRole("heading", { level: 1, name: "medium" }),
  ).toBeVisible();
  const crumbs = page.getByRole("navigation", { name: "Breadcrumb" });
  await expect(crumbs.locator('[aria-current="page"]')).toHaveText("medium");
  const inheriting = page
    .getByRole("region", { name: "Inheriting resources" })
    .getByRole("row", { name: /orders/ });
  await expect(inheriting.getByText("Production")).toBeVisible();
  await expect(inheriting.getByText("team-orders")).toBeVisible();
  await expect(inheriting.getByText("Ready", { exact: true })).toBeVisible();
  await expect(
    page.getByText(
      "Delete is possible when no resource, claim, or request uses the preset.",
    ),
  ).toBeVisible();
  await checkPage(page, "presets-detail");
});

test("an edit of the preset of a claim previews a roll of its cluster", async ({
  page,
}) => {
  await page.goto("/setup/presets/cluster/medium");
  const replicas = page
    .getByRole("region", { name: "Brokers settings" })
    .getByRole("group", { name: "replicas" })
    .getByRole("spinbutton");
  await replicas.fill("5");
  await page.getByRole("button", { name: "Review", exact: true }).click();
  const review = page.getByRole("region", { name: "Review the preset" });
  await expect(review.getByText("team-orders/orders")).toBeVisible();
  await expect(review.getByText("Pods roll")).toBeVisible();
  await expect(
    review.getByText(
      "The brokers restart one at a time. The other processes restart.",
    ),
  ).toBeVisible();
  await checkPage(page, "presets-edit-review");
});

test("New preset creates an Elasticsearch preset with one field", async ({
  page,
  scenario,
}) => {
  await page.goto("/setup/presets");
  await page.getByRole("link", { name: "New preset" }).click();
  await expect(page).toHaveURL(/\/setup\/new-preset$/);
  await expect(
    page.getByRole("heading", { level: 1, name: "New preset" }),
  ).toBeVisible();
  await page.getByRole("radio", { name: "Elasticsearch" }).check();
  await page.getByLabel("Name", { exact: true }).fill("search-lab");
  await page
    .getByRole("region", { name: "Elasticsearch settings" })
    .getByRole("group", { name: "replicas" })
    .getByRole("spinbutton")
    .fill("2");
  await checkPage(page, "presets-create");
  await page.getByRole("button", { name: "Review", exact: true }).click();
  const create = page.getByRole("button", { name: "Create" });
  await expect(create).toBeEnabled();
  await checkPage(page, "presets-create-review");
  await create.click();
  await expect(page).toHaveURL(/\/setup\/presets\/elasticsearch\/search-lab$/);

  const created = await kubeRequest<ElasticsearchClusterPreset>(
    scenario.admin,
    "GET",
    "/apis/core.camunda.io/v1/elasticsearchclusterpresets/search-lab",
  );
  expect(created.spec).toEqual({ cluster: { replicas: 2 } });
});
