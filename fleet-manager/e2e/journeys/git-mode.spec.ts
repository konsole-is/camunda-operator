import { execFileSync } from "node:child_process";
import path from "node:path";
import { expect, test } from "@playwright/test";
import { parse } from "yaml";
import { REVISION_ANNOTATION } from "../../src/lib/labels";
import { blockingViolations } from "../axe";
import { JOURNEYS_DIR, shot } from "../shot";
import {
  GIT_PATH,
  committedFile,
  pauseApplyLoop,
  switchToDirectMode,
  switchToGitMode,
} from "./git";

const NAMESPACE = "fleet-smoke";
const NAME = "git-mode-sample";
const CLAIM_FILE = `${GIT_PATH}/${NAMESPACE}/camundaclusterclaims/${NAME}.yaml`;
const REVISION_PATH = `{.metadata.annotations.${REVISION_ANNOTATION.replaceAll(".", "\\.")}}`;

function kubectl(...args: string[]): string {
  return execFileSync("kubectl", args, { encoding: "utf8" });
}

function claimField(jsonpath: string): string {
  return kubectl(
    "get",
    "camundaclusterclaim",
    NAME,
    "-n",
    NAMESPACE,
    "-o",
    `jsonpath=${jsonpath}`,
  );
}

test.beforeAll(async () => {
  // A second apply would drop the revision of a pushed save from the live
  // claim, and the writer would then refuse the next save with a conflict.
  const found = kubectl(
    "get",
    "camundaclusterclaim",
    NAME,
    "-n",
    NAMESPACE,
    "--ignore-not-found",
    "-o",
    "name",
  );
  if (found === "") {
    kubectl(
      "apply",
      "--server-side",
      "--field-manager=fleet-harness-gitops",
      "-f",
      path.resolve("e2e/journeys/git-mode-claim.yaml"),
    );
  }
  // The cluster page shows a claim only with the CamundaCluster that the
  // claim controller writes from it.
  await expect
    .poll(
      () =>
        kubectl(
          "get",
          "camundacluster",
          NAME,
          "-n",
          NAMESPACE,
          "--ignore-not-found",
          "-o",
          "name",
        ),
      { timeout: 90_000 },
    )
    .not.toBe("");
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

test("a release change pushed from the Claim tab reaches the repository and the claim", async ({
  page,
}) => {
  test.setTimeout(240_000);
  await switchToGitMode(page);
  await shot(page, "settings-git-mode", { dir: JOURNEYS_DIR });

  // An earlier run on the same harness can have saved either release.
  const target =
    claimField("{.spec.releaseRef}") === "fleet-harness-8-9-1"
      ? "fleet-harness-8-9-0"
      : "fleet-harness-8-9-1";
  await page.goto(`/fleet/clusters/${NAMESPACE}/${NAME}?tab=claim`);
  await page.getByRole("button", { name: "Edit Release and preset" }).click();
  await page.getByLabel("Release", { exact: true }).selectOption(target);
  await page
    .getByRole("region", { name: "Unsaved changes" })
    .getByRole("button", { name: "Review" })
    .click();
  const push = page
    .getByRole("dialog")
    .getByRole("button", { name: "Push to Git" });
  await expect(push).toBeEnabled({ timeout: 30_000 });
  expect(
    await blockingViolations(page),
    "serious or critical axe violations in the Git review",
  ).toEqual([]);
  await shot(page, "claim-review-git", { dir: JOURNEYS_DIR });

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
    await expect(waiting.filter({ hasText: CLAIM_FILE })).toBeVisible();
    await shot(page, "claim-pushed", { dir: JOURNEYS_DIR });
  } finally {
    resumeApplyLoop();
  }

  let file: string | undefined;
  await expect
    .poll(
      async () => {
        file = await committedFile(CLAIM_FILE);
        return file === undefined ? undefined : parse(file).spec?.releaseRef;
      },
      { timeout: 30_000 },
    )
    .toBe(target);
  const revision = parse(file!).metadata?.annotations?.[REVISION_ANNOTATION];
  expect(revision, `${CLAIM_FILE} carries ${REVISION_ANNOTATION}`).toMatch(
    /^[0-9a-f]{64}$/,
  );

  await expect
    .poll(() => claimField("{.spec.releaseRef}"), { timeout: 120_000 })
    .toBe(target);
  await expect
    .poll(() => claimField(REVISION_PATH), { timeout: 30_000 })
    .toBe(revision);

  await expect(
    page.getByRole("status").filter({ hasText: `The claim ${NAME} is saved.` }),
  ).toBeVisible({ timeout: 60_000 });
  await expect(waiting).toHaveCount(0);
  await shot(page, "claim-applied", { dir: JOURNEYS_DIR });
});
