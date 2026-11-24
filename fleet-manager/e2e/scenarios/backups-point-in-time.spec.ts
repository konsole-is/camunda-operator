import { expect, test } from "./fixture";
import { checkPage } from "./themes";

test.use({ scenarioName: "cluster-rdbms-backups" });

const BACKUPS = "/fleet/clusters/team-a/camundacluster-sample?tab=backups";

test("restore to a point in time is off while the server of the cluster has published no identity", async ({
  page,
}) => {
  await page.goto(BACKUPS);
  const button = page.getByRole("button", {
    name: "Restore to a point in time",
  });
  await expect(button).toBeDisabled();
  await expect(button).toHaveAccessibleDescription(
    "A DatabaseServerConfig of this namespace has not published the identity of its server yet, so the fleet manager cannot tell which databases the server of this cluster holds.",
  );
  await checkPage(page, "cluster-backups-point-in-time-off");
});
