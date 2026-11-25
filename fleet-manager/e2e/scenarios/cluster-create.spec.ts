import type { CamundaClusterClaim } from "../../src/generated/crds/camundaclusterclaim";
import { kubeRequest } from "../../test/scenarios/kube";
import { docShot } from "./docs-shot";
import { expect, test } from "./fixture";
import { checkPage } from "./themes";

test.use({ scenarioName: "cluster-create" });

test("creates a cluster with a new namespace and a new Elasticsearch", async ({
  page,
  scenario,
}) => {
  const next = page.getByRole("button", { name: "Next" });
  await page.goto("/fleet/clusters/new");

  // Environment and name: the claim name comes first, the name of the new
  // namespace second.
  await page.getByRole("radio", { name: /Apps/ }).check();
  await page.getByRole("textbox", { name: "Name" }).first().fill("orders");
  await page.getByRole("radio", { name: "Create a namespace" }).check();
  await page.getByRole("textbox", { name: "Name" }).nth(1).fill("team-new");
  await page.getByRole("textbox", { name: "team" }).fill("orders");
  await page.getByRole("radio", { name: /^small/ }).check();
  await docShot(page, "create-cluster-environment-and-name", {
    ready: page.getByRole("radio", { name: /^small/ }),
  });
  await next.click();

  // Storage and access: the default option of Secondary storage stays.
  await expect(
    page
      .getByRole("group", { name: /Secondary storage/ })
      .getByRole("radio", { name: /New Elasticsearch cluster/ }),
  ).toBeChecked();
  await docShot(page, "create-cluster-storage-and-access", {
    ready: page.getByRole("heading", { level: 2, name: "Storage and access" }),
  });
  await next.click();

  await docShot(page, "create-cluster-settings", {
    ready: page.getByRole("heading", { level: 2, name: "Cluster settings" }),
  });
  // Cluster settings and Attachments keep their defaults.
  await next.click();
  await next.click();

  // Review
  const create = page.getByRole("button", { name: "Create" });
  await expect(
    page.getByRole("heading", { level: 2, name: "Review" }),
  ).toBeFocused();
  await expect(create).toBeEnabled();
  await expect(
    page.getByRole("figure", { name: "The namespace" }),
  ).toBeVisible();
  await checkPage(page, "cluster-create-review");
  await docShot(page, "create-cluster-review", { ready: create });
  await create.click();

  // Progress: no controller runs, so each row stays pending.
  await expect(
    page.getByRole("heading", { name: "Creating the cluster orders" }),
  ).toBeVisible();
  for (const row of [
    "Secondary storage",
    "Requests",
    "Cluster",
    "Attachments",
  ]) {
    await expect(page.getByText(row, { exact: true })).toBeVisible();
  }
  await checkPage(page, "cluster-create-progress");
  await docShot(page, "create-cluster-progress", {
    ready: page.getByRole("heading", { name: "Creating the cluster orders" }),
  });

  const namespace = await kubeRequest<{
    metadata: { labels: Record<string, string> };
  }>(scenario.admin, "GET", "/api/v1/namespaces/team-new");
  expect(namespace.metadata.labels).toMatchObject({
    "fleet.camunda.io/environment": "apps",
    team: "orders",
  });
  const claim = await kubeRequest<CamundaClusterClaim>(
    scenario.admin,
    "GET",
    "/apis/fleet.camunda.io/v1alpha1/namespaces/team-new/camundaclusterclaims/orders",
  );
  expect(claim.spec).toEqual({
    presetRef: "small",
    releaseRef: "camunda-8-9-0",
    plugins: [{ section: "secondary-storage", registration: "elasticsearch" }],
  });
  expect(claim.metadata.managedFields?.map((f) => f.manager)).toContain(
    "fleet-manager",
  );
});
