import { readFileSync } from "node:fs";
import type { CamundaManagementClusterClaim } from "../../src/generated/crds/camundamanagementclusterclaim";
import { parseImagesTable } from "../../src/startup/bundled-data";
import { kubeRequest } from "../../test/scenarios/kube";
import { blockingViolations } from "../axe";
import { PAGES_DIR, shot } from "../shot";
import { expect, test } from "./fixture";

test.use({ scenarioName: "plane-create" });

const URL_BASE = "https://management.example.com";

test("creates a plane with no access from outside and existing database configs", async ({
  page,
  scenario,
}) => {
  const next = page.getByRole("button", { name: "Next" });
  await page.goto("/fleet/planes/new");

  // Basics
  await page.getByRole("radio", { name: /Platform/ }).check();
  await page.getByLabel("Name", { exact: true }).fill("plane-a");
  await page
    .getByRole("combobox", { name: "Namespace" })
    .selectOption("platform");
  await page
    .getByRole("radio", { name: /Keycloak, run by this plane/ })
    .check();
  await next.click();

  // Environments served
  await expect(page.getByRole("checkbox", { name: /Platform/ })).toBeChecked();
  await next.click();

  // Components and versions
  await page.getByLabel("Username").fill("admin");
  await page.getByLabel("Email").fill("admin@example.com");
  await page.getByRole("checkbox", { name: "Run Console" }).click();
  await next.click();

  // Storage and access
  const databases = page.getByRole("group", { name: /Databases/ });
  await expect(
    databases.getByRole("radio", { name: /Existing databases/ }),
  ).toBeChecked();
  await databases
    .getByRole("combobox", { name: "Keycloak" })
    .selectOption("keycloak-db");
  await databases
    .getByRole("combobox", { name: "Management Identity" })
    .selectOption("identity-db");
  await page.getByLabel("Keycloak URL").fill(`${URL_BASE}/auth`);
  await page.getByLabel("Management Identity URL").fill(`${URL_BASE}/identity`);
  await page.getByLabel("Console URL").fill(`${URL_BASE}/console`);
  await next.click();

  // Review
  const create = page.getByRole("button", { name: "Create" });
  await expect(create).toBeEnabled();
  await shot(page, "plane-create-review", { dir: PAGES_DIR });
  expect(await blockingViolations(page)).toEqual([]);
  await create.click();

  // Progress: no controller runs, so each row stays pending.
  await expect(
    page.getByRole("heading", {
      name: "Creating the management plane plane-a",
    }),
  ).toBeVisible();
  for (const row of [
    "Environment and policy",
    "Databases",
    "Management cluster",
  ]) {
    await expect(page.getByText(row, { exact: true })).toBeVisible();
  }
  await expect(page.getByText("Access", { exact: true })).toHaveCount(0);
  await shot(page, "plane-create-progress", { dir: PAGES_DIR });

  const claim = await kubeRequest<CamundaManagementClusterClaim>(
    scenario.admin,
    "GET",
    "/apis/fleet.camunda.io/v1alpha1/namespaces/platform/camundamanagementclusterclaims/plane-a",
  );
  const table = parseImagesTable(
    JSON.parse(readFileSync("../pkg/images/table.json", "utf8")),
  );
  const row = table.minors["8.9"];
  const { spec } = claim;
  expect(spec.environments).toEqual(["platform"]);
  expect(spec.clusterSelector).toEqual({});
  expect(spec).not.toHaveProperty("platformConfigRef");
  expect(spec.management.identityProvider).toEqual({
    keycloak: { externalUrl: `${URL_BASE}/auth` },
  });
  expect(spec.management.identity).toEqual({
    externalUrl: `${URL_BASE}/identity`,
    admin: { username: "admin", email: "admin@example.com" },
  });
  expect(spec.management.console).toEqual({
    externalUrl: `${URL_BASE}/console`,
  });
  expect(spec.management).not.toHaveProperty("webModeler");
  expect(spec.versions).toEqual({
    identity: row.identity.version,
    console: row.console.version,
    keycloak: row.keycloak.version,
  });
  expect(spec.plugins).toEqual([
    {
      section: "management-database",
      registration: "existing-database-configs",
      parameters: { keycloak: "keycloak-db", identity: "identity-db" },
    },
  ]);
  expect(spec).not.toHaveProperty("managementAuthConfigName");
});
