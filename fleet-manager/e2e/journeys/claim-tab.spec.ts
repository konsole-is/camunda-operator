import { execFileSync } from "node:child_process";
import { expect, test } from "@playwright/test";
import { blockingViolations } from "../axe";
import { JOURNEYS_DIR, shot } from "../shot";

const [namespace, name] = (process.env.FLEET_JOURNEYS_CLUSTER ?? "").split("/");

function kubectl(...args: string[]): string {
  return execFileSync("kubectl", args, { encoding: "utf8" });
}

// The harness runs no Camunda, so the journey checks the spec that the claim
// controller writes, not a version that the cluster reports.
test("a release change in the Claim tab reaches the CamundaCluster", async ({
  page,
}) => {
  expect(
    namespace && name,
    "FLEET_JOURNEYS_CLUSTER is <namespace>/<name>",
  ).toBeTruthy();
  await page.goto(`/fleet/clusters/${namespace}/${name}?tab=claim`);
  await page.getByRole("button", { name: "Edit Release and preset" }).click();
  await page
    .getByLabel("Release", { exact: true })
    .selectOption("fleet-harness-8-9-1");
  await page
    .getByRole("region", { name: "Unsaved changes" })
    .getByRole("button", { name: "Review" })
    .click();
  const save = page.getByRole("dialog").getByRole("button", { name: "Save" });
  await expect(save).toBeEnabled({ timeout: 30_000 });
  expect(
    await blockingViolations(page),
    "serious or critical axe violations in the claim review",
  ).toEqual([]);
  await shot(page, "claim-review", { dir: JOURNEYS_DIR });
  await save.click();
  await expect(
    page.getByRole("status").filter({ hasText: `The claim ${name} is saved.` }),
  ).toBeVisible();
  await expect
    .poll(
      () =>
        kubectl(
          "get",
          "camundacluster",
          name,
          "-n",
          namespace,
          "-o",
          "jsonpath={.spec.releaseRef}",
        ),
      { timeout: 120_000 },
    )
    .toBe("fleet-harness-8-9-1");
  await shot(page, "claim-saved", { dir: JOURNEYS_DIR });
});
