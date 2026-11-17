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

// The scenario applied the claim with kubectl, which owns its fields. A
// field that kubectl owns stays until the review takes it over. The save
// removes the fields of kubectl after it applies the new provider, and the
// CRD refuses a claim with two providers, so this move cannot save.
test("a move of the identity mode shows what it does, and the take-over of the kubectl provider meets the one-provider rule", async ({
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
  const fields = dialog.getByRole("group", {
    name: "Fields that another tool set",
  });
  await expect(fields).toContainText(
    "spec.management.identityProvider.oidc: kubectl set it. It stays unless you take it over.",
  );
  await expect(fields).toContainText(
    "spec.management.identity.admin.claimName: kubectl set it.",
  );
  await expect(dialog.getByText(/Sign-in moves/)).toHaveCount(0);
  await checkPage(page, "plane-claim-review");

  const save = dialog.getByRole("button", { name: "Save" });
  await expect(save).toBeDisabled();
  const claim = await kubeRequest<Claim>(scenario.admin, "GET", CLAIM);
  expect(claim.spec.management.identityProvider).toEqual({ oidc: {} });

  await fields
    .getByRole("checkbox", { name: "Take over these fields" })
    .click();
  await expect(fields).toContainText(
    "spec.management.identityProvider.oidc: kubectl set it. The save removes it.",
  );
  await expect(dialog.getByText(/Sign-in moves/).first()).toBeVisible();
  await expect(dialog.getByRole("alert")).toContainText(
    "exactly one identity provider",
  );
  await expect(save).toBeDisabled();
  await checkPage(page, "plane-claim-review-take-over");
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
