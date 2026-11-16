import { execFileSync } from "node:child_process";
import { expect, test } from "@playwright/test";
import { blockingViolations } from "../axe";
import { JOURNEYS_DIR, shot } from "../shot";

const [namespace] = (process.env.FLEET_JOURNEYS_CLUSTER ?? "").split("/");
const name = "request-logs";
const request = `${name}-backup-storage`;

function kubectl(...args: string[]): string {
  return execFileSync("kubectl", args, { encoding: "utf8" });
}

// The journey reads a claim of its own and never touches the claim of the
// harness. Its cluster names the SecondaryStorageConfig of that claim, so
// the operator holds it back; the journey reads only the request of its
// backup storage, which the fake plugin of the harness serves.
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

test.afterAll(() => {
  kubectl(
    "delete",
    "camundaclusterclaim",
    name,
    "-n",
    namespace,
    "--ignore-not-found",
    "--wait",
    "--timeout=180s",
  );
});

test("the Logs tab of a section shows a line of its controller about the request", async ({
  page,
}) => {
  test.setTimeout(240_000);
  await page.goto(
    `/fleet/clusters/${namespace}/${name}?tab=overview&component=section%3Abackup-storage`,
  );
  await expect(
    page.getByRole("heading", { level: 2, name: "Backup storage" }),
  ).toBeVisible();
  await page.getByRole("tab", { name: "Logs" }).click();
  await expect(
    page.getByRole("log").getByText("Set Ready of PluginRequest").first(),
  ).toBeVisible({ timeout: 60_000 });
  expect(
    await blockingViolations(page),
    "serious or critical axe violations on the Logs tab of a section",
  ).toEqual([]);
  await shot(page, "request-logs", { dir: JOURNEYS_DIR });
});
