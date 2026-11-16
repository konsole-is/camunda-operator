import { kubeRequest } from "../../test/scenarios/kube";
import { expect, test } from "./fixture";
import { checkPage } from "./themes";

test.use({ scenarioName: "plane-ready" });

const PAGE = "/fleet/planes/platform/plane-a?tab=claim";
const CLAIM =
  "/apis/fleet.camunda.io/v1alpha1/namespaces/platform/camundamanagementclusterclaims/plane-a";

test("the Claim tab of a plane shows each section of its claim", async ({
  page,
}) => {
  await page.goto(PAGE);
  const basics = page.getByRole("region", { name: "Basics" });
  await expect(basics).toContainText("plane-a");
  await expect(basics).toContainText("Set at creation");
  await expect(basics).toContainText("oidc-platform");
  await expect(page.getByRole("region", { name: "Identity" })).toContainText(
    "OIDC",
  );
  await expect(page.getByRole("region", { name: "Access" })).toContainText(
    "https://management.example.com/identity",
  );
  await checkPage(page, "plane-claim-tab");
});

type Claim = {
  spec: {
    deletionPolicy?: string;
    management: { identityProvider: Record<string, unknown> };
  };
};

// The scenario applied the claim with kubectl, which owns its fields. The
// fleet manager does not take a field over, so a change of such a field
// stops in the review, and a field that no tool owns saves.
test("a move of the identity mode shows what it does, and a field that kubectl owns stops the save", async ({
  page,
  scenario,
}) => {
  await page.goto(PAGE);
  await page.getByRole("button", { name: "Edit Identity" }).click();
  await page.getByRole("radio", { name: /External Keycloak/ }).click();
  await page
    .getByLabel("Keycloak URL")
    .fill("https://keycloak.example.com/auth");
  await page
    .getByLabel("Administrator credentials Secret")
    .fill("keycloak-admin");
  await page.getByLabel("Username").fill("admin");
  await page.getByLabel("Email").fill("admin@example.com");
  await page
    .getByRole("region", { name: "Unsaved changes" })
    .getByRole("button", { name: "Review" })
    .click();

  const dialog = page.getByRole("dialog", { name: "Review the change" });
  const effects = dialog.getByRole("list", { name: "What this change does" });
  await expect(effects).toContainText(
    "Sign-in moves from OIDC to External Keycloak.",
  );
  await expect(effects).toContainText("The first administrator stays.");
  await expect(effects).toContainText("restart");
  await expect(dialog.getByLabel(/to confirm/)).toBeVisible();
  const caution = dialog.getByRole("note", { name: "Caution" });
  await expect(caution).toContainText("another tool set them");
  await expect(caution).toContainText(
    "spec.management.identityProvider.oidc (kubectl)",
  );
  await checkPage(page, "plane-claim-review");

  await dialog.getByLabel(/to confirm/).fill("plane-a");
  await expect(dialog.getByRole("button", { name: "Save" })).toBeDisabled();
  const claim = await kubeRequest<Claim>(scenario.admin, "GET", CLAIM);
  expect(claim.spec.management.identityProvider).toEqual({ oidc: {} });
});

test("a change of the deletion policy saves only the claim", async ({
  page,
  scenario,
}) => {
  await page.goto(PAGE);
  await page
    .getByRole("button", { name: "Edit When the plane is deleted" })
    .click();
  await page.getByRole("radio", { name: /Keep the resources/ }).click();
  await page
    .getByRole("region", { name: "Unsaved changes" })
    .getByRole("button", { name: "Review" })
    .click();
  const dialog = page.getByRole("dialog", { name: "Review the change" });
  await expect(dialog.getByLabel(/to confirm/)).toHaveCount(0);
  const save = dialog.getByRole("button", { name: "Save" });
  await expect(save).toBeEnabled();
  await save.click();
  await expect(
    page.getByRole("status").filter({ hasText: "The claim plane-a is saved." }),
  ).toBeVisible();

  const claim = await kubeRequest<Claim>(scenario.admin, "GET", CLAIM);
  expect(claim.spec.deletionPolicy).toBe("Orphan");
  // No controller runs during a page scenario, so the plane keeps its
  // spec: the page wrote only the claim.
  const plane = await kubeRequest<{
    spec: { identityProvider: Record<string, unknown> };
  }>(
    scenario.admin,
    "GET",
    "/apis/core.camunda.io/v1/namespaces/platform/camundamanagementclusters/plane-a",
  );
  expect(Object.keys(plane.spec.identityProvider)).toEqual(["oidc"]);
});
