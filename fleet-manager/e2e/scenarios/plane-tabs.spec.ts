import { expect, test } from "./fixture";
import { checkPage } from "./themes";

test.use({ scenarioName: "plane-ready" });

test("the tabs of a plane show its served cluster, its callbacks, its contract, and its routes", async ({
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

  // The scenario holds the CamundaOptimize orders, which reads the contract
  // and sets an external URL. The identity provider holds its callbacks, so
  // the plane reports them as Disabled.
  await page.getByRole("tab", { name: "Optimize" }).click();
  const optimize = page.getByRole("region", {
    name: "Optimize",
    exact: true,
  });
  await expect(optimize.getByText("Callbacks", { exact: true })).toBeVisible();
  await expect(
    optimize.getByText(
      "The identity provider of the platform config holds the callback URLs of Optimize",
    ),
  ).toBeVisible();
  await expect(
    page.getByText("Add the callbacks to your identity provider."),
  ).toBeVisible();
  const optimizeRow = optimize
    .getByRole("table", { name: "Optimize" })
    .getByRole("row", { name: /orders/ });
  await expect(
    optimizeRow.getByRole("cell", { name: "team-orders", exact: true }),
  ).toBeVisible();
  await expect(
    optimizeRow.getByRole("link", {
      name: "https://orders.example.com/optimize",
      exact: true,
    }),
  ).toBeVisible();
  await expect(
    optimizeRow.getByText(
      "https://orders.example.com/optimize/api/authentication/callback",
      { exact: true },
    ),
  ).toBeVisible();
  await checkPage(page, "plane-optimize");

  await page.getByRole("tab", { name: "Contract" }).click();
  const contract = page.getByRole("region", {
    name: "Contract",
    exact: true,
  });
  await expect(
    contract
      .locator("dt", { hasText: /^Client secret name$/ })
      .locator("xpath=following-sibling::dd[1]"),
  ).toHaveText("optimize-credentials");
  const reader = contract
    .getByRole("table", { name: "Readers" })
    .getByRole("row", { name: /orders/ });
  await expect(
    reader.getByRole("cell", { name: "team-orders", exact: true }),
  ).toBeVisible();
  await expect(
    reader.getByRole("link", { name: "orders", exact: true }),
  ).toHaveAttribute("href", "/fleet/clusters/team-orders/orders");
  await expect(
    reader.getByText("https://orders.example.com/optimize", { exact: true }),
  ).toBeVisible();
  await checkPage(page, "plane-contract");

  await page.getByRole("tab", { name: "Endpoints" }).click();
  const endpoints = page.getByRole("table", { name: "Endpoints" });
  await expect(
    endpoints
      .getByRole("row", { name: /^identity / })
      .getByRole("link", { name: "https://management.example.com/identity" }),
  ).toBeVisible();
  for (const [name, url] of [
    ["web-modeler", "https://management.example.com/modeler"],
    ["web-modeler-websockets", "https://management.example.com/modeler-ws"],
  ]) {
    const modeler = endpoints.getByRole("row", {
      name: `${name} /`,
      exact: false,
    });
    await expect(
      modeler.getByRole("link", { name: url, exact: true }),
    ).toBeVisible();
    await expect(
      modeler.getByText("Web Modeler", { exact: false }),
    ).toBeVisible();
  }
  await checkPage(page, "plane-endpoints");
});
