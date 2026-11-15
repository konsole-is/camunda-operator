import { execFileSync } from "node:child_process";
import { expect, type Page } from "@playwright/test";

/** GIT_REPOSITORY is the in-cluster repository of the harness. */
export const GIT_REPOSITORY = "https://git.fleet-harness.svc:8443/fleet.git";

/** GIT_PATH is the folder of the repository that apply-loop.sh applies. */
export const GIT_PATH = "fleet";

function gitInPod(...args: string[]): string {
  return execFileSync(
    "kubectl",
    [
      "exec",
      "-n",
      "fleet-harness",
      "deploy/git",
      "--",
      "git",
      "-C",
      "/srv/git/fleet.git",
      ...args,
    ],
    { encoding: "utf8" },
  );
}

/**
 * switchToGitMode turns Git mode on in the Settings page, with the
 * repository, the branch main, and the folder that the apply loop applies,
 * and waits until "Test the connection" reaches the repository. It also
 * passes when Git mode is already on with these values. The page stays on
 * the Settings page.
 */
export async function switchToGitMode(page: Page): Promise<void> {
  await page.goto("/setup/settings");
  await page.getByRole("checkbox", { name: "Git mode" }).check();
  await page.getByLabel("Repository", { exact: true }).fill(GIT_REPOSITORY);
  await page.getByLabel("Branch", { exact: true }).fill("main");
  // The chart seeds the path as "", the root of the repository, which the
  // apply loop does not apply.
  await page.getByLabel("Path", { exact: true }).fill(GIT_PATH);
  await saveChange(page);
  const check = page.getByRole("button", { name: "Test the connection" });
  // The test uses the saved settings, so it is off until the page reads them.
  await expect(check).toBeEnabled({ timeout: 30_000 });
  await check.click();
  await expect(
    page
      .getByRole("status")
      .filter({ hasText: `Connected to ${GIT_REPOSITORY}.` }),
  ).toBeVisible({ timeout: 30_000 });
}

/**
 * switchToDirectMode turns Git mode off in the Settings page. It also
 * passes when Git mode is already off.
 */
export async function switchToDirectMode(page: Page): Promise<void> {
  await page.goto("/setup/settings");
  await page.getByRole("checkbox", { name: "Git mode" }).uncheck();
  await saveChange(page);
  await page.reload();
  await expect(
    page.getByRole("checkbox", { name: "Git mode" }),
  ).not.toBeChecked();
}

// saveChange saves the form of the Settings page when it holds a change.
// Save is off when the form holds none.
async function saveChange(page: Page): Promise<void> {
  const save = page.getByRole("button", { name: "Save", exact: true });
  if (await save.isDisabled()) return;
  await save.click();
  await expect(
    page.getByRole("status").filter({ hasText: "Saved the settings." }),
  ).toBeVisible();
}

/**
 * committedFile returns the file at path on the branch main of the
 * in-cluster repository, or undefined when main has no such file. It
 * throws when kubectl or git fails.
 */
export async function committedFile(path: string): Promise<string | undefined> {
  // ls-tree prints nothing and exits 0 for a missing path, so only a real
  // failure throws.
  if (gitInPod("ls-tree", "--name-only", "main", "--", path).trim() !== path) {
    return undefined;
  }
  return gitInPod("show", `main:${path}`);
}
