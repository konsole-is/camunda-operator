import { blockingViolations } from "../axe";
import { PAGES_DIR, shot } from "../shot";
import { expect, test } from "./fixture";

test.use({ scenarioName: "cluster-ready" });
test.afterEach(({ scenario }) => scenario.prometheus.respond());

const BROKERS =
  "/fleet/clusters/team-a/camundacluster-sample?tab=overview&component=zeebe";

test("the Metrics tab of the brokers shows the live usage and a chart that the server rendered", async ({
  page,
  scenario,
}) => {
  scenario.prometheus.respond((url) =>
    url.pathname === "/api/v1/query_range" &&
    url.searchParams
      .get("query")
      ?.includes("zeebe_element_instance_events_total")
      ? {
          status: "success",
          data: {
            resultType: "matrix",
            result: [
              {
                metric: { pod: "camundacluster-sample-zeebe-0" },
                values: [
                  [Date.now() / 1000 - 60, "2"],
                  [Date.now() / 1000, "3"],
                ],
              },
            ],
          },
        }
      : undefined,
  );
  const metricsCalls: URL[] = [];
  page.on("request", (request) => {
    const url = new URL(request.url());
    if (url.pathname === "/api/metrics") metricsCalls.push(url);
  });

  await page.goto(BROKERS);
  await page.getByRole("tab", { name: "Metrics" }).click();

  const live = page.getByRole("region", { name: "Live usage" });
  await expect(
    live.getByRole("rowheader", { name: "camundacluster-sample-zeebe-0" }),
  ).toBeVisible();
  await expect(
    page.getByRole("figure", { name: /Process instances activated/ }),
  ).toBeVisible();

  const queries = scenario.prometheus.requests.map(
    (u) => u.searchParams.get("query") ?? "",
  );
  expect(queries).toContainEqual(expect.stringContaining('namespace="team-a"'));
  expect(queries).toContainEqual(
    expect.stringContaining("camundacluster-sample-zeebe-0"),
  );
  expect(metricsCalls.length).toBeGreaterThan(0);
  for (const url of metricsCalls) {
    expect([...url.searchParams.keys()].sort()).toEqual([
      "cluster",
      "component",
      "namespace",
      "range",
    ]);
    expect(url.searchParams.get("component")).toBe("zeebe");
  }
  expect(await blockingViolations(page)).toEqual([]);
  await shot(page, "component-metrics", { dir: PAGES_DIR });
});

test("the Logs tab of the brokers shows a line of each broker pod", async ({
  page,
}) => {
  await page.goto(BROKERS);
  await page.getByRole("tab", { name: "Logs" }).click();

  const log = page.getByRole("log");
  await expect(log.getByText("Broker 0 is ready")).toBeVisible();
  await expect(log.getByText("Broker 1 is ready")).toBeVisible();
  await expect(
    log.getByText("camundacluster-sample-zeebe-1").first(),
  ).toBeVisible();
  expect(await blockingViolations(page)).toEqual([]);
  await shot(page, "component-logs", { dir: PAGES_DIR });
});

test("the Events tab of the brokers shows the events of the cluster", async ({
  page,
}) => {
  await page.goto(BROKERS);
  await page.getByRole("tab", { name: "Events" }).click();

  await expect(page.getByText("CreatedStatefulSet")).toBeVisible();
  await expect(
    page.getByText("CamundaCluster/camundacluster-sample").first(),
  ).toBeVisible();
  expect(await blockingViolations(page)).toEqual([]);
  await shot(page, "component-events", { dir: PAGES_DIR });
});
