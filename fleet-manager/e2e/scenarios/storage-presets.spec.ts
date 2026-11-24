import { expect, test } from "./fixture";
import { checkPage } from "./themes";

test.use({ scenarioName: "cluster-create" });

test("an Elasticsearch preset page edits a field with its field editor and shows its merge rule", async ({
  page,
}) => {
  await page.goto("/setup/presets/elasticsearch/small-es");
  const settings = page.getByRole("region", { name: "Elasticsearch settings" });
  await settings
    .getByRole("group", { name: "replicas" })
    .getByRole("spinbutton")
    .fill("12");
  const rules = page.getByRole("list", {
    name: "Merge rules of Elasticsearch",
  });
  await expect(rules.getByText("replicas")).toBeVisible();
  await expect(
    rules.getByText("A resource value replaces it").first(),
  ).toBeVisible();
  await checkPage(page, "presets-elasticsearch-edit");
  await page.getByRole("button", { name: "Review", exact: true }).click();
  await expect(
    page.getByRole("region", { name: "Review the preset" }),
  ).toBeVisible();
  await checkPage(page, "presets-elasticsearch-edit-review");
});
