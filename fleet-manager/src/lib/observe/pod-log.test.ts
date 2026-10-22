import http from "node:http";
import type { AddressInfo } from "node:net";
import { KubeConfig } from "@kubernetes/client-node";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { KubernetesRequestError, kubeConfig } from "@/lib/k8s/client";
import { get } from "@/lib/k8s/read";
import { PodLogNotFound, followPodLog } from "./pod-log";

vi.mock("@/lib/k8s/client", async (real) => ({
  ...(await real<typeof import("@/lib/k8s/client")>()),
  kubeConfig: vi.fn(),
}));
vi.mock("@/lib/k8s/read", () => ({ get: vi.fn() }));

type Handler = (req: http.IncomingMessage, res: http.ServerResponse) => void;
let server: http.Server | undefined;
const asked: URL[] = [];

// serve points the kubeconfig of the app at a local API server.
async function serve(handler: Handler): Promise<void> {
  server = http.createServer((req, res) => {
    asked.push(new URL(req.url!, "http://x"));
    handler(req, res);
  });
  await new Promise<void>((resolve) => server!.listen(0, "127.0.0.1", resolve));
  const kc = new KubeConfig();
  kc.loadFromOptions({
    clusters: [
      {
        name: "c",
        server: `http://127.0.0.1:${(server.address() as AddressInfo).port}`,
        skipTLSVerify: true,
      },
    ],
    users: [{ name: "u" }],
    contexts: [{ name: "x", cluster: "c", user: "u" }],
    currentContext: "x",
  });
  vi.mocked(kubeConfig).mockReturnValue(kc);
}

const pod = (...containers: string[]) =>
  ({
    metadata: { name: "p", namespace: "n" },
    spec: { containers: containers.map((name) => ({ name })) },
  }) as never;

async function readAll(stream: ReadableStream<string>): Promise<string[]> {
  const lines: string[] = [];
  for await (const line of stream) lines.push(line);
  return lines;
}

const tick = () => new Promise((resolve) => setTimeout(resolve, 20));

beforeEach(() => {
  vi.resetAllMocks();
  asked.length = 0;
});
afterEach(async () => {
  vi.restoreAllMocks();
  server?.closeAllConnections();
  if (server) await new Promise((resolve) => server!.close(resolve));
  server = undefined;
});

