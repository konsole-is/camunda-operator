// Playwright loads this module, so it has no import.meta.
import { spawn, type ChildProcess } from "node:child_process";
import net from "node:net";
import {
  type Envtest,
  type EnvtestOptions,
  startEnvtest,
} from "../test/envtest/envtest";

/** EnvtestApp is an envtest control plane and a server of the gallery build that reads it. */
export interface EnvtestApp {
  envtest: Envtest;
  /** baseURL is the http:// address of the app on 127.0.0.1. */
  baseURL: string;
  /** stop stops the app and then envtest. */
  stop(): Promise<void>;
}

export interface EnvtestAppOptions {
  envtest?: EnvtestOptions;
  /** readyPath answers 200 when the app serves. */
  readyPath: string;
  /**
   * prepare runs after envtest started and before the app starts. It
   * returns the kubeconfig of the app (default: the admin) and more
   * environment for the app.
   */
  prepare?: (
    envtest: Envtest,
  ) => Promise<{ kubeconfigPath?: string; env?: Record<string, string> }>;
}

const READY_TIMEOUT_MS = 60_000;

/**
 * startEnvtestApp starts envtest and then next start of the
 * FLEET_DEV_GALLERY=1 build in .next-gallery, which make fleet-build-gallery
 * builds. It resolves when readyPath answers 200. It rejects when the app
 * exits or does not answer within 60 s, and when prepare rejects. On a
 * rejection nothing keeps running.
 */
export async function startEnvtestApp(
  options: EnvtestAppOptions,
): Promise<EnvtestApp> {
  const envtest = await startEnvtest(options.envtest);
  let server: ChildProcess | undefined;
  try {
    const prepared = (await options.prepare?.(envtest)) ?? {};
    const port = await freePort();
    const baseURL = `http://127.0.0.1:${port}`;
    const stderr: string[] = [];
    server = spawn(
      "./node_modules/.bin/next",
      ["start", "--hostname", "127.0.0.1", "--port", String(port)],
      {
        env: {
          ...process.env,
          ...prepared.env,
          FLEET_DEV_GALLERY: "1",
          KUBECONFIG: prepared.kubeconfigPath ?? envtest.kubeconfigPath,
          NEXT_TELEMETRY_DISABLED: "1",
        },
        stdio: ["ignore", "ignore", "pipe"],
      },
    );
    server.stderr!.on("data", (chunk) => stderr.push(String(chunk)));
    const child = server;
    const exited = new Promise<never>((_, reject) => {
      child.once("error", reject);
      child.once("exit", (code) =>
        reject(new Error(`next start exited with ${code}: ${stderr.join("")}`)),
      );
    });
    await Promise.race([
      waitForApp(`${baseURL}${options.readyPath}`, stderr),
      exited,
    ]);
    // A later exit is a failure of the test, not of the start.
    exited.catch(() => {});
    return {
      envtest,
      baseURL,
      async stop() {
        await stopServer(child);
        await envtest.stop();
      },
    };
  } catch (error) {
    if (server) await stopServer(server);
    await envtest.stop();
    throw error;
  }
}

async function waitForApp(url: string, stderr: string[]): Promise<void> {
  const deadline = Date.now() + READY_TIMEOUT_MS;
  while (Date.now() < deadline) {
    try {
      if ((await fetch(url)).status === 200) return;
    } catch {
      // The server does not listen yet.
    }
    await new Promise((wake) => setTimeout(wake, 250));
  }
  throw new Error(
    `next start did not serve ${new URL(url).pathname} within 60 s: ${stderr.join("")}`,
  );
}

async function stopServer(server: ChildProcess): Promise<void> {
  if (server.exitCode !== null || server.signalCode !== null) return;
  const exit = new Promise((resolve) => server.once("exit", resolve));
  server.kill();
  await exit;
}

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
