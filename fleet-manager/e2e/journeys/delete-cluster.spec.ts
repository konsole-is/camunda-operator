import { execFileSync } from "node:child_process";
import { expect, test } from "@playwright/test";
import { blockingViolations } from "../axe";
import { JOURNEYS_DIR, shot } from "../shot";

const [namespace] = (process.env.FLEET_JOURNEYS_CLUSTER ?? "").split("/");
const name = "fleet-delete";
const request = `${name}-backup-storage`;

function kubectl(...args: string[]): string {
  return execFileSync("kubectl", args, { encoding: "utf8" });
}

// The journey deletes a claim of its own and never touches the claim of the
// harness. Its cluster names the SecondaryStorageConfig of that claim, so
// the operator holds it back; the journey checks only the delete.
const claim = `apiVersion: fleet.camunda.io/v1alpha1
kind: CamundaClusterClaim
metadata:
  name: ${name}
  namespace: ${namespace}
spec:
  presetRef: fleet-harness
  releaseRef: fleet-harness-8-9-0
  suspend: true
  plugins:
    - section: backup-storage
      registration: fleet-harness
    - section: secondary-storage
      registration: existing-secondary-storage
      parameters:
        name: fleet-harness-storage
`;

test.beforeAll(async () => {
  expect(
    namespace,
    "FLEET_JOURNEYS_CLUSTER is <namespace>/<name>",
  ).toBeTruthy();
  execFileSync(
    "kubectl",
    ["apply", "--server-side", "--field-manager=fleet-manager", "-f", "-"],
    { input: claim, encoding: "utf8" },
  );
  await expect
    .poll(
      () =>
        kubectl(
          "get",
          "camundacluster",
          name,
          "-n",
          namespace,
          "--ignore-not-found",
          "-o",
          "name",
        ),
      { timeout: 300_000 },
    )
    .not.toBe("");
  await expect
    .poll(
      () =>
        kubectl(
          "get",
          "pluginrequest",
          request,
          "-n",
          namespace,
          "--ignore-not-found",
          "-o",
          "jsonpath={.status.conditions[?(@.type=='Ready')].status}",
        ),
      { timeout: 300_000 },
    )
    .toBe("True");
});

// The resources that Keep leaves behind carry the request label.
test.afterAll(() => {
  kubectl(
    "delete",
    "camundaclusterclaim",
    name,
    "-n",
    namespace,
    "--ignore-not-found",
    "--wait=false",
  );
  kubectl(
    "delete",
    "configmap,grants.plugins.camunda.io",
    "-n",
    namespace,
    "-l",
    `plugins.camunda.io/request=${request}`,
    "--ignore-not-found",
  );
  kubectl(
    "delete",
    "storageclass",
    "-l",
    `plugins.camunda.io/request=${request}`,
    "--ignore-not-found",
  );
});

test("a delete with Keep the resources removes the cluster and keeps the resource of the option without its owner", async ({
  page,
}) => {
  test.setTimeout(420_000);
  await page.goto(`/fleet/clusters/${namespace}/${name}`);
  await page.getByRole("link", { name: "Delete", exact: true }).click();
  await page.getByRole("radio", { name: /Keep the resources/ }).check();
  await expect(
    page.getByRole("row").filter({ hasText: `${request}-echo` }),
  ).toContainText("Kept");
  await page.getByLabel(`Type ${name} to confirm`).fill(name);
  const confirm = page.getByRole("button", { name: "Delete the cluster" });
  await expect(confirm).toBeEnabled({ timeout: 30_000 });
  expect(
    await blockingViolations(page),
    "serious or critical axe violations on the delete page",
  ).toEqual([]);
  await shot(page, "delete-review", { dir: JOURNEYS_DIR });
  await confirm.click();
  await expect(page.getByText(`The cluster ${name} is deleted.`)).toBeVisible({
    timeout: 300_000,
  });
  await shot(page, "delete-done", { dir: JOURNEYS_DIR });

  expect(
    kubectl(
      "get",
      "camundacluster",
      name,
      "-n",
      namespace,
      "--ignore-not-found",
      "-o",
      "name",
    ),
  ).toBe("");
  const kept = JSON.parse(
    kubectl(
      "get",
      "configmap",
      `${request}-echo`,
      "-n",
      namespace,
      "-o",
      "json",
    ),
  );
  expect(kept.metadata.labels["plugins.camunda.io/request"]).toBe(request);
  expect(kept.metadata.ownerReferences ?? []).toEqual([]);
});
