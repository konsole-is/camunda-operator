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
  field,
  fillCreateForm,
} from "./create";
import {
  GIT_PATH,
  committedFile,
  pauseApplyLoop,
  removeCommittedFile,
  restoreDirectMode,
  switchToGitMode,
} from "./git";

// A new name on each run, so that a second run on the same harness finds
// no taken name and no file of that name in the repository.
const name = `create-git-${Date.now().toString(36)}`;
const CLAIM_FILE = `${GIT_PATH}/${HARNESS_NAMESPACE}/camundaclusterclaims/${name}.yaml`;

const claimField = (jsonpath: string) =>
  field("camundaclusterclaim", name, jsonpath);

// A test that timed out reaches its finally block only after afterEach, so
// afterEach lets the apply loop go on first.
let resumeApplyLoop: (() => void) | undefined;

// The apply loop applies the whole folder on each new commit, so the claim
// file must leave the repository. A loop pass that already read the commit
// with the file can apply the claim after deleteClaim. The claim exists
// only after that pass, so the hook waits for it first.
test.afterEach(async () => {
  resumeApplyLoop?.();
  resumeApplyLoop = undefined;
  dumpClaim(test.info(), name);
  try {
    if ((await committedFile(CLAIM_FILE)) !== undefined) {
      try {
        await expect
          .poll(() => claimField(".metadata.name"), { timeout: 120_000 })
          .toBe(name);
      } finally {
        removeCommittedFile(CLAIM_FILE);
      }
    }
  } finally {
    deleteClaim(name);
  }
});

// A later journey then runs in direct mode, also when this one failed.
test.afterAll(({ browser }) => restoreDirectMode(browser));

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
  resumeApplyLoop = pauseApplyLoop();
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
    resumeApplyLoop = undefined;
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
