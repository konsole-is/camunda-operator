import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";
import { shot } from "../shot";

const BLOCKING = new Set(["serious", "critical"]);
const [namespace, name] = (process.env.FLEET_JOURNEYS_CLUSTER ?? "").split("/");

test("the chart install serves the app and shows the sample cluster", async ({
  page,
  baseURL,
}) => {
  expect(
    namespace && name,
    "FLEET_JOURNEYS_CLUSTER is <namespace>/<name>",
  ).toBeTruthy();
  const appHost = new URL(baseURL!).host;
  const foreign: string[] = [];
  page.on("request", (request) => {
    const url = new URL(request.url());
    if (
      (url.protocol === "http:" || url.protocol === "https:") &&
      url.host !== appHost
    )
      foreign.push(request.url());
  });

  await page.goto("/fleet");
  const nav = page.getByRole("navigation", { name: "Main" });
  await expect(nav.getByRole("link", { name: "Fleet" })).toBeVisible();
  await expect(nav.getByRole("link", { name: "Setup" })).toBeVisible();
  await expect(
    page.getByRole("heading", { level: 1, name: "Fleet" }),
  ).toBeVisible();
  const { violations } = await new AxeBuilder({ page }).analyze();
  expect(
    violations
      .filter((v) => BLOCKING.has(v.impact ?? ""))
      .map(
        (v) =>
          `rule ${v.id} (${v.impact}) at ${v.nodes.map((n) => n.target.join(" ")).join(", ")}`,
      ),
    "serious or critical axe violations on the fleet page",
  ).toEqual([]);
  await shot(page, "fleet");

  // TODO(#625): assert the cluster name on the fleet page when the fleet view
  // lists clusters. The cluster page reads the CamundaCluster through the
  // same informer, which lists every namespace.
  await page.goto(`/fleet/clusters/${namespace}/${name}`);
  await expect(page.getByRole("heading", { level: 1, name })).toBeVisible();
  await shot(page, "cluster");

  expect(
    foreign,
    "requests to a host other than the port-forward address",
  ).toEqual([]);
});
