import { beforeEach, describe, expect, it, vi } from "vitest";
import { PodLogNotFound, followPodLog } from "@/lib/observe/pod-log";
import { GET } from "./route";

vi.mock("@/lib/observe/pod-log", async (real) => ({
  ...(await real<typeof import("@/lib/observe/pod-log")>()),
  followPodLog: vi.fn(),
}));

const lines = (...values: string[]) =>
  new ReadableStream<string>({
    start(controller) {
      values.forEach((v) => controller.enqueue(v));
      controller.close();
    },
  });

const call = (query: string) =>
  GET(new Request(`http://localhost/api/logs?${query}`));

beforeEach(() => vi.resetAllMocks());

describe("GET /api/logs", () => {
  it.each([
    ["no namespace", "pod=p", "namespace"],
    ["no pod", "namespace=n", "pod"],
    ["sinceSeconds=0", "namespace=n&pod=p&sinceSeconds=0", "sinceSeconds"],
    ["tailLines=x", "namespace=n&pod=p&tailLines=x", "tailLines"],
    ["tailLines=1.5", "namespace=n&pod=p&tailLines=1.5", "tailLines"],
  ])("answers 400 for %s and names the parameter", async (_, query, name) => {
    const response = await call(query);

    expect(response.status).toBe(400);
    expect(await response.text()).toContain(name);
    expect(followPodLog).not.toHaveBeenCalled();
  });

  it("answers 404 when the pod does not exist", async () => {
    vi.mocked(followPodLog).mockRejectedValue(
      new PodLogNotFound("The pod n/p does not exist"),
    );

    const response = await call("namespace=n&pod=p");

    expect(response.status).toBe(404);
    expect(await response.text()).toBe("The pod n/p does not exist");
  });

  it("passes the parameters and the signal of the request", async () => {
    vi.mocked(followPodLog).mockResolvedValue(lines());

    await (
      await call("namespace=n&pod=p&container=c&sinceSeconds=60&tailLines=5")
    ).text();

    expect(followPodLog).toHaveBeenCalledWith(
      {
        namespace: "n",
        pod: "p",
        container: "c",
        sinceSeconds: 60,
        tailLines: 5,
      },
      expect.any(AbortSignal),
    );
  });

  it("sends each line as a line event with the line as JSON, then an end event", async () => {
    vi.mocked(followPodLog).mockResolvedValue(lines("a", '{"msg":"x"}'));

    const response = await call("namespace=n&pod=p");

    expect(response.headers.get("Content-Type")).toBe(
      "text/event-stream; charset=utf-8",
    );
    expect(response.headers.get("Cache-Control")).toBe(
      "no-cache, no-transform",
    );
    expect(await response.text()).toBe(
      'event: line\ndata: "a"\n\nevent: line\ndata: "{\\"msg\\":\\"x\\"}"\n\nevent: end\ndata: {}\n\n',
    );
  });

  it("sends a failure event when the log stream breaks", async () => {
    vi.mocked(followPodLog).mockResolvedValue(
      // An error drops the queued chunks, so the stream errors on the
      // read after the first line.
      new ReadableStream<string>({
        start(controller) {
          controller.enqueue("a");
        },
        pull(controller) {
          controller.error(new Error("socket hang up from 10.0.0.1"));
        },
      }),
    );

    const response = await call("namespace=n&pod=p");

    expect(await response.text()).toBe(
      'event: line\ndata: "a"\n\nevent: failure\ndata: {"message":"The log stream of pod p broke"}\n\n',
    );
  });
});
