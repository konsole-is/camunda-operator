import { kubeRequest } from "../../test/scenarios/kube";
import { docShot } from "./docs-shot";
import { expect, test } from "./fixture";
import { checkPage } from "./themes";

test.use({ scenarioName: "release-rollout" });

const claimPath =
  "/apis/fleet.camunda.io/v1alpha1/namespaces/team-orders/camundaclusterclaims/orders";

// The tests share the app and run in order: the first one allows
// camunda-8-9-1 in Production, and the second one moves the claim again.

test("an Environment that allows a release lets the Rollout tab move a claim to it", async ({
  page,
  scenario,
}) => {
  await page.goto("/setup/environments/production");
  await docShot(page, "environment-storage-and-access-options", {
    ready: page.getByRole("group", { name: "Allowed releases" }),
    target: page
      .locator("details")
      .filter({ has: page.locator("summary", { hasText: /^Sections$/ }) }),
  });
  await page
    .getByRole("group", { name: "Allowed releases" })
    .getByRole("checkbox", { name: "camunda-8-9-1" })
    .click();
  await page
    .getByRole("region", { name: "Unsaved changes" })
    .getByRole("button", { name: "Review" })
    .click();
  const save = page.getByRole("button", { name: "Save" });
  await expect(save).toBeEnabled();
  await save.click();
  await expect(
    page.getByRole("status").filter({
      hasText: "The environment production is saved.",
    }),
  ).toBeVisible();

  await page.goto("/setup/releases/camunda-8-9-1?tab=rollout");
  await expect(page.getByRole("tab", { name: "Rollout" })).toHaveAttribute(
    "aria-selected",
    "true",
  );
  const row = page.getByRole("row", { name: /orders/ });
  await expect(row.getByText("Can move.")).toBeVisible();
  await expect(
    row.getByRole("checkbox", { name: "Select team-orders/orders" }),
  ).not.toBeChecked();
  await checkPage(page, "release-rollout");
  await docShot(page, "release-rollout", { ready: row });

  await row.getByRole("button", { name: "Move" }).click();
  await expect(
    page.getByText(/team-orders\/orders: .*The brokers restart one at a time/),
  ).toBeVisible();
  const move = page
    .getByRole("region", { name: "Review the move" })
    .getByRole("button", { name: "Move" });
  await expect(move).toBeEnabled();
  await move.click();
  await expect(
    page.getByRole("status").filter({
      hasText: "Moved team-orders/orders to camunda-8-9-1.",
    }),
  ).toBeVisible();

  const claim = await kubeRequest<{ spec: { releaseRef?: string } }>(
    scenario.admin,
    "GET",
    claimPath,
  );
  expect(claim.spec.releaseRef).toBe("camunda-8-9-1");
  // No controller runs during a page scenario, so the cluster keeps its
  // release: the page wrote only the claim.
  const cluster = await kubeRequest<{ spec: { releaseRef?: string } }>(
    scenario.admin,
    "GET",
    "/apis/core.camunda.io/v1/namespaces/team-orders/camundaclusters/orders",
  );
  expect(cluster.spec.releaseRef).toBe("camunda-8-9-0");
});

test("a move of a claim whose release kubectl set asks to take the field over", async ({
  page,
  scenario,
}) => {
  // kubectl apply --server-side sets the release back, so kubectl owns
  // spec.releaseRef.
  await kubeRequest(
    scenario.admin,
    "PATCH",
    `${claimPath}?fieldManager=kubectl&force=true`,
    {
      apiVersion: "fleet.camunda.io/v1alpha1",
      kind: "CamundaClusterClaim",
      metadata: { name: "orders", namespace: "team-orders" },
      spec: { releaseRef: "camunda-8-9-0" },
    },
    "application/apply-patch+yaml",
  );

  await page.goto("/setup/releases/camunda-8-9-1?tab=rollout");
  const row = page.getByRole("row", { name: /orders/ });
  await expect(row.getByText("Can move.")).toBeVisible();
  await row.getByRole("button", { name: "Move" }).click();
  const review = page.getByRole("region", { name: "Review the move" });
  const fields = review.getByRole("group", {
    name: "Fields that another tool set",
  });
  await expect(fields).toContainText(
    "spec.releaseRef: kubectl set it. The save changes it.",
  );
  const move = review.getByRole("button", { name: "Move" });
  await expect(move).toBeDisabled();
  await checkPage(page, "release-rollout-take-over");

  await fields
    .getByRole("checkbox", { name: "Take over these fields" })
    .click();
  await expect(move).toBeEnabled();
  await move.click();
  await expect(
    page.getByRole("status").filter({
      hasText: "Moved team-orders/orders to camunda-8-9-1.",
    }),
  ).toBeVisible();

  const claim = await kubeRequest<{
    metadata: { managedFields: { manager: string; fieldsV1: object }[] };
    spec: { releaseRef?: string };
  }>(scenario.admin, "GET", claimPath);
  expect(claim.spec.releaseRef).toBe("camunda-8-9-1");
  const kubectl = claim.metadata.managedFields.find(
    (e) => e.manager === "kubectl",
  );
  expect(JSON.stringify(kubectl?.fieldsV1 ?? {})).not.toContain("f:releaseRef");
});
