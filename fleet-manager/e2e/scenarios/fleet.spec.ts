import type { Page } from "@playwright/test";
import { blockingViolations } from "../axe";
import { PAGES_DIR, shot } from "../shot";
import { expect, test } from "./fixture";

test.use({ scenarioName: "cluster-ready" });

const CLUSTER = "/fleet/clusters/team-a/camundacluster-sample";

// header is the header of the cluster page, which shows the Ready condition
// of the cluster.
const header = (page: Page) =>
  page
    .locator("header")
    .filter({ has: page.getByRole("heading", { level: 1 }) });

test("the fleet page shows the shell", async ({ page }) => {
  await page.goto("/fleet");
  const nav = page.getByRole("navigation", { name: "Main" });
  await expect(nav.getByRole("link", { name: "Fleet" })).toBeVisible();
  await expect(nav.getByRole("link", { name: "Setup" })).toBeVisible();
  await expect(
    page.getByRole("heading", { level: 1, name: "Fleet" }),
  ).toBeVisible();
  expect(await blockingViolations(page)).toEqual([]);
  await shot(page, "fleet", { dir: PAGES_DIR });
});

test("the cluster page shows the cluster of the scenario", async ({ page }) => {
  await page.goto(CLUSTER);
  await expect(
    page.getByRole("heading", { level: 1, name: "camundacluster-sample" }),
  ).toBeVisible();
  await expect(header(page).getByText("Ready", { exact: true })).toBeVisible();
  expect(await blockingViolations(page)).toEqual([]);
  await shot(page, "cluster", { dir: PAGES_DIR });
});

test("advance to three-brokers-ready changes the open cluster page without a reload", async ({
  page,
  scenario,
}) => {
  await page.goto(CLUSTER);
  await expect(header(page).getByText("Ready", { exact: true })).toBeVisible();
  // A reload or a navigation drops this value of the window.
  await page.evaluate(() => {
    (window as unknown as { scenarioMark: boolean }).scenarioMark = true;
  });

  await scenario.advance("three-brokers-ready");

  await expect(header(page).getByText("Not ready · Scaling")).toBeVisible({
    timeout: 15_000,
  });
  expect(
    await page.evaluate(
      () => (window as unknown as { scenarioMark?: boolean }).scenarioMark,
    ),
  ).toBe(true);
  expect(await blockingViolations(page)).toEqual([]);
  await shot(page, "three-brokers-ready", { dir: PAGES_DIR });
});

test("the log stream reads observe.yaml", async ({ page }) => {
  const response = await page.request.get(
    "/api/logs?namespace=team-a&pod=camundacluster-sample-zeebe-0",
  );
  expect(response.status()).toBe(200);
  const body = await response.text();
  const events = body
    .split("\n\n")
    .filter((frame) => frame !== "")
    .map((frame) => {
      const [event, data] = frame.split("\n");
      return [
        event.replace("event: ", ""),
        JSON.parse(data.replace("data: ", "")),
      ];
    });
  expect(events).toEqual([
    ["line", "Broker 0 is ready"],
    ["line", "Partition 1 is healthy"],
    ["end", {}],
  ]);
});

test("the usage route reads observe.yaml", async ({ page }) => {
  const selector =
    "statefulset.kubernetes.io/pod-name=camundacluster-sample-zeebe-0";
  const response = await page.request.get(
    `/api/usage?namespace=team-a&selector=${encodeURIComponent(selector)}`,
  );
  expect(response.status()).toBe(200);
  const { usage } = (await response.json()) as {
    usage: {
      pod: string;
      cpu: { usedCores: number };
      memory: { usedBytes: number };
    }[];
  };
  expect(usage).toEqual([
    expect.objectContaining({
      pod: "camundacluster-sample-zeebe-0",
      cpu: expect.objectContaining({ usedCores: 0.25 }),
      memory: expect.objectContaining({ usedBytes: 2 ** 30 }),
    }),
  ]);
});
