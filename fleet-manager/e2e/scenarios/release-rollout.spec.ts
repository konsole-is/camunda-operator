import { kubeRequest } from "../../test/scenarios/kube";
import { expect, test } from "./fixture";
import { checkPage } from "./themes";

test.use({ scenarioName: "release-rollout" });

test("an Environment that allows a release lets the Rollout tab move a claim to it", async ({
  page,
  scenario,
}) => {
  await page.goto("/setup/environments/production");
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
    "/apis/fleet.camunda.io/v1alpha1/namespaces/team-orders/camundaclusterclaims/orders",
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
