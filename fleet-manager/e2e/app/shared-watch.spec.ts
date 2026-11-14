import { expect, test } from "@playwright/test";
import { type EnvtestApp, startEnvtestApp } from "../envtest-app";

// Chromium speaks HTTP/2 only over TLS, so this plain http:// server keeps
// the browser at its limit of 6 connections for one origin.
let app: EnvtestApp | undefined;

test.beforeAll(async () => {
  test.setTimeout(180_000);
  app = await startEnvtestApp({ readyPath: "/dev/watch-probe" });
});

test.afterAll(async () => {
  await app?.stop();
});

test("8 hooks over HTTP/1.1 open one watch connection, and navigation and a server action still complete", async ({
  page,
}) => {
  const watches: string[] = [];
  page.on("request", (r) => {
    if (new URL(r.url()).pathname === "/api/watch") watches.push(r.url());
  });
  const stream = page.waitForResponse(
    (r) => new URL(r.url()).pathname === "/api/watch",
  );
  await page.goto(`${app!.baseURL}/dev/watch-probe`);
  expect((await stream).headers()["content-type"]).toContain(
    "text/event-stream",
  );
  // Namespace "default" exists in every API server, so its count shows that
  // the snapshot arrived.
  await expect(page.locator('[data-kind="Namespace"]')).not.toHaveText(
    /\b0\b/,
    { timeout: 15_000 },
  );

  await page.getByRole("button", { name: "Ping" }).click();
  await expect(page.getByTestId("ping")).toHaveText("pong", {
    timeout: 10_000,
  });
  expect(watches).toHaveLength(1);
  expect(new URL(watches[0]).searchParams.getAll("watch")).toHaveLength(8);

  await page.getByRole("link", { name: "Next page" }).click();
  await expect(
    page.getByRole("heading", { name: "Navigation done" }),
  ).toBeVisible({ timeout: 10_000 });
});