describe("followPodLog", () => {
  it("sends one chunk per line across write boundaries", async () => {
    vi.mocked(get).mockResolvedValue(pod("main"));
    await serve(async (_req, res) => {
      res.writeHead(200);
      res.write("a\nb");
      await tick();
      res.write("c\r\n");
      await tick();
      res.end("d");
    });

    const stream = await followPodLog(
      { namespace: "n", pod: "p" },
      new AbortController().signal,
    );

    expect(await readAll(stream)).toEqual(["a", "bc", "d"]);
  });

  it("uses the first container of the pod when none is given", async () => {
    vi.mocked(get).mockResolvedValue(pod("main", "sidecar"));
    await serve((_req, res) => res.end());

    await readAll(
      await followPodLog(
        { namespace: "n", pod: "p", sinceSeconds: 60, tailLines: 10 },
        new AbortController().signal,
      ),
    );

    expect(asked[0].pathname).toBe("/api/v1/namespaces/n/pods/p/log");
    expect(Object.fromEntries(asked[0].searchParams)).toEqual({
      container: "main",
      follow: "true",
      sinceSeconds: "60",
      tailLines: "10",
    });
    expect(get).toHaveBeenCalledWith("Pod", "n", "p");
  });

  it("follows the container that the request names", async () => {
    vi.mocked(get).mockResolvedValue(pod("main", "sidecar"));
    await serve((_req, res) => res.end());

    await readAll(
      await followPodLog(
        { namespace: "n", pod: "p", container: "sidecar" },
        new AbortController().signal,
      ),
    );

    expect(asked[0].searchParams.get("container")).toBe("sidecar");
  });

  it("follows an ephemeral container that the request names", async () => {
    vi.mocked(get).mockResolvedValue({
      metadata: { name: "p", namespace: "n" },
      spec: {
        containers: [{ name: "main" }],
        ephemeralContainers: [{ name: "debugger" }],
      },
    } as never);
    await serve((_req, res) => res.end());

    await readAll(
      await followPodLog(
        { namespace: "n", pod: "p", container: "debugger" },
        new AbortController().signal,
      ),
    );

    expect(asked[0].searchParams.get("container")).toBe("debugger");
  });

  it.each([
    ["a pod that is not in the cache", undefined, {}],
    ["a container that the pod does not have", pod("main"), { container: "x" }],
  ])("throws PodLogNotFound for %s", async (_, found, extra) => {
    vi.mocked(get).mockResolvedValue(found);
    await serve((_req, res) => res.end());

    await expect(
      followPodLog(
        { namespace: "n", pod: "p", ...extra },
        new AbortController().signal,
      ),
    ).rejects.toBeInstanceOf(PodLogNotFound);
    expect(asked).toEqual([]);
  });

  it("throws PodLogNotFound when the API server answers 404", async () => {
    vi.mocked(get).mockResolvedValue(pod("main"));
    await serve((_req, res) => res.writeHead(404).end());

    await expect(
      followPodLog({ namespace: "n", pod: "p" }, new AbortController().signal),
    ).rejects.toBeInstanceOf(PodLogNotFound);
  });

  it("rejects with the status of another answer outside 200", async () => {
    vi.mocked(get).mockResolvedValue(pod("main"));
    await serve((_req, res) =>
      res
        .writeHead(403)
        .end(
          JSON.stringify({ kind: "Status", code: 403, message: "forbidden" }),
        ),
    );

    const failure = followPodLog(
      { namespace: "n", pod: "p" },
      new AbortController().signal,
    );

    await expect(failure).rejects.toBeInstanceOf(KubernetesRequestError);
    await expect(failure).rejects.toMatchObject({ statusCode: 403 });
  });

  it("errors the stream when the connection breaks", async () => {
    vi.mocked(get).mockResolvedValue(pod("main"));
    await serve(async (_req, res) => {
      res.writeHead(200);
      res.write("a\n");
      await tick();
      res.destroy();
    });

    const reader = (
      await followPodLog(
        { namespace: "n", pod: "p" },
        new AbortController().signal,
      )
    ).getReader();

    expect(await reader.read()).toEqual({ done: false, value: "a" });
    await expect(reader.read()).rejects.toThrow();
  });

  it("pauses the log while the reader does not read, and goes on after", async () => {
    vi.mocked(get).mockResolvedValue(pod("main"));
    const pause = vi.spyOn(http.IncomingMessage.prototype, "pause");
    const lines = Array.from({ length: 500 }, (_, i) => `line ${i}`);
    await serve(async (_req, res) => {
      res.writeHead(200);
      res.write(lines.join("\n") + "\n");
      await tick();
      await tick();
      res.end("last");
    });

    const stream = await followPodLog(
      { namespace: "n", pod: "p" },
      new AbortController().signal,
    );
    await tick();

    expect(pause).toHaveBeenCalled();
    expect(await readAll(stream)).toEqual([...lines, "last"]);
  });

  it("sends no log request when the signal aborted during the pod lookup", async () => {
    const abort = new AbortController();
    vi.mocked(get).mockImplementation(async () => {
      abort.abort();
      return pod("main");
    });
    await serve((_req, res) => res.end());

    await expect(
      followPodLog({ namespace: "n", pod: "p" }, abort.signal),
    ).rejects.toThrow();
    await tick();
    expect(asked).toEqual([]);
  });

  it("stops the log request when the signal aborts", async () => {
    vi.mocked(get).mockResolvedValue(pod("main"));
    let closed: () => void;
    const requestClosed = new Promise<void>((resolve) => (closed = resolve));
    await serve((req, res) => {
      res.writeHead(200);
      res.write("a\n");
      req.socket.on("close", () => closed());
    });
    const abort = new AbortController();

    const reader = (
      await followPodLog({ namespace: "n", pod: "p" }, abort.signal)
    ).getReader();
    await reader.read();
    abort.abort();

    await requestClosed;
  });
});
