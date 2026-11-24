import { docShot } from "./docs-shot";
import { expect, test } from "./fixture";
import { checkPage } from "./themes";

test.use({ scenarioName: "cluster-ready" });
test.afterEach(({ scenario }) => scenario.prometheus.respond());

const BROKERS =
  "/fleet/clusters/team-a/camundacluster-sample?tab=overview&component=zeebe";

// Fixed times, so the axis labels of the chart are the same on each run.
const RANGE_START = Date.parse("2026-10-30T11:00:00Z") / 1000;

function brokersRange(url: URL): unknown {
  if (
    url.pathname !== "/api/v1/query_range" ||
    !url.searchParams
      .get("query")
      ?.includes("zeebe_element_instance_events_total")
  ) {
    return undefined;
  }
  return {
    status: "success",
    data: {
      resultType: "matrix",
      result: [
        {
          metric: { pod: "camundacluster-sample-zeebe-0" },
          values: Array.from({ length: 30 }, (_, i) => [
            RANGE_START + i * 120,
            String(400 + Math.round(120 * Math.sin(i / 4))),
          ]),
        },
      ],
    },
  };
}

test("the Metrics tab of the brokers shows the live usage and a chart that the server rendered", async ({
  page,
  scenario,
}) => {
  scenario.prometheus.respond(brokersRange);
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
  await checkPage(page, "component-metrics");
});

test("the chart of the brokers shows the same time labels when the font arrives after the data", async ({
  page,
  scenario,
}) => {
  scenario.prometheus.respond(brokersRange);
  let hold: Promise<void> | undefined;
  let held = 0;
  await page.route("**/ibm-plex-sans-*-400-*.woff2", async (route) => {
    if (hold) {
      held++;
      await hold;
    }
    await route.continue();
  });

  const fontLoaded = () =>
    page.waitForFunction(() =>
      document.fonts.check('400 11px "IBM Plex Sans"'),
    );
  // A held font holds back the load event, so that load waits only for the
  // DOM and clicks the tab again until the page has hydrated.
  const timeLabels = async (release?: () => void): Promise<string[]> => {
    await page.goto(BROKERS, {
      waitUntil: release ? "domcontentloaded" : "load",
    });
    if (!release) await fontLoaded();
    const tab = page.getByRole("tab", { name: "Metrics" });
    await expect(async () => {
      await tab.click();
      await expect(tab).toHaveAttribute("aria-selected", "true", {
        timeout: 1000,
      });
    }).toPass();
    const figure = page.getByRole("figure", {
      name: /Process instances activated/,
    });
    await expect(figure).toBeVisible();
    release?.();
    await fontLoaded();
    const ticks = figure.locator(
      ".recharts-xAxis-tick-labels .recharts-cartesian-axis-tick-value",
    );
    await expect(ticks.first()).toBeVisible();
    return ticks.allTextContents();
  };

  const present = await timeLabels();

  let releaseFont = () => {};
  hold = new Promise((resolve) => {
    releaseFont = resolve;
  });
  const late = await timeLabels(releaseFont);
  hold = undefined;

  expect(held).toBeGreaterThan(0);
  expect(late.length).toBeGreaterThan(1);
  expect(late).toEqual(present);
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
  await checkPage(page, "component-logs");
  // With All pods, the lines of two pods arrive in either order.
  await page.getByLabel("Pod").selectOption("camundacluster-sample-zeebe-0");
  await docShot(page, "cluster-component-logs", {
    ready: log.getByText("Partition 1 is healthy"),
  });
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
  await checkPage(page, "component-events");
});
