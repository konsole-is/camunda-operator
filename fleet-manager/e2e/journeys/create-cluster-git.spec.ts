import { expect, test } from "@playwright/test";
import { parse } from "yaml";
import { REVISION_ANNOTATION } from "../../src/lib/labels";
import { blockingViolations } from "../axe";
import { JOURNEYS_DIR, shot } from "../shot";
import {
  HARNESS_NAMESPACE,
  HARNESS_REFUSAL,
  bySection,
  createdSpec,
  deleteClaim,
  dumpClaim,
  fillCreateForm,
  kubectl,
} from "./create";
import {
  GIT_PATH,
  committedFile,
  pauseApplyLoop,
  removeCommittedFile,
  switchToDirectMode,
  switchToGitMode,
} from "./git";

// A new name on each run, so that a second run on the same harness finds
// no taken name and no file of that name in the repository.
const name = `create-git-${Date.now().toString(36)}`;
const CLAIM_FILE = `${GIT_PATH}/${HARNESS_NAMESPACE}/camundaclusterclaims/${name}.yaml`;

function claimField(jsonpath: string): string {
  return kubectl(
    "get",
    "camundaclusterclaim",
    name,
    "-n",
    HARNESS_NAMESPACE,
    "--ignore-not-found",
    "-o",
    `jsonpath={${jsonpath}}`,
  );
}

// The apply loop applies the whole folder on each new commit, so the claim
// file leaves the repository before the claim leaves the cluster. The loop
// applies a commit once, so the claim must exist before the removal: the
// loop then never applies the commit that holds the file again.
test.afterEach(async () => {
  dumpClaim(test.info(), name);
  if ((await committedFile(CLAIM_FILE)) !== undefined) {
    await expect
      .poll(() => claimField(".metadata.name"), { timeout: 120_000 })
      .toBe(name);
    removeCommittedFile(CLAIM_FILE);
  }
  deleteClaim(name);
});

// A hook gets no baseURL fixture, so the page takes the address that
// journeys.sh sets. A later journey then runs in direct mode, also when
// this one failed.
test.afterAll(async ({ browser }) => {
  const page = await browser.newPage({
    baseURL: process.env.FLEET_JOURNEYS_BASE_URL,
  });
  try {
    await switchToDirectMode(page);
  } finally {
    await page.close();
  }
});

// The operator refuses a cluster with the options of the harness claim, so
// no Camunda pod starts on kind.
test("a cluster created in Git mode waits for GitOps, then the claim appears", async ({
  page,
}) => {
  test.setTimeout(360_000);
  await switchToGitMode(page);
  await fillCreateForm(page, name);
  const push = page.getByRole("button", { name: "Push to Git" });
  await expect(push).toBeEnabled({ timeout: 30_000 });
  expect(
    await blockingViolations(page),
    "serious or critical axe violations in the Git review",
  ).toEqual([]);
  await shot(page, "create-git-review", { dir: JOURNEYS_DIR });

  const waiting = page
    .getByRole("status")
    .filter({ hasText: "Waiting for GitOps to apply" });
  // The apply loop can apply the commit right after the push, and the app
  // then ends the waiting state.
  const resumeApplyLoop = pauseApplyLoop();
  try {
    await push.click();
    await expect(waiting.filter({ hasText: /Commit [0-9a-f]{7}/ })).toBeVisible(
      { timeout: 30_000 },
    );
    await expect(
      page.getByRole("heading", { name: `Creating the cluster ${name}` }),
    ).toBeVisible();
    await shot(page, "create-git-waiting", { dir: JOURNEYS_DIR });
  } finally {
    resumeApplyLoop();
  }

  const file = await committedFile(CLAIM_FILE);
  expect(file, `${CLAIM_FILE} is on main`).toBeDefined();
  const committed = parse(file!);
  expect({ ...committed, spec: bySection(committed.spec) }).toEqual({
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "CamundaClusterClaim",
    metadata: {
      name,
      namespace: HARNESS_NAMESPACE,
      annotations: {
        [REVISION_ANNOTATION]: expect.stringMatching(/^[0-9a-f]{64}$/),
      },
    },
    spec: createdSpec,
  });

  await expect
    .poll(() => claimField(".metadata.name"), { timeout: 120_000 })
    .toBe(name);
  expect(
    claimField(
      `.metadata.annotations.${REVISION_ANNOTATION.replaceAll(".", "\\.")}`,
    ),
  ).toBe(committed.metadata.annotations[REVISION_ANNOTATION]);
  await expect(
    page
      .getByRole("status")
      .filter({ hasText: `Created the cluster ${name}.` }),
  ).toBeVisible({ timeout: 60_000 });
  await expect(waiting).toHaveCount(0);
  await expect(
    page
      .getByRole("listitem")
      .filter({ has: page.getByText("Cluster", { exact: true }) }),
  ).toContainText(`${HARNESS_REFUSAL.reason}: ${HARNESS_REFUSAL.message}`, {
    timeout: 180_000,
  });
  await shot(page, "create-git-applied", { dir: JOURNEYS_DIR });
});
