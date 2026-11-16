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

test("a move of the identity mode shows what it does and saves only the claim", async ({
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
  const save = dialog.getByRole("button", { name: "Save" });
  await expect(save).toBeDisabled();
  await checkPage(page, "plane-claim-review");

  await dialog.getByLabel(/to confirm/).fill("plane-a");
  await expect(save).toBeEnabled();
  await save.click();
  await expect(
    page.getByRole("status").filter({ hasText: "The claim plane-a is saved." }),
  ).toBeVisible();

  const claim = await kubeRequest<{
    spec: {
      management: {
        identityProvider: Record<string, unknown>;
        identity: { admin: Record<string, unknown> };
      };
    };
  }>(scenario.admin, "GET", CLAIM);
  expect(claim.spec.management.identityProvider).toEqual({
    externalKeycloak: expect.objectContaining({
      url: "https://keycloak.example.com/auth",
      adminCredentialsSecretRef: expect.objectContaining({
        name: "keycloak-admin",
      }),
    }),
  });
  expect(claim.spec.management.identity.admin).toEqual({
    username: "admin",
    email: "admin@example.com",
  });
  // No controller runs during a page scenario, so the plane keeps its
  // mode: the page wrote only the claim.
  const plane = await kubeRequest<{
    spec: { identityProvider: Record<string, unknown> };
  }>(
    scenario.admin,
    "GET",
    "/apis/core.camunda.io/v1/namespaces/platform/camundamanagementclusters/plane-a",
  );
  expect(Object.keys(plane.spec.identityProvider)).toEqual(["oidc"]);
});
