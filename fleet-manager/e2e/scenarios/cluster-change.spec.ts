import { docShot } from "./docs-shot";
import { expect, test } from "./fixture";
import { checkPage } from "./themes";

test.use({ scenarioName: "claim-ready" });

const ORDERS = "/fleet/clusters/team-orders/orders";

// No test of this file saves, so each one starts from the scenario.

test("an edit of a broker setting shows its source and counts in the bar", async ({
  page,
}) => {
  await page.goto(`${ORDERS}?tab=overview&component=zeebe`);
  await page.getByRole("tab", { name: "Settings" }).click();
  await page.getByRole("button", { name: "Edit settings" }).click();
  const field = page.getByRole("group", { name: "replicas" });
  const replicas = field.getByRole("spinbutton");
  await replicas.fill("15");
  await expect(field.getByText("Changed in the claim")).toBeVisible();
  const bar = page.getByRole("region", { name: "Unsaved changes" });
  await expect(bar.getByText("1 unsaved change")).toBeVisible();
  await expect(bar.getByRole("button", { name: "Review" })).toBeEnabled();
  await checkPage(page, "cluster-settings-edit");
  await field.evaluate((el) => el.scrollIntoView({ block: "center" }));
  await docShot(page, "cluster-component-settings-editor", {
    ready: replicas,
  });
});

test("Change topology shows the topology now and after the change", async ({
  page,
}) => {
  await page.goto(ORDERS);
  await page.getByRole("button", { name: "Change topology" }).click();
  const dialog = page.getByRole("dialog", { name: "Change topology" });
  await dialog
    .getByRole("group", { name: "Operate" })
    .getByText("Standalone", { exact: true })
    .click();
  await expect(dialog.getByRole("heading", { name: "Now" })).toBeVisible();
  await expect(
    dialog.getByRole("heading", { name: "After the change" }),
  ).toBeVisible();
  await expect(
    dialog
      .getByRole("region", { name: "What happens" })
      .getByRole("listitem")
      .filter({ hasText: "Deployment orders-operate" }),
  ).toBeVisible();
  // Review turns on with the pick and fades in from its disabled look. axe
  // reads a low contrast during the fade.
  const review = dialog.getByRole("button", { name: "Review" });
  await expect(review).toBeEnabled();
  await review.evaluate((el) =>
    Promise.all(el.getAnimations().map((a) => a.finished)),
  );
  await checkPage(page, "cluster-topology-dialog");
  await docShot(page, "cluster-topology-dialog", {
    ready: dialog,
    target: dialog,
  });
});

test("the Claim tab of a ready cluster shows a panel for each choice", async ({
  page,
}) => {
  await page.goto(`${ORDERS}?tab=claim`);
  const basics = page.getByRole("button", { name: "Edit Basics" });
  await expect(basics).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Edit Release and preset" }),
  ).toBeVisible();
  await expect(
    page.getByRole("region", {
      name: "Secondary storage: Existing secondary storage",
      exact: true,
    }),
  ).toBeVisible();
  await checkPage(page, "cluster-claim-tab");
  await docShot(page, "cluster-claim-tab", { ready: basics });
});
