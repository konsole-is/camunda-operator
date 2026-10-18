import { spawn, type ChildProcess } from "node:child_process";
import net from "node:net";
import { expect, test } from "@playwright/test";
import { startEnvtest, type Envtest } from "../../test/envtest/envtest";

// Chromium speaks HTTP/2 only over TLS, so this plain http:// server keeps
// the browser at its limit of 6 connections for one origin.
let envtest: Envtest | undefined;
let server: ChildProcess | undefined;
let baseURL = "";

function freePort(): Promise<number> {
  return new Promise((resolve, reject) => {
    const probe = net.createServer();
    probe.once("error", reject);
    probe.listen(0, "127.0.0.1", () => {
      const { port } = probe.address() as net.AddressInfo;
      probe.close(() => resolve(port));
    });
  });
}

test.beforeAll(async () => {
  test.setTimeout(180_000);
  envtest = await startEnvtest();
  const port = await freePort();
  baseURL = `http://127.0.0.1:${port}`;
  const stderr: string[] = [];
  const child = spawn(
    "./node_modules/.bin/next",
    ["start", "--hostname", "127.0.0.1", "--port", String(port)],
    {
      env: {
        ...process.env,
        FLEET_DEV_GALLERY: "1",
        KUBECONFIG: envtest.kubeconfigPath,
        NEXT_TELEMETRY_DISABLED: "1",
      },
      stdio: ["ignore", "ignore", "pipe"],
    },
  );
  server = child;
  child.stderr!.on("data", (chunk) => stderr.push(String(chunk)));
  const exited = new Promise<never>((_, reject) =>
    child.once("exit", (code) =>
      reject(new Error(`next start exited with ${code}: ${stderr.join("")}`)),
    ),
  );
  const ready = (async () => {
    const deadline = Date.now() + 60_000;
    while (Date.now() < deadline) {
      try {
        if ((await fetch(`${baseURL}/dev/watch-probe`)).status === 200) return;
      } catch {
        // The server does not listen yet.
      }
      await new Promise((wake) => setTimeout(wake, 250));
    }
    throw new Error(
      `next start did not serve /dev/watch-probe within 60 s: ${stderr.join("")}`,
    );
  })();
  await Promise.race([ready, exited]);
});

test.afterAll(async () => {
  if (server && server.exitCode === null) {
    const exit = new Promise((resolve) => server!.once("exit", resolve));
    server.kill();
    await exit;
  }
  await envtest?.stop();
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
  await page.goto(`${baseURL}/dev/watch-probe`);
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
