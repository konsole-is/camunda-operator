import { execFileSync } from "node:child_process";
import { expect, type Page, type TestInfo } from "@playwright/test";
import { CLAIM_LABEL } from "../../src/lib/labels";

/** HARNESS_NAMESPACE is the namespace of the harness claim and of the claims that the create journeys make. */
export const HARNESS_NAMESPACE = "fleet-smoke";

/** HARNESS_STORAGE is the SecondaryStorageConfig of the harness claim. */
export const HARNESS_STORAGE = "fleet-harness-storage";

/**
 * HARNESS_REFUSAL is how the operator refuses a cluster with the options of
 * the harness claim: HARNESS_STORAGE has no snapshot repository, and the
 * backup storage option gives the cluster a bucket. The cluster then runs
 * no Camunda pod on kind. A cluster with another Ready reason can run one.
 */
export const HARNESS_REFUSAL = {
  reason: "InvalidReference",
  message: `SecondaryStorageConfig "${HARNESS_NAMESPACE}/${HARNESS_STORAGE}" has no elasticsearch.snapshotRepository`,
};

/** kubectl runs kubectl with args and returns its output. It throws when kubectl fails. */
export function kubectl(...args: string[]): string {
  return execFileSync("kubectl", args, { encoding: "utf8" });
}

/**
 * field returns the value at jsonpath, without braces, of the object kind
 * name in HARNESS_NAMESPACE. It returns "" when the object does not exist.
 */
export function field(kind: string, name: string, jsonpath: string): string {
  return kubectl(
    "get",
    kind,
    name,
    "-n",
    HARNESS_NAMESPACE,
    "--ignore-not-found",
    "-o",
    `jsonpath={${jsonpath}}`,
  );
}

/**
 * createdSpec is the spec that fillCreateForm makes the form write for a
 * claim, with plugins in section order.
 */
export const createdSpec = {
  presetRef: "fleet-harness",
  releaseRef: "fleet-harness-8-9-0",
  plugins: [
    { section: "access", registration: "fleet-harness" },
    { section: "backup-storage", registration: "fleet-harness" },
    {
      section: "secondary-storage",
      registration: "existing-secondary-storage",
      parameters: { name: HARNESS_STORAGE },
    },
  ],
};

/**
 * bySection returns spec with its plugins in section order, so that a spec
 * compares with createdSpec whatever order the form wrote them in.
 */
export function bySection<T extends { plugins?: { section: string }[] }>(
  spec: T,
): T {
  return {
    ...spec,
    plugins: [...(spec.plugins ?? [])].sort((a, b) =>
      a.section.localeCompare(b.section),
    ),
  };
}

/**
 * fillCreateForm opens the cluster create flow and fills steps 1 to 4 with
 * the options of the harness claim: the environment Fleet harness, the
 * claim name, the existing namespace fleet-smoke, the preset fleet-harness,
 * Echo for Access and Backup storage, and Existing secondary storage with
 * HARNESS_STORAGE. Steps 3 and 4 keep their defaults. It returns on the
 * Review step.
 */
export async function fillCreateForm(page: Page, name: string): Promise<void> {
  const next = page.getByRole("button", { name: "Next" });
  const step = (label: string) =>
    expect(page.getByRole("heading", { level: 2, name: label })).toBeVisible();
  await page.goto("/fleet/clusters/new");

  await step("Environment and name");
  await page.getByRole("radio", { name: /^Fleet harness/ }).check();
  await page.getByRole("textbox", { name: "Name" }).first().fill(name);
  await page.getByRole("radio", { name: "Use an existing namespace" }).check();
  await page
    .getByRole("combobox", { name: "Namespace" })
    .selectOption(HARNESS_NAMESPACE);
  await page.getByRole("radio", { name: /^fleet-harness/ }).check();
  await next.click();

  await step("Storage and access");
  for (const section of ["Access", "Backup storage"]) {
    await page
      .getByRole("group", { name: section, exact: true })
      .getByRole("radio", { name: /^Echo/ })
      .check();
  }
  const storage = page.getByRole("group", { name: /^Secondary storage/ });
  await storage
    .getByRole("radio", { name: /^Existing secondary storage/ })
    .check();
  await storage
    .getByRole("combobox", { name: "Name" })
    .selectOption({ label: HARNESS_STORAGE });
  await next.click();

  await step("Cluster settings");
  await next.click();
  await step("Attachments");
  await next.click();
  await step("Review");
}

/**
 * dumpClaim prints the claim name in HARNESS_NAMESPACE, its CamundaCluster,
 * and its PluginRequests to the log when the test of testInfo failed. Call
 * it before a cleanup deletes them.
 */
export function dumpClaim(testInfo: TestInfo, name: string): void {
  if (testInfo.status === testInfo.expectedStatus) return;
  for (const args of [
    ["camundaclusterclaim", name],
    ["camundacluster", name],
    ["pluginrequests", "-l", `${CLAIM_LABEL}=${name}`],
  ]) {
    try {
      process.stdout.write(
        kubectl("get", ...args, "-n", HARNESS_NAMESPACE, "-o", "yaml"),
      );
    } catch (error) {
      process.stdout.write(`could not read ${args.join(" ")}: ${error}\n`);
    }
  }
}

/**
 * deleteClaim deletes the claim name in HARNESS_NAMESPACE and waits until
 * it is gone, with its cluster and its requests. It also deletes what the
 * fake plugin made for each request and left behind.
 */
export function deleteClaim(name: string): void {
  kubectl(
    "delete",
    "camundaclusterclaim",
    name,
    "-n",
    HARNESS_NAMESPACE,
    "--ignore-not-found",
    "--timeout=180s",
  );
  const requests = `plugins.camunda.io/request in (${name}-access,${name}-backup-storage)`;
  kubectl(
    "delete",
    "configmap,ingress,grants.plugins.camunda.io",
    "-n",
    HARNESS_NAMESPACE,
    "-l",
    requests,
    "--ignore-not-found",
  );
  kubectl("delete", "storageclass", "-l", requests, "--ignore-not-found");
}
