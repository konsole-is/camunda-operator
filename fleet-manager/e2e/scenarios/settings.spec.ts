import { docShot } from "./docs-shot";
import { expect, test } from "./fixture";
import { checkPage } from "./themes";

test.use({ scenarioName: "claim-ready" });

test("the Settings page shows the Git settings when Git mode is on", async ({
  page,
}) => {
  await page.goto("/setup/settings");
  await expect(
    page.getByRole("heading", { level: 1, name: "Settings" }),
  ).toBeVisible();
  const repository = page.getByRole("textbox", { name: "Repository" });
  await expect(repository).toHaveCount(0);

  await page.getByRole("checkbox", { name: "Git mode" }).click();
  await expect(
    page.getByText("Set the repository to turn on Git mode."),
  ).toBeVisible();
  await repository.fill("https://github.com/acme/fleet.git");
  await page.getByRole("textbox", { name: "Branch" }).fill("main");
  await page.getByRole("textbox", { name: "Path" }).fill("clusters");
  await expect(
    page.getByText("Set the repository to turn on Git mode."),
  ).toHaveCount(0);
  await expect(
    page.getByRole("combobox", { name: "Git credentials Secret" }),
  ).toBeVisible();
  await checkPage(page, "settings-git-mode");
  await docShot(page, "settings-page-git-mode", { ready: repository });
});
