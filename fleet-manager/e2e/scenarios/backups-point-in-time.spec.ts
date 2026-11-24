import { kubeRequest } from "../../test/scenarios/kube";
import { expect, test } from "./fixture";
import { checkPage } from "./themes";

test.use({ scenarioName: "cluster-rdbms-backups" });

const BACKUPS = "/fleet/clusters/team-a/camundacluster-sample?tab=backups";

test("restore to a point in time after a rollback outside the operator creates the PointInTimeRestore", async ({
  page,
  scenario,
}) => {
  await page.goto(BACKUPS);
  await page
    .getByRole("button", { name: "Restore to a point in time" })
    .click();
  const dialog = page.getByRole("dialog", {
    name: "Restore camundacluster-sample to a point in time",
  });
  const at = new Date(Date.now() - 3_600_000).toISOString().slice(0, 19);
  await dialog.getByLabel("Time (UTC)").fill(at);
  await expect(
    dialog.getByText(
      "The operator does not roll the database server back. Suspend the cluster, so that it writes nothing more to the database. Then roll the server back to this time before you confirm.",
    ),
  ).toBeVisible();
  await expect(
    dialog.getByLabel("Type camundacluster-sample to confirm"),
  ).toHaveCount(0);
  await dialog
    .getByRole("checkbox", {
      name: "I rolled the database server back to this time",
    })
    .click();
  await checkPage(page, "cluster-backups-point-in-time-after-rollback");
  await dialog
    .getByLabel("Type camundacluster-sample to confirm")
    .pressSequentially("camundacluster-sample");
  await dialog.getByRole("button", { name: "Restore", exact: true }).click();

  const status = page.getByRole("status").filter({
    hasText: /Restore camundacluster-sample-pitr-\d{8}-\d{6} created\./,
  });
  await expect(status).toBeVisible();
  const name = /camundacluster-sample-pitr-\d{8}-\d{6}/.exec(
    (await status.textContent()) ?? "",
  )?.[0];
  const restore = await kubeRequest<{ spec: unknown }>(
    scenario.admin,
    "GET",
    `/apis/core.camunda.io/v1/namespaces/team-a/pointintimerestores/${name}`,
  );
  expect(restore.spec).toEqual({
    clusterRef: { name: "camundacluster-sample" },
    timestamp: `${at}Z`,
  });
});
