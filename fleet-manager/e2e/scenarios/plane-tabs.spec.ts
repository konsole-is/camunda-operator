import { expect, test } from "./fixture";
import { checkPage } from "./themes";

test.use({ scenarioName: "plane-ready" });

test("the tabs of a plane show its served cluster, its Optimize, its contract, and its routes", async ({
  page,
  scenario,
}) => {
  await scenario.advance("serves-a-cluster");
  await page.goto("/fleet/planes/platform/plane-a?tab=clusters");

  const clusters = page.getByRole("region", {
    name: "Clusters served",
    exact: true,
  });
  const row = clusters.getByRole("row", { name: /orders/ });
  await expect(row.getByRole("link", { name: "orders" })).toHaveAttribute(
    "href",
    "/fleet/clusters/team-orders/orders",
  );
  await expect(
    row.getByRole("cell", { name: "platform", exact: true }),
  ).toBeVisible();
  await expect(
    row.getByRole("cell", { name: "Yes", exact: true }),
  ).toBeVisible();
  await checkPage(page, "plane-clusters-served");

  await page.getByRole("tab", { name: "Optimize" }).click();
  const optimize = page.getByRole("table", { name: "Optimize" });
  await expect(
    optimize.getByText(
      "https://orders.example.com/optimize/api/authentication/callback",
    ),
  ).toBeVisible();
  await expect(
    page.getByText("Add the callbacks to your identity provider."),
  ).toBeVisible();
  await checkPage(page, "plane-optimize");

  await page.getByRole("tab", { name: "Contract" }).click();
  const contract = page.getByRole("region", {
    name: "Contract",
    exact: true,
  });
  await expect(
    contract
      .locator("dt", { hasText: "Client secret name" })
      .locator("xpath=following-sibling::dd"),
  ).toHaveText("optimize-credentials");
  await expect(
    page.getByRole("table", { name: "Readers" }).getByText("orders-optimize"),
  ).toBeVisible();
  await checkPage(page, "plane-contract");

  await page.getByRole("tab", { name: "Endpoints" }).click();
  const endpoints = page.getByRole("table", { name: "Endpoints" });
  await expect(
    endpoints
      .getByRole("row", { name: /^identity / })
      .getByRole("link", { name: "https://management.example.com/identity" }),
  ).toBeVisible();
  await checkPage(page, "plane-endpoints");
});
