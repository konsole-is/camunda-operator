import { kubeRequest } from "../../test/scenarios/kube";
import { expect, test } from "./fixture";
import { checkPage } from "./themes";

test.use({ scenarioName: "plane-ready" });

const claimPath =
  "/apis/fleet.camunda.io/v1alpha1/namespaces/platform/camundamanagementclusterclaims/plane-a";

// The tests share the app and run in order: Suspend writes the claim, and
// the delete then removes it.
test("Suspend asks first and writes spec.suspend on the claim", async ({
  page,
  scenario,
}) => {
  await scenario.advance("serves-a-cluster");
  await page.goto("/fleet/planes/platform/plane-a");
  const header = page
    .locator("header")
    .filter({ has: page.getByRole("heading", { level: 1, name: "plane-a" }) });
  await header.getByRole("button", { name: "Suspend" }).click();
  const confirm = header.getByRole("group", { name: "Suspend" });
  await expect(confirm).toContainText(
    "Sign-in to Console, Web Modeler, and Optimize stops for every cluster that this plane serves (1).",
  );
  await expect(confirm).toContainText("The served clusters keep running");
  await checkPage(page, "plane-suspend-confirm");

  await confirm
    .getByRole("button", { name: "Suspend the management plane" })
    .click();
  await expect(
    page.getByText("Suspended the management plane plane-a."),
  ).toBeVisible();
  await expect
    .poll(
      async () =>
        (
          await kubeRequest<{ spec: { suspend?: boolean } }>(
            scenario.admin,
            "GET",
            claimPath,
          )
        ).spec.suspend,
    )
    .toBe(true);
  // No controller runs during a page scenario, so the plane stays Ready;
  // the live claim turns the button into Resume.
  await expect(header.getByRole("button", { name: "Resume" })).toBeVisible();
});

test("Delete asks for the policy and the name, warns about the served cluster, then deletes the claim", async ({
  page,
  scenario,
}) => {
  await page.goto("/fleet/planes/platform/plane-a");
  await page.getByRole("link", { name: "Delete", exact: true }).click();
  await expect(page).toHaveURL("/fleet/planes/platform/plane-a/delete");
  await expect(
    page.getByRole("heading", { name: "Delete the management plane plane-a" }),
  ).toBeVisible();
  await expect(
    page.getByRole("radio", { name: /Delete the resources/ }),
  ).toBeChecked();
  await expect(
    page
      .getByText(
        "Deleting a Database keeps its data on its server. Deleting a DatabaseServer erases the server and its data.",
      )
      .first(),
  ).toBeVisible();
  await expect(
    page.getByText(/These clusters stay and keep running\./).first(),
  ).toBeVisible();
  await expect(page.getByText("team-orders/orders").first()).toBeVisible();
  const confirm = page.getByRole("button", {
    name: "Delete the management plane",
  });
  await expect(confirm).toBeDisabled();
  await checkPage(page, "plane-delete");

  await page.getByLabel("Type plane-a to confirm").fill("plane-a");
  await expect(confirm).toBeEnabled();
  await confirm.click();
  // The finalizer of the claim keeps it while no controller runs.
  await expect
    .poll(
      async () =>
        (
          await kubeRequest<{ metadata: { deletionTimestamp?: string } }>(
            scenario.admin,
            "GET",
            claimPath,
          )
        ).metadata.deletionTimestamp,
    )
    .toBeTruthy();
});
