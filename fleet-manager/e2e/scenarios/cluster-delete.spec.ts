import { docShot } from "./docs-shot";
import { expect, test } from "./fixture";
import { checkPage } from "./themes";

test.use({ scenarioName: "claim-ready" });

// The page writes nothing here: the review only previews the delete, and
// nobody types the name.
test("the cluster header opens the delete page, which defaults to Delete the resources", async ({
  page,
}) => {
  await page.goto("/fleet/clusters/team-orders/orders");
  await page.getByRole("link", { name: "Delete", exact: true }).click();
  await expect(page).toHaveURL("/fleet/clusters/team-orders/orders/delete");
  await expect(
    page.getByRole("heading", { name: "Delete the cluster orders" }),
  ).toBeVisible();
  await expect(
    page.getByRole("radio", { name: /Delete the resources/ }),
  ).toBeChecked();
  await expect(page.getByText("This option made no resources.")).toBeVisible();
  await expect(
    page.getByText("Checking the change with the API server."),
  ).toHaveCount(0);
  await expect(
    page.getByRole("button", { name: "Delete the cluster" }),
  ).toBeDisabled();
  await checkPage(page, "cluster-delete");
  await docShot(page, "cluster-delete-page", {
    ready: page.getByRole("heading", { name: "Delete the cluster orders" }),
  });
});
