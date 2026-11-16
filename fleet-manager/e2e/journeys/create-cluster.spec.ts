import { expect, test } from "@playwright/test";
import { blockingViolations } from "../axe";
import { JOURNEYS_DIR, shot } from "../shot";
import {
  HARNESS_NAMESPACE,
  bySection,
  createdSpec,
  deleteClaim,
  dumpClaim,
  fillCreateForm,
  kubectl,
} from "./create";

// A new name on each run, so that a second run on the same harness finds
// no taken name.
const name = `create-${Date.now().toString(36)}`;

function field(kind: string, jsonpath: string): string {
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

test.afterEach(() => {
  dumpClaim(test.info(), name);
  deleteClaim(name);
});

// The new cluster names the secondary storage that camundacluster-sample
// holds, so the operator keeps it at zero and no Camunda pod starts on kind.
// So the journey cannot wait for the cluster to be ready.
test("a cluster created through the form gets its sections, its requests, and the access URL", async ({
  page,
}) => {
  test.setTimeout(360_000);
  await fillCreateForm(page, name);
  const create = page.getByRole("button", { name: "Create", exact: true });
  await expect(create).toBeEnabled({ timeout: 30_000 });
  expect(
    await blockingViolations(page),
    "serious or critical axe violations in the review",
  ).toEqual([]);
  await shot(page, "create-review", { dir: JOURNEYS_DIR });
  await create.click();

  await expect(
    page.getByRole("heading", { name: `Creating the cluster ${name}` }),
  ).toBeVisible();
  await expect(
    page
      .getByRole("status")
      .filter({ hasText: `Created the cluster ${name}.` }),
  ).toBeVisible();
  const claim = JSON.parse(
    kubectl(
      "get",
      "camundaclusterclaim",
      name,
      "-n",
      HARNESS_NAMESPACE,
      "-o",
      "json",
    ),
  );
  expect(bySection(claim.spec)).toEqual(createdSpec);

  await expect
    .poll(() => field("camundaclusterclaim", ".status.requests[*].ready"), {
      timeout: 180_000,
    })
    .toBe("True True True");
  await expect
    .poll(
      () =>
        field(
          "camundaclusterclaim",
          ".status.conditions[?(@.type=='RequestsReady')].status",
        ),
      { timeout: 60_000 },
    )
    .toBe("True");
  // The guard: a cluster that does not report StorageAlreadyAttached would
  // start Camunda on kind.
  await expect
    .poll(
      () =>
        field(
          "camundacluster",
          ".status.conditions[?(@.type=='Ready')].reason",
        ),
      { timeout: 180_000 },
    )
    .toBe("StorageAlreadyAttached");
  // The fake plugin gives every access request the same URL, so the cluster
  // is compared with the output of its own request.
  const url = kubectl(
    "get",
    "pluginrequest",
    `${name}-access`,
    "-n",
    HARNESS_NAMESPACE,
    "-o",
    "jsonpath={.status.outputs.externalUrl}",
  );
  expect(url).not.toBe("");
  expect(field("camundacluster", ".spec.externalUrl")).toBe(url);

  const row = (label: string) =>
    page
      .getByRole("listitem")
      .filter({ has: page.getByText(label, { exact: true }) });
  for (const section of ["Access", "Backup storage", "Secondary storage"]) {
    await expect(
      row(section).getByRole("img", { name: "Ready: Ready" }),
    ).toBeVisible({ timeout: 60_000 });
  }
  await expect(row("Cluster")).toContainText("StorageAlreadyAttached", {
    timeout: 60_000,
  });
  expect(
    await blockingViolations(page),
    "serious or critical axe violations in the progress",
  ).toEqual([]);
  await shot(page, "create-progress", { dir: JOURNEYS_DIR });
});
