import { kubeRequest } from "../../test/scenarios/kube";
import { blockingViolations } from "../axe";
import { PAGES_DIR, shot } from "../shot";
import { expect, test } from "./fixture";

test.use({ scenarioName: "cluster-es-backups" });

const BACKUPS = "/fleet/clusters/team-a/camundacluster-sample?tab=backups";

test("back up now creates a backup with the cluster reference", async ({
  page,
  scenario,
}) => {
  await page.goto(BACKUPS);
  await page.getByRole("button", { name: "Back up now" }).click();
  const field = page.getByLabel("Name");
  await expect(field).toHaveValue(/^camundacluster-sample-manual-\d{8}-\d{6}$/);
  const name = await field.inputValue();
  await page.getByRole("button", { name: "Start backup" }).click();
  await expect(
    page.getByRole("status").filter({ hasText: `Backup ${name} created.` }),
  ).toBeVisible();

  const backup = await kubeRequest<{
    spec: { clusterRef: { name: string } };
    metadata: { managedFields: { manager: string }[] };
  }>(
    scenario.admin,
    "GET",
    `/apis/core.camunda.io/v1/namespaces/team-a/logicalbackupelasticsearches/${name}`,
  );
  expect(backup.spec).toEqual({
    clusterRef: { name: "camundacluster-sample" },
  });
  expect(backup.metadata.managedFields.map((f) => f.manager)).toContain(
    "fleet-manager",
  );

  await expect(
    page.getByRole("button", { name: `Details of ${name}` }),
  ).toBeVisible();
  expect(await blockingViolations(page)).toEqual([]);
  await shot(page, "cluster-backups", { dir: PAGES_DIR });
});
